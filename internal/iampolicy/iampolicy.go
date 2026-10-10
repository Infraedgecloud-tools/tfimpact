// Package iampolicy parses IAM policy documents (as the JSON strings that
// appear in plan attributes) and explains how an after document is wider
// than a before document.
package iampolicy

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// Statement is a normalised IAM statement.
type Statement struct {
	Sid           string
	Effect        string
	Actions       []string
	NotActions    []string
	Resources     []string
	NotResources  []string
	Principals    []string // "*" or "Type:identifier"
	NotPrincipals []string
	Conditions    map[string]map[string][]string // operator -> key -> values
}

// Policy is a parsed document.
type Policy struct {
	Statements []Statement
}

// Parse decodes a policy document from its JSON text.
func Parse(doc string) (*Policy, error) {
	var raw struct {
		Statement any `json:"Statement"`
	}
	if err := json.Unmarshal([]byte(doc), &raw); err != nil {
		return nil, fmt.Errorf("policy is not JSON: %w", err)
	}
	var stmts []any
	switch s := raw.Statement.(type) {
	case []any:
		stmts = s
	case map[string]any:
		stmts = []any{s}
	case nil:
		return &Policy{}, nil
	default:
		return nil, fmt.Errorf("unexpected Statement type %T", raw.Statement)
	}
	p := &Policy{}
	for _, s := range stmts {
		m, ok := s.(map[string]any)
		if !ok {
			continue
		}
		st := Statement{
			Sid:           str(m["Sid"]),
			Effect:        str(m["Effect"]),
			Actions:       lower(list(m["Action"])),
			NotActions:    lower(list(m["NotAction"])),
			Resources:     list(m["Resource"]),
			NotResources:  list(m["NotResource"]),
			Principals:    principals(m["Principal"]),
			NotPrincipals: principals(m["NotPrincipal"]),
			Conditions:    conditions(m["Condition"]),
		}
		if st.Effect == "" {
			st.Effect = "Allow"
		}
		p.Statements = append(p.Statements, st)
	}
	return p, nil
}

func str(v any) string { s, _ := v.(string); return s }

