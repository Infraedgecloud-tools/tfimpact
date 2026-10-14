// Package render writes a Result as Markdown (for pull-request comments),
// plain text (for terminals and logs) or JSON (the findings contract).
package render

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/infraedgecloud-tools/tfimpact/internal/findings"
	"github.com/infraedgecloud-tools/tfimpact/internal/ghcomment"
)

// Headline is the one-sentence verdict that opens every output.
func Headline(r *findings.Result) string {
	active := r.Active()
	if len(active) == 0 {
		routine := r.Counts.Create + r.Counts.Update + r.Counts.Delete + r.Counts.Replace
		if routine == 0 && r.Counts.Move == 0 {
			return "nothing changes"
		}
		s := fmt.Sprintf("no consequences found, %s", pluralAdj(routine, "routine change"))
		var extra []string
		if r.Counts.TagOnly > 0 {
			extra = append(extra, fmt.Sprintf("%d tags only", r.Counts.TagOnly))
		}
		if r.Counts.Move > 0 {
			extra = append(extra, plural(r.Counts.Move, "move"))
		}
		if len(extra) > 0 {
			s += " (" + strings.Join(extra, ", ") + ")"
		}
		return s
	}
	var data, security, availability, replaced, other, minor int
	for _, f := range active {
		if f.Severity == findings.Low {
			minor++
			continue
		}
		switch f.Domain {
		case "data":
			data++
		case "security":
			security++
		case "availability":
			// A replacement the knowledge file does not mark as interrupting
			// service is reported at medium: an identifier changes, nothing
			// stops. Counting it as "interrupts service" overstated the plan.
			if f.RuleID == "PLAN-REPLACE-001" && f.Severity.Rank() < findings.High.Rank() {
				replaced++
			} else {
				availability++
			}
		default:
			if f.RuleID == "PLAN-RENAME-001" && f.Severity == findings.Critical {
				data++
			} else {
				other++
			}
		}
	}
	var parts []string
	if data > 0 {
		parts = append(parts, fmt.Sprintf("%s data", pluralVerb(data, "change", "destroys", "destroy")))
	}
	if security > 0 {
		parts = append(parts, fmt.Sprintf("%s exposure", pluralVerb(security, "change", "widens", "widen")))
	}
	if availability > 0 {
		parts = append(parts, fmt.Sprintf("%s service", pluralVerb(availability, "change", "interrupts", "interrupt")))
	}
	if replaced > 0 {
		parts = append(parts, fmt.Sprintf("%d replaced", replaced))
	}
	if other > 0 {
		parts = append(parts, fmt.Sprintf("%s attention", pluralVerb(other, "change", "needs", "need")))
	}
	if minor > 0 {
		parts = append(parts, fmt.Sprintf("%d minor", minor))
	}
	routine := 0
	for _, g := range r.Routine {
		routine += len(g.Addresses)
	}
	if routine > 0 {
		parts = append(parts, fmt.Sprintf("%d routine", routine))
	}
	return strings.Join(parts, ", ")
}

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func pluralAdj(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func pluralVerb(n int, noun, singular, pluralV string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s %s", n, noun, singular)
	}
	return fmt.Sprintf("%d %ss %s", n, noun, pluralV)
}

// PlanLine renders Terraform's familiar "N to add, N to change, N to destroy".
func PlanLine(c findings.ChangeCounts) string {
	add := c.Create + c.Replace
	destroy := c.Delete + c.Replace
	s := fmt.Sprintf("%d to add, %d to change, %d to destroy", add, c.Update, destroy)
	var extra []string
	if c.Replace > 0 {
		extra = append(extra, plural(c.Replace, "replacement"))
	}
	if c.Move > 0 {
		extra = append(extra, plural(c.Move, "move"))
	}
	if c.Import > 0 {
		extra = append(extra, plural(c.Import, "import"))
	}
	if c.Forget > 0 {
		extra = append(extra, fmt.Sprintf("%d forgotten", c.Forget))
	}
	if c.NoOp > 0 {
		extra = append(extra, fmt.Sprintf("%d unchanged", c.NoOp))
	}
	if len(extra) > 0 {
		s += " (" + strings.Join(extra, ", ") + ")"
	}
	return s
}

