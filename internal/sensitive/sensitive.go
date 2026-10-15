// Package sensitive masks values that Terraform marks as sensitive and
// computes attribute-level differences between before and after values.
//
// `terraform show -json` writes sensitive values in plain text; the
// before_sensitive and after_sensitive objects only say which leaves to hide.
// Everything tfimpact prints goes through this package.
package sensitive

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// Redacted is what a sensitive value is replaced with.
const Redacted = "(sensitive)"

// Unknown is what a value not known until apply is rendered as.
const Unknown = "(known after apply)"

// alwaysRedact lists attribute names whose values tfimpact never prints even
// when Terraform does not mark them sensitive: they routinely carry secrets.
var alwaysRedact = []string{"password", "secret", "token", "private_key", "user_data", "credential", "api_key", "access_key", "auth"}

// AlwaysRedacted reports whether an attribute name is on the never-print list.
func AlwaysRedacted(attr string) bool {
	l := strings.ToLower(attr)
	for _, needle := range alwaysRedact {
		if strings.Contains(l, needle) {
			return true
		}
	}
	return false
}

// Mask returns value with every leaf marked sensitive replaced by Redacted.
// mask has the same shape as value, with `true` at sensitive leaves.
func Mask(value, mask any) any {
	if b, ok := mask.(bool); ok && b {
		return Redacted
	}
	switch v := value.(type) {
	case map[string]any:
		mm, _ := mask.(map[string]any)
		out := make(map[string]any, len(v))
		for k, child := range v {
			if AlwaysRedacted(k) && child != nil {
				out[k] = Redacted
				continue
			}
			out[k] = Mask(child, mm[k])
		}
		return out
	case []any:
		ms, _ := mask.([]any)
		out := make([]any, len(v))
		for i, child := range v {
			var m any
			if i < len(ms) {
				m = ms[i]
			}
			out[i] = Mask(child, m)
		}
		return out
	default:
		return value
	}
}

// IsSensitive reports whether the mask marks the leaf at path as sensitive,
// where path uses the same steps as plan replace_paths (strings and ints).
func IsSensitive(mask any, path []any) bool {
	cur := mask
	for _, step := range path {
		if isTrue(cur) {
			return true // A sensitive container marks all descendants.
		}
		switch s := step.(type) {
		case string:
			m, ok := cur.(map[string]any)
			if !ok {
				return false
			}
			cur = m[s]
		case int:
			l, ok := cur.([]any)
			if !ok || s < 0 || s >= len(l) {
				return false
			}
			cur = l[s]
		}
	}
	b, ok := cur.(bool)
	return ok && b
}

// Lookup returns the value at path (strings and ints) or nil.
func Lookup(value any, path []any) (any, bool) {
	cur := value
	for _, step := range path {
		switch s := step.(type) {
		case string:
			m, ok := cur.(map[string]any)
			if !ok {
				return nil, false
			}
			cur, ok = m[s]
			if !ok {
				return nil, false
			}
		case int:
			l, ok := cur.([]any)
			if !ok || s < 0 || s >= len(l) {
				return nil, false
			}
			cur = l[s]
		default:
			return nil, false
		}
	}
	return cur, true
}

// AttrChange is one leaf-level difference between before and after.
type AttrChange struct {
	Path      string // dotted path, e.g. ingress[0].cidr_blocks[0]
	Top       string // top-level attribute name
	Before    any    // masked
	After     any    // masked, or Unknown
	Sensitive bool
	Unknown   bool
}

