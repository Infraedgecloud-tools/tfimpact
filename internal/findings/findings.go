// Package findings is the public result contract: the same envelope as the
// InfraEdge toolkit findings schema 1.1, with additive plan-specific fields.
package findings

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strings"
	"time"
)

// Severity is the toolkit ladder.
type Severity string

const (
	Low      Severity = "low"
	Medium   Severity = "medium"
	High     Severity = "high"
	Critical Severity = "critical"
)

// Rank orders severities; higher is worse. Unknown severities rank 0.
func (s Severity) Rank() int {
	switch s {
	case Low:
		return 1
	case Medium:
		return 2
	case High:
		return 3
	case Critical:
		return 4
	}
	return 0
}

// Parse accepts a severity name or "none".
func Parse(s string) (Severity, bool) {
	switch strings.ToLower(s) {
	case "low":
		return Low, true
	case "medium":
		return Medium, true
	case "high":
		return High, true
	case "critical":
		return Critical, true
	}
	return "", false
}

// Finding is one explained consequence of the plan.
type Finding struct {
	RuleID             string         `json:"rule_id"`
	Product            string         `json:"product"`
	Severity           Severity       `json:"severity"`
	Title              string         `json:"title"`
	Observed           string         `json:"observed"`
	Recommendation     string         `json:"recommendation"`
	Domain             string         `json:"domain"`
	Path               *string        `json:"path"` // resource address
	Line               *int           `json:"line"`
	Tags               []string       `json:"tags"`
	Metadata           map[string]any `json:"metadata"`
	Suppressed         bool           `json:"suppressed"`
	SuppressionReason  *string        `json:"suppression_reason"`
	SuppressionExpires *string        `json:"suppression_expires"`
}

// Address returns the resource address or "".
func (f Finding) Address() string {
	if f.Path == nil {
		return ""
	}
	return *f.Path
}

// ID is stable across runs for the same rule, address and observation.
func (f Finding) ID() string {
	seed := strings.Join([]string{f.RuleID, f.Address(), "", f.Observed}, "|")
	sum := sha256.Sum256([]byte(seed))
	return "fnd_" + hex.EncodeToString(sum[:])[:16]
}

// ChangeCounts summarises the plan's actions.
type ChangeCounts struct {
	Create  int `json:"create"`
	Update  int `json:"update"`
	Delete  int `json:"delete"`
	Replace int `json:"replace"`
	NoOp    int `json:"no_op"`
	Read    int `json:"read"`
	Forget  int `json:"forget"`
	Move    int `json:"move"`
	Import  int `json:"import"`
	Drift   int `json:"drift"`
	TagOnly int `json:"tag_only"`
}

// Routine is a group of benign changes that share a type and a change shape.
type Routine struct {
	Type      string   `json:"type"`
	Addresses []string `json:"addresses"`
	Summary   string   `json:"summary"` // e.g. "retention_in_days 30 -> 90" or "tags only"
}

// Move records a resource whose address changed via a moved block.
type Move struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// Drift records an object that changed outside Terraform.
type Drift struct {
	Address string   `json:"address"`
	Actions []string `json:"actions"`
	Changed []string `json:"changed_attributes"`
}

// Result is the full output of one run.
type Result struct {
	Tool             string
	ToolVersion      string
	Source           string // file path or "-"
	TerraformVersion string
	FormatVersion    string
	GeneratedAt      time.Time
	ConfigPath       string
	Findings         []Finding
	Counts           ChangeCounts
	Routine          []Routine
	Moves            []Move
	Drift            []Drift
	Resources        int
}

// Active returns unsuppressed findings sorted by severity then address.
func (r Result) Active() []Finding {
	var out []Finding
	for _, f := range r.Findings {
		if !f.Suppressed {
			out = append(out, f)
		}
	}
	Sort(out)
	return out
}

// Suppressed returns suppressed findings.
func (r Result) Suppressed() []Finding {
	var out []Finding
	for _, f := range r.Findings {
		if f.Suppressed {
			out = append(out, f)
		}
	}
	Sort(out)
	return out
}

// Sort orders by severity (worst first), then address, then rule.
func Sort(fs []Finding) {
	sort.SliceStable(fs, func(i, j int) bool {
		if fs[i].Severity.Rank() != fs[j].Severity.Rank() {
			return fs[i].Severity.Rank() > fs[j].Severity.Rank()
		}
		if fs[i].Address() != fs[j].Address() {
			return fs[i].Address() < fs[j].Address()
		}
		return fs[i].RuleID < fs[j].RuleID
	})
}

// SeverityCounts counts active findings per severity.
func (r Result) SeverityCounts() map[string]int {
	c := map[string]int{"critical": 0, "high": 0, "medium": 0, "low": 0}
	for _, f := range r.Active() {
		c[string(f.Severity)]++
	}
	return c
}

// Reaches reports whether any active finding is at or above the threshold.
func (r Result) Reaches(threshold Severity) bool {
	for _, f := range r.Active() {
		if f.Severity.Rank() >= threshold.Rank() {
			return true
		}
	}
	return false
}

// Envelope renders the schema 1.1 JSON document as a map (ordered by the
// encoder). Field names and the privacy block match the toolkit contract.
func (r Result) Envelope() map[string]any {
	fs := make([]map[string]any, 0, len(r.Findings))
	all := append([]Finding{}, r.Findings...)
	Sort(all)
	for _, f := range all {
		m := map[string]any{
			"id":                  f.ID(),
			"rule_id":             f.RuleID,
			"product":             f.Product,
			"severity":            f.Severity,
			"title":               f.Title,
			"observed":            f.Observed,
			"recommendation":      f.Recommendation,
			"domain":              f.Domain,
			"path":                f.Path,
			"line":                f.Line,
			"tags":                nonNil(f.Tags),
			"metadata":            f.Metadata,
			"suppressed":          f.Suppressed,
			"suppression_reason":  f.SuppressionReason,
			"suppression_expires": f.SuppressionExpires,
		}
		if m["metadata"] == nil {
			m["metadata"] = map[string]any{}
		}
		fs = append(fs, m)
	}
	routine := r.Routine
	if routine == nil {
		routine = []Routine{}
	}
	moves := r.Moves
	if moves == nil {
		moves = []Move{}
	}
	drift := r.Drift
	if drift == nil {
		drift = []Drift{}
	}
	return map[string]any{
		"schema_version": "1.1",
		"tool":           r.Tool,
		"tool_version":   r.ToolVersion,
		"generated_at":   r.GeneratedAt.UTC().Format(time.RFC3339),
		"source": map[string]any{
			"kind":              "terraform-plan",
			"root":              r.Source,
			"terraform_version": r.TerraformVersion,
			"format_version":    r.FormatVersion,
		},
		"policy": map[string]any{
			"configured": r.ConfigPath != "",
			"path":       nullable(r.ConfigPath),
			"exclusions": []string{},
		},
		"summary": map[string]any{
			"findings":            len(r.Active()),
			"suppressed_findings": len(r.Suppressed()),
			"total_findings":      len(r.Findings),
			"scanned_files":       1,
			"skipped_files":       0,
			"excluded_files":      0,
			"severity":            r.SeverityCounts(),
			"resources":           r.Resources,
			"changes":             r.Counts,
		},
		"findings": fs,
		"routine":  routine,
		"moves":    moves,
		"drift":    drift,
		"privacy": map[string]any{
			"uploaded":  false,
			"telemetry": false,
			"note":      "tfimpact runs locally. Nothing is sent anywhere; sensitive values are masked.",
		},
	}
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}

func nullable(s string) any {
	if s == "" {
		return nil
	}
	return s
}
