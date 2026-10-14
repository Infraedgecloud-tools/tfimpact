package iampolicy

import (
	"strings"
	"testing"
)

func mustParse(t *testing.T, doc string) *Policy {
	t.Helper()
	p, err := Parse(doc)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func summaries(ws []Widening) string {
	var b strings.Builder
	for _, w := range ws {
		b.WriteString(w.Severity + ": " + w.Summary + "\n")
	}
	return b.String()
}

func TestWildcardActionAndResource(t *testing.T) {
	before := mustParse(t, `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":["s3:GetObject","s3:PutObject"],"Resource":"arn:aws:s3:::fixture-artifacts/*"}]}`)
	after := mustParse(t, `{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Action":"s3:*","Resource":"*"}]}`)
	got := summaries(Diff(before, after, Identity))
	if !strings.Contains(got, "high: grants `s3:*` on all resources") {
		t.Fatalf("unexpected widenings:\n%s", got)
	}
	if strings.Contains(got, "adds ") {
		t.Fatalf("wildcard should not also be reported as added actions:\n%s", got)
	}
}

func TestTrustSubWidened(t *testing.T) {
	before := mustParse(t, `{"Statement":[{"Effect":"Allow","Action":"sts:AssumeRoleWithWebIdentity","Principal":{"Federated":"arn:aws:iam::123456789012:oidc-provider/token.actions.githubusercontent.com"},"Condition":{"StringEquals":{"token.actions.githubusercontent.com:aud":"sts.amazonaws.com","token.actions.githubusercontent.com:sub":"repo:acme/platform:ref:refs/heads/main"}}}]}`)
	after := mustParse(t, `{"Statement":[{"Effect":"Allow","Action":"sts:AssumeRoleWithWebIdentity","Principal":{"Federated":"arn:aws:iam::123456789012:oidc-provider/token.actions.githubusercontent.com"},"Condition":{"StringEquals":{"token.actions.githubusercontent.com:aud":"sts.amazonaws.com"},"StringLike":{"token.actions.githubusercontent.com:sub":"repo:acme/*"}}}]}`)
	got := summaries(Diff(before, after, Trust))
	if !strings.Contains(got, "widens the `token.actions.githubusercontent.com:sub` condition") {
		t.Fatalf("expected sub widening:\n%s", got)
	}
}

func TestNoSubOnNewFederatedTrust(t *testing.T) {
	after := mustParse(t, `{"Statement":[{"Effect":"Allow","Action":"sts:AssumeRoleWithWebIdentity","Principal":{"Federated":"arn:aws:iam::1:oidc-provider/x"},"Condition":{"StringEquals":{"x:aud":"sts.amazonaws.com"}}}]}`)
	got := summaries(Diff(nil, after, Trust))
	if !strings.Contains(got, "no `sub` condition") {
		t.Fatalf("expected missing-sub finding:\n%s", got)
	}
}

func TestAnyPrincipal(t *testing.T) {
	before := mustParse(t, `{"Statement":[{"Effect":"Allow","Action":"sqs:SendMessage","Resource":"arn:aws:sqs:eu-west-2:1:q","Principal":{"AWS":"arn:aws:iam::1:root"}}]}`)
	after := mustParse(t, `{"Statement":[{"Effect":"Allow","Action":"sqs:SendMessage","Resource":"arn:aws:sqs:eu-west-2:1:q","Principal":"*"}]}`)
	got := summaries(Diff(before, after, Resource))
	if !strings.Contains(got, "critical: allows any principal") {
		t.Fatalf("expected any-principal:\n%s", got)
	}
}

func TestAddedActionsAreLow(t *testing.T) {
	before := mustParse(t, `{"Statement":[{"Effect":"Allow","Action":["s3:GetObject"],"Resource":"arn:aws:s3:::b/*"}]}`)
	after := mustParse(t, `{"Statement":[{"Effect":"Allow","Action":["s3:GetObject","s3:DeleteObject"],"Resource":"arn:aws:s3:::b/*"}]}`)
	got := summaries(Diff(before, after, Identity))
	if got != "low: adds 1 action(s): `s3:DeleteObject`\n" {
		t.Fatalf("unexpected:\n%s", got)
	}
}

// Action names keep their case in the report.
func TestActionCaseIsKept(t *testing.T) {
	before := mustParse(t, `{"Statement":[{"Effect":"Allow","Action":["s3:GetObject"],"Resource":"arn:aws:s3:::b/*"}]}`)
	after := mustParse(t, `{"Statement":[{"Effect":"Allow","Action":["s3:GetObject","s3:PutObject"],"Resource":"arn:aws:s3:::b/*"}]}`)
	got := summaries(Diff(before, after, Identity))
	if !strings.Contains(got, "`s3:PutObject`") || strings.Contains(got, "putobject") {
		t.Fatalf("expected the action in its own case:\n%s", got)
	}
}

// The same action on a broader resource is a widening of scope, and an
// escalation primitive is more than "adds 1 action".
func TestResourceWideningAndEscalation(t *testing.T) {
	before := mustParse(t, `{"Statement":[{"Effect":"Allow","Action":["s3:GetObject"],"Resource":"arn:aws:s3:::b/*"}]}`)
	after := mustParse(t, `{"Statement":[{"Effect":"Allow","Action":["s3:GetObject","iam:PassRole"],"Resource":"arn:aws:s3:::*"}]}`)
	got := summaries(Diff(before, after, Identity))
	for _, want := range []string{
		"medium: widens `s3:GetObject` from `arn:aws:s3:::b/*` to `arn:aws:s3:::*`",
		"medium: adds `iam:PassRole`, which lets this principal hand a role",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("wanted %q in:\n%s", want, got)
		}
	}
	if strings.Contains(got, "adds 1 action") || strings.Contains(got, "adds 2 action") {
		t.Errorf("neither change is a plain added action:\n%s", got)
	}
}

