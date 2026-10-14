// Package rules turns a plan into findings: what will be lost, interrupted,
// renamed or widened, and which changes are routine.
package rules

import (
	"fmt"
	"sort"
	"strings"

	"github.com/infraedgecloud-tools/tfimpact/internal/findings"
	"github.com/infraedgecloud-tools/tfimpact/internal/iampolicy"
	"github.com/infraedgecloud-tools/tfimpact/internal/knowledge"
	"github.com/infraedgecloud-tools/tfimpact/internal/plan"
	"github.com/infraedgecloud-tools/tfimpact/internal/sensitive"
)

// Rule identifiers. They are part of the public contract: suppressions refer
// to them and they never change meaning.
const (
	RuleDataDestroyed = "PLAN-DATA-001"    // stateful resource destroyed
	RuleDataReplaced  = "PLAN-DATA-002"    // stateful resource replaced
	RuleRename        = "PLAN-RENAME-001"  // rename without a moved block
	RuleReplaced      = "PLAN-REPLACE-001" // other resource replaced
	RuleDeleted       = "PLAN-DELETE-001"  // other resource destroyed
	RuleDisruptive    = "PLAN-DISRUPT-001" // in-place change with a service effect
	RulePolicyWidened = "PLAN-IAM-001"     // IAM policy document widened
	RuleIngressOpened = "PLAN-NET-001"     // ingress opened to the world
	RuleExposure      = "PLAN-EXPOSE-001"  // protection removed or exposure flag set
)

const product = "tfimpact"

// Options tunes the analysis.
type Options struct {
	// NoValues keeps attribute values out of every message; only attribute
	// names are shown. For plans whose values are themselves confidential.
	NoValues bool
}

// Analyze explains the plan using the knowledge base.
func Analyze(p *plan.Plan, kb *knowledge.Base, opts Options) *findings.Result {
	a := &analysis{p: p, kb: kb, opts: opts, res: &findings.Result{TerraformVersion: p.TerraformVersion, FormatVersion: p.FormatVersion}, explained: map[string]bool{}}
	a.run()
	return a.res
}

type analysis struct {
	p         *plan.Plan
	kb        *knowledge.Base
	opts      Options
	res       *findings.Result
	explained map[string]bool // addresses already covered by a rename finding
	flagged   map[string]bool // addresses with at least one finding
}

type change struct {
	rc   plan.ResourceChange
	kind plan.Kind
	diff []sensitive.AttrChange
	tops map[string]bool
	know *knowledge.Resource
}

func (a *analysis) run() {
	a.flagged = map[string]bool{}
	var changes []change
	for _, rc := range a.p.ResourceChanges {
		if !rc.Managed() {
			if rc.Change.Kind() == plan.Read {
				a.res.Counts.Read++
			}
			continue
		}
		a.res.Resources++
		c := change{rc: rc, kind: rc.Change.Kind(), know: a.kb.Get(rc.Type)}
		c.diff = sensitive.Diff(rc.Change.Before, rc.Change.After, rc.Change.BeforeSensitive, rc.Change.AfterSensitive, rc.Change.AfterUnknown)
		c.tops = map[string]bool{}
		for _, d := range c.diff {
			c.tops[d.Top] = true
		}
		a.count(c)
		changes = append(changes, c)
	}
	a.renames(changes)
	for _, c := range changes {
		switch c.kind {
		case plan.Delete:
			a.deleted(c)
		case plan.Replace:
			a.replaced(c)
		case plan.Update:
			a.updated(c)
		case plan.Create:
			a.created(c)
		}
	}
	a.routine(changes)
	a.drift()
	findings.Sort(a.res.Findings)
}

func (a *analysis) count(c change) {
	rc := c.rc
	switch c.kind {
	case plan.Create:
		a.res.Counts.Create++
	case plan.Update:
		a.res.Counts.Update++
	case plan.Delete:
		a.res.Counts.Delete++
	case plan.Replace:
		a.res.Counts.Replace++
	case plan.NoOp:
		a.res.Counts.NoOp++
	case plan.Read:
		a.res.Counts.Read++
	case plan.Forget:
		a.res.Counts.Forget++
	}
	if rc.PreviousAddress != "" && rc.PreviousAddress != rc.Address {
		a.res.Counts.Move++
		a.res.Moves = append(a.res.Moves, findings.Move{From: rc.PreviousAddress, To: rc.Address})
	}
	if rc.Change.Importing != nil {
		a.res.Counts.Import++
	}
	if c.kind == plan.Update && tagOnly(c.tops) {
		a.res.Counts.TagOnly++
	}
}

