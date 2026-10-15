package rules

// RuleInfo documents one rule for `tfimpact rules` and the docs.
type RuleInfo struct {
	ID       string
	Domain   string
	Severity string // range a finding of this rule can carry
	Summary  string
}

// Catalogue lists every rule tfimpact can emit, in display order.
var Catalogue = []RuleInfo{
	{"PLAN-COVERAGE-001", "coverage", "low", "A resource type, provider or action combination is unassessed. Generic plan action facts are still reported."},
	{RuleDataDestroyed, "data", "high–critical", "A resource that holds data is destroyed. Names what is lost and which mitigations (final snapshot, deletion protection, recovery window, backups) apply or are missing."},
	{RuleDataReplaced, "data", "high–critical", "A resource that holds data is destroyed and recreated, with the attribute that forced the replacement."},
	{RuleRename, "change", "high–critical", "A resource was renamed in code without a moved block: one address is destroyed and another is created with the same identity."},
	{RuleReplaced, "availability", "medium–high", "A resource without data is destroyed and recreated; high when the type interrupts service (new DNS name, new IP, dependants recreated)."},
	{RuleDeleted, "change", "low–high", "A resource is destroyed; raised when the resource is protective (a trail, a flow log, a public access block, an OIDC provider)."},
	{RuleDisruptive, "availability", "low–high", "An in-place update that still interrupts service: an instance class change, a user_data change that stops and starts the instance, an identifier change that alters an endpoint."},
	{RulePolicyWidened, "security", "low–critical", "An IAM, trust or resource policy document grants more: wildcard actions, Resource \"*\", Principal \"*\", a removed or loosened condition, a federated trust without a sub condition, or added actions (low)."},
	{RulePolicyUnknown, "security", "low", "An IAM policy document is not known until apply (a data source, a template, another resource's attribute), so it was not assessed. The finding exists so a routine line never stands in for a check that did not run."},
	{RuleIngressOpened, "security", "high–critical", "A security group or rule opens ingress to 0.0.0.0/0 or ::/0; critical when the range covers an administrative or database port, or all ports."},
	{RuleExposure, "security", "low–critical", "A protection is turned off or an exposure flag is turned on: public access block, encryption, deletion protection, logging, publicly_accessible, IMDSv1."},
}
