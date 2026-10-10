package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/infraedge-tools/tfimpact/internal/findings"
)

func write(t *testing.T, body string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), ".tfimpact.toml")
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestMissingDefaultIsNil(t *testing.T) {
	c, err := Load(filepath.Join(t.TempDir(), "nope.toml"), false)
	if err != nil || c != nil {
		t.Fatalf("got %v, %v", c, err)
	}
}

func TestRejectsUnknownKeysAndMissingReason(t *testing.T) {
	if _, err := Load(write(t, "version = 1\nfoo = 1\n"), true); err == nil {
		t.Fatal("expected unknown key error")
	}
	if _, err := Load(write(t, "version = 1\n[[suppress]]\nrule = \"PLAN-DATA-001\"\n"), true); err == nil {
		t.Fatal("expected missing reason error")
	}
	if _, err := Load(write(t, "version = 2\n"), true); err == nil {
		t.Fatal("expected version error")
	}
}

func TestApplyWithExpiry(t *testing.T) {
	c, err := Load(write(t, `version = 1
[[suppress]]
rule = "PLAN-DATA-002"
address = "aws_dynamodb_table.*"
reason = "sessions table is rebuilt nightly"
expires = "2026-12-31"
[[suppress]]
rule = "PLAN-NET-001"
reason = "expired"
expires = "2020-01-01"
`), true)
	if err != nil {
		t.Fatal(err)
	}
	addr1, addr2 := "aws_dynamodb_table.sessions", "aws_security_group.bastion"
	fs := []findings.Finding{{RuleID: "PLAN-DATA-002", Path: &addr1}, {RuleID: "PLAN-NET-001", Path: &addr2}}
	c.Apply(fs, time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC))
	if !fs[0].Suppressed || fs[0].SuppressionExpires == nil || *fs[0].SuppressionExpires != "2026-12-31" {
		t.Fatalf("first finding should be suppressed: %+v", fs[0])
	}
	if fs[1].Suppressed {
		t.Fatal("expired suppression must not apply")
	}
}