func tagOnly(tops map[string]bool) bool {
	if len(tops) == 0 {
		return false
	}
	for t := range tops {
		if t != "tags" && t != "tags_all" {
			return false
		}
	}
	return true
}

var defaultIdentity = []string{"name", "bucket", "identifier", "function_name", "table_name", "domain_name", "cluster_identifier", "key_name", "role_name", "user_name", "queue_name", "repository_name", "log_group_name", "topic_name"}

// renames pairs a delete with a create of the same type and identity.
func (a *analysis) renames(changes []change) {
	creates := map[string][]change{}
	for _, c := range changes {
		if c.kind == plan.Create {
			creates[c.rc.Type] = append(creates[c.rc.Type], c)
		}
	}
	for _, del := range changes {
		if del.kind != plan.Delete {
			continue
		}
		identity := defaultIdentity
		if del.know != nil && len(del.know.Identity) > 0 {
			identity = del.know.Identity
		}
		before := del.rc.Change.BeforeMap()
		for _, cr := range creates[del.rc.Type] {
			if a.explained[cr.rc.Address] {
				continue
			}
			after := cr.rc.Change.AfterMap()
			attr, ok := sharedIdentity(before, after, identity)
			if !ok {
				continue
			}
			a.explained[del.rc.Address] = true
			a.explained[cr.rc.Address] = true
			a.renameFinding(del, cr, attr)
			break
		}
	}
}

func sharedIdentity(before, after map[string]any, identity []string) (string, bool) {
	for _, attr := range identity {
		bv, ok1 := before[attr]
		av, ok2 := after[attr]
		if !ok1 || !ok2 || bv == nil || av == nil {
			continue
		}
		if s, ok := bv.(string); ok && s == "" {
			continue
		}
		if fmt.Sprintf("%v", bv) == fmt.Sprintf("%v", av) {
			return attr, true
		}
	}
	return "", false
}

func (a *analysis) renameFinding(del, cr change, attr string) {
	value := sensitive.Format(sensitive.Mask(del.rc.Change.BeforeMap()[attr], lookupMask(del.rc.Change.BeforeSensitive, attr)))
	if a.opts.NoValues {
		value = "same value"
	}
	display := displayName(del)
	sev := findings.High
	loss := ""
	if del.know != nil && del.know.HoldsData {
		sev = findings.Critical
		loss = fmt.Sprintf(" The %s would be destroyed with it, losing %s.", display, del.know.Data)
	}
	why := "it left the configuration"
	switch del.rc.ActionReason {
	case "delete_because_each_key":
		why = "its for_each key changed"
	case "delete_because_count_index":
		why = "its count index changed"
	case "delete_because_no_module":
		why = "its module was removed"
	}
	observed := fmt.Sprintf("`%s` will be destroyed because %s, and `%s` will be created with the same `%s` (%s). Terraform does not know they are the same object.%s",
		del.rc.Address, why, cr.rc.Address, attr, value, loss)
	a.add(findings.Finding{
		RuleID:   RuleRename,
		Severity: sev,
		Title:    fmt.Sprintf("`%s` is renamed to `%s` without a moved block, so it will be destroyed and recreated", del.rc.Address, cr.rc.Address),
		Observed: observed,
		Recommendation: fmt.Sprintf("Add a moved block (`from = %s`, `to = %s`) so Terraform changes the address in state instead. Applying as planned also tries to create a resource with the same %s, which fails for globally named resources.",
			del.rc.Address, cr.rc.Address, attr),
		Domain: "change",
		Path:   ptr(del.rc.Address),
		Tags:   []string{del.rc.Type, "rename"},
		Metadata: map[string]any{
			"resource_type": del.rc.Type,
			"deleted":       del.rc.Address,
			"created":       cr.rc.Address,
			"identity":      attr,
			"action_reason": del.rc.ActionReason,
		},
	})
	a.flagged[del.rc.Address] = true
	a.flagged[cr.rc.Address] = true
}

func lookupMask(mask any, attr string) any {
	m, _ := mask.(map[string]any)
	if m == nil {
		return nil
	}
	return m[attr]
}

func displayName(c change) string {
	if c.know != nil {
		return c.know.Display
	}
	return c.rc.Type
}

