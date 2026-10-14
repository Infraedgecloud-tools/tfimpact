package render

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/infraedgecloud-tools/tfimpact/internal/findings"
	"github.com/infraedgecloud-tools/tfimpact/internal/rules"
)

// SARIF writes the findings as a SARIF 2.1.0 log, the format code-scanning
// tabs and most security dashboards ingest. A plan has no source positions,
// so each result carries the plan file as its artifact and the resource
// address as a logical location; the fingerprint is the finding's own ID,
// which is stable across runs for the same rule, address and observation.
func SARIF(w io.Writer, r *findings.Result) error {
	ruleIndex := map[string]int{}
	var ruleDefs []map[string]any
	for i, info := range rules.Catalogue {
		ruleIndex[info.ID] = i
		ruleDefs = append(ruleDefs, map[string]any{
			"id":               info.ID,
			"name":             ruleName(info.ID),
			"shortDescription": map[string]any{"text": firstSentence(info.Summary)},
			"fullDescription":  map[string]any{"text": info.Summary},
			"helpUri":          "https://github.com/infraedgecloud-tools/tfimpact/blob/main/docs/rules.md",
			"properties":       map[string]any{"domain": info.Domain, "severity": info.Severity},
		})
	}

	results := []map[string]any{}
	for _, f := range r.Findings {
		res := map[string]any{
			"ruleId":  f.RuleID,
			"level":   sarifLevel(f.Severity),
			"message": map[string]any{"text": sarifMessage(f)},
			"locations": []map[string]any{{
				"physicalLocation": map[string]any{
					"artifactLocation": map[string]any{"uri": artifactURI(r.Source)},
				},
				"logicalLocations": []map[string]any{{
					"fullyQualifiedName": f.Address(),
					"kind":               "resource",
				}},
			}},
			"partialFingerprints": map[string]any{"tfimpact/finding/v1": f.ID()},
			"properties": map[string]any{
				"severity": string(f.Severity),
				"domain":   f.Domain,
				"tags":     f.Tags,
			},
		}
		if i, ok := ruleIndex[f.RuleID]; ok {
			res["ruleIndex"] = i
		}
		if f.Suppressed {
			sup := map[string]any{"kind": "external"}
			if f.SuppressionReason != nil {
				sup["justification"] = *f.SuppressionReason
			}
			res["suppressions"] = []map[string]any{sup}
		}
		results = append(results, res)
	}

	doc := map[string]any{
		"$schema": "https://json.schemastore.org/sarif-2.1.0.json",
		"version": "2.1.0",
		"runs": []map[string]any{{
			"tool": map[string]any{
				"driver": map[string]any{
					"name":           "tfimpact",
					"version":        r.ToolVersion,
					"informationUri": "https://github.com/infraedgecloud-tools/tfimpact",
					"rules":          ruleDefs,
				},
			},
			"results": results,
			"properties": map[string]any{
				"headline": Headline(r),
				"plan":     PlanLine(r.Counts),
			},
		}},
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(doc)
}

// sarifLevel maps tfimpact's severities onto SARIF's three levels.
func sarifLevel(s findings.Severity) string {
	switch s {
	case findings.Critical, findings.High:
		return "error"
	case findings.Medium:
		return "warning"
	}
	return "note"
}

func sarifMessage(f findings.Finding) string {
	parts := []string{strip(f.Title)}
	if f.Observed != "" {
		parts = append(parts, strip(f.Observed))
	}
	if f.Recommendation != "" {
		parts = append(parts, strip(f.Recommendation))
	}
	return strings.Join(parts, "\n\n")
}

// ruleName turns PLAN-DATA-002 into PlanData002, the identifier form SARIF
// viewers show beside the id.
func ruleName(id string) string {
	var b strings.Builder
	for _, part := range strings.Split(id, "-") {
		if part == "" {
			continue
		}
		b.WriteString(strings.ToUpper(part[:1]) + strings.ToLower(part[1:]))
	}
	return b.String()
}

func firstSentence(s string) string {
	if i := strings.Index(s, ". "); i > 0 {
		return s[:i+1]
	}
	return s
}

// artifactURI is the plan as the user named it, or a placeholder when the
// plan came from stdin. SARIF wants a relative URI; absolute paths are kept
// as given since they are what the user will recognise.
func artifactURI(source string) string {
	if source == "" || source == "-" {
		return "plan.json"
	}
	return strings.ReplaceAll(source, "\\", "/")
}
