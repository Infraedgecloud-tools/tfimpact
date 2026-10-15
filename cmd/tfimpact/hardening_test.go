package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPlanExecutionStatus(t *testing.T) {
	for _, tc := range []struct {
		name, fields string
		want         int
	}{
		{"empty no-op", `"complete":true,"errored":false,"applyable":false`, 0},
		{"legacy", `"future_minor_field":{}`, 0},
		{"incomplete", `"complete":false`, 2},
		{"errored", `"errored":true`, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var out, stderr bytes.Buffer
			input := `{"format_version":"1.999","resource_changes":[],` + tc.fields + `}`
			code := run([]string{"explain", "-", "--format", "json"}, strings.NewReader(input), &out, &stderr)
			if code != tc.want {
				t.Fatalf("exit %d want %d", code, tc.want)
			}
		})
	}
}

func TestJSONOnlyNeverRunsSavedPlanHelper(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "saved")
	os.WriteFile(p, []byte("PK\x03\x04synthetic"), 0600)
	t.Setenv("TFIMPACT_TERRAFORM", filepath.Join(dir, "must-not-run"))
	code, _, stderr := runExplain(t, p, "--json-only")
	if code != 2 || !strings.Contains(stderr, "--json-only rejects") {
		t.Fatalf("exit %d: %s", code, stderr)
	}
}

func TestUnsupportedChangesAreExplicitlyUnassessed(t *testing.T) {
	for _, tc := range []struct {
		typ, provider string
		actions       []string
	}{
		{"future_resource", "future/provider", []string{"create"}},
		{"aws_instance", "other/provider", []string{"update"}},
		{"aws_instance", "registry.terraform.io/hashicorp/aws", []string{"create", "delete", "future"}},
	} {
		p := map[string]any{"format_version": "1.2", "resource_changes": []any{map[string]any{"address": "resource.example", "type": tc.typ, "provider_name": tc.provider, "change": map[string]any{"actions": tc.actions}}}}
		b, _ := json.Marshal(p)
		var out, stderr bytes.Buffer
		if code := run([]string{"explain", "-", "--format", "json"}, bytes.NewReader(b), &out, &stderr); code != 0 {
			t.Fatalf("exit %d", code)
		}
		if !strings.Contains(out.String(), "PLAN-COVERAGE-001") || !strings.Contains(out.String(), "unassessed") {
			t.Fatal("unassessed change appeared clean")
		}
	}
}

func TestMalformedPlansCannotLookClean(t *testing.T) {
	for _, input := range []string{
		`{"format_version":"1.2","unrelated":{"planned_values":{}}}`,
		`{"format_version":"1.2","planned_values":null}`,
		`{"format_version":"1.2","planned_values":"value"}`,
		`{"format_version":"1.2","resource_changes":null}`,
		`{"format_version":"1.2","resource_changes":[],"errored":true,"errored":false}`,
		`{"format_version":"1.2","resource_changes":[{}]}`,
		`{"format_version":"1.x","resource_changes":[]}`,
		`{"format_version":"1.2","resource_changes":[],"deferred_changes":[{}]}`,
	} {
		var out, stderr bytes.Buffer
		if code := run([]string{"explain", "-"}, bytes.NewBufferString(input), &out, &stderr); code != 2 {
			t.Fatalf("malformed plan returned %d", code)
		}
	}
}

func TestOutputCannotClobberInputOrEachOther(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "plan.json")
	original := []byte(`{"format_version":"1.2","resource_changes":[]}`)
	os.WriteFile(p, original, 0600)
	if code, _, _ := runExplain(t, p, "--output", p); code != 2 {
		t.Fatal("output overwrote input")
	}
	got, _ := os.ReadFile(p)
	if !bytes.Equal(got, original) {
		t.Fatal("input changed")
	}
	same := filepath.Join(dir, "same")
	if code, _, _ := runExplain(t, p, "--output", same, "--json", same); code != 2 {
		t.Fatal("outputs collided")
	}
}

func TestSubprocessOutputBudgetCancels(t *testing.T) {
	cancelled := false
	b := boundedOutput{max: 3, cancel: func() { cancelled = true }}
	if _, err := b.Write([]byte("123")); err != nil {
		t.Fatal(err)
	}
	if _, err := b.Write([]byte("4")); err == nil || !cancelled || b.Len() != 3 {
		t.Fatal("subprocess output budget not enforced")
	}
}

func TestSavedPlanHelperOutputLimits(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("synthetic POSIX helper")
	}
	for _, stream := range []string{"stdout", "stderr"} {
		t.Run(stream, func(t *testing.T) {
			dir := t.TempDir()
			p := filepath.Join(dir, "saved")
			os.WriteFile(p, []byte("PK\x03\x04synthetic"), 0600)
			helper := filepath.Join(dir, "helper")
			body := "#!/bin/sh\nhead -c 67108865 /dev/zero\n"
			if stream == "stderr" {
				body = "#!/bin/sh\nhead -c 65537 /dev/zero >&2\n"
			}
			os.WriteFile(helper, []byte(body), 0700)
			t.Setenv("TFIMPACT_TERRAFORM", helper)
			code, _, stderr := runExplain(t, p)
			if code != 2 || !strings.Contains(stderr, "output withheld") {
				t.Fatalf("helper exceeded budget: exit %d", code)
			}
		})
	}
}
