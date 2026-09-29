package semantic

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportTextCorpusIsDeterministicAndReviewedOnly(t *testing.T) {
	p, repo := traceFixtureWithRepo(t)
	first := filepath.Join(t.TempDir(), "corpus")
	second := filepath.Join(t.TempDir(), "corpus")
	a := ActiveProjection{projection: p}
	m1, err := a.ExportTextCorpus(first, repo)
	if err != nil {
		t.Fatal(err)
	}
	m2, err := a.ExportTextCorpus(second, repo)
	if err != nil || len(m1.Files) != 2 || len(m2.Files) != 2 || m1.ProjectionSHA256 != m2.ProjectionSHA256 {
		t.Fatalf("nondeterministic reviewed corpus: %+v, %+v, %v", m1, m2, err)
	}
	for i, file := range m1.Files {
		if file.Path != m2.Files[i].Path || file.SHA256 != m2.Files[i].SHA256 {
			t.Fatal("corpus file changed across exports")
		}
		body, err := os.ReadFile(filepath.Join(first, file.Path))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), p.Snapshot.Commit) {
			t.Fatal("source snapshot missing")
		}
	}
	if _, err := a.ExportTextCorpus(first, repo); err == nil {
		t.Fatal("existing corpus overwritten")
	}
	if _, err := a.ExportTextCorpus(filepath.Join(repo, "generated"), repo); err == nil {
		t.Fatal("corpus written inside source tree")
	}
	p.Concepts[0].Status, p.Concepts[0].ReviewedBy = StatusProposed, ""
	p.Requirements[0].Status, p.Requirements[0].ReviewedBy = StatusProposed, ""
	for i := range p.Assertions {
		p.Assertions[i].Status, p.Assertions[i].ReviewedBy = StatusProposed, ""
	}
	m3, err := (ActiveProjection{projection: p}).ExportTextCorpus(filepath.Join(t.TempDir(), "proposed"), repo)
	if err != nil || len(m3.Files) != 0 {
		t.Fatalf("proposed content exported: %+v, %v", m3, err)
	}
}