var severityOrder = []findings.Severity{findings.Critical, findings.High, findings.Medium, findings.Low}

var severityIcon = map[findings.Severity]string{findings.Critical: "🔴", findings.High: "🟠", findings.Medium: "🟡", findings.Low: "⚪"}

// Markdown writes the pull-request comment form.
func Markdown(w io.Writer, r *findings.Result) error {
	var b strings.Builder
	b.WriteString(ghcomment.Marker + "\n")
	fmt.Fprintf(&b, "## tfimpact: %s\n\n", Headline(r))
	fmt.Fprintf(&b, "**Plan:** %s · %s\n", PlanLine(r.Counts), engine(r))
	active := r.Active()
	for _, sev := range severityOrder {
		group := bySeverity(active, sev)
		if len(group) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n### %s %s (%d)\n\n", severityIcon[sev], title(sev), len(group))
		for _, f := range group {
			fmt.Fprintf(&b, "- **%s.** %s _%s_ `%s`\n", f.Title, f.Observed, f.Recommendation, f.RuleID)
		}
	}
	if len(r.Routine) > 0 {
		total := 0
		for _, g := range r.Routine {
			total += len(g.Addresses)
		}
		fmt.Fprintf(&b, "\n### Routine changes (%d)\n\n", total)
		for _, g := range r.Routine {
			writeRoutineMarkdown(&b, g)
		}
	}
	if len(r.Moves) > 0 {
		fmt.Fprintf(&b, "\n### Moved (%d)\n\n", len(r.Moves))
		for _, m := range r.Moves {
			fmt.Fprintf(&b, "- `%s` → `%s` (address change only, nothing is recreated)\n", m.From, m.To)
		}
	}
	if len(r.Drift) > 0 {
		fmt.Fprintf(&b, "\n### Changed outside Terraform (%d)\n\n", len(r.Drift))
		for _, d := range r.Drift {
			fmt.Fprintf(&b, "- `%s`: %s\n", d.Address, driftText(d, true))
		}
	}
	if sup := r.Suppressed(); len(sup) > 0 {
		fmt.Fprintf(&b, "\n### Suppressed (%d)\n\n", len(sup))
		for _, f := range sup {
			reason := ""
			if f.SuppressionReason != nil {
				reason = *f.SuppressionReason
			}
			exp := ""
			if f.SuppressionExpires != nil {
				exp = ", until " + *f.SuppressionExpires
			}
			fmt.Fprintf(&b, "- %s `%s` — %s%s\n", f.Title, f.RuleID, reason, exp)
		}
	}
	fmt.Fprintf(&b, "\n<sub>%s</sub>\n", footer(r))
	_, err := io.WriteString(w, b.String())
	return err
}

func writeRoutineMarkdown(b *strings.Builder, g findings.Routine) {
	switch {
	case len(g.Addresses) == 1:
		fmt.Fprintf(b, "- `%s`: %s\n", g.Addresses[0], g.Summary)
	case len(g.Addresses) <= 3:
		fmt.Fprintf(b, "- %s: %s\n", joinCodes(g.Addresses), g.Summary)
	default:
		fmt.Fprintf(b, "- <details><summary>%d × `%s`: %s</summary>\n\n", len(g.Addresses), g.Type, g.Summary)
		for _, a := range g.Addresses {
			fmt.Fprintf(b, "  - `%s`\n", a)
		}
		b.WriteString("\n  </details>\n")
	}
}