func (a *analysis) deleted(c change) {
	if a.explained[c.rc.Address] {
		return
	}
	before := c.rc.Change.BeforeMap()
	why := deleteReason(c.rc.ActionReason)
	if c.know != nil && c.know.HoldsData {
		mit, safe, kept := conditions(c.know, before)
		if kept != "" {
			a.add(findings.Finding{
				RuleID:         RuleDataDestroyed,
				Severity:       findings.Low,
				Title:          fmt.Sprintf("`%s` is removed from Terraform state; %s stays in AWS", c.rc.Address, c.know.Data),
				Observed:       joinSentences(fmt.Sprintf("The %s leaves this configuration%s.", c.know.Display, why), capitalise(kept)+"."),
				Recommendation: "Nothing is deleted, but the resource is no longer managed here. Import it elsewhere or delete it by hand when it is no longer needed.",
				Domain:         "data",
				Path:           ptr(c.rc.Address),
				Tags:           []string{c.rc.Type, "state-removal"},
				Metadata:       map[string]any{"resource_type": c.rc.Type, "action_reason": c.rc.ActionReason, "keeps_data": kept},
			})
			return
		}
		sev := findings.Critical
		if len(safe) > 0 {
			sev = findings.High
		}
		a.add(findings.Finding{
			RuleID:         RuleDataDestroyed,
			Severity:       sev,
			Title:          fmt.Sprintf("`%s` will be destroyed, losing %s", c.rc.Address, c.know.Data),
			Observed:       joinSentences(fmt.Sprintf("The %s is destroyed%s.", c.know.Display, why), c.know.LossNote, sentence(mit), sentence(safe)),
			Recommendation: "If removing it is intended, take a backup or snapshot first and keep it until the change has settled. If it was renamed or moved in code, add a `moved` block instead of recreating it.",
			Domain:         "data",
			Path:           ptr(c.rc.Address),
			Tags:           []string{c.rc.Type, "data-loss", "destroy"},
			Metadata:       map[string]any{"resource_type": c.rc.Type, "action_reason": c.rc.ActionReason, "mitigations": mit, "safeguards": safe},
		})
		return
	}
	sev := findings.Low
	effect := ""
	domain := "change"
	recommendation := "Confirm nothing outside this configuration still depends on it."
	if c.know != nil && c.know.DowntimeOnReplace {
		sev = findings.Medium
		domain = "availability"
		effect = " Anything still pointing at it (DNS names, clients, attachments, dependants in other configurations) stops working."
	}
	if c.know != nil && c.know.Delete != nil {
		if s, ok := findings.Parse(c.know.Delete.Severity); ok {
			sev = s
		}
		effect = " " + capitalise(c.know.Delete.Message) + "."
		domain = "security"
		recommendation = "This removes a control. If that is intended, say who approved it in the change; otherwise keep the resource or move it with a moved block."
	}
	a.add(findings.Finding{
		RuleID:         RuleDeleted,
		Severity:       sev,
		Title:          fmt.Sprintf("`%s` will be destroyed", c.rc.Address),
		Observed:       strings.TrimSpace(fmt.Sprintf("The %s is destroyed%s.%s", displayName(c), why, effect)),
		Recommendation: recommendation,
		Domain:         domain,
		Path:           ptr(c.rc.Address),
		Tags:           []string{c.rc.Type, "destroy"},
		Metadata:       map[string]any{"resource_type": c.rc.Type, "action_reason": c.rc.ActionReason},
	})
}

func deleteReason(reason string) string {
	switch reason {
	case "delete_because_no_resource_config":
		return " because it is no longer in the configuration"
	case "delete_because_no_module":
		return " because its module was removed"
	case "delete_because_wrong_repetition":
		return " because the resource changed between count and for_each"
	case "delete_because_count_index":
		return " because its count index no longer exists"
	case "delete_because_each_key":
		return " because its for_each key no longer exists"
	case "delete_because_no_move_target":
		return " because a moved block pointed at it with no target"
	}
	return ""
}

func (a *analysis) replaced(c change) {
	before, after := c.rc.Change.BeforeMap(), c.rc.Change.AfterMap()
	a.replacement(c, before)
	// A replacement is also a change from before to after. A security group
	// replaced with SSH open to the world, or a policy replaced with a
	// wildcard, used to report only "destroyed and recreated".
	a.transitions(c, before, after)
}

