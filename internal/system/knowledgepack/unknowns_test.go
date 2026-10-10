package knowledgepack

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const validUnknown = `id: UNK-1
statement: The recovery objective is not decided.
scope: {subsystem: ops}
recorded_on: "2026-03-01"
resolved_on: ""
match_any: [[recovery, objective], [RTO]]
basis:
  - {path: docs/ops.md, start_line: 2, end_line: 3}
status: verified
reviewed_by: reviewer
review_reason: Compared with docs/ops.md.
visibility: public
source_ref: {origin_id: repo, path: .cks/knowledge/unknowns/UNK-1.yaml}
`

func loadUnknown(t *testing.T, body string) (Instances, error) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "unknowns"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "unknowns", "UNK-1.yaml"), []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return LoadInstances(root, nil)
}

func TestDeclaredUnknownLoadsAndSelects(t *testing.T) {
	instances, err := loadUnknown(t, validUnknown)
	if err != nil || len(instances.Unknowns) != 1 {
		t.Fatalf("valid unknown rejected: %v", err)
	}
	for query, want := range map[string]bool{
		"what rto applies": true, "Recovery and OBJECTIVE": true, "recovery steps": false, "objective only": false,
	} {
		got, restricted, err := instances.SelectDeclaredUnknowns("2026-06-01", map[string]string{"subsystem": "ops"}, query)
		if err != nil || restricted || (len(got) == 1) != want {
			t.Fatalf("%q: got %v restricted %v err %v", query, got, restricted, err)
		}
	}
	for _, c := range []struct {
		asOf  string
		scope map[string]string
	}{{"2026-02-28", map[string]string{"subsystem": "ops"}}, {"2026-06-01", map[string]string{"subsystem": "other"}}} {
		if got, _, err := instances.SelectDeclaredUnknowns(c.asOf, c.scope, "rto"); err != nil || len(got) != 0 {
			t.Fatalf("out-of-range unknown selected: %+v %v", c, err)
		}
	}
	if _, _, err := instances.SelectDeclaredUnknowns("June", nil, "rto"); err == nil {
		t.Fatal("invalid date accepted")
	}
}

func TestDeclaredUnknownRejectsInvalidRecords(t *testing.T) {
	for name, mutate := range map[string][2]string{
		"one-character term":   {"[RTO]]", "[R]]"},
		"padded term":          {"[RTO]]", "[\" RTO\"]]"},
		"empty group":          {"[RTO]]", "[]]"},
		"no basis":             {"  - {path: docs/ops.md, start_line: 2, end_line: 3}\n", ""},
		"escaping basis":       {"docs/ops.md", "../ops.md"},
		"inverted basis":       {"start_line: 2, end_line: 3", "start_line: 3, end_line: 2"},
		"resolved before":      {`resolved_on: ""`, `resolved_on: "2026-01-01"`},
		"bad recorded date":    {`recorded_on: "2026-03-01"`, `recorded_on: "March"`},
		"reviewed without who": {"reviewed_by: reviewer", "reviewed_by: \"\""},
		"foreign source ref":   {"unknowns/UNK-1.yaml}", "unknowns/OTHER.yaml}"},
		"no scope":             {"scope: {subsystem: ops}", "scope: {}"},
	} {
		body := strings.Replace(validUnknown, mutate[0], mutate[1], 1)
		if body == validUnknown {
			t.Fatalf("%s: mutation did not apply", name)
		}
		if _, err := loadUnknown(t, body); err == nil || !strings.Contains(err.Error(), "evidence_unverified") {
			t.Fatalf("%s: invalid unknown accepted: %v", name, err)
		}
	}
}
