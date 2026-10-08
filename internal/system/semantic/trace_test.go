package semantic

import (
	"strings"
	"testing"
)

func traceFixture(t *testing.T) Projection {
	p, _ := traceFixtureWithRepo(t)
	return p
}

func traceFixtureWithRepo(t *testing.T) (Projection, string) {
	t.Helper()
	p, root := fixture(t)
	proof := func(id string, kind SourceKind, path string, start, end int, canonical, extractor string) EvidenceSpan {
		return EvidenceSpan{ID: id, Snapshot: p.Snapshot, Kind: kind, Path: path,
			StartLine: start, EndLine: end, CanonicalID: canonical, Extractor: extractor,
			ContentSHA256: strings.Repeat("a", 64)}
	}
	p.Evidence = append(p.Evidence,
		proof("ontology", SourceDocument, "ontology.yaml", 1, 5, "", "ontology-yaml-v1"),
		proof("requirement", SourceDocument, "spec.yaml", 1, 12, "", "spec-yaml-v1"),
		proof("criterion", SourceDocument, "spec.yaml", 8, 12, "", "spec-yaml-v1"),
		proof("code", SourceCode, "main.go", 2, 5, "pkg.Alpha", "ckg-ast-v1"),
		proof("test", SourceTest, "main_test.go", 2, 7, "pkg.TestAlpha", "ckg-ast-v1"))
	p.Concepts = []Concept{{ID: "alpha", Kind: "entity", Definition: "Alpha behavior",
		Includes: []string{"Alpha"}, Excludes: []string{"Beta"},
		Terms:      []Term{{Lang: "en", Value: "Alpha", Preferred: true}},
		EvidenceID: "ontology", Status: StatusVerified, ReviewedBy: "reviewer"}}
	p.Requirements = []Requirement{{ID: "req-alpha", Version: 1, Title: "Alpha works", Statement: "Alpha responds.",
		Status: StatusVerified, ReviewedBy: "reviewer", EvidenceID: "requirement", ConceptIDs: []string{"alpha"},
		AcceptanceCriteria: []AcceptanceCriterion{{ID: "ac-alpha", Given: "Alpha exists", When: "called", Then: "it responds", EvidenceID: "criterion"}}}}
	p.Assertions = []Assertion{
		{ID: "impl", Predicate: PredicateImplementedBy, SubjectID: "alpha", ObjectID: "pkg.Alpha",
			EvidenceIDs: []string{"ontology", "code"}, Status: StatusVerified, ReviewedBy: "reviewer"},
		{ID: "tested", Predicate: PredicateTestedBy, SubjectID: "pkg.Alpha", ObjectID: "pkg.TestAlpha",
			EvidenceIDs: []string{"code", "test"}, Status: StatusVerified, ReviewedBy: "reviewer"},
		{ID: "accepted", Predicate: PredicateCheckedBy, SubjectID: "ac-alpha", ObjectID: "pkg.TestAlpha",
			EvidenceIDs: []string{"criterion", "test"}, Status: StatusVerified, ReviewedBy: "reviewer"},
	}
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	return p, root
}

func TestTraceRequiresReviewedSpecCodeTestAndCriterionLinks(t *testing.T) {
	p := traceFixture(t)
	check := func(p Projection, want TraceState) {
		t.Helper()
		r, err := (ActiveProjection{projection: p}).Trace()
		if err != nil || len(r.Requirements) != 1 || r.Requirements[0].State != want {
			t.Fatalf("trace state = %+v, err=%v; want %s", r, err, want)
		}
	}
	check(p, TraceLinked)
	if got := (func() RequirementTrace { r, _ := (ActiveProjection{projection: p}).Trace(); return r.Requirements[0] })(); len(got.Paths) != 1 || len(got.Paths[0].CheckedCriterionIDs) != 1 || len(got.Paths[0].AcceptedCriterionIDs) != 0 {
		t.Fatalf("missing exact path: %+v", got)
	}

	without := p
	without.Assertions = append([]Assertion(nil), p.Assertions[:2]...)
	check(without, TraceMissingAcceptanceTest)
	without.Assertions = append([]Assertion(nil), p.Assertions[:1]...)
	check(without, TraceMissingTest)
	without.Assertions = nil
	check(without, TraceMissingCode)

	without = p
	without.Requirements = append([]Requirement(nil), p.Requirements...)
	without.Requirements[0].Status, without.Requirements[0].ReviewedBy = StatusProposed, ""
	without.Assertions = append([]Assertion(nil), p.Assertions...)
	without.Assertions[2].Status, without.Assertions[2].ReviewedBy = StatusProposed, ""
	check(without, TraceSpecUnapproved)
	without.Requirements[0].Status, without.Requirements[0].ReviewedBy = StatusVerified, "reviewer"
	without.Requirements[0].ConceptIDs = nil
	check(without, TraceMissingConcept)
}