// Resource = "*" is judged per action: a statement that already granted
// other actions on "*" does not make a new one routine.
func TestStarResourceIsPerAction(t *testing.T) {
	before := mustParse(t, `{"Statement":[{"Sid":"Logs","Effect":"Allow","Action":["logs:CreateLogStream"],"Resource":"*"}]}`)
	after := mustParse(t, `{"Statement":[{"Sid":"Logs","Effect":"Allow","Action":["logs:CreateLogStream"],"Resource":"*"},{"Sid":"Data","Effect":"Allow","Action":["dynamodb:Scan"],"Resource":"*"}]}`)
	got := summaries(Diff(before, after, Identity))
	if !strings.Contains(got, "high: grants `dynamodb:Scan` on all resources") {
		t.Fatalf("expected the new star grant to be raised:\n%s", got)
	}
}

// SAML and Cognito trusts scope on their own condition keys; only a
// web-identity trust needs a sub condition.
func TestSAMLAndCognitoTrustsAreNotOIDC(t *testing.T) {
	saml := mustParse(t, `{"Statement":[{"Effect":"Allow","Principal":{"Federated":"arn:aws:iam::123456789012:saml-provider/okta"},"Action":"sts:AssumeRoleWithSAML","Condition":{"StringEquals":{"SAML:aud":"https://signin.aws.amazon.com/saml"}}}]}`)
	cognito := mustParse(t, `{"Statement":[{"Effect":"Allow","Principal":{"Federated":"cognito-identity.amazonaws.com"},"Action":"sts:AssumeRoleWithWebIdentity","Condition":{"StringEquals":{"cognito-identity.amazonaws.com:aud":"eu-west-1:pool"}}}]}`)
	for name, p := range map[string]*Policy{"saml": saml, "cognito": cognito} {
		if got := summaries(Diff(nil, p, Trust)); strings.Contains(got, "no `sub` condition") {
			t.Errorf("%s trust must not be reported as an OIDC trust without sub:\n%s", name, got)
		}
	}
	oidc := mustParse(t, `{"Statement":[{"Effect":"Allow","Principal":{"Federated":"arn:aws:iam::123456789012:oidc-provider/token.actions.githubusercontent.com"},"Action":"sts:AssumeRoleWithWebIdentity","Condition":{"StringEquals":{"token.actions.githubusercontent.com:aud":"sts.amazonaws.com"}}}]}`)
	if got := summaries(Diff(nil, oidc, Trust)); !strings.Contains(got, "no `sub` condition") {
		t.Errorf("an OIDC trust without sub must still be reported:\n%s", got)
	}
}

func TestUnchangedIsSilent(t *testing.T) {
	doc := `{"Statement":[{"Effect":"Allow","Action":["s3:GetObject"],"Resource":"arn:aws:s3:::b/*","Condition":{"StringEquals":{"aws:SourceVpce":"vpce-1"}}}]}`
	if got := Diff(mustParse(t, doc), mustParse(t, doc), Identity); len(got) != 0 {
		t.Fatalf("expected no widenings, got %v", got)
	}
}

func TestRemovedDeny(t *testing.T) {
	before := mustParse(t, `{"Statement":[{"Sid":"A","Effect":"Allow","Action":"s3:GetObject","Resource":"*"},{"Sid":"D","Effect":"Deny","Action":"s3:DeleteBucket","Resource":"*"}]}`)
	after := mustParse(t, `{"Statement":[{"Sid":"A","Effect":"Allow","Action":"s3:GetObject","Resource":"*"}]}`)
	got := summaries(Diff(before, after, Identity))
	if !strings.Contains(got, "removes a Deny statement (Sid D)") {
		t.Fatalf("unexpected:\n%s", got)
	}
}
