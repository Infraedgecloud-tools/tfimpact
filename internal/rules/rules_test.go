package rules

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/infraedgecloud-tools/tfimpact/internal/findings"
	"github.com/infraedgecloud-tools/tfimpact/internal/knowledge"
	"github.com/infraedgecloud-tools/tfimpact/internal/plan"
)

// Small synthetic plans for edge cases the moto fixtures do not reach. Real
// provider behaviour is covered by the fixtures under testdata/plans.
func load(t *testing.T, doc string) *plan.Plan {
	t.Helper()
	p, err := plan.Load(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func analyze(t *testing.T, doc string) *findings.Result {
	t.Helper()
	kb, err := knowledge.Load()
	if err != nil {
		t.Fatal(err)
	}
	return Analyze(load(t, doc), kb, Options{})
}

func rc(address, typ string, actions []string, extra string, before, after any) string {
	b, _ := json.Marshal(before)
	a, _ := json.Marshal(after)
	acts, _ := json.Marshal(actions)
	addr, _ := json.Marshal(address)
	return `{"address":` + string(addr) + `,"mode":"managed","type":"` + typ + `","name":"x","provider_name":"registry.terraform.io/hashicorp/aws",` + extra + `"change":{"actions":` + string(acts) + `,"before":` + string(b) + `,"after":` + string(a) + `}}`
}

func wrap(changes ...string) string {
	return `{"format_version":"1.2","terraform_version":"1.15.5","resource_changes":[` + strings.Join(changes, ",") + `]}`
}

func titles(r *findings.Result) string {
	var b strings.Builder
	for _, f := range r.Active() {
		b.WriteString(string(f.Severity) + " " + f.RuleID + " " + f.Title + "\n")
	}
	return b.String()
}

func TestTaintedReplacementUsesReason(t *testing.T) {
	r := analyze(t, wrap(rc("aws_instance.web", "aws_instance", []string{"delete", "create"}, `"action_reason":"replace_because_tainted",`,
		map[string]any{"ami": "ami-1"}, map[string]any{"ami": "ami-1"})))
	got := titles(r)
	if !strings.Contains(got, "high PLAN-REPLACE-001") {
		t.Fatalf("expected a high replacement finding for an EC2 instance:\n%s", got)
	}
	if !strings.Contains(r.Active()[0].Observed, "tainted") {
		t.Fatalf("observed should mention taint: %s", r.Active()[0].Observed)
	}
}

func TestSafeguardLowersDataLoss(t *testing.T) {
	r := analyze(t, wrap(rc("aws_db_instance.a", "aws_db_instance", []string{"delete"}, `"action_reason":"delete_because_no_resource_config",`,
		map[string]any{"identifier": "a", "deletion_protection": true, "skip_final_snapshot": false, "final_snapshot_identifier": "a-final"}, nil)))
	f := r.Active()[0]
	if f.RuleID != RuleDataDestroyed || f.Severity != findings.High {
		t.Fatalf("expected high PLAN-DATA-001 with safeguards, got %s %s", f.Severity, f.RuleID)
	}
	if !strings.Contains(strings.ToLower(f.Observed), "deletion protection is on") || !strings.Contains(f.Observed, "final snapshot") {
		t.Fatalf("safeguards not mentioned: %s", f.Observed)
	}
}

func TestForEachKeyChangeIsARename(t *testing.T) {
	r := analyze(t, wrap(
		rc(`aws_sqs_queue.q["old"]`, "aws_sqs_queue", []string{"delete"}, `"action_reason":"delete_because_each_key",`, map[string]any{"name": "jobs"}, nil),
		rc(`aws_sqs_queue.q["new"]`, "aws_sqs_queue", []string{"create"}, ``, nil, map[string]any{"name": "jobs"}),
	))
	got := titles(r)
	if !strings.Contains(got, "critical PLAN-RENAME-001") {
		t.Fatalf("expected a rename finding:\n%s", got)
	}
	if strings.Contains(got, "PLAN-DATA-001") {
		t.Fatalf("the delete must not also be reported as data loss:\n%s", got)
	}
	if !strings.Contains(r.Active()[0].Observed, "for_each key changed") {
		t.Fatalf("observed should explain the key change: %s", r.Active()[0].Observed)
	}
}

func TestUnknownTypeGetsGenericWording(t *testing.T) {
	r := analyze(t, wrap(rc("aws_made_up_thing.a", "aws_made_up_thing", []string{"delete", "create"}, ``,
		map[string]any{"name": "a", "size": 1.0}, map[string]any{"name": "a", "size": 2.0})))
	f := r.Active()[0]
	if f.RuleID != RuleReplaced || f.Severity != findings.Medium {
		t.Fatalf("got %s %s", f.Severity, f.RuleID)
	}
}

func TestProtectiveDeleteIsRaised(t *testing.T) {
	r := analyze(t, wrap(rc("aws_cloudtrail.main", "aws_cloudtrail", []string{"delete"}, ``, map[string]any{"name": "t"}, nil)))
	f := r.Active()[0]
	if f.RuleID != RuleDeleted || f.Severity != findings.High || !strings.Contains(f.Observed, "logging") {
		t.Fatalf("got %s %s %s", f.Severity, f.RuleID, f.Observed)
	}
}

// A range that covers several administrative ports names all of them, in
// port order, on every run. The services used to come from ranging over a
// map, so the title named a different one each time and the finding's ID
// moved with it.
func TestIngressRangeNamesEveryServiceDeterministically(t *testing.T) {
	doc := wrap(rc("aws_security_group.wide", "aws_security_group", []string{"create"}, ``, nil, map[string]any{"name": "wide", "ingress": []any{
		map[string]any{"from_port": 0.0, "to_port": 10000.0, "protocol": "tcp", "cidr_blocks": []any{"0.0.0.0/0"}, "ipv6_cidr_blocks": []any{}},
	}}))
	want := "critical PLAN-NET-001 `aws_security_group.wide` opens SSH, Telnet, SMB, SQL Server, NFS, MySQL, RDP, PostgreSQL, Redshift, Redis and OpenSearch to the internet"
	for i := 0; i < 20; i++ {
		got := titles(analyze(t, doc))
		if !strings.Contains(got, want) {
			t.Fatalf("run %d: wanted %q in:\n%s", i, want, got)
		}
	}
}

func TestCreateWithPublicIngressAndWildcardPolicy(t *testing.T) {
	r := analyze(t, wrap(
		rc("aws_security_group.web", "aws_security_group", []string{"create"}, ``, nil, map[string]any{"name": "web", "ingress": []any{
			map[string]any{"from_port": 443.0, "to_port": 443.0, "protocol": "tcp", "cidr_blocks": []any{"0.0.0.0/0"}, "ipv6_cidr_blocks": []any{}},
			map[string]any{"from_port": 0.0, "to_port": 65535.0, "protocol": "tcp", "cidr_blocks": []any{"0.0.0.0/0"}, "ipv6_cidr_blocks": []any{}},
		}}),
		rc("aws_iam_role_policy.p", "aws_iam_role_policy", []string{"create"}, ``, nil, map[string]any{"name": "p", "policy": `{"Statement":[{"Effect":"Allow","Action":"*","Resource":"*"}]}`}),
		rc("aws_iam_role_policy.ok", "aws_iam_role_policy", []string{"create"}, ``, nil, map[string]any{"name": "ok", "policy": `{"Statement":[{"Effect":"Allow","Action":["s3:GetObject"],"Resource":"arn:aws:s3:::b/*"}]}`}),
	))
	got := titles(r)
	if !strings.Contains(got, "critical PLAN-NET-001 `aws_security_group.web` opens every port to the internet") {
		t.Fatalf("expected all-ports ingress to be critical:\n%s", got)
	}
	if !strings.Contains(got, "high PLAN-IAM-001 `aws_iam_role_policy.p`") {
		t.Fatalf("expected wildcard policy on create:\n%s", got)
	}
	if strings.Contains(got, "aws_iam_role_policy.ok") {
		t.Fatalf("a scoped new policy must not be reported:\n%s", got)
	}
	if len(r.Routine) != 1 || r.Routine[0].Summary != "created" {
		t.Fatalf("the scoped policy should be routine: %+v", r.Routine)
	}
}

func TestUnknownPolicyValueIsSkipped(t *testing.T) {
	doc := `{"format_version":"1.2","resource_changes":[{"address":"aws_iam_policy.p","mode":"managed","type":"aws_iam_policy","change":{"actions":["update"],"before":{"name":"p","policy":"{}"},"after":{"name":"p"},"after_unknown":{"policy":true}}}]}`
	r := analyze(t, doc)
	if len(r.Active()) != 0 {
		t.Fatalf("a policy unknown until apply must not produce a finding: %s", titles(r))
	}
}

func TestDriftAndMovesAreRecorded(t *testing.T) {
	doc := `{"format_version":"1.2","resource_changes":[{"address":"aws_sqs_queue.b","previous_address":"aws_sqs_queue.a","mode":"managed","type":"aws_sqs_queue","change":{"actions":["no-op"],"before":{"name":"q"},"after":{"name":"q"}}}],"resource_drift":[{"address":"aws_sqs_queue.b","mode":"managed","type":"aws_sqs_queue","change":{"actions":["update"],"before":{"name":"q","visibility_timeout_seconds":30},"after":{"name":"q","visibility_timeout_seconds":120}}}]}`
	r := analyze(t, doc)
	if len(r.Moves) != 1 || r.Moves[0].From != "aws_sqs_queue.a" {
		t.Fatalf("move not recorded: %+v", r.Moves)
	}
	if len(r.Drift) != 1 || r.Drift[0].Changed[0] != "visibility_timeout_seconds" {
		t.Fatalf("drift not recorded: %+v", r.Drift)
	}
	if r.Counts.Move != 1 || r.Counts.Drift != 1 || r.Counts.NoOp != 1 {
		t.Fatalf("counts wrong: %+v", r.Counts)
	}
}

func TestDataSourcesAreOnlyCounted(t *testing.T) {
	doc := `{"format_version":"1.2","resource_changes":[{"address":"data.aws_iam_policy_document.x","mode":"data","type":"aws_iam_policy_document","change":{"actions":["read"],"before":null,"after":{"json":"{}"}}}]}`
	r := analyze(t, doc)
	if r.Counts.Read != 1 || r.Resources != 0 || len(r.Active()) != 0 {
		t.Fatalf("data source handling wrong: %+v %s", r.Counts, titles(r))
	}
}

func TestForgetIsRoutine(t *testing.T) {
	r := analyze(t, wrap(rc("aws_cloudwatch_log_group.l", "aws_cloudwatch_log_group", []string{"forget"}, ``, map[string]any{"name": "l"}, nil)))
	if len(r.Active()) != 0 || r.Counts.Forget != 1 || len(r.Routine) != 1 || !strings.Contains(r.Routine[0].Summary, "not destroyed") {
		t.Fatalf("forget handling wrong: %s %+v %+v", titles(r), r.Counts, r.Routine)
	}
}
