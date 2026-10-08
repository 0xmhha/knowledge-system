package semantic

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const specFixture = `version: 1
project_id: sample
requirements:
  - id: req-filter
    version: 1
    title: Exact filtered retrieval
    statement: A filtered query keeps eligible results.
    status: proposed
    acceptance_criteria:
      - id: ac-filter
        given: Twelve excluded neighbors and two eligible chunks
        when: The query requests two filtered results
        then: Both eligible chunks are returned
  - id: req-empty
    version: 1
    title: No answer
    statement: A query without citations is abstained from.
    status: proposed
    acceptance_criteria:
      - id: ac-empty
        given: No grounded citation
        when: The system composes an answer
        then: It marks the answer as unsupported
`

func TestExtractSpecCommittedEvidenceAndTamperDetection(t *testing.T) {
	root := t.TempDir()
	gitOutput(t, root, "init", "-q")
	if err := os.WriteFile(filepath.Join(root, "spec.yaml"), []byte(specFixture), 0o644); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, root, "add", "spec.yaml")
	gitOutput(t, root, "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "-qm", "fixture")
	snap := Snapshot{ProjectID: "sample", DatasetID: "d1", Commit: gitOutput(t, root, "rev-parse", "HEAD")}
	p, _, err := ExtractSpec(snap, "spec.yaml", []byte(specFixture))
	if err != nil || len(p.Requirements) != 2 || len(p.Evidence) != 4 {
		t.Fatalf("extract: %+v, %v", p, err)
	}
	if err := p.ValidateSources(context.Background(), root); err != nil {
		t.Fatal(err)
	}
	review, err := p.Review(1, "sample")
	if err != nil || review.RequirementTotal != 2 || review.RequirementProposed != 2 || len(review.RequirementSample) != 1 {
		t.Fatalf("spec review queue: %+v, %v", review, err)
	}
	if p.Requirements[0].AcceptanceCriteria[0].EvidenceID == p.Requirements[0].EvidenceID {
		t.Fatal("criterion lost its own source span")
	}
	p.Requirements[0].AcceptanceCriteria[0].Then = "It returns an unrelated chunk"
	if err := p.ValidateSources(context.Background(), root); err == nil || !strings.Contains(err.Error(), "differs from source") {
		t.Fatalf("tampered criterion passed: %v", err)
	}
}

func TestSpecRejectsMalformedAndUnprovenApproval(t *testing.T) {
	snap := Snapshot{ProjectID: "sample", DatasetID: "d1", Commit: strings.Repeat("a", 40)}
	cases := []struct{ name, source string }{
		{"unknown field", strings.Replace(specFixture, "title: Exact", "unknown: x\n    title: Exact", 1)},
		{"duplicate criterion", strings.Replace(specFixture, "id: ac-empty", "id: ac-filter", 1)},
		{"wrong project", strings.Replace(specFixture, "project_id: sample", "project_id: other", 1)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p, _, err := ExtractSpec(snap, "spec.yaml", []byte(tc.source))
			if err == nil {
				err = p.Validate()
			}
			if err == nil {
				t.Fatal("invalid specification accepted")
			}
		})
	}
	p, _, err := ExtractSpec(snap, "spec.yaml", []byte(specFixture))
	if err != nil {
		t.Fatal(err)
	}
	p.Requirements[0].Status = StatusVerified
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "reviewed_by") {
		t.Fatalf("unreviewed approval accepted: %v", err)
	}
	p.Requirements[0].ReviewedBy = "reviewer"
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	p.Requirements[0].ConceptIDs = []string{"missing"}
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "missing concept") {
		t.Fatalf("missing ontology concept accepted: %v", err)
	}
}