// Diff computes leaf-level changes between before and after, masking sensitive
// leaves and marking leaves that are unknown after apply.
func Diff(before, after, beforeMask, afterMask, afterUnknown any) []AttrChange {
	var out []AttrChange
	diff(nil, before, after, beforeMask, afterMask, afterUnknown, &out)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func diff(path []any, before, after, bm, am, au any, out *[]AttrChange) {
	// Unknown after apply: the after side is absent and afterUnknown is true.
	if b, ok := au.(bool); ok && b {
		// Unknown is a property of the after side, never permission to
		// disclose the prior value. Preserve nested masks when the whole
		// container becomes unknown, and honour either side's sensitivity.
		hidden := isTrue(bm) || isTrue(am) || lastIsRedacted(path)
		shown := Mask(Mask(before, bm), am)
		if hidden {
			shown = Redacted
		}
		*out = append(*out, change(path, shown, Unknown, hidden, true))
		return
	}
	if isTrue(bm) || isTrue(am) || lastIsRedacted(path) {
		if !equal(before, after) {
			*out = append(*out, change(path, Redacted, Redacted, true, false))
		}
		return
	}
	bmap, bIsMap := before.(map[string]any)
	amap, aIsMap := after.(map[string]any)
	if bIsMap || aIsMap {
		keys := map[string]bool{}
		for k := range bmap {
			keys[k] = true
		}
		for k := range amap {
			keys[k] = true
		}
		if um, ok := au.(map[string]any); ok {
			for k := range um {
				keys[k] = true
			}
		}
		for k := range keys {
			bmm, _ := bm.(map[string]any)
			amm, _ := am.(map[string]any)
			aum, _ := au.(map[string]any)
			diff(append(append([]any{}, path...), k), index(bmap, k, bIsMap), index(amap, k, aIsMap), index(bmm, k, bmm != nil), index(amm, k, amm != nil), index(aum, k, aum != nil), out)
		}
		return
	}
	blist, bIsList := before.([]any)
	alist, aIsList := after.([]any)
	if bIsList || aIsList {
		n := max(len(blist), len(alist))
		if ul, ok := au.([]any); ok {
			n = max(n, len(ul))
		}
		for i := 0; i < n; i++ {
			bml, _ := bm.([]any)
			aml, _ := am.([]any)
			aul, _ := au.([]any)
			diff(append(append([]any{}, path...), i), at(blist, i), at(alist, i), at(bml, i), at(aml, i), at(aul, i), out)
		}
		return
	}
	if !equal(before, after) {
		*out = append(*out, change(path, before, after, false, false))
	}
}

func change(path []any, before, after any, sensitive, unknown bool) AttrChange {
	top := ""
	if len(path) > 0 {
		if s, ok := path[0].(string); ok {
			top = s
		}
	}
	return AttrChange{Path: PathString(path), Top: top, Before: before, After: after, Sensitive: sensitive, Unknown: unknown}
}

func index(m map[string]any, k string, present bool) any {
	if !present || m == nil {
		return nil
	}
	return m[k]
}

func at(l []any, i int) any {
	if i < 0 || i >= len(l) {
		return nil
	}
	return l[i]
}

func isTrue(v any) bool { b, ok := v.(bool); return ok && b }

func lastIsRedacted(path []any) bool {
	for i := len(path) - 1; i >= 0; i-- {
		if s, ok := path[i].(string); ok {
			return AlwaysRedacted(s)
		}
	}
	return false
}

func equal(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	ab, err1 := json.Marshal(a)
	bb, err2 := json.Marshal(b)
	return err1 == nil && err2 == nil && string(ab) == string(bb)
}

// PathString renders a path of strings and ints as a.b[0].c.
func PathString(path []any) string {
	var b strings.Builder
	for _, step := range path {
		switch v := step.(type) {
		case string:
			if b.Len() > 0 {
				b.WriteByte('.')
			}
			b.WriteString(v)
		case int:
			b.WriteString("[" + strconv.Itoa(v) + "]")
		case float64:
			b.WriteString("[" + strconv.Itoa(int(v)) + "]")
		default:
			b.WriteString(fmt.Sprintf("[%v]", v))
		}
	}
	return b.String()
}

// Format renders a (masked) value compactly for a message. Long strings are
// truncated so a policy document does not swamp a comment.
func Format(v any) string {
	switch t := v.(type) {
	case nil:
		return "null"
	case string:
		if t == Redacted || t == Unknown {
			return t
		}
		if len(t) > 80 {
			return strconv.Quote(t[:77] + "...")
		}
		return strconv.Quote(t)
	case bool:
		return strconv.FormatBool(t)
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		b, err := json.Marshal(t)
		if err != nil {
			return fmt.Sprintf("%v", t)
		}
		s := string(b)
		if len(s) > 80 {
			return s[:77] + "..."
		}
		return s
	}
}
