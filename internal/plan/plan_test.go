package plan

import (
	"strings"
	"testing"
)

func TestKind(t *testing.T) {
	for _, actions := range [][]string{{"create", "delete", "future"}, {"forget", "future"}, {"create", "create"}} {
		if (Change{Actions: actions}).Kind() != Unknown {
			t.Error("unsupported action combination must be unknown")
		}
	}
	cases := map[string][]string{
		string(NoOp): {"no-op"}, string(Create): {"create"}, string(Update): {"update"}, string(Delete): {"delete"},
		string(Replace): {"delete", "create"}, string(Read): {"read"}, string(Forget): {"forget"},
	}
	for want, actions := range cases {
		if got := (Change{Actions: actions}).Kind(); string(got) != want {
			t.Errorf("%v: got %s want %s", actions, got, want)
		}
	}
	if !(Change{Actions: []string{"create", "delete"}}).CreateBeforeDestroy() {
		t.Error("create,delete is create_before_destroy")
	}
	if (Change{Actions: []string{"create", "forget"}}).Kind() != Forget {
		t.Error("create,forget should classify as forget")
	}
}

func TestLoadErrors(t *testing.T) {
	cases := map[string]string{
		"":                                `{"@level":"info","type":"version"}` + "\n{}",
		"not a Terraform plan":            `[1,2]`,
		"no format_version":               `{"resource_changes":[]}`,
		"unsupported plan format_version": `{"format_version":"2.0","resource_changes":[]}`,
		"state rather than a plan":        `{"format_version":"1.0","values":{}}`,
	}
	for want, input := range cases {
		_, err := Load(strings.NewReader(input))
		if err == nil {
			t.Errorf("%q should fail", input)
			continue
		}
		if want == "" {
			want = "terraform plan -json"
		}
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%q: error %q should mention %q", input, err, want)
		}
	}
	p, err := Load(strings.NewReader(`{"format_version":"1.2","terraform_version":"1.15.5","resource_changes":[{"address":"a.b","type":"a","change":{"actions":["update"],"replace_paths":[["x",0,"y"]]}}]}`))
	if err != nil {
		t.Fatal(err)
	}
	if got := p.ResourceChanges[0].Change.ReplacePathStrings()[0]; got != "x[0].y" {
		t.Errorf("replace path rendering: %s", got)
	}
}
