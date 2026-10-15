package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/infraedgecloud-tools/tfimpact/internal/findings"
)

func TestHumanReportsContainUntrustedText(t *testing.T) {
	hostile := "resource`[link](https://example.invalid)<img src=x>\x1b[31m\nforged"
	address := "aws_instance.demo[\"`[link](https://example.invalid)\"]"
	r := &findings.Result{Findings: []findings.Finding{{RuleID: "RULE", Severity: findings.High, Title: hostile, Observed: hostile, Recommendation: hostile}}, Routine: []findings.Routine{{Type: "aws_instance", Addresses: []string{address}, Summary: hostile}}, Moves: []findings.Move{{From: address, To: address}}}
	var md, plain bytes.Buffer
	if err := Markdown(&md, r); err != nil {
		t.Fatal(err)
	}
	if err := Text(&plain, r); err != nil {
		t.Fatal(err)
	}
	for _, out := range []string{md.String(), plain.String()} {
		if strings.Contains(out, "\x1b") || strings.Contains(out, "\nforged") {
			t.Fatal("untrusted control characters forged report structure")
		}
	}
	for _, raw := range []string{"[link](https://example.invalid)", "<img src=x>", "resource`"} {
		if strings.Contains(md.String(), raw) {
			t.Fatal("untrusted Markdown or HTML remained active")
		}
	}
	if !strings.Contains(md.String(), "&#34;") || !strings.Contains(md.String(), "&#96;") {
		t.Fatal("punctuation was not retained as visible text")
	}
}
