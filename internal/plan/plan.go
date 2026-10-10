// Package plan loads the JSON produced by `terraform show -json <planfile>` or
// `tofu show -json <planfile>` and exposes the parts tfimpact reasons about.
//
// Only the documented, forward-compatible subset of the format is modelled.
// Unknown properties are ignored, as the format documentation asks.
package plan

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// Plan is the subset of the plan representation tfimpact uses.
type Plan struct {
	FormatVersion    string           `json:"format_version"`
	TerraformVersion string           `json:"terraform_version"`
	Timestamp        string           `json:"timestamp"`
	ResourceChanges  []ResourceChange `json:"resource_changes"`
	ResourceDrift    []ResourceChange `json:"resource_drift"`
}

// ResourceChange is one entry of resource_changes or resource_drift.
type ResourceChange struct {
	Address         string `json:"address"`
	PreviousAddress string `json:"previous_address"`
	ModuleAddress   string `json:"module_address"`
	Mode            string `json:"mode"`
	Type            string `json:"type"`
	Name            string `json:"name"`
	Index           any    `json:"index"`
	ProviderName    string `json:"provider_name"`
	ActionReason    string `json:"action_reason"`
	Change          Change `json:"change"`
}

// Change carries the before/after values and their sensitivity masks.
type Change struct {
	Actions         []string   `json:"actions"`
	Before          any        `json:"before"`
	After           any        `json:"after"`
	AfterUnknown    any        `json:"after_unknown"`
	BeforeSensitive any        `json:"before_sensitive"`
	AfterSensitive  any        `json:"after_sensitive"`
	ReplacePaths    [][]any    `json:"replace_paths"`
	Importing       *Importing `json:"importing"`
}

// Importing is present when the object is being imported as part of the change.
type Importing struct {
	ID string `json:"id"`
}

// Kind is a coarse classification of a change.
type Kind string

const (
	NoOp    Kind = "no-op"
	Create  Kind = "create"
	Read    Kind = "read"
	Update  Kind = "update"
	Delete  Kind = "delete"
	Replace Kind = "replace"
	Forget  Kind = "forget"
	Unknown Kind = "unknown"
)

// Kind classifies the actions list. Any list containing both "create" and
// "delete" is a replacement; "forget" variants are reported as Forget.
func (c Change) Kind() Kind {
	has := map[string]bool{}
	for _, a := range c.Actions {
		has[a] = true
	}
	switch {
	case has["create"] && has["delete"]:
		return Replace
	case has["forget"]:
		return Forget
	case len(c.Actions) == 1 && has["no-op"]:
		return NoOp
	case len(c.Actions) == 1 && has["create"]:
		return Create
	case len(c.Actions) == 1 && has["read"]:
		return Read
	case len(c.Actions) == 1 && has["update"]:
		return Update
	case len(c.Actions) == 1 && has["delete"]:
		return Delete
	}
	return Unknown
}

// CreateBeforeDestroy reports whether a replacement creates the new object first.
func (c Change) CreateBeforeDestroy() bool {
	return len(c.Actions) == 2 && c.Actions[0] == "create" && c.Actions[1] == "delete"
}

// BeforeMap returns the before value as an attribute map (nil when absent).
func (c Change) BeforeMap() map[string]any {
	m, _ := c.Before.(map[string]any)
	return m
}

// AfterMap returns the after value as an attribute map (nil when absent).
func (c Change) AfterMap() map[string]any {
	m, _ := c.After.(map[string]any)
	return m
}

// ReplacePathStrings renders replace_paths as dotted attribute paths.
func (c Change) ReplacePathStrings() []string {
	out := make([]string, 0, len(c.ReplacePaths))
	for _, p := range c.ReplacePaths {
		out = append(out, PathString(p))
	}
	return out
}

// PathString renders a JSON path (strings and numbers) as a.b[0].c.
func PathString(path []any) string {
	var b strings.Builder
	for _, step := range path {
		switch v := step.(type) {
		case string:
			if b.Len() > 0 {
				b.WriteByte('.')
			}
			b.WriteString(v)
		case float64:
			b.WriteString("[" + strconv.Itoa(int(v)) + "]")
		case int:
			b.WriteString("[" + strconv.Itoa(v) + "]")
		default:
			b.WriteString(fmt.Sprintf("[%v]", v))
		}
	}
	return b.String()
}

// Managed reports whether the change is for a managed resource (not a data source).
func (r ResourceChange) Managed() bool { return r.Mode == "" || r.Mode == "managed" }

// ErrNotAPlan is returned when the input is JSON but not a plan representation.
var ErrNotAPlan = errors.New("input is not a Terraform plan JSON document")

// Load parses plan JSON from r.
func Load(r io.Reader) (*Plan, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read plan: %w", err)
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("%w: input is empty", ErrNotAPlan)
	}
	if trimmed[0] != '{' {
		return nil, fmt.Errorf("%w: expected a JSON object", ErrNotAPlan)
	}
	var p Plan
	if err := json.Unmarshal(trimmed, &p); err != nil {
		if bytes.Contains(trimmed[:min(len(trimmed), 4096)], []byte(`"@level"`)) {
			return nil, fmt.Errorf("%w: this looks like the streaming output of `terraform plan -json`; tfimpact needs `terraform plan -out=tfplan && terraform show -json tfplan`", ErrNotAPlan)
		}
		return nil, fmt.Errorf("parse plan JSON: %w", err)
	}
	if p.FormatVersion == "" {
		if bytes.Contains(trimmed[:min(len(trimmed), 4096)], []byte(`"@level"`)) {
			return nil, fmt.Errorf("%w: this looks like the streaming output of `terraform plan -json`; tfimpact needs `terraform plan -out=tfplan && terraform show -json tfplan`", ErrNotAPlan)
		}
		return nil, fmt.Errorf("%w: no format_version", ErrNotAPlan)
	}
	major, _, _ := strings.Cut(p.FormatVersion, ".")
	if major != "1" {
		return nil, fmt.Errorf("unsupported plan format_version %q: tfimpact understands major version 1", p.FormatVersion)
	}
	if p.ResourceChanges == nil && p.ResourceDrift == nil && !bytes.Contains(trimmed, []byte(`"planned_values"`)) {
		return nil, fmt.Errorf("%w: no resource_changes (is this `terraform show -json` of a state rather than a plan?)", ErrNotAPlan)
	}
	return &p, nil
}
