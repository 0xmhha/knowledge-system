package semantic

import (
	"strings"
	"testing"
)

func TestActiveProjectionExposesOnlyVerifiedImplementations(t *testing.T) {
	p, _ := fixture(t)
	conceptEvidence := EvidenceSpan{ID: "ontology-proof", Snapshot: p.Snapshot, Kind: SourceDocument,
		Path: "ontology.yaml", StartLine: 1, EndLine: 4, ContentSHA256: strings.Repeat("a", 64), Extractor: "ontology-yaml-v1"}
	codeEvidence := EvidenceSpan{ID: "code-proof", Snapshot: p.Snapshot, Kind: SourceCode,
		Path: "main.go", StartLine: 2, EndLine: 4, ContentSHA256: strings.Repeat("b", 64),
		Extractor: "ckg-ast-v1", CanonicalID: "pkg.Func"}
	p.Evidence = append(p.Evidence, conceptEvidence, codeEvidence)
	p.Concepts = []Concept{{ID: "feature", Kind: "entity", Definition: "A source-backed feature.",
		Includes: []string{"Function"}, Excludes: []string{"File"},
		Terms:      []Term{{Lang: "en", Value: "feature", Preferred: true}},
		EvidenceID: conceptEvidence.ID, Status: StatusVerified, ReviewedBy: "reviewer"}}
	p.Assertions = []Assertion{{ID: "relation", Predicate: PredicateImplementedBy,
		SubjectID: "feature", ObjectID: "pkg.Func", EvidenceIDs: []string{conceptEvidence.ID, codeEvidence.ID},
		Status: StatusVerified, ReviewedBy: "reviewer"}}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	a := ActiveProjection{projection: p}
	got := a.VerifiedImplementations("feature")
	if len(got) != 1 || got[0].CanonicalID != "pkg.Func" || got[0].Evidence != codeEvidence {
		t.Fatalf("verified implementation: %+v", got)
	}
	p.Assertions[0].Status, p.Assertions[0].ReviewedBy = StatusProposed, ""
	if got := (ActiveProjection{projection: p}).VerifiedImplementations("feature"); len(got) != 0 {
		t.Fatalf("proposed assertion leaked: %+v", got)
	}
	p.Assertions[0].Status, p.Assertions[0].ReviewedBy = StatusVerified, "reviewer"
	p.Concepts[0].Status, p.Concepts[0].ReviewedBy = StatusProposed, ""
	if got := (ActiveProjection{projection: p}).VerifiedImplementations("feature"); len(got) != 0 {
		t.Fatalf("proposed concept leaked: %+v", got)
	}
}
