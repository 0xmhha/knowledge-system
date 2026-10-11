package patch

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/semantic"
)

// These attributed records are a unit fixture, not actual human reviews.
func linkedRunFixture() (semantic.Projection, semantic.TestRun) {
	snapshot := semantic.Snapshot{ProjectID: "p", DatasetID: "d", Commit: strings.Repeat("a", 40)}
	evidence := func(id string, kind semantic.SourceKind, path, canonical string) semantic.EvidenceSpan {
		extractor := "ckg-ast-v1"
		switch path {
		case "ontology.yaml":
			extractor = "ontology-yaml-v1"
		case "spec.yaml":
			extractor = "spec-yaml-v1"
		}
		return semantic.EvidenceSpan{ID: id, Snapshot: snapshot, Kind: kind, Path: path,
			StartLine: 1, EndLine: 3, ContentSHA256: strings.Repeat("a", 64), CanonicalID: canonical, Extractor: extractor}
	}
	edge := func(id, predicate, subject, object string, proof ...string) semantic.Assertion {
		return semantic.Assertion{ID: id, Predicate: predicate, SubjectID: subject, ObjectID: object,
			EvidenceIDs: proof, Status: semantic.StatusVerified, ReviewedBy: "unit-fixture"}
	}
	p := semantic.Projection{SchemaVersion: semantic.SchemaVersion, Snapshot: snapshot,
		Evidence: []semantic.EvidenceSpan{
			evidence("concept-proof", semantic.SourceDocument, "ontology.yaml", ""),
			evidence("requirement-proof", semantic.SourceDocument, "spec.yaml", ""),
			evidence("criterion-proof", semantic.SourceDocument, "spec.yaml", ""),
			evidence("code-proof", semantic.SourceCode, "main.go", "pkg.Alpha"),
			evidence("test-proof", semantic.SourceTest, "main_test.go", "pkg.TestAlpha"),
		},
		Concepts: []semantic.Concept{{ID: "feature", Kind: "rule", Definition: "Source-backed fixture behavior.",
			Includes: []string{"Alpha"}, Excludes: []string{"Unrelated code"}, Terms: []semantic.Term{{Lang: "en", Value: "feature", Preferred: true}},
			EvidenceID: "concept-proof", Status: semantic.StatusVerified, ReviewedBy: "unit-fixture"}},
		Requirements: []semantic.Requirement{{ID: "R", Version: 1, Title: "Fixture requirement", Statement: "Alpha behaves as specified.",
			ConceptIDs: []string{"feature"}, EvidenceID: "requirement-proof", Status: semantic.StatusVerified, ReviewedBy: "unit-fixture",
			AcceptanceCriteria: []semantic.AcceptanceCriterion{{ID: "C", Given: "Alpha input", When: "Alpha runs", Then: "Expected result", EvidenceID: "criterion-proof"}}}},
		Assertions: []semantic.Assertion{
			edge("impl", semantic.PredicateImplementedBy, "feature", "pkg.Alpha", "concept-proof", "code-proof"),
			edge("tested", semantic.PredicateTestedBy, "pkg.Alpha", "pkg.TestAlpha", "code-proof", "test-proof"),
			edge("check", semantic.PredicateCheckedBy, "C", "pkg.TestAlpha", "criterion-proof", "test-proof"),
		},
	}
	argv, _ := json.Marshal([]string{"go", "test", "-json", "-count=1", "-run", "^TestAlpha$", "."})
	digest := sha256.Sum256(argv)
	run := semantic.TestRun{Snapshot: snapshot, CriterionID: "C", TestCanonicalID: "pkg.TestAlpha",
		TestedByAssertions: []string{"tested"}, CheckedAssertions: []string{"check"}, Executable: "go",
		CommandSHA256: hex.EncodeToString(digest[:]), OutputSHA256: strings.Repeat("b", 64),
		CommandPassed: true, SnapshotConsistent: true, ExitCode: 0,
		Framework: "go-test-json", TestName: "TestAlpha", TestObserved: true, TestPassed: true}
	return p, run
}

