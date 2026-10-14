package render

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/infraedgecloud-tools/tfimpact/internal/findings"
)

// The published schema and the envelope the tool writes must not drift.
// This is a structural check, not a full validation: every key the
// envelope writes is declared, every required key is written, and the
// finding object matches the same way.
func TestEnvelopeMatchesPublishedSchema(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "docs", "findings.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Required   []string                   `json:"required"`
		Properties map[string]json.RawMessage `json:"properties"`
		Defs       map[string]struct {
			Required   []string                   `json:"required"`
			Properties map[string]json.RawMessage `json:"properties"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}

	addr := "aws_db_instance.main"
	reason := "approved"
	r := &findings.Result{Tool: "tfimpact", ToolVersion: "test", Source: "plan.json", Findings: []findings.Finding{{
		RuleID: "PLAN-DATA-001", Product: "tfimpact", Severity: findings.High, Title: "t", Observed: "o",
		Recommendation: "r", Domain: "data", Path: &addr, Tags: []string{"x"}, Metadata: map[string]any{},
		Suppressed: true, SuppressionReason: &reason,
	}}}
	var buf strings.Builder
	if err := JSON(&buf, r); err != nil {
		t.Fatal(err)
	}
	var env map[string]json.RawMessage
	if err := json.Unmarshal([]byte(buf.String()), &env); err != nil {
		t.Fatal(err)
	}
	compareKeys(t, "envelope", keys(env), schema.Required, keysOf(schema.Properties))

	var fs []map[string]json.RawMessage
	if err := json.Unmarshal(env["findings"], &fs); err != nil {
		t.Fatal(err)
	}
	if len(fs) != 1 {
		t.Fatalf("expected one finding, got %d", len(fs))
	}
	fdef := schema.Defs["finding"]
	compareKeys(t, "finding", keys(fs[0]), fdef.Required, keysOf(fdef.Properties))
}

func compareKeys(t *testing.T, what string, written, required, declared []string) {
	t.Helper()
	sort.Strings(written)
	sort.Strings(required)
	sort.Strings(declared)
	if strings.Join(written, ",") != strings.Join(declared, ",") {
		t.Errorf("%s: written keys %v differ from the schema's properties %v", what, written, declared)
	}
	if strings.Join(written, ",") != strings.Join(required, ",") {
		t.Errorf("%s: written keys %v differ from the schema's required list %v", what, written, required)
	}
}

func keys(m map[string]json.RawMessage) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func keysOf(m map[string]json.RawMessage) []string { return keys(m) }

// SARIF: one result per finding, suppressed ones marked, levels mapped.
func TestSARIFShape(t *testing.T) {
	addr := "aws_security_group.web"
	reason := "public on purpose"
	r := &findings.Result{Tool: "tfimpact", ToolVersion: "test", Source: "plan.json", Findings: []findings.Finding{
		{RuleID: "PLAN-NET-001", Severity: findings.Critical, Title: "a", Domain: "security", Path: &addr},
		{RuleID: "PLAN-REPLACE-001", Severity: findings.Medium, Title: "b", Domain: "availability", Path: &addr, Suppressed: true, SuppressionReason: &reason},
		{RuleID: "PLAN-IAM-002", Severity: findings.Low, Title: "c", Domain: "security", Path: &addr},
	}}
	var buf strings.Builder
	if err := SARIF(&buf, r); err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Version string `json:"version"`
		Runs    []struct {
			Tool struct {
				Driver struct {
					Rules []struct {
						ID string `json:"id"`
					} `json:"rules"`
				} `json:"driver"`
			} `json:"tool"`
			Results []struct {
				RuleID       string            `json:"ruleId"`
				Level        string            `json:"level"`
				Suppressions []map[string]any  `json:"suppressions"`
				Locations    []map[string]any  `json:"locations"`
				Fingerprints map[string]string `json:"partialFingerprints"`
			} `json:"results"`
		} `json:"runs"`
	}
	if err := json.Unmarshal([]byte(buf.String()), &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Version != "2.1.0" || len(doc.Runs) != 1 {
		t.Fatalf("not a single-run SARIF 2.1.0 log: %s", buf.String())
	}
	run := doc.Runs[0]
	if len(run.Tool.Driver.Rules) == 0 {
		t.Fatal("the driver lists no rules")
	}
	if len(run.Results) != 3 {
		t.Fatalf("expected 3 results, got %d", len(run.Results))
	}
	if got := []string{run.Results[0].Level, run.Results[1].Level, run.Results[2].Level}; strings.Join(got, ",") != "error,warning,note" {
		t.Fatalf("levels = %v", got)
	}
	if len(run.Results[1].Suppressions) != 1 || run.Results[1].Suppressions[0]["justification"] != reason {
		t.Fatalf("suppressed finding not marked: %+v", run.Results[1])
	}
	if run.Results[0].Fingerprints["tfimpact/finding/v1"] == "" || len(run.Results[0].Locations) != 1 {
		t.Fatalf("result lacks a fingerprint or location: %+v", run.Results[0])
	}
}
