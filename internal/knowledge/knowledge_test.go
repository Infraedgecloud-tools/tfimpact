package knowledge

import (
	"testing"
)

func TestEmbeddedKnowledgeLoads(t *testing.T) {
	b, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if n := len(b.Types()); n < 50 {
		t.Fatalf("expected at least 50 resource types, got %d", n)
	}
	for _, typ := range []string{"aws_db_instance", "aws_dynamodb_table", "aws_s3_bucket", "aws_iam_role", "aws_security_group"} {
		if b.Get(typ) == nil {
			t.Errorf("missing knowledge for %s", typ)
		}
	}
	db := b.Get("aws_db_instance")
	if !db.HoldsData || len(db.Mitigations) == 0 || len(db.Disruptive) == 0 {
		t.Errorf("aws_db_instance knowledge incomplete: %+v", db)
	}
}

func TestLookupPathAndConditions(t *testing.T) {
	attrs := map[string]any{
		"deletion_protection_enabled": false,
		"point_in_time_recovery":      []any{map[string]any{"enabled": true}},
		"name":                        "sessions",
	}
	if v, ok := LookupPath(attrs, "point_in_time_recovery[0].enabled"); !ok || v != true {
		t.Fatalf("nested lookup failed: %v %v", v, ok)
	}
	if _, ok := LookupPath(attrs, "point_in_time_recovery[1].enabled"); ok {
		t.Fatal("out of range index should not resolve")
	}
	if Top("point_in_time_recovery[0].enabled") != "point_in_time_recovery" {
		t.Fatal("Top failed")
	}
	if !(Condition{Attr: "deletion_protection_enabled", Equals: false}).Matches(attrs) {
		t.Fatal("equals false should match")
	}
	if !(Condition{Attr: "point_in_time_recovery[0].enabled", Equals: true}).Matches(attrs) {
		t.Fatal("nested equals should match")
	}
	if !(Condition{Attr: "final_snapshot_identifier", Absent: true}).Matches(attrs) {
		t.Fatal("absent should match a missing key")
	}
	if (Condition{Attr: "name", Present: true}).Matches(map[string]any{"name": ""}) {
		t.Fatal("empty string is not present")
	}
	if !(Condition{Attr: "backup_retention_period", Equals: int64(0)}).Matches(map[string]any{"backup_retention_period": float64(0)}) {
		t.Fatal("TOML integer should match JSON number")
	}
}

func TestRejectsBadKnowledge(t *testing.T) {
	dir := t.TempDir()
	write := func(name, body string) {
		if err := writeFile(dir, name, body); err != nil {
			t.Fatal(err)
		}
	}
	write("aws_thing.toml", "type = \"aws_thing\"\ndisplay = \"thing\"\nholds_data = true\n")
	if _, err := LoadDir(dir); err == nil {
		t.Fatal("holds_data without data must fail")
	}
	write("aws_thing.toml", "type = \"aws_other\"\ndisplay = \"thing\"\n")
	if _, err := LoadDir(dir); err == nil {
		t.Fatal("file name must match type")
	}
	write("aws_thing.toml", "type = \"aws_thing\"\ndisplay = \"thing\"\nbogus = 1\n")
	if _, err := LoadDir(dir); err == nil {
		t.Fatal("unknown keys must fail")
	}
	write("aws_thing.toml", "type = \"aws_thing\"\ndisplay = \"thing\"\n[[flag]]\nattr = \"x\"\nunsafe = true\nmessage = \"m\"\nseverity = \"urgent\"\n")
	if _, err := LoadDir(dir); err == nil {
		t.Fatal("invalid severity must fail")
	}
}
