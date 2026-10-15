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
	"unicode"
)

// Plan is the subset of the plan representation tfimpact uses.
type Plan struct {
	FormatVersion    string           `json:"format_version"`
	TerraformVersion string           `json:"terraform_version"`
	Timestamp        string           `json:"timestamp"`
	Complete         *bool            `json:"complete"`
	Errored          *bool            `json:"errored"`
	Applyable        *bool            `json:"applyable"`
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
	case len(c.Actions) == 2 && has["create"] && has["delete"]:
		return Replace
	case (len(c.Actions) == 1 && has["forget"]) || (len(c.Actions) == 2 && has["create"] && has["forget"]):
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
	const maxPlanBytes = 64 << 20
	raw, err := io.ReadAll(io.LimitReader(r, maxPlanBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read plan: %w", err)
	}
	if len(raw) > maxPlanBytes {
		return nil, errors.New("plan JSON exceeds the 64 MiB input limit")
	}
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("%w: input is empty", ErrNotAPlan)
	}
	if trimmed[0] != '{' {
		return nil, fmt.Errorf("%w: expected a JSON object", ErrNotAPlan)
	}
	var p Plan
	if err := validateJSON(trimmed); err != nil {
		if bytes.Contains(trimmed[:min(len(trimmed), 4096)], []byte(`"@level"`)) {
			return nil, fmt.Errorf("%w: this looks like streaming `terraform plan -json`; supply saved-plan show-JSON", ErrNotAPlan)
		}
		return nil, errors.New("parse plan JSON: invalid, duplicate or excessively nested JSON")
	}
	if err := json.Unmarshal(trimmed, &p); err != nil {
		if bytes.Contains(trimmed[:min(len(trimmed), 4096)], []byte(`"@level"`)) {
			return nil, fmt.Errorf("%w: this looks like the streaming output of `terraform plan -json`; tfimpact needs `terraform plan -out=tfplan && terraform show -json tfplan`", ErrNotAPlan)
		}
		// JSON type/syntax errors can contain attacker-controlled values.
		return nil, errors.New("parse plan JSON: invalid JSON or field type")
	}
	if p.FormatVersion == "" {
		if bytes.Contains(trimmed[:min(len(trimmed), 4096)], []byte(`"@level"`)) {
			return nil, fmt.Errorf("%w: this looks like the streaming output of `terraform plan -json`; tfimpact needs `terraform plan -out=tfplan && terraform show -json tfplan`", ErrNotAPlan)
		}
		return nil, fmt.Errorf("%w: no format_version", ErrNotAPlan)
	}
	major, _, _ := strings.Cut(p.FormatVersion, ".")
	if major != "1" {
		return nil, errors.New("unsupported plan format_version: tfimpact understands major version 1")
	}
	parts := strings.Split(p.FormatVersion, ".")
	if len(parts) != 2 || !digits(parts[0]) || !digits(parts[1]) {
		return nil, errors.New("invalid plan format_version: expected numeric major.minor")
	}
	if p.Errored != nil && *p.Errored {
		return nil, errors.New("plan reports errored=true; analysis requires successful planning")
	}
	if p.Complete != nil && !*p.Complete {
		return nil, errors.New("plan reports complete=false; analysis requires a complete plan")
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(trimmed, &fields); err != nil {
		return nil, ErrNotAPlan
	}
	for _, name := range []string{"resource_changes", "resource_drift"} {
		if v, ok := fields[name]; ok && (len(bytes.TrimSpace(v)) == 0 || bytes.TrimSpace(v)[0] != '[') {
			return nil, errors.New("plan resource change collections must be arrays")
		}
	}
	planned, hasPlanned := fields["planned_values"]
	if deferred, ok := fields["deferred_changes"]; ok {
		var changes []json.RawMessage
		if err := json.Unmarshal(deferred, &changes); err != nil {
			return nil, errors.New("deferred_changes must be an array")
		}
		if len(changes) > 0 {
			return nil, errors.New("plan contains deferred changes; analysis requires a complete plan")
		}
	}
	if hasPlanned && (len(bytes.TrimSpace(planned)) == 0 || bytes.TrimSpace(planned)[0] != '{') {
		return nil, errors.New("planned_values must be an object")
	}
	if p.ResourceChanges == nil && p.ResourceDrift == nil && !hasPlanned {
		return nil, fmt.Errorf("%w: no resource_changes (is this `terraform show -json` of a state rather than a plan?)", ErrNotAPlan)
	}
	for _, changes := range [][]ResourceChange{p.ResourceChanges, p.ResourceDrift} {
		for _, rc := range changes {
			for _, identifier := range []string{rc.Address, rc.PreviousAddress, rc.Type, rc.ProviderName, rc.Name, rc.Mode} {
				for _, r := range identifier {
					if unicode.IsControl(r) {
						return nil, errors.New("plan identifiers contain control characters")
					}
				}
			}
			if rc.Address == "" || rc.Type == "" || len(rc.Change.Actions) == 0 {
				return nil, errors.New("resource changes require address, type and nonempty actions")
			}
			for _, action := range rc.Change.Actions {
				if action == "" {
					return nil, errors.New("resource actions must be nonempty strings")
				}
			}
		}
	}
	return &p, nil
}

func digits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// Validate all container keys before decoding into structs: duplicate masks or
// action fields must not depend on a decoder's last-key-wins behaviour.
func validateJSON(raw []byte) error {
	d := json.NewDecoder(bytes.NewReader(raw))
	d.UseNumber()
	var value func(int) error
	value = func(depth int) error {
		if depth > 128 {
			return errors.New("depth")
		}
		tok, err := d.Token()
		if err != nil {
			return err
		}
		if delim, ok := tok.(json.Delim); ok {
			switch delim {
			case '{':
				seen := map[string]bool{}
				for d.More() {
					key, err := d.Token()
					if err != nil {
						return err
					}
					s, ok := key.(string)
					if !ok || seen[s] {
						return errors.New("duplicate key")
					}
					seen[s] = true
					if err := value(depth + 1); err != nil {
						return err
					}
				}
			case '[':
				for d.More() {
					if err := value(depth + 1); err != nil {
						return err
					}
				}
			default:
				return errors.New("delimiter")
			}
			_, err = d.Token()
			return err
		}
		return nil
	}
	if err := value(0); err != nil {
		return err
	}
	if _, err := d.Token(); err != io.EOF {
		return errors.New("trailing JSON")
	}
	return nil
}
