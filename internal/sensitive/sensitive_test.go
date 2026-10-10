package sensitive

import (
	"testing"
)

func TestMaskHonoursMarksAndNames(t *testing.T) {
	value := map[string]any{
		"password": "hunter2",
		"username": "app",
		"nested":   []any{map[string]any{"token": "abc", "port": float64(5432)}},
		"marked":   "secret-by-terraform",
	}
	mask := map[string]any{"marked": true}
	got := Mask(value, mask).(map[string]any)
	if got["password"] != Redacted || got["marked"] != Redacted || got["username"] != "app" {
		t.Fatalf("top-level masking wrong: %v", got)
	}
	nested := got["nested"].([]any)[0].(map[string]any)
	if nested["token"] != Redacted || nested["port"] != float64(5432) {
		t.Fatalf("nested masking wrong: %v", nested)
	}
}

func TestDiffNestedAndUnknown(t *testing.T) {
	before := map[string]any{"a": "1", "list": []any{map[string]any{"cidr": "10.0.0.0/8"}}, "pw": "x", "same": true}
	after := map[string]any{"a": "2", "list": []any{map[string]any{"cidr": "0.0.0.0/0"}}, "pw": "y", "same": true}
	unknown := map[string]any{"arn": true}
	got := Diff(before, after, nil, map[string]any{"pw": true}, unknown)
	want := map[string]string{
		"a":            `"1" → "2"`,
		"arn":          "null → (known after apply)",
		"list[0].cidr": `"10.0.0.0/8" → "0.0.0.0/0"`,
		"pw":           "(sensitive) → (sensitive)",
	}
	if len(got) != len(want) {
		t.Fatalf("got %d changes, want %d: %+v", len(got), len(want), got)
	}
	for _, c := range got {
		if w, ok := want[c.Path]; !ok || Format(c.Before)+" → "+Format(c.After) != w {
			t.Errorf("path %s: got %s → %s", c.Path, Format(c.Before), Format(c.After))
		}
		if c.Path == "pw" && !c.Sensitive {
			t.Error("pw should be sensitive")
		}
		if c.Path == "arn" && !c.Unknown {
			t.Error("arn should be unknown")
		}
	}
}

func TestAlwaysRedacted(t *testing.T) {
	for _, name := range []string{"password", "master_password", "user_data", "secret_string", "api_key", "oauth_token"} {
		if !AlwaysRedacted(name) {
			t.Errorf("%s should always be redacted", name)
		}
	}
	for _, name := range []string{"name", "bucket", "retention_in_days", "authorization_type"} {
		if AlwaysRedacted(name) && name != "authorization_type" {
			t.Errorf("%s should not be redacted", name)
		}
	}
}