// replacement raises the finding for the destroy-and-create itself.
func (a *analysis) replacement(c change, before map[string]any) {
	reason := a.replaceReason(c)
	order := ""
	if c.rc.Change.CreateBeforeDestroy() {
		order = " The new object is created before the old one is destroyed (create_before_destroy)."
	}
	if c.know != nil && c.know.HoldsData {
		mit, safe, kept := conditions(c.know, before)
		sev := findings.Critical
		if len(safe) > 0 || kept != "" {
			sev = findings.High
		}
		if kept != "" {
			safe = append(safe, kept)
		}
		a.add(findings.Finding{
			RuleID:         RuleDataReplaced,
			Severity:       sev,
			Title:          fmt.Sprintf("`%s` will be destroyed and recreated, losing %s", c.rc.Address, c.know.Data),
			Observed:       joinSentences(reason+order, c.know.LossNote, sentence(mit), sentence(safe)),
			Recommendation: fmt.Sprintf("If the change is intended, take a snapshot or backup first and plan the cutover. If the replacement is a surprise, revert %s. `lifecycle { prevent_destroy = true }` makes Terraform refuse a plan like this.", attrList(c.rc.Change.ReplacePathStrings())),
			Domain:         "data",
			Path:           ptr(c.rc.Address),
			Tags:           []string{c.rc.Type, "data-loss", "replace"},
			Metadata:       map[string]any{"resource_type": c.rc.Type, "replace_paths": c.rc.Change.ReplacePathStrings(), "action_reason": c.rc.ActionReason, "mitigations": mit, "safeguards": safe},
		})
		return
	}
	sev := findings.Medium
	effect := ""
	if c.know != nil && c.know.DowntimeOnReplace {
		sev = findings.High
		effect = " " + capitalise(c.know.Downtime) + "."
	}
	a.add(findings.Finding{
		RuleID:         RuleReplaced,
		Severity:       sev,
		Title:          fmt.Sprintf("`%s` will be destroyed and recreated", c.rc.Address),
		Observed:       strings.TrimSpace(reason + order + effect),
		Recommendation: "Check that the identifiers other systems hold for this resource (ARN, ID, DNS name, IP) are not referenced outside Terraform, or plan a cutover.",
		Domain:         "availability",
		Path:           ptr(c.rc.Address),
		Tags:           []string{c.rc.Type, "replace"},
		Metadata:       map[string]any{"resource_type": c.rc.Type, "replace_paths": c.rc.Change.ReplacePathStrings(), "action_reason": c.rc.ActionReason},
	})
}

func (a *analysis) replaceReason(c change) string {
	switch c.rc.ActionReason {
	case "replace_because_tainted":
		return fmt.Sprintf("The %s is tainted, so Terraform replaces it.", displayName(c))
	case "replace_by_request":
		return fmt.Sprintf("The %s is replaced because `-replace` was requested on the command line.", displayName(c))
	case "replace_by_triggers":
		return fmt.Sprintf("The %s is replaced because a resource in its `replace_triggered_by` changed.", displayName(c))
	}
	paths := c.rc.Change.ReplacePathStrings()
	if len(paths) == 0 {
		return fmt.Sprintf("The %s is replaced.", displayName(c))
	}
	var parts []string
	for _, pth := range paths {
		parts = append(parts, fmt.Sprintf("`%s` (%s)", pth, a.valueChange(c, pth)))
	}
	noun := "cannot be changed"
	if len(paths) == 1 {
		noun = "cannot be changed"
	}
	return fmt.Sprintf("The %s is replaced because %s %s in place.", displayName(c), strings.Join(parts, ", "), noun)
}

// valueChange renders "before -> after" for a dotted path, masked.
func (a *analysis) valueChange(c change, path string) string {
	if a.opts.NoValues {
		return "changed"
	}
	for _, d := range c.diff {
		if d.Path == path {
			return sensitive.Format(d.Before) + " → " + sensitive.Format(d.After)
		}
	}
	return "changed"
}

// conditions evaluates mitigations and safeguards; kept is the message of a
// safeguard that keeps the data entirely (skip_destroy and the like).
func conditions(k *knowledge.Resource, attrs map[string]any) (mitigations, safeguards []string, kept string) {
	for _, m := range k.Mitigations {
		if m.Matches(attrs) {
			mitigations = append(mitigations, m.Message)
		}
	}
	for _, s := range k.Safeguards {
		if s.Matches(attrs) {
			if s.KeepsData {
				kept = s.Message
				continue
			}
			safeguards = append(safeguards, s.Message)
		}
	}
	return
}

// joinSentences joins non-empty sentences with single spaces.
func joinSentences(parts ...string) string {
	var out []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return strings.Join(out, " ")
}

func sentence(parts []string) string {
	if len(parts) == 0 {
		return ""
	}
	return capitalise(strings.Join(parts, "; ")) + "."
}

