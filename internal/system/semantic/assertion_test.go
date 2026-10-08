package semantic

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestAssertionsRequireTypedEndpointsProofAndReview(t *testing.T) {
	p, _ := fixture(t)
	p.Assertions = []Assertion{{ID: "a1", Predicate: PredicateSupports, SubjectID: "s1", ObjectID: "c1",
		EvidenceIDs: []string{"e1"}, Status: StatusProposed}}
	if err := p.Validate(); err != nil {
		t.Fatalf("proposed section support: %v", err)
	}
	r, err := p.Review(10, "a")
	if err != nil || r.AssertionProposed != 1 || r.AssertionPrecision != nil || len(r.AssertionSample) != 1 {
		t.Fatalf("assertion queue: %+v, err=%v", r, err)
	}
	checks := []struct {
		name string
		edit func(*Assertion)
		want string
	}{
		{"reversed", func(a *Assertion) { a.SubjectID, a.ObjectID = a.ObjectID, a.SubjectID }, "subject"},
		{"unknown type", func(a *Assertion) { a.Predicate = "RELATED_TO" }, "predicate"},
		{"orphan", func(a *Assertion) { a.EvidenceIDs = []string{"missing"} }, "missing evidence"},
		{"unproved", func(a *Assertion) { a.EvidenceIDs = nil }, "no evidence"},
		{"unreviewed", func(a *Assertion) { a.Status = StatusVerified }, "unverified claim"},
	}
	for _, tc := range checks {
		t.Run(tc.name, func(t *testing.T) {
			broken := p
			broken.Assertions = append([]Assertion(nil), p.Assertions...)
			tc.edit(&broken.Assertions[0])
			if err := broken.Validate(); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("expected %q, got %v", tc.want, err)
			}
		})
	}
	p.Claims[0].Status, p.Claims[0].ReviewedBy = StatusVerified, "reviewer"
	p.Assertions[0].Status, p.Assertions[0].ReviewedBy = StatusVerified, "reviewer"
	if err := p.Validate(); err != nil {
		t.Fatalf("reviewed support: %v", err)
	}
	r, err = p.Review(10, "a")
	if err != nil || r.AssertionVerified != 1 || r.AssertionPrecision == nil || *r.AssertionPrecision != 1 {
		t.Fatalf("reviewed assertion report: %+v, err=%v", r, err)
	}
	p.Assertions[0].ReviewedBy = ""
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "reviewed_by") {
		t.Fatalf("unattributed assertion accepted: %v", err)
	}
}

func TestConceptRelationsRequireSourcesAndCodeAnchor(t *testing.T) {
	p, _ := fixture(t)
	conceptProof := EvidenceSpan{ID: "concept-source", Snapshot: p.Snapshot, Kind: SourceDocument,
		Path: "ontology.yaml", StartLine: 5, EndLine: 12, ContentSHA256: strings.Repeat("a", 64),
		Extractor: "ontology-yaml-v1"}
	codeProof := EvidenceSpan{ID: "code-source", Snapshot: p.Snapshot, Kind: SourceCode,
		Path: "main.go", StartLine: 2, EndLine: 4, ContentSHA256: strings.Repeat("b", 64),
		Extractor: "ckg-ast-v1", CanonicalID: "pkg.Func"}
	p.Evidence = append(p.Evidence, conceptProof, codeProof)
	p.Concepts = []Concept{{ID: "feature", Kind: "entity", Definition: "A source-backed feature.",
		Includes: []string{"Function"}, Excludes: []string{"File"},
		Terms:      []Term{{Lang: "en", Value: "feature", Preferred: true}},
		EvidenceID: conceptProof.ID, Status: StatusProposed}}
	p.Assertions = []Assertion{
		{ID: "about", Predicate: PredicateAbout, SubjectID: "c1", ObjectID: "feature",
			EvidenceIDs: []string{"e1", conceptProof.ID}, Status: StatusProposed},
		{ID: "implemented", Predicate: PredicateImplementedBy, SubjectID: "feature", ObjectID: "pkg.Func",
			EvidenceIDs: []string{conceptProof.ID, codeProof.ID}, Status: StatusProposed},
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("typed proposed relations: %v", err)
	}
	if err := ValidateCodeAnchors(p, filepath.Join(t.TempDir(), "missing")); err == nil || !strings.Contains(err.Error(), "CKG manifest") {
		t.Fatalf("missing graph accepted for code anchor: %v", err)
	}
	broken := p
	broken.Assertions = append([]Assertion(nil), p.Assertions...)
	broken.Assertions[0].SubjectID, broken.Assertions[0].ObjectID = "feature", "c1"
	if err := broken.Validate(); err == nil || !strings.Contains(err.Error(), "claim to concept") {
		t.Fatalf("reversed ABOUT accepted: %v", err)
	}
	broken = p
	broken.Assertions = append([]Assertion(nil), p.Assertions...)
	broken.Assertions[1].ObjectID = "pkg.Missing"
	if err := broken.Validate(); err == nil || !strings.Contains(err.Error(), "matching CKG code anchor") {
		t.Fatalf("unproven implementation accepted: %v", err)
	}
	broken = p
	broken.Assertions = append([]Assertion(nil), p.Assertions...)
	broken.Assertions[1].Status, broken.Assertions[1].ReviewedBy = StatusVerified, "reviewer"
	if err := broken.Validate(); err == nil || !strings.Contains(err.Error(), "unverified concept") {
		t.Fatalf("premature implementation verification accepted: %v", err)
	}
}

func TestContradictionRequiresBothClaimsAndBothSources(t *testing.T) {
	p, _ := fixture(t)
	second := p.Claims[0]
	second.ID = "c2"
	second.Statement = "The switch does not need review"
	p.Claims = append(p.Claims, second)
	p.Assertions = []Assertion{{ID: "conflict", Predicate: PredicateContradicts,
		SubjectID: "c1", ObjectID: "c2", EvidenceIDs: []string{"e1"}, Status: StatusProposed}}
	if err := p.Validate(); err != nil {
		t.Fatalf("proposed conflict: %v", err)
	}
	p.Assertions[0].Status, p.Assertions[0].ReviewedBy = StatusVerified, "reviewer"
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "unverified claim") {
		t.Fatalf("promoted unverified conflict: %v", err)
	}
	for i := range p.Claims {
		p.Claims[i].Status, p.Claims[i].ReviewedBy = StatusVerified, "reviewer"
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("reviewed conflict: %v", err)
	}
	p.Assertions[0].ObjectID = "s1"
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "endpoints must be claims") {
		t.Fatalf("claim-to-section conflict accepted: %v", err)
	}
}
