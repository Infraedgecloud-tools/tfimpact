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
	if got != "low: adds 1 action(s): `s3:deleteobject`\n" {
		t.Fatalf("unexpected:\n%s", got)
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