func TestCheckedByMigrationReadsLegacyButRejectsNewAcceptedByWrites(t *testing.T) {
	legacy := traceFixture(t)
	legacy.SchemaVersion = 2
	legacy.Assertions[2].Predicate = PredicateAcceptedBy
	if err := legacy.Validate(); err != nil {
		t.Fatalf("legacy v2 projection rejected: %v", err)
	}
	trace, err := (ActiveProjection{projection: legacy}).Trace()
	if err != nil || len(trace.Requirements[0].Paths[0].AcceptedCriterionIDs) != 1 ||
		len(trace.Requirements[0].Paths[0].CheckedCriterionIDs) != 0 {
		t.Fatalf("legacy trace was rewritten or mistaken for v3: %+v %v", trace, err)
	}
	legacy.SchemaVersion = 3
	if err := legacy.Validate(); err == nil || !strings.Contains(err.Error(), "read-only") {
		t.Fatalf("new ACCEPTED_BY write accepted: %v", err)
	}
	legacy.SchemaVersion = 2
	legacy.Assertions[2].Predicate = PredicateCheckedBy
	if err := legacy.Validate(); err == nil || !strings.Contains(err.Error(), "requires schema 3") {
		t.Fatalf("CHECKED_BY back-written into old projection: %v", err)
	}
}

func TestTraceRejectsFalseTestLinksAndReportsConflict(t *testing.T) {
	p := traceFixture(t)
	p.Assertions[1].EvidenceIDs = []string{"code", "criterion"}
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "TESTED_BY") {
		t.Fatalf("false test edge accepted: %v", err)
	}
	p = traceFixture(t)
	p.Assertions[2].EvidenceIDs = []string{"requirement", "test"}
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "CHECKED_BY") {
		t.Fatalf("criterion omission accepted: %v", err)
	}
	p = traceFixture(t)
	p.Claims[0].Status, p.Claims[0].ReviewedBy = StatusVerified, "reviewer"
	other := p.Claims[0]
	other.ID, other.Statement = "c2", "Alpha does not require review"
	p.Claims = append(p.Claims, other)
	p.Assertions = append(p.Assertions,
		Assertion{ID: "about1", Predicate: PredicateAbout, SubjectID: "c1", ObjectID: "alpha",
			EvidenceIDs: []string{"e1", "ontology"}, Status: StatusVerified, ReviewedBy: "reviewer"},
		Assertion{ID: "about2", Predicate: PredicateAbout, SubjectID: "c2", ObjectID: "alpha",
			EvidenceIDs: []string{"e1", "ontology"}, Status: StatusVerified, ReviewedBy: "reviewer"},
		Assertion{ID: "conflict", Predicate: PredicateContradicts, SubjectID: "c1", ObjectID: "c2",
			EvidenceIDs: []string{"e1"}, Status: StatusVerified, ReviewedBy: "reviewer"})
	r, err := (ActiveProjection{projection: p}).Trace()
	if err != nil || r.Requirements[0].State != TraceConflict || len(r.Requirements[0].ConflictAssertions) != 1 {
		t.Fatalf("reviewed conflict hidden: %+v, %v", r, err)
	}
}