// Text writes the terminal form.
func Text(w io.Writer, r *findings.Result) error {
	var b strings.Builder
	fmt.Fprintf(&b, "tfimpact: %s\n", Headline(r))
	fmt.Fprintf(&b, "Plan: %s. %s.\n", PlanLine(r.Counts), engine(r))
	active := r.Active()
	for _, sev := range severityOrder {
		group := bySeverity(active, sev)
		if len(group) == 0 {
			continue
		}
		fmt.Fprintf(&b, "\n%s (%d)\n", strings.ToUpper(string(sev)), len(group))
		for _, f := range group {
			fmt.Fprintf(&b, "  * %s [%s]\n", strip(f.Title), f.RuleID)
			fmt.Fprintf(&b, "    %s\n", strip(f.Observed))
			fmt.Fprintf(&b, "    -> %s\n", strip(f.Recommendation))
		}
	}
	if len(r.Routine) > 0 {
		total := 0
		for _, g := range r.Routine {
			total += len(g.Addresses)
		}
		fmt.Fprintf(&b, "\nROUTINE (%d)\n", total)
		for _, g := range r.Routine {
			if len(g.Addresses) <= 3 {
				fmt.Fprintf(&b, "  * %s: %s\n", strings.Join(g.Addresses, ", "), strip(g.Summary))
			} else {
				fmt.Fprintf(&b, "  * %d x %s: %s\n", len(g.Addresses), g.Type, strip(g.Summary))
			}
		}
	}
	if len(r.Moves) > 0 {
		fmt.Fprintf(&b, "\nMOVED (%d)\n", len(r.Moves))
		for _, m := range r.Moves {
			fmt.Fprintf(&b, "  * %s -> %s\n", m.From, m.To)
		}
	}
	if len(r.Drift) > 0 {
		fmt.Fprintf(&b, "\nCHANGED OUTSIDE TERRAFORM (%d)\n", len(r.Drift))
		for _, d := range r.Drift {
			fmt.Fprintf(&b, "  * %s: %s\n", d.Address, strip(driftText(d, false)))
		}
	}
	if sup := r.Suppressed(); len(sup) > 0 {
		fmt.Fprintf(&b, "\nSUPPRESSED (%d)\n", len(sup))
		for _, f := range sup {
			reason := ""
			if f.SuppressionReason != nil {
				reason = *f.SuppressionReason
			}
			fmt.Fprintf(&b, "  * %s [%s]: %s\n", strip(f.Title), f.RuleID, reason)
		}
	}
	fmt.Fprintf(&b, "\n%s\n", strip(footer(r)))
	_, err := io.WriteString(w, b.String())
	return err
}

// JSON writes the schema 1.1 envelope.
func JSON(w io.Writer, r *findings.Result) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(r.Envelope())
}

// driftText says what happened to an object outside Terraform.
func driftText(d findings.Drift, md bool) string {
	for _, a := range d.Actions {
		if a == "delete" {
			return "deleted outside Terraform"
		}
	}
	attrs := strings.Join(d.Changed, ", ")
	if md {
		attrs = joinCodes(d.Changed)
	}
	return "changed outside Terraform: " + attrs
}

func engine(r *findings.Result) string {
	if r.TerraformVersion == "" {
		return "plan format " + r.FormatVersion
	}
	return fmt.Sprintf("Terraform/OpenTofu %s · %s", r.TerraformVersion, plural(r.Resources, "resource"))
}

func footer(r *findings.Result) string {
	return fmt.Sprintf("tfimpact %s by [InfraEdge](https://infraedge.org) · deterministic, runs on the runner, sends nothing anywhere · sensitive values masked", r.ToolVersion)
}

func bySeverity(fs []findings.Finding, sev findings.Severity) []findings.Finding {
	var out []findings.Finding
	for _, f := range fs {
		if f.Severity == sev {
			out = append(out, f)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Address() < out[j].Address() })
	return out
}

func title(sev findings.Severity) string {
	return strings.ToUpper(string(sev)[:1]) + string(sev)[1:]
}

func joinCodes(xs []string) string {
	q := make([]string, len(xs))
	for i, x := range xs {
		q[i] = "`" + x + "`"
	}
	return strings.Join(q, ", ")
}

// strip removes Markdown emphasis for plain-text output.
func strip(s string) string {
	s = strings.ReplaceAll(s, "`", "")
	s = strings.ReplaceAll(s, "**", "")
	s = strings.ReplaceAll(s, "[InfraEdge](https://infraedge.org)", "InfraEdge")
	return s
}
