// Package knowledge holds what tfimpact knows about AWS resource types: which
// hold data, which attributes mitigate loss, which in-place changes disrupt,
// and where policy documents and ingress rules live.
//
// The knowledge is data, not code: one TOML file per resource type under
// aws/, embedded into the binary. Contributors add a file and a fixture.
package knowledge

import (
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

//go:embed aws/*.toml
var embedded embed.FS

// Resource is everything known about one resource type.
type Resource struct {
	Type    string `toml:"type"`
	Display string `toml:"display"` // short noun phrase, e.g. "RDS database instance"
	Doc     string `toml:"doc"`     // provider documentation URL

	// HoldsData marks a resource whose destruction loses user data. Data is a
	// noun phrase for what is lost, e.g. "the database and its contents".
	HoldsData bool   `toml:"holds_data"`
	Data      string `toml:"data"`

	// DowntimeOnReplace marks a resource whose replacement interrupts service
	// even though nothing is lost; Downtime says what happens.
	DowntimeOnReplace bool   `toml:"downtime_on_replace"`
	Downtime          string `toml:"downtime"`

	// Identity lists attributes whose equality between a deleted and a created
	// resource of this type indicates a rename without a moved block.
	Identity []string `toml:"identity"`

	// Mitigations are checked on destroy or replace; each one that matches is
	// reported as a reason the loss is real. Safeguards are checked the same way
	// and reported as what will soften or stop the loss.
	Mitigations []Condition `toml:"mitigation"`
	Safeguards  []Condition `toml:"safeguard"`

	// Disruptive lists in-place attribute changes with a service effect.
	Disruptive []Disruptive `toml:"disruptive"`

	// Policies lists attributes that hold IAM policy documents as JSON strings.
	Policies []PolicyAttr `toml:"policy"`

	// Flags are single attributes whose transition to an unsafe value widens
	// exposure or removes protection.
	Flags []Flag `toml:"flag"`

	// Delete, when set, raises the severity of destroying this resource and
	// says what stops working (for protective resources such as trails).
	Delete *DeleteNote `toml:"delete"`
}

// DeleteNote explains why destroying a non-data resource matters.
type DeleteNote struct {
	Severity string `toml:"severity"` // medium, high or critical
	Message  string `toml:"message"`
}

// Condition matches an attribute value on the before side of a destroy.
type Condition struct {
	Attr    string `toml:"attr"`
	Equals  any    `toml:"equals"`  // match when the value equals this
	Absent  bool   `toml:"absent"`  // match when the value is null or missing
	Present bool   `toml:"present"` // match when the value is non-null
	Message string `toml:"message"`
}

// Disruptive describes an in-place attribute change with an effect.
type Disruptive struct {
	Attr     string `toml:"attr"`
	Effect   string `toml:"effect"`
	Severity string `toml:"severity"` // low, medium, high; default medium
}

// PolicyAttr names an attribute carrying an IAM policy document.
type PolicyAttr struct {
	Attr string `toml:"attr"`
	Kind string `toml:"kind"` // identity, resource or trust
}

// Flag is an attribute whose transition to Unsafe is reported.
type Flag struct {
	Attr     string `toml:"attr"`
	Unsafe   any    `toml:"unsafe"`
	Message  string `toml:"message"`
	Severity string `toml:"severity"` // default high
}

// Base is the set of resource knowledge, keyed by type.
type Base struct {
	resources map[string]*Resource
}

// Load returns the embedded knowledge base.
func Load() (*Base, error) {
	return loadFS(embedded, "aws")
}

// LoadDir loads knowledge from a directory on disk instead of the embedded
// copy, for contributors testing new files.
func LoadDir(dir string) (*Base, error) {
	return loadFS(os.DirFS(dir), ".")
}

func loadFS(fsys fs.FS, dir string) (*Base, error) {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return nil, fmt.Errorf("read knowledge dir: %w", err)
	}
	b := &Base{resources: map[string]*Resource{}}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".toml") {
			continue
		}
		data, err := fs.ReadFile(fsys, filepath.ToSlash(filepath.Join(dir, e.Name())))
		if err != nil {
			return nil, err
		}
		var r Resource
		md, err := toml.Decode(string(data), &r)
		if err != nil {
			return nil, fmt.Errorf("knowledge %s: %w", e.Name(), err)
		}
		if und := md.Undecoded(); len(und) > 0 {
			return nil, fmt.Errorf("knowledge %s: unknown keys %v", e.Name(), und)
		}
		if err := r.validate(e.Name()); err != nil {
			return nil, err
		}
		if _, dup := b.resources[r.Type]; dup {
			return nil, fmt.Errorf("knowledge %s: duplicate type %s", e.Name(), r.Type)
		}
		b.resources[r.Type] = &r
	}
	return b, nil
}