func capitalise(s string) string {
	if s == "" || s[0] == '`' {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func attrList(paths []string) string {
	if len(paths) == 0 {
		return "the change"
	}
	q := make([]string, len(paths))
	for i, p := range paths {
		q[i] = "`" + p + "`"
	}
	return strings.Join(q, ", ")
}

func (a *analysis) updated(c change) {
	before, after := c.rc.Change.BeforeMap(), c.rc.Change.AfterMap()
	if c.know != nil {
		for _, d := range c.know.Disruptive {
			if !c.tops[d.Attr] {
				continue
			}
			sev := findings.Medium
			if s, ok := findings.Parse(d.Severity); ok {
				sev = s
			}
			a.add(findings.Finding{
				RuleID:         RuleDisruptive,
				Severity:       sev,
				Title:          fmt.Sprintf("`%s`: changing `%s` in place has a service effect", c.rc.Address, d.Attr),
				Observed:       fmt.Sprintf("`%s` changes %s: %s.", d.Attr, a.topChange(c, d.Attr), d.Effect),
				Recommendation: "Apply in a maintenance window or confirm that clients tolerate the interruption.",
				Domain:         "availability",
				Path:           ptr(c.rc.Address),
				Tags:           []string{c.rc.Type, "disruptive", d.Attr},
				Metadata:       map[string]any{"resource_type": c.rc.Type, "attribute": d.Attr},
			})
		}
	}
	a.transitions(c, before, after)
}

// transitions raises what changes between before and after regardless of
// whether Terraform gets there in place or by replacement: a protection
// turned off, a policy widened, an ingress opened.
func (a *analysis) transitions(c change, before, after map[string]any) {
	if c.know != nil {
		for _, f := range c.know.Flags {
			if !c.tops[knowledge.Top(f.Attr)] {
				continue
			}
			av, _ := knowledge.LookupPath(after, f.Attr)
			bv, _ := knowledge.LookupPath(before, f.Attr)
			if equalValue(av, f.Unsafe) && !equalValue(bv, f.Unsafe) {
				a.exposure(c, f, "")
			}
		}
		for _, pa := range c.know.Policies {
			if c.tops[pa.Attr] {
				a.policy(c, pa, before[pa.Attr], after[pa.Attr])
			}
		}
	}
	a.ingress(c)
}

func (a *analysis) created(c change) {
	after := c.rc.Change.AfterMap()
	if c.know != nil {
		for _, f := range c.know.Flags {
			if v, ok := knowledge.LookupPath(after, f.Attr); ok && equalValue(v, f.Unsafe) {
				a.exposure(c, f, "created with ")
			}
		}
		for _, pa := range c.know.Policies {
			a.policy(c, pa, nil, after[pa.Attr])
		}
	}
	a.ingress(c)
}

func (a *analysis) exposure(c change, f knowledge.Flag, prefix string) {
	sev := findings.High
	if s, ok := findings.Parse(f.Severity); ok {
		sev = s
	}
	observed := fmt.Sprintf("`%s` changes %s.", f.Attr, a.pathChange(c, f.Attr))
	if c.kind == plan.Create {
		v, _ := knowledge.LookupPath(c.rc.Change.AfterMap(), f.Attr)
		shown := "(value hidden)"
		if !a.opts.NoValues {
			shown = sensitive.Format(sensitive.Mask(v, nil))
		}
		observed = fmt.Sprintf("`%s` is set to %s on creation.", f.Attr, shown)
	}
	a.add(findings.Finding{
		RuleID:         RuleExposure,
		Severity:       sev,
		Title:          fmt.Sprintf("`%s`: %s", c.rc.Address, f.Message),
		Observed:       observed,
		Recommendation: "Confirm this is intended and recorded in the change; if not, revert it.",
		Domain:         "security",
		Path:           ptr(c.rc.Address),
		Tags:           []string{c.rc.Type, "exposure", knowledge.Top(f.Attr)},
		Metadata:       map[string]any{"resource_type": c.rc.Type, "attribute": f.Attr},
	})
}

// pathChange renders before → after for a possibly nested path.
func (a *analysis) pathChange(c change, path string) string {
	if a.opts.NoValues {
		return "(changed)"
	}
	for _, d := range c.diff {
		if d.Path == path {
			return sensitive.Format(d.Before) + " → " + sensitive.Format(d.After)
		}
	}
	if v, ok := knowledge.LookupPath(c.rc.Change.AfterMap(), path); ok {
		return sensitive.Format(sensitive.Mask(v, nil))
	}
	return a.topChange(c, knowledge.Top(path))
}

func (a *analysis) topChange(c change, top string) string {
	if a.opts.NoValues {
		return "(changed)"
	}
	var parts []string
	for _, d := range c.diff {
		if d.Top == top {
			if d.Path == top {
				if d.Sensitive && !d.Unknown {
					return "(value hidden)"
				}
				return sensitive.Format(d.Before) + " → " + sensitive.Format(d.After)
			}
			parts = append(parts, fmt.Sprintf("`%s` %s → %s", d.Path, sensitive.Format(d.Before), sensitive.Format(d.After)))
		}
	}
	if len(parts) == 0 {
		if c.kind == plan.Create {
			return sensitive.Format(sensitive.Mask(c.rc.Change.AfterMap()[top], lookupMask(c.rc.Change.AfterSensitive, top)))
		}
		return "(changed)"
	}
	if len(parts) > 4 {
		parts = append(parts[:4], fmt.Sprintf("and %d more", len(parts)-4))
	}
	return strings.Join(parts, ", ")
}

func (a *analysis) policy(c change, pa knowledge.PolicyAttr, beforeV, afterV any) {
	if sensitive.IsSensitive(c.rc.Change.AfterSensitive, []any{pa.Attr}) {
		return
	}
	if u, ok := sensitive.Lookup(c.rc.Change.AfterUnknown, []any{pa.Attr}); ok {
		if b, ok := u.(bool); ok && b {
			return // policy not known until apply
		}
	}
	afterDoc, ok := afterV.(string)
	if !ok || afterDoc == "" {
		return
	}
	afterP, err := iampolicy.Parse(afterDoc)
	if err != nil {
		return
	}
	var beforeP *iampolicy.Policy
	if s, ok := beforeV.(string); ok && s != "" {
		beforeP, _ = iampolicy.Parse(s)
	}
	ws := iampolicy.Diff(beforeP, afterP, pa.Kind)
	if beforeP == nil {
		// A new policy: report only material widenings, not every grant.
		var kept []iampolicy.Widening
		for _, w := range ws {
			if w.Severity == "high" || w.Severity == "critical" {
				kept = append(kept, w)
			}
		}
		ws = kept
	}
	if len(ws) == 0 {
		return
	}
	sev := findings.Low
	var lines []string
	for _, w := range ws {
		if s, ok := findings.Parse(w.Severity); ok && s.Rank() > sev.Rank() {
			sev = s
		}
		lines = append(lines, w.Summary)
	}
	kindName := map[string]string{iampolicy.Identity: "policy", iampolicy.Resource: "resource policy", iampolicy.Trust: "trust policy"}[pa.Kind]
	verb := "is widened"
	observed := fmt.Sprintf("`%s` now %s.", pa.Attr, strings.Join(lines, "; "))
	if beforeP == nil {
		verb = "is created with broad access"
		observed = fmt.Sprintf("`%s` %s.", pa.Attr, strings.Join(lines, "; "))
	}
	a.add(findings.Finding{
		RuleID:         RulePolicyWidened,
		Severity:       sev,
		Title:          fmt.Sprintf("`%s`: the %s %s", c.rc.Address, kindName, verb),
		Observed:       observed,
		Recommendation: "Scope actions, resources and principals to what the workload needs and keep the conditions. If the widening is intended, say why in the change.",
		Domain:         "security",
		Path:           ptr(c.rc.Address),
		Tags:           []string{c.rc.Type, "iam", pa.Kind},
		Metadata:       map[string]any{"resource_type": c.rc.Type, "attribute": pa.Attr, "policy_kind": pa.Kind, "widenings": lines},
	})
}

// ingressRule is one normalised ingress permission.
type ingressRule struct {
	proto    string
	from, to int
	cidr     string
}

func (r ingressRule) String() string {
	ports := "all ports"
	switch {
	case r.proto == "-1" || r.proto == "all":
		ports = "all traffic"
	case r.from == 0 && r.to == 65535:
		ports = "all " + r.proto + " ports"
	case r.from == r.to:
		ports = fmt.Sprintf("%s port %d", r.proto, r.from)
	default:
		ports = fmt.Sprintf("%s ports %d-%d", r.proto, r.from, r.to)
	}
	return fmt.Sprintf("%s from %s", ports, r.cidr)
}

// adminPorts are the services whose exposure makes an open ingress critical
// rather than high. Kept as a sorted slice: a range such as 0-10000 covers
// several of them, and the finding must name them in the same order every run.
var adminPorts = []struct {
	port int
	name string
}{
	{22, "SSH"}, {23, "Telnet"}, {445, "SMB"}, {1433, "SQL Server"}, {2049, "NFS"},
	{3306, "MySQL"}, {3389, "RDP"}, {5432, "PostgreSQL"}, {5439, "Redshift"},
	{6379, "Redis"}, {9200, "OpenSearch"}, {11211, "Memcached"}, {27017, "MongoDB"},
}

// sensitiveServices names every administrative or database service the rule
// exposes, in port order, or "every port" when the rule covers them all.
func (r ingressRule) sensitiveServices() []string {
	if r.proto == "-1" || r.proto == "all" || (r.from == 0 && r.to == 65535) {
		return []string{"every port"}
	}
	var out []string
	for _, p := range adminPorts {
		if r.from <= p.port && p.port <= r.to {
			out = append(out, p.name)
		}
	}
	return out
}

func (a *analysis) ingress(c change) {
	before := openIngress(c.rc.Type, c.rc.Change.BeforeMap())
	after := openIngress(c.rc.Type, c.rc.Change.AfterMap())
	if len(after) == 0 {
		return
	}
	bset := map[string]bool{}
	for _, r := range before {
		bset[r.String()] = true
	}
	var opened []ingressRule
	for _, r := range after {
		if !bset[r.String()] {
			opened = append(opened, r)
		}
	}
	if len(opened) == 0 {
		return
	}
	sort.Slice(opened, func(i, j int) bool { return opened[i].String() < opened[j].String() })
	sev := findings.High
	var descs []string
	var services []string
	for _, r := range opened {
		descs = append(descs, r.String())
		if s := r.sensitiveServices(); len(s) > 0 {
			sev = findings.Critical
			services = append(services, s...)
		}
	}
	title := fmt.Sprintf("`%s` opens %s to the internet", c.rc.Address, strings.Join(descs, ", "))
	if len(services) > 0 {
		title = fmt.Sprintf("`%s` opens %s to the internet", c.rc.Address, joinAnd(uniq(services)))
	}
	verb := "gains"
	if c.kind == plan.Create {
		verb = "is created with"
	}
	a.add(findings.Finding{
		RuleID:         RuleIngressOpened,
		Severity:       sev,
		Title:          title,
		Observed:       fmt.Sprintf("The %s %s ingress for %s.", displayName(c), verb, strings.Join(descs, "; ")),
		Recommendation: "Restrict the source to known CIDR ranges or a security group. If public access is intended, limit it to the listening port and say so in the change.",
		Domain:         "security",
		Path:           ptr(c.rc.Address),
		Tags:           []string{c.rc.Type, "network", "public-ingress"},
		Metadata:       map[string]any{"resource_type": c.rc.Type, "opened": descs},
	})
}

// joinAnd writes a list the way a sentence does: "SSH", "SSH and RDP",
// "SSH, RDP and MySQL".
func joinAnd(xs []string) string {
	switch len(xs) {
	case 0:
		return ""
	case 1:
		return xs[0]
	}
	return strings.Join(xs[:len(xs)-1], ", ") + " and " + xs[len(xs)-1]
}

func uniq(xs []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, x := range xs {
		if !seen[x] {
			seen[x] = true
			out = append(out, x)
		}
	}
	return out
}

func isWorld(cidr string) bool { return cidr == "0.0.0.0/0" || cidr == "::/0" }

// openIngress extracts world-open ingress rules from the three security group
// resource shapes. Unknown types return nil.
func openIngress(typ string, attrs map[string]any) []ingressRule {
	if attrs == nil {
		return nil
	}
	var out []ingressRule
	switch typ {
	case "aws_security_group":
		rules, _ := attrs["ingress"].([]any)
		for _, r := range rules {
			m, _ := r.(map[string]any)
			if m == nil {
				continue
			}
			for _, key := range []string{"cidr_blocks", "ipv6_cidr_blocks"} {
				for _, cidr := range strs(m[key]) {
					if isWorld(cidr) {
						out = append(out, ingressRule{proto: proto(m["protocol"]), from: num(m["from_port"]), to: num(m["to_port"]), cidr: cidr})
					}
				}
			}
		}
	case "aws_security_group_rule":
		if attrs["type"] != "ingress" {
			return nil
		}
		for _, key := range []string{"cidr_blocks", "ipv6_cidr_blocks"} {
			for _, cidr := range strs(attrs[key]) {
				if isWorld(cidr) {
					out = append(out, ingressRule{proto: proto(attrs["protocol"]), from: num(attrs["from_port"]), to: num(attrs["to_port"]), cidr: cidr})
				}
			}
		}
	case "aws_vpc_security_group_ingress_rule":
		for _, key := range []string{"cidr_ipv4", "cidr_ipv6"} {
			if cidr, ok := attrs[key].(string); ok && isWorld(cidr) {
				out = append(out, ingressRule{proto: proto(attrs["ip_protocol"]), from: num(attrs["from_port"]), to: num(attrs["to_port"]), cidr: cidr})
			}
		}
	}
	return out
}

func proto(v any) string {
	s, _ := v.(string)
	if s == "" {
		return "all"
	}
	return strings.ToLower(s)
}

func num(v any) int {
	f, _ := v.(float64)
	return int(f)
}

func strs(v any) []string {
	l, _ := v.([]any)
	var out []string
	for _, e := range l {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func equalValue(a, b any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
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

// routine groups the changes that produced no finding.
func (a *analysis) routine(changes []change) {
	type group struct {
		typ   string
		attrs []string
		addrs []string
		vals  map[string]string // attr -> "before → after" if identical across the group
		kind  plan.Kind
	}
	groups := map[string]*group{}
	var order []string
	for _, c := range changes {
		if a.flagged[c.rc.Address] || a.explained[c.rc.Address] {
			continue
		}
		var attrs []string
		var key string
		switch c.kind {
		case plan.Update:
			for t := range c.tops {
				attrs = append(attrs, t)
			}
			sort.Strings(attrs)
			key = c.rc.Type + "|update|" + strings.Join(attrs, ",")
		case plan.Create:
			key = c.rc.Type + "|create"
		case plan.Forget:
			key = c.rc.Type + "|forget"
		default:
			continue
		}
		g, ok := groups[key]
		if !ok {
			g = &group{typ: c.rc.Type, attrs: attrs, vals: map[string]string{}, kind: c.kind}
			for _, t := range attrs {
				g.vals[t] = a.topChange(c, t)
			}
			groups[key] = g
			order = append(order, key)
		} else {
			for _, t := range attrs {
				if g.vals[t] != a.topChange(c, t) {
					g.vals[t] = ""
				}
			}
		}
		g.addrs = append(g.addrs, c.rc.Address)
	}
	sort.Strings(order)
	for _, key := range order {
		g := groups[key]
		sort.Strings(g.addrs)
		var summary string
		switch g.kind {
		case plan.Create:
			summary = "created"
		case plan.Forget:
			summary = "removed from state but not destroyed"
		default:
			if tagOnlySlice(g.attrs) {
				summary = "tags only"
			} else {
				var parts []string
				for _, t := range g.attrs {
					if v := g.vals[t]; scalarSummary(v) {
						parts = append(parts, fmt.Sprintf("`%s` %s", t, v))
					} else {
						parts = append(parts, "`"+t+"`")
					}
				}
				summary = strings.Join(parts, ", ")
			}
		}
		a.res.Routine = append(a.res.Routine, findings.Routine{Type: g.typ, Addresses: g.addrs, Summary: summary})
	}
}

// scalarSummary reports whether a rendered "before → after" is short and
// scalar enough to show in a routine summary line.
func scalarSummary(v string) bool {
	return v != "" && len(v) <= 60 && !strings.ContainsAny(v, "`{[")
}

func tagOnlySlice(attrs []string) bool {
	m := map[string]bool{}
	for _, t := range attrs {
		m[t] = true
	}
	return tagOnly(m)
}

func (a *analysis) drift() {
	for _, rc := range a.p.ResourceDrift {
		d := sensitive.Diff(rc.Change.Before, rc.Change.After, rc.Change.BeforeSensitive, rc.Change.AfterSensitive, rc.Change.AfterUnknown)
		seen := map[string]bool{}
		var changed []string
		for _, ch := range d {
			if !seen[ch.Top] {
				seen[ch.Top] = true
				changed = append(changed, ch.Top)
			}
		}
		sort.Strings(changed)
		if len(changed) == 0 && rc.Change.Kind() == plan.Update {
			continue // refresh noise: nothing visible changed
		}
		a.res.Counts.Drift++
		a.res.Drift = append(a.res.Drift, findings.Drift{Address: rc.Address, Actions: rc.Change.Actions, Changed: changed})
	}
}

func (a *analysis) add(f findings.Finding) {
	f.Product = product
	if f.Metadata == nil {
		f.Metadata = map[string]any{}
	}
	a.res.Findings = append(a.res.Findings, f)
	a.flagged[f.Address()] = true
}

func ptr(s string) *string { return &s }
