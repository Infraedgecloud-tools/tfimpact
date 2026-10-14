package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

var update = flag.Bool("update", false, "rewrite golden files")

var fixtures = []string{"rds-replacement", "rename", "iam-widening", "tag-only", "noisy", "hardening-regression", "fresh-create", "destroy-all", "keys-and-lifecycle", "drift"}

func init() {
	now = func() time.Time { return time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC) }
	version = "test"
}

func runExplain(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(append([]string{"explain"}, args...), strings.NewReader(""), &out, &errb)
	return code, out.String(), errb.String()
}

// planPath is written with forward slashes on every platform: the path is
// echoed into the JSON envelope as source.root, and the goldens hold the
// slash form.
func planPath(fixture string) string {
	return filepath.ToSlash(filepath.Join("..", "..", "testdata", "plans", fixture, "plan.json"))
}

func checkGolden(t *testing.T, name, got string) {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "golden", name)
	if *update {
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("missing golden file %s (run go test ./... -update): %v", path, err)
	}
	// A checkout with autocrlf hands the goldens back with CRLF; the tool
	// writes LF everywhere. Compare the bytes that matter.
	want = bytes.ReplaceAll(want, []byte("\r\n"), []byte("\n"))
	if string(want) != got {
		t.Errorf("%s differs from golden file; run `go test ./cmd/tfimpact -update` after checking the diff.\n--- got ---\n%s", name, got)
	}
}

func TestGolden(t *testing.T) {
	for _, f := range fixtures {
		for _, format := range []string{"markdown", "text", "json"} {
			t.Run(f+"/"+format, func(t *testing.T) {
				code, out, errOut := runExplain(t, planPath(f), "--format", format)
				if code != 0 && code != 1 {
					t.Fatalf("exit %d: %s", code, errOut)
				}
				ext := map[string]string{"markdown": "md", "text": "txt", "json": "json"}[format]
				checkGolden(t, f+"."+ext, out)
			})
		}
	}
}

func TestNoSecretsInAnyOutput(t *testing.T) {
	// The RDS fixture carries a plaintext master password in plan.json and the
	// noisy fixture carries a user_data script. Neither may ever be printed.
	for _, tc := range []struct{ fixture, needle string }{
		{"rds-replacement", "fixture-password-not-real"},
		{"hardening-regression", "fixture-password-not-real"},
		{"destroy-all", "fixture-password-not-real"},
		{"noisy", "echo fixture v"},
	} {
		for _, format := range []string{"markdown", "text", "json"} {
			_, out, _ := runExplain(t, planPath(tc.fixture), "--format", format)
			if strings.Contains(out, tc.needle) {
				t.Errorf("%s output of %s leaks %q", format, tc.fixture, tc.needle)
			}
		}
	}
}

func TestOpenTofuPlanGivesSameFindings(t *testing.T) {
	_, tf, _ := runExplain(t, planPath("rename"), "--format", "json")
	_, tofu, _ := runExplain(t, filepath.Join("..", "..", "testdata", "plans", "rename", "plan.opentofu-1.13.1.json"), "--format", "json")
	var a, b map[string]any
	if err := json.Unmarshal([]byte(tf), &a); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal([]byte(tofu), &b); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"findings", "summary", "routine", "moves"} {
		aj, _ := json.Marshal(a[key])
		bj, _ := json.Marshal(b[key])
		if string(aj) != string(bj) {
			t.Errorf("%s differs between Terraform and OpenTofu plans:\n%s\n%s", key, aj, bj)
		}
	}
}

func TestExitCodes(t *testing.T) {
	if code, _, _ := runExplain(t, planPath("rds-replacement"), "--fail-on", "critical", "--quiet"); code != 1 {
		t.Errorf("critical finding with --fail-on critical: exit %d, want 1", code)
	}
	if code, _, _ := runExplain(t, planPath("tag-only"), "--fail-on", "low", "--quiet"); code != 0 {
		t.Errorf("tag-only plan with --fail-on low: exit %d, want 0", code)
	}
	if code, _, _ := runExplain(t, planPath("iam-widening"), "--fail-on", "critical", "--quiet"); code != 1 {
		t.Errorf("SSH open to the world is critical: exit %d, want 1", code)
	}
	if code, _, errOut := runExplain(t, planPath("missing"), "--quiet"); code != 2 || !strings.Contains(errOut, "no such file") {
		t.Errorf("missing file: exit %d, %q", code, errOut)
	}
	if code, _, _ := runExplain(t, planPath("tag-only"), "--fail-on", "bogus"); code != 2 {
		t.Errorf("bad --fail-on: exit %d, want 2", code)
	}
	if code, _, _ := runExplain(t, "--format", "text", planPath("tag-only")); code != 0 {
		t.Errorf("flags before the positional argument should work: exit %d", code)
	}
}