func (r *Resource) validate(file string) error {
	if r.Type == "" {
		return fmt.Errorf("knowledge %s: type is required", file)
	}
	if strings.TrimSuffix(file, ".toml") != r.Type {
		return fmt.Errorf("knowledge %s: file name must equal type %q", file, r.Type)
	}
	if r.Display == "" {
		return fmt.Errorf("knowledge %s: display is required", file)
	}
	if r.HoldsData && r.Data == "" {
		return fmt.Errorf("knowledge %s: data is required when holds_data = true", file)
	}
	if r.DowntimeOnReplace && r.Downtime == "" {
		return fmt.Errorf("knowledge %s: downtime is required when downtime_on_replace = true", file)
	}
	for _, c := range append(append([]Condition{}, r.Mitigations...), r.Safeguards...) {
		n := 0
		if c.Equals != nil {
			n++
		}
		if c.Absent {
			n++
		}
		if c.Present {
			n++
		}
		if c.Attr == "" || c.Message == "" || n != 1 {
			return fmt.Errorf("knowledge %s: condition on %q needs attr, message and exactly one of equals/absent/present", file, c.Attr)
		}
	}
	for _, d := range r.Disruptive {
		if d.Attr == "" || d.Effect == "" {
			return fmt.Errorf("knowledge %s: disruptive entries need attr and effect", file)
		}
		if d.Severity != "" && d.Severity != "low" && d.Severity != "medium" && d.Severity != "high" {
			return fmt.Errorf("knowledge %s: disruptive %s has invalid severity %q", file, d.Attr, d.Severity)
		}
	}
	for _, p := range r.Policies {
		if p.Attr == "" || (p.Kind != "identity" && p.Kind != "resource" && p.Kind != "trust") {
			return fmt.Errorf("knowledge %s: policy entries need attr and kind identity|resource|trust", file)
		}
	}
	if r.Delete != nil {
		if r.Delete.Message == "" || (r.Delete.Severity != "medium" && r.Delete.Severity != "high" && r.Delete.Severity != "critical") {
			return fmt.Errorf("knowledge %s: [delete] needs message and severity medium|high|critical", file)
		}
	}
	for _, f := range r.Flags {
		if f.Attr == "" || f.Unsafe == nil || f.Message == "" {
			return fmt.Errorf("knowledge %s: flag entries need attr, unsafe and message", file)
		}
		if f.Severity != "" && f.Severity != "low" && f.Severity != "medium" && f.Severity != "high" && f.Severity != "critical" {
			return fmt.Errorf("knowledge %s: flag %s has invalid severity %q", file, f.Attr, f.Severity)
		}
	}
	return nil
}

// Get returns the knowledge for a resource type, or nil.
func (b *Base) Get(resourceType string) *Resource {
	return b.resources[resourceType]
}

// Types lists known resource types in sorted order.
func (b *Base) Types() []string {
	out := make([]string, 0, len(b.resources))
	for t := range b.resources {
		out = append(out, t)
	}
	sort.Strings(out)
	return out
}

// Top returns the top-level attribute name of a path such as a[0].b.
func Top(path string) string {
	if i := strings.IndexAny(path, ".["); i >= 0 {
		return path[:i]
	}
	return path
}

// LookupPath resolves a path such as point_in_time_recovery[0].enabled against
// an attribute map. The second result is false when any step is missing.
func LookupPath(attrs map[string]any, path string) (any, bool) {
	var cur any = attrs
	for _, step := range splitPath(path) {
		switch cur2 := cur.(type) {
		case map[string]any:
			v, ok := cur2[step]
			if !ok {
				return nil, false
			}
			cur = v
		case []any:
			var i int
			if _, err := fmt.Sscanf(step, "%d", &i); err != nil || i < 0 || i >= len(cur2) {
				return nil, false
			}
			cur = cur2[i]
		default:
			return nil, false
		}
	}
	return cur, true
}

func splitPath(path string) []string {
	var steps []string
	for _, part := range strings.Split(path, ".") {
		for part != "" {
			i := strings.IndexByte(part, '[')
			if i < 0 {
				steps = append(steps, part)
				break
			}
			if i > 0 {
				steps = append(steps, part[:i])
			}
			j := strings.IndexByte(part, ']')
			if j < i {
				break
			}
			steps = append(steps, part[i+1:j])
			part = part[j+1:]
		}
	}
	return steps
}

// Matches reports whether the condition holds for the given attribute map.
func (c Condition) Matches(attrs map[string]any) bool {
	v, ok := LookupPath(attrs, c.Attr)
	switch {
	case c.Absent:
		return !ok || v == nil
	case c.Present:
		return ok && v != nil && v != "" && v != false
	default:
		return ok && equalValue(v, c.Equals)
	}
}

func equalValue(a, b any) bool {
	switch bv := b.(type) {
	case int64:
		if af, ok := a.(float64); ok {
			return af == float64(bv)
		}
	case float64:
		if af, ok := a.(float64); ok {
			return af == bv
		}
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}