func list(v any) []string {
	switch t := v.(type) {
	case string:
		return []string{t}
	case []any:
		out := make([]string, 0, len(t))
		for _, e := range t {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		sort.Strings(out)
		return out
	}
	return nil
}

func lower(in []string) []string {
	out := make([]string, len(in))
	for i, s := range in {
		out[i] = strings.ToLower(s)
	}
	sort.Strings(out)
	return out
}

func principals(v any) []string {
	switch t := v.(type) {
	case string:
		return []string{t}
	case map[string]any:
		var out []string
		for typ, ids := range t {
			for _, id := range list(ids) {
				if id == "*" {
					out = append(out, "*")
				} else {
					out = append(out, typ+":"+id)
				}
			}
		}
		sort.Strings(out)
		return out
	}
	return nil
}

func conditions(v any) map[string]map[string][]string {
	m, ok := v.(map[string]any)
	if !ok {
		return nil
	}
	out := map[string]map[string][]string{}
	for op, keys := range m {
		km, ok := keys.(map[string]any)
		if !ok {
			continue
		}
		out[op] = map[string][]string{}
		for k, vals := range km {
			out[op][k] = list(vals)
		}
	}
	return out
}

// Widening is one way the after policy grants more than the before policy.
type Widening struct {
	Severity string // low, medium, high, critical
	Summary  string
}

// Kind of policy: identity (attached to a role/user/group), resource (bucket,
// queue, key) or trust (assume-role policy).
const (
	Identity = "identity"
	Resource = "resource"
	Trust    = "trust"
)

// Diff lists the ways after is wider than before. before may be nil (a new policy).
func Diff(before, after *Policy, kind string) []Widening {
	var out []Widening
	if after == nil {
		return nil
	}
	if before == nil {
		before = &Policy{}
	}
	bGrants := grants(before)
	bWildActions := map[string]bool{}
	bStarResource := false
	bPrincipals := map[string]bool{}
	for _, s := range before.Statements {
		if s.Effect != "Allow" {
			continue
		}
		for _, a := range s.Actions {
			if isWildcardAction(a) {
				bWildActions[a] = true
			}
		}
		for _, r := range s.Resources {
			if r == "*" {
				bStarResource = true
			}
		}
		for _, p := range s.Principals {
			bPrincipals[p] = true
		}
	}

	var newActions []string
	seenNew := map[string]bool{}
	for _, s := range after.Statements {
		if s.Effect != "Allow" {
			continue
		}
		if len(s.NotActions) > 0 && !hasNotAction(before) {
			out = append(out, Widening{"high", fmt.Sprintf("allows every action except %s (NotAction), which is almost always broader than intended", quoteList(s.NotActions))})
		}
		if len(s.NotResources) > 0 && !hasNotResource(before) {
			out = append(out, Widening{"high", "allows every resource except a list (NotResource)"})
		}
		for _, a := range s.Actions {
			if isWildcardAction(a) && !bWildActions[a] {
				scope := describeResources(s.Resources)
				out = append(out, Widening{"high", fmt.Sprintf("grants `%s` %s", a, scope)})
			}
		}
		starNow := false
		for _, r := range s.Resources {
			if r == "*" {
				starNow = true
			}
		}
		if starNow && !bStarResource && !allWildcard(s.Actions) {
			out = append(out, Widening{"high", fmt.Sprintf("grants %s on all resources (`Resource = \"*\"`)", quoteList(s.Actions))})
		}
		for _, p := range s.Principals {
			if bPrincipals[p] {
				continue
			}
			if p == "*" {
				if hasAccountCondition(s) {
					out = append(out, Widening{"high", "allows any principal (`Principal = \"*\"`) subject to a condition; check the condition keys"})
				} else {
					out = append(out, Widening{"critical", "allows any principal (`Principal = \"*\"`) with no restricting condition"})
				}
			} else {
				verb := "access"
				if kind == Trust {
					verb = "assume this role"
				}
				out = append(out, Widening{"medium", fmt.Sprintf("lets a new principal `%s` %s", p, verb)})
			}
		}
		for _, a := range s.Actions {
			if isWildcardAction(a) {
				continue
			}
			for _, r := range s.Resources {
				if !bGrants[a+"|"+r] && !seenNew[a] && !coveredByWildcard(before, a, r) {
					newActions = append(newActions, a)
					seenNew[a] = true
				}
			}
		}
		if m := match(before, s); m != nil {
			out = append(out, conditionWidenings(m, &s)...)
		} else if kind == Trust && isFederated(s) && !hasSubCondition(s) {
			out = append(out, Widening{"high", "a federated (OIDC) trust statement has no `sub` condition, so any identity from that provider can assume the role"})
		}
	}
	if len(newActions) > 0 {
		sort.Strings(newActions)
		out = append(out, Widening{"low", fmt.Sprintf("adds %d action(s): %s", len(newActions), quoteList(newActions))})
	}
	for _, s := range before.Statements {
		if s.Effect == "Deny" && match(after, s) == nil {
			out = append(out, Widening{"medium", fmt.Sprintf("removes a Deny statement%s", sidSuffix(s))})
		}
	}
	return out
}

func grants(p *Policy) map[string]bool {
	g := map[string]bool{}
	for _, s := range p.Statements {
		if s.Effect != "Allow" {
			continue
		}
		for _, a := range s.Actions {
			for _, r := range s.Resources {
				g[a+"|"+r] = true
			}
			if len(s.Resources) == 0 {
				g[a+"|"] = true
			}
		}
	}
	return g
}

func coveredByWildcard(p *Policy, action, resource string) bool {
	for _, s := range p.Statements {
		if s.Effect != "Allow" {
			continue
		}
		for _, a := range s.Actions {
			if !matchGlob(a, action) {
				continue
			}
			for _, r := range s.Resources {
				if r == "*" || r == resource {
					return true
				}
			}
		}
	}
	return false
}

func matchGlob(pattern, s string) bool {
	if pattern == s {
		return true
	}
	if strings.HasSuffix(pattern, "*") {
		return strings.HasPrefix(s, strings.TrimSuffix(pattern, "*"))
	}
	return false
}

func isWildcardAction(a string) bool { return a == "*" || strings.HasSuffix(a, ":*") }

func allWildcard(actions []string) bool {
	for _, a := range actions {
		if !isWildcardAction(a) {
			return false
		}
	}
	return len(actions) > 0
}

func hasNotAction(p *Policy) bool {
	for _, s := range p.Statements {
		if len(s.NotActions) > 0 {
			return true
		}
	}
	return false
}

func hasNotResource(p *Policy) bool {
	for _, s := range p.Statements {
		if len(s.NotResources) > 0 {
			return true
		}
	}
	return false
}

func describeResources(rs []string) string {
	if len(rs) == 0 {
		return ""
	}
	for _, r := range rs {
		if r == "*" {
			return "on all resources"
		}
	}
	if len(rs) == 1 {
		return "on `" + rs[0] + "`"
	}
	return fmt.Sprintf("on %d resources", len(rs))
}

func quoteList(xs []string) string {
	if len(xs) > 6 {
		return "`" + strings.Join(xs[:6], "`, `") + fmt.Sprintf("` and %d more", len(xs)-6)
	}
	return "`" + strings.Join(xs, "`, `") + "`"
}

func sidSuffix(s Statement) string {
	if s.Sid != "" {
		return " (Sid " + s.Sid + ")"
	}
	return ""
}

func isFederated(s Statement) bool {
	for _, p := range s.Principals {
		if strings.HasPrefix(p, "Federated:") {
			return true
		}
	}
	return false
}

func hasSubCondition(s Statement) bool {
	for _, keys := range s.Conditions {
		for k := range keys {
			if strings.HasSuffix(strings.ToLower(k), ":sub") {
				return true
			}
		}
	}
	return false
}

func hasAccountCondition(s Statement) bool {
	for _, keys := range s.Conditions {
		for k := range keys {
			lk := strings.ToLower(k)
			if strings.Contains(lk, "sourceaccount") || strings.Contains(lk, "sourcearn") || strings.Contains(lk, "principalorgid") || strings.Contains(lk, "sourcevpce") || strings.Contains(lk, "sourceip") {
				return true
			}
		}
	}
	return false
}

// match finds the before statement that corresponds to s: same Sid, else the
// same effect, actions and principals.
func match(p *Policy, s Statement) *Statement {
	for i := range p.Statements {
		b := &p.Statements[i]
		if s.Sid != "" && b.Sid == s.Sid {
			return b
		}
	}
	for i := range p.Statements {
		b := &p.Statements[i]
		if b.Effect == s.Effect && equalStrings(b.Actions, s.Actions) && equalStrings(b.Principals, s.Principals) {
			return b
		}
	}
	return nil
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

var likeOps = map[string]bool{"stringlike": true, "arnlike": true, "stringnotequals": true, "forallvalues:stringlike": true, "foranyvalue:stringlike": true}

func conditionWidenings(before, after *Statement) []Widening {
	var out []Widening
	if after.Effect != "Allow" {
		return nil
	}
	// Keys present before (any operator) and absent after.
	bKeys := map[string]struct {
		op   string
		vals []string
	}{}
	for op, keys := range before.Conditions {
		for k, v := range keys {
			bKeys[strings.ToLower(k)] = struct {
				op   string
				vals []string
			}{op, v}
		}
	}
	aKeys := map[string]struct {
		op   string
		vals []string
	}{}
	for op, keys := range after.Conditions {
		for k, v := range keys {
			aKeys[strings.ToLower(k)] = struct {
				op   string
				vals []string
			}{op, v}
		}
	}
	var removed []string
	for k := range bKeys {
		if _, ok := aKeys[k]; !ok {
			removed = append(removed, k)
		}
	}
	sort.Strings(removed)
	for _, k := range removed {
		out = append(out, Widening{"high", fmt.Sprintf("removes the condition on `%s`%s", k, sidSuffix(*after))})
	}
	keys := make([]string, 0, len(aKeys))
	for k := range aKeys {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		a := aKeys[k]
		b, ok := bKeys[k]
		if !ok {
			continue
		}
		bHasStar, aHasStar := hasStar(b.vals), hasStar(a.vals)
		opWidened := !likeOps[strings.ToLower(b.op)] && likeOps[strings.ToLower(a.op)]
		if (aHasStar && !bHasStar) || (opWidened && aHasStar) {
			out = append(out, Widening{"high", fmt.Sprintf("widens the `%s` condition from %s to %s", k, quoteList(b.vals), quoteList(a.vals))})
			continue
		}
		if len(a.vals) > len(b.vals) && containsAll(a.vals, b.vals) {
			out = append(out, Widening{"medium", fmt.Sprintf("adds %d value(s) to the `%s` condition: %s", len(a.vals)-len(b.vals), k, quoteList(difference(a.vals, b.vals)))})
		}
	}
	return out
}

func hasStar(vals []string) bool {
	for _, v := range vals {
		if strings.Contains(v, "*") {
			return true
		}
	}
	return false
}

func containsAll(sup, sub []string) bool {
	set := map[string]bool{}
	for _, s := range sup {
		set[s] = true
	}
	for _, s := range sub {
		if !set[s] {
			return false
		}
	}
	return true
}

func difference(a, b []string) []string {
	set := map[string]bool{}
	for _, s := range b {
		set[s] = true
	}
	var out []string
	for _, s := range a {
		if !set[s] {
			out = append(out, s)
		}
	}
	return out
}