func TestStreamingPlanJSONIsRejectedWithHint(t *testing.T) {
	var out, errb bytes.Buffer
	stream := `{"@level":"info","@message":"Terraform 1.15.5","type":"version"}` + "\n" + `{"@level":"info","type":"planned_change"}` + "\n"
	code := run([]string{"explain", "-"}, strings.NewReader(stream), &out, &errb)
	if code != 2 || !strings.Contains(errb.String(), "terraform plan -json") {
		t.Fatalf("exit %d, stderr %q", code, errb.String())
	}
}

func TestSuppressionAppliesAndIsReported(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), ".tfimpact.toml")
	if err := os.WriteFile(cfg, []byte(`version = 1
[[suppress]]
rule = "PLAN-DATA-002"
address = "aws_db_instance.app"
reason = "encryption migration approved in CHG-123, snapshot taken by hand"
`), 0o600); err != nil {
		t.Fatal(err)
	}
	code, out, _ := runExplain(t, planPath("rds-replacement"), "--config", cfg, "--fail-on", "critical", "--format", "markdown")
	if code != 0 {
		t.Errorf("suppressed finding must not trip --fail-on: exit %d", code)
	}
	if !strings.Contains(out, "### Suppressed (1)") || !strings.Contains(out, "CHG-123") {
		t.Errorf("suppressed section missing:\n%s", out)
	}
}

func TestNoValues(t *testing.T) {
	_, out, _ := runExplain(t, planPath("noisy"), "--no-values", "--format", "text")
	if strings.Contains(out, "session_id") || strings.Contains(out, "30 → 90") {
		t.Errorf("--no-values printed a value:\n%s", out)
	}
	if !strings.Contains(out, "hash_key") {
		t.Errorf("--no-values should still name attributes:\n%s", out)
	}
}

// A directory is the commonest wrong argument: the hint says what to run.
func TestDirectoryGetsAHint(t *testing.T) {
	var out, errb strings.Builder
	code := run([]string{"explain", t.TempDir()}, nil, &out, &errb)
	if code != exitUsage || !strings.Contains(errb.String(), "terraform plan -out=tfplan") {
		t.Fatalf("code %d, stderr %q", code, errb.String())
	}
}

// A saved plan is a zip. With terraform on PATH it is read through
// `terraform show -json` on the plan's absolute path, in the current
// directory; when that fails, the error carries terraform's own message.
func TestSavedPlanIsReadThroughShow(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the fake terraform is a shell script")
	}
	dir := t.TempDir()
	planPath := filepath.Join(dir, "tfplan")
	if err := os.WriteFile(planPath, []byte("PK\x03\x04not really a zip"), 0o644); err != nil {
		t.Fatal(err)
	}
	fixture, err := os.ReadFile(filepath.Join("..", "..", "testdata", "plans", "noisy", "plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plan.json"), fixture, 0o644); err != nil {
		t.Fatal(err)
	}
	// The fake records its arguments and working directory, then prints the
	// fixture, which is what a real show -json would produce.
	fake := filepath.Join(dir, "fake-terraform")
	script := "#!/bin/sh\nprintf '%s %s' \"$PWD\" \"$*\" > \"" + filepath.Join(dir, "called") + "\"\ncat \"" + filepath.Join(dir, "plan.json") + "\"\n"
	if err := os.WriteFile(fake, []byte(script), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Setenv("TFIMPACT_TERRAFORM", fake)
	var out, errb strings.Builder
	if code := run([]string{"explain", planPath, "--format", "text"}, nil, &out, &errb); code != exitOK {
		t.Fatalf("code %d, stderr %q", code, errb.String())
	}
	if !strings.Contains(out.String(), "destroys data") {
		t.Fatalf("expected the fixture's findings, got:\n%s", out.String())
	}
	called, _ := os.ReadFile(filepath.Join(dir, "called"))
	if !strings.HasSuffix(string(called), "show -no-color -json "+planPath) {
		t.Fatalf("show was run as %q, wanted `show -no-color -json %s`", string(called), planPath)
	}

	t.Setenv("TFIMPACT_TERRAFORM", filepath.Join(dir, "missing"))
	out.Reset()
	errb.Reset()
	if code := run([]string{"explain", planPath}, nil, &out, &errb); code != exitUsage || !strings.Contains(errb.String(), "failed") {
		t.Fatalf("a missing terraform should fail with its error: code %d, stderr %q", code, errb.String())
	}
}

func TestVersionAndUsage(t *testing.T) {
	var out, errb bytes.Buffer
	if code := run([]string{"version"}, nil, &out, &errb); code != 0 || !strings.HasPrefix(out.String(), "tfimpact test") {
		t.Fatalf("version: %d %q", code, out.String())
	}
	out.Reset()
	if code := run(nil, nil, &out, &errb); code != 2 {
		t.Fatalf("no args should be usage error, got %d", code)
	}
}
