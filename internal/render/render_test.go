package render

import (
	"testing"

	"github.com/infraedgecloud-tools/tfimpact/internal/findings"
)

// A medium replacement is "replaced", not "interrupts service": nothing
// stops, an identifier changes. A high one, or a disruptive update, still
// interrupts.
func TestHeadlineSeparatesReplacementsFromInterruptions(t *testing.T) {
	r := &findings.Result{Findings: []findings.Finding{
		{RuleID: "PLAN-REPLACE-001", Severity: findings.Medium, Domain: "availability", Title: "a"},
		{RuleID: "PLAN-REPLACE-001", Severity: findings.High, Domain: "availability", Title: "b"},
		{RuleID: "PLAN-DISRUPT-001", Severity: findings.Medium, Domain: "availability", Title: "c"},
	}}
	if got, want := Headline(r), "2 changes interrupt service, 1 replaced"; got != want {
		t.Fatalf("Headline = %q, want %q", got, want)
	}
}
