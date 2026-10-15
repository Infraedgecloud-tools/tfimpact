package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func actionScript(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("../../action.yml")
	if err != nil {
		t.Fatal(err)
	}
	_, part, ok := strings.Cut(string(b), "    - name: "+name+"\n")
	if !ok {
		t.Fatal("missing action step")
	}
	part, _, _ = strings.Cut(part, "\n    - name:")
	_, part, ok = strings.Cut(part, "      run: |\n")
	if !ok {
		t.Fatal("missing run block")
	}
	var out []string
	for _, line := range strings.Split(part, "\n") {
		out = append(out, strings.TrimPrefix(line, "        "))
	}
	return strings.Join(out, "\n")
}

func TestActionInstallConsumer(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Bash consumer harness: Linux/macOS Action only")
	}
	if _, err := exec.LookPath("python3"); err != nil {
		t.Skip("Action hosted runners require python3")
	}
	for _, tc := range []struct {
		name, version                            string
		badChecksum, badMember, provenance, deny bool
		want                                     bool
	}{
		{name: "exact", version: "0.1.0", want: true},
		{name: "moving", version: "latest"}, {name: "path", version: "../../1"},
		{name: "checksum", version: "0.1.0", badChecksum: true},
		{name: "member", version: "0.1.0", badMember: true},
		{name: "provenance", version: "0.1.0", provenance: true, want: true},
		{name: "denied attestation", version: "0.1.0", provenance: true, deny: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			bin := filepath.Join(dir, "bin")
			os.Mkdir(bin, 0700)
			var archive bytes.Buffer
			gz := gzip.NewWriter(&archive)
			tw := tar.NewWriter(gz)
			payload := []byte("#!/bin/bash\necho 'tfimpact 0.1.0 linux/amd64'\n")
			name := "tfimpact"
			if tc.badMember {
				name = "../outside"
			}
			if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0755, Size: int64(len(payload))}); err != nil {
				t.Fatal(err)
			}
			tw.Write(payload)
			tw.Close()
			gz.Close()
			os.WriteFile(filepath.Join(dir, "fixture.tar.gz"), archive.Bytes(), 0600)
			sum := fmt.Sprintf("%x", sha256.Sum256(archive.Bytes()))
			if tc.badChecksum {
				sum = strings.Repeat("0", 64)
			}
			os.WriteFile(filepath.Join(dir, "checksums.txt"), []byte(sum+"  tfimpact_0.1.0_linux_amd64.tar.gz\n"), 0600)
			os.WriteFile(filepath.Join(bin, "curl"), []byte("#!/bin/bash\nwhile [ \"$1\" != '-o' ]; do shift; done\ncase \"$3\" in *checksums.txt) cp \"$FIXTURES/checksums.txt\" \"$2\";; *) cp \"$FIXTURES/fixture.tar.gz\" \"$2\";; esac\n"), 0700)
			os.WriteFile(filepath.Join(bin, "gh"), []byte("#!/bin/bash\nprintf '%s\\n' \"$@\" > \"$FIXTURES/gh-args\"\nexit \"$GH_STATUS\"\n"), 0700)
			script := actionScript(t, "Install tfimpact")
			cmd := exec.Command("bash", "-e", "-o", "pipefail", "-c", script)
			verify := "false"
			if tc.provenance {
				verify = "true"
			}
			status := "0"
			if tc.deny {
				status = "1"
			}
			cmd.Env = append(os.Environ(), "PATH="+bin+string(os.PathListSeparator)+os.Getenv("PATH"), "VERSION="+tc.version, "VERIFY_PROVENANCE="+verify, "RUNNER_TEMP="+dir, "RUNNER_OS_NAME=Linux", "RUNNER_ARCH_NAME=X64", "GITHUB_OUTPUT="+filepath.Join(dir, "output"), "FIXTURES="+dir, "GH_STATUS="+status)
			out, err := cmd.CombinedOutput()
			if (err == nil) != tc.want {
				t.Fatalf("success=%v want %v: %s", err == nil, tc.want, out)
			}
			if _, err := os.Stat(filepath.Join(dir, "outside")); err == nil {
				t.Fatal("archive escaped installation")
			}
			if tc.provenance {
				args, _ := os.ReadFile(filepath.Join(dir, "gh-args"))
				for _, required := range []string{"--repo\nInfraedgecloud-tools/tfimpact", "--source-ref\nrefs/tags/v0.1.0", "--signer-workflow\nInfraedgecloud-tools/tfimpact/.github/workflows/release.yml", "--cert-identity\nhttps://github.com/Infraedgecloud-tools/tfimpact/.github/workflows/release.yml@refs/tags/v0.1.0"} {
					if !strings.Contains(string(args), required) {
						t.Error("missing provenance constraint")
					}
				}
			}
		})
	}
}

func TestActionPreservesScanExitAndIsolation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Bash consumer harness")
	}
	for _, code := range []int{0, 1, 2} {
		t.Run(fmt.Sprint(code), func(t *testing.T) {
			dir := t.TempDir()
			install := filepath.Join(dir, "custom install")
			os.Mkdir(install, 0700)
			plan := filepath.Join(dir, "plan with spaces.json")
			os.WriteFile(plan, []byte(`{"format_version":"1.2","resource_changes":[]}`), 0600)
			script := fmt.Sprintf("#!/bin/bash\nif [ \"$1\" = help ]; then echo --sarif; exit; fi\nwhile [ $# -gt 0 ]; do case \"$1\" in --output|--json|--sarif) echo report > \"$2\"; shift;; esac; shift; done\nexit %d\n", code)
			os.WriteFile(filepath.Join(install, "tfimpact"), []byte(script), 0700)
			cmd := exec.Command("bash", "-e", "-o", "pipefail", "-c", actionScript(t, "Explain plan"))
			cmd.Env = append(os.Environ(), "INSTALL_DIR="+install, "PLAN="+plan, "FAIL_ON=high", "NO_VALUES=false", "CONFIG=", "GITHUB_OUTPUT="+filepath.Join(dir, "output"), "GITHUB_STEP_SUMMARY="+filepath.Join(dir, "summary"))
			if out, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("scan artifact step: %v %s", err, out)
			}
			data, _ := os.ReadFile(filepath.Join(dir, "output"))
			if !strings.Contains(string(data), fmt.Sprintf("exit-code=%d", code)) {
				t.Fatal("lost scan code")
			}
			apply := exec.Command("bash", "-c", actionScript(t, "Apply exit code"))
			apply.Env = append(os.Environ(), fmt.Sprintf("EXIT_CODE=%d", code))
			err := apply.Run()
			got := 0
			if e, ok := err.(*exec.ExitError); ok {
				got = e.ExitCode()
			}
			if got != code {
				t.Fatalf("final exit %d want %d", got, code)
			}
		})
	}
	b, _ := os.ReadFile("../../action.yml")
	if !strings.Contains(string(b), "continue-on-error: true") || !strings.Contains(string(b), "if: always() && steps.explain.outputs.exit-code") {
		t.Fatal("comments must not mask scan outcome")
	}
}
