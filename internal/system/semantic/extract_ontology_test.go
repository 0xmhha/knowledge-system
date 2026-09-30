package semantic

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPilotOntologyIsReviewableAndSourceBacked(t *testing.T) {
	source, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "spec-driven", "ontology-pilot.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	repo := t.TempDir()
	gitOutput(t, repo, "init", "-q")
	if err := os.WriteFile(filepath.Join(repo, "ontology.yaml"), source, 0o644); err != nil {
		t.Fatal(err)
	}
	specSource, err := os.ReadFile(filepath.Join("..", "..", "..", "docs", "spec-driven", "spec-pilot.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(repo, "spec.yaml"), specSource, 0o644); err != nil {
		t.Fatal(err)
	}
	gitOutput(t, repo, "add", "ontology.yaml", "spec.yaml")
	gitOutput(t, repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "-qm", "ontology")
	snapshot := Snapshot{ProjectID: "knowledge-system", DatasetID: "pilot-v1", Commit: gitOutput(t, repo, "rev-parse", "HEAD")}
	p, pack, err := ExtractOntology(snapshot, "ontology.yaml", source)
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Concepts) != 20 || len(p.Evidence) != 20 || len(pack.CompetencyQuestions) < 5 {
		t.Fatalf("pilot shape: %d concepts, %d spans, %d questions", len(p.Concepts), len(p.Evidence), len(pack.CompetencyQuestions))
	}
	for _, concept := range p.Concepts {
		if concept.Status != StatusProposed || concept.ReviewedBy != "" {
			t.Fatalf("pilot concept prematurely promoted: %+v", concept)
		}
	}
	if err := p.ValidateSources(context.Background(), repo); err != nil {
		t.Fatalf("source-backed pilot: %v", err)
	}
	spec, _, err := ExtractSpec(snapshot, "spec.yaml", specSource)
	if err != nil {
		t.Fatal(err)
	}
	p.Evidence = append(p.Evidence, spec.Evidence...)
	p.Requirements = spec.Requirements
	if err := p.Validate(); err != nil {
		t.Fatalf("pilot ontology/spec concept links: %v", err)
	}
	if err := p.ValidateSources(context.Background(), repo); err != nil {
		t.Fatalf("source-backed pilot ontology/spec: %v", err)
	}
	tampered := p
	tampered.Concepts = append([]Concept(nil), p.Concepts...)
	tampered.Concepts[0].Definition = "A different definition with the same evidence ID."
	if err := tampered.ValidateSources(context.Background(), repo); err == nil || !strings.Contains(err.Error(), "differs from source") {
		t.Fatalf("rewritten concept passed source verification: %v", err)
	}
	review, err := p.Review(5, "domain-audit")
	if err != nil || review.ConceptProposed != 20 || review.ConceptPrecision != nil || len(review.ConceptSample) != 5 {
		t.Fatalf("pilot review queue: %+v, err=%v", review, err)
	}
	if len(review.AmbiguousTerms) != 0 {
		t.Fatalf("unexpected ambiguous pilot terms: %+v", review.AmbiguousTerms)
	}
	ambiguous := p
	ambiguous.Concepts = append([]Concept(nil), p.Concepts...)
	ambiguous.Concepts[1].Terms = append([]Term(nil), p.Concepts[1].Terms...)
	ambiguous.Concepts[1].Terms = append(ambiguous.Concepts[1].Terms,
		Term{Lang: "en", Value: "project", Preferred: false})
	review, err = ambiguous.Review(5, "domain-audit")
	if err != nil || len(review.AmbiguousTerms) != 1 || review.AmbiguousTerms[0].Normalized != "project" {
		t.Fatalf("ambiguous synonym not surfaced: %+v, err=%v", review.AmbiguousTerms, err)
	}
	broken, _, err := ExtractOntology(snapshot, "ontology.yaml", []byte(strings.Replace(string(source),
		"A named isolation boundary", "A changed isolation boundary", 1)))
	if err != nil {
		t.Fatal(err)
	}
	if err := broken.ValidateSources(context.Background(), repo); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("stale ontology evidence accepted: %v", err)
	}
}

func TestOntologyRejectsUnreviewedVerifiedAndDuplicateTerms(t *testing.T) {
	base := []byte(`version: 1
project_id: sample
domain: product
competency_questions: ['What is a model?']
concepts:
  - id: model
    kind: entity
    definition: A model.
    includes: [Model]
    excludes: [Code]
    terms: [{lang: en, value: model, preferred: true}]
    status: proposed
`)
	snapshot := Snapshot{ProjectID: "sample", DatasetID: "one", Commit: strings.Repeat("a", 40)}
	if _, _, err := ExtractOntology(snapshot, "ontology.yaml", base); err != nil {
		t.Fatalf("valid ontology: %v", err)
	}
	verified := []byte(strings.Replace(string(base), "status: proposed", "status: verified", 1))
	if _, _, err := ExtractOntology(snapshot, "ontology.yaml", verified); err == nil || !strings.Contains(err.Error(), "reviewed_by") {
		t.Fatalf("unreviewed ontology accepted: %v", err)
	}
	duplicate := []byte(strings.Replace(string(base), "preferred: true}]", "preferred: true}, {lang: en, value: MODEL, preferred: false}]", 1))
	if _, _, err := ExtractOntology(snapshot, "ontology.yaml", duplicate); err == nil || !strings.Contains(err.Error(), "repeats term") {
		t.Fatalf("duplicate term accepted: %v", err)
	}
	wrongProject := []byte(strings.Replace(string(base), "project_id: sample", "project_id: other", 1))
	if _, _, err := ExtractOntology(snapshot, "ontology.yaml", wrongProject); err == nil {
		t.Fatal("cross-project ontology accepted")
	}
}
