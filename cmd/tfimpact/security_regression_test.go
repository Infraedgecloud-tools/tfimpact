package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRelatedResourceAndIAMValuesRemainPrivate(t *testing.T) {
	const canary = "synthetic-related-canary"
	policy := func(op, value string) string {
		b, _ := json.Marshal(map[string]any{"Version": "2012-10-17", "Statement": []any{map[string]any{"Effect": "Allow", "Action": "s3:GetObject", "Resource": "*", "Condition": map[string]any{op: map[string]any{"aws:SourceArn": value}}}}})
		return string(b)
	}
	for _, test := range []string{"parent rename", "new sensitive rename", "prior sensitive policy", "no-values policy"} {
		for _, format := range []string{"text", "markdown", "json", "sarif"} {
			t.Run(test+"/"+format, func(t *testing.T) {
				var changes []any
				if test == "parent rename" || test == "new sensitive rename" {
					var bm any = true
					if test == "new sensitive rename" {
						bm = nil
					}
					changes = []any{map[string]any{"address": "aws_s3_bucket.old", "type": "aws_s3_bucket", "change": map[string]any{"actions": []string{"delete"}, "before": map[string]any{"bucket": canary}, "before_sensitive": bm}}, map[string]any{"address": "aws_s3_bucket.new", "type": "aws_s3_bucket", "change": map[string]any{"actions": []string{"create"}, "after": map[string]any{"bucket": canary}, "after_sensitive": map[string]any{"bucket": true}}}}
				} else {
					var bm any = map[string]any{"policy": true}
					if test == "no-values policy" {
						bm = nil
					}
					changes = []any{map[string]any{"address": "aws_iam_policy.demo", "type": "aws_iam_policy", "change": map[string]any{"actions": []string{"update"}, "before": map[string]any{"policy": policy("StringEquals", canary)}, "after": map[string]any{"policy": policy("StringLike", "*")}, "before_sensitive": bm}}}
				}
				b, _ := json.Marshal(map[string]any{"format_version": "1.2", "resource_changes": changes})
				args := []string{"explain", "-", "--format", format}
				if test == "no-values policy" {
					args = append(args, "--no-values")
				}
				var out, stderr bytes.Buffer
				if code := run(args, bytes.NewReader(b), &out, &stderr); code != 0 {
					t.Fatalf("exit %d", code)
				}
				if bytes.Contains(out.Bytes(), []byte(canary)) || bytes.Contains(stderr.Bytes(), []byte(canary)) {
					t.Fatal("sensitive identity/policy value disclosed")
				}
			})
		}
	}
}

func TestDerivedIngressDescriptionsHonourPrivacy(t *testing.T) {
	const canary = "synthetic-protocol-canary"
	for _, noValues := range []bool{false, true} {
		for _, format := range []string{"text", "markdown", "json", "sarif"} {
			p := map[string]any{"format_version": "1.2", "resource_changes": []any{map[string]any{"address": "aws_security_group.demo", "type": "aws_security_group", "change": map[string]any{"actions": []string{"create"}, "after": map[string]any{"ingress": []any{map[string]any{"protocol": canary, "from_port": 1234, "to_port": 1234, "cidr_blocks": []any{"0.0.0.0/0"}}}}, "after_sensitive": map[string]any{"ingress": true}}}}}
			b, _ := json.Marshal(p)
			args := []string{"explain", "-", "--format", format}
			if noValues {
				args = append(args, "--no-values")
			}
			var out, stderr bytes.Buffer
			if code := run(args, bytes.NewReader(b), &out, &stderr); code != 0 {
				t.Fatalf("exit %d", code)
			}
			if bytes.Contains(out.Bytes(), []byte(canary)) || bytes.Contains(stderr.Bytes(), []byte(canary)) {
				t.Fatal("sensitive derived network value disclosed")
			}
		}
	}
}

// These are deliberately synthetic adversarial plans, not provider fixtures.
func TestUnknownValuesNeverDisclosePriorSensitiveData(t *testing.T) {
	const canary = "synthetic-canary-do-not-disclose"
	cases := []struct {
		name                           string
		before, after, bm, am, unknown any
	}{
		{"marked", map[string]any{"payload": canary}, map[string]any{}, map[string]any{"payload": true}, nil, map[string]any{"payload": true}},
		{"named", map[string]any{"api_token": canary}, map[string]any{}, nil, nil, map[string]any{"api_token": true}},
		{"parent", map[string]any{"payload": map[string]any{"child": canary}}, map[string]any{}, map[string]any{"payload": true}, nil, map[string]any{"payload": true}},
		{"nested mask", map[string]any{"payload": map[string]any{"child": canary}}, map[string]any{}, map[string]any{"payload": map[string]any{"child": true}}, nil, map[string]any{"payload": true}},
		{"array", map[string]any{"payload": []any{canary}}, map[string]any{}, map[string]any{"payload": []any{true}}, nil, map[string]any{"payload": true}},
		{"removed", map[string]any{"payload": canary}, map[string]any{}, map[string]any{"payload": true}, nil, nil},
	}
	for _, tc := range cases {
		for _, format := range []string{"text", "markdown", "json", "sarif"} {
			for _, noValues := range []bool{false, true} {
				t.Run(tc.name+"/"+format+map[bool]string{true: "/no-values", false: ""}[noValues], func(t *testing.T) {
					p := map[string]any{"format_version": "1.2", "resource_changes": []any{map[string]any{"address": "aws_instance.example", "mode": "managed", "type": "aws_instance", "change": map[string]any{"actions": []string{"update"}, "before": tc.before, "after": tc.after, "before_sensitive": tc.bm, "after_sensitive": tc.am, "after_unknown": tc.unknown}}}}
					b, err := json.Marshal(p)
					if err != nil {
						t.Fatal(err)
					}
					dir := t.TempDir()
					args := []string{"explain", "-", "--format", format, "--output", filepath.Join(dir, "comment.md"), "--json", filepath.Join(dir, "result.json"), "--sarif", filepath.Join(dir, "result.sarif")}
					if noValues {
						args = append(args, "--no-values")
					}
					var out, stderr bytes.Buffer
					if code := run(args, bytes.NewReader(b), &out, &stderr); code != 0 {
						t.Fatalf("exit %d", code)
					}
					for name, content := range map[string][]byte{"stdout": out.Bytes(), "stderr": stderr.Bytes()} {
						if bytes.Contains(content, []byte(canary)) {
							t.Errorf("canary disclosed in %s", name)
						}
					}
					for _, name := range []string{"comment.md", "result.json", "result.sarif"} {
						content, err := os.ReadFile(filepath.Join(dir, name))
						if err != nil {
							t.Fatal(err)
						}
						if bytes.Contains(content, []byte(canary)) {
							t.Errorf("canary disclosed in %s", name)
						}
					}
				})
			}
		}
	}
}