func TestRunValidationRejectsUnreviewedOrMismatchedProof(t *testing.T) {
	p, run := linkedRunFixture()
	trace, err := p.Trace()
	if err != nil || len(trace.Requirements) != 1 || trace.Requirements[0].State != semantic.TraceLinked {
		t.Fatalf("invalid control fixture: %+v %v", trace, err)
	}
	if err := validateRun(p, "C", run); err != nil {
		t.Fatalf("valid exact linked fixture rejected: %v", err)
	}
	cases := []struct {
		name string
		edit func(*semantic.Projection, *semantic.TestRun)
	}{
		{"unreviewed requirement", func(p *semantic.Projection, _ *semantic.TestRun) { p.Requirements[0].Status = semantic.StatusProposed }},
		{"missing implementation", func(p *semantic.Projection, _ *semantic.TestRun) { p.Assertions = p.Assertions[1:] }},
		{"missing tested relation", func(p *semantic.Projection, _ *semantic.TestRun) {
			p.Assertions = append(p.Assertions[:1], p.Assertions[2:]...)
		}},
		{"missing tested proof", func(_ *semantic.Projection, r *semantic.TestRun) { r.TestedByAssertions = nil }},
		{"foreign tested proof", func(_ *semantic.Projection, r *semantic.TestRun) { r.TestedByAssertions = []string{"unrelated"} }},
		{"extra tested proof", func(_ *semantic.Projection, r *semantic.TestRun) {
			r.TestedByAssertions = []string{"tested", "unrelated"}
		}},
		{"duplicate tested proof", func(_ *semantic.Projection, r *semantic.TestRun) { r.TestedByAssertions = []string{"tested", "tested"} }},
		{"foreign checked proof", func(_ *semantic.Projection, r *semantic.TestRun) { r.CheckedAssertions = []string{"unrelated"} }},
		{"extra checked proof", func(_ *semantic.Projection, r *semantic.TestRun) {
			r.CheckedAssertions = []string{"check", "unrelated"}
		}},
		{"duplicate checked proof", func(_ *semantic.Projection, r *semantic.TestRun) { r.CheckedAssertions = []string{"check", "check"} }},
		{"different observed test", func(_ *semantic.Projection, r *semantic.TestRun) { r.TestName = "TestOther" }},
		{"invalid command digest", func(_ *semantic.Projection, r *semantic.TestRun) { r.CommandSHA256 = "x" }},
		{"different command digest", func(_ *semantic.Projection, r *semantic.TestRun) { r.CommandSHA256 = strings.Repeat("c", 64) }},
		{"invalid output digest", func(_ *semantic.Projection, r *semantic.TestRun) { r.OutputSHA256 = strings.Repeat("z", 64) }},
		{"different executable", func(_ *semantic.Projection, r *semantic.TestRun) { r.Executable = "sh" }},
		{"checked target outside code path", func(p *semantic.Projection, r *semantic.TestRun) {
			proof := p.Evidence[4]
			proof.ID, proof.CanonicalID, proof.Path = "orphan-proof", "pkg.TestOrphan", "orphan_test.go"
			p.Evidence = append(p.Evidence, proof)
			p.Assertions = append(p.Assertions, semantic.Assertion{ID: "orphan-check", Predicate: semantic.PredicateCheckedBy,
				SubjectID: "C", ObjectID: proof.CanonicalID, EvidenceIDs: []string{"criterion-proof", proof.ID},
				Status: semantic.StatusVerified, ReviewedBy: "unit-fixture"})
			r.TestCanonicalID, r.TestName, r.CheckedAssertions = proof.CanonicalID, "TestOrphan", []string{"orphan-check"}
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, run := linkedRunFixture()
			tc.edit(&p, &run)
			if err := validateRun(p, "C", run); err == nil {
				t.Fatal("unreviewed or mismatched execution proof accepted")
			}
		})
	}
}

func TestRunValidationAcceptsCompleteProofSetsAndNestedPackage(t *testing.T) {
	p, run := linkedRunFixture()
	secondTested := p.Assertions[1]
	secondTested.ID = "tested-second"
	secondChecked := p.Assertions[2]
	secondChecked.ID = "check-second"
	p.Assertions = append(p.Assertions, secondTested, secondChecked)
	run.TestedByAssertions = []string{"tested-second", "tested"}
	run.CheckedAssertions = []string{"check-second", "check"}
	p.Evidence[4].Path = "nested/main_test.go"
	argv, _ := json.Marshal([]string{"go", "test", "-json", "-count=1", "-run", "^TestAlpha$", "./nested"})
	digest := sha256.Sum256(argv)
	run.CommandSHA256 = hex.EncodeToString(digest[:])
	if err := validateRun(p, "C", run); err != nil {
		t.Fatalf("complete proof sets for nested package rejected: %v", err)
	}
	// The runner records every matching reviewed assertion; a proper subset
	// must not silently become the report for the full path.
	run.TestedByAssertions = run.TestedByAssertions[:1]
	if err := validateRun(p, "C", run); err == nil {
		t.Fatal("partial reviewed proof set accepted")
	}
}
