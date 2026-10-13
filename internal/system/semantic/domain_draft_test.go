package semantic

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// This validates the authored draft's source/review boundary in a dedicated
// fixture repository. It does not bind the draft to the operating code dataset.
func TestStablenetMeaningDraftPreservesProposedSourceAndMissingTrace(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "projects", "stablenet", "domain-knowledge", "meaning-draft")
	repo := t.TempDir()
	gitOutput(t, repo, "init", "-q")
	for _, name := range []string{"ontology.yaml", "spec.yaml"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	gitOutput(t, repo, "add", "ontology.yaml", "spec.yaml")
	gitOutput(t, repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "-qm", "proposed meaning fixture")
	s := Snapshot{ProjectID: "go-stablenet", DatasetID: "draft-parser-fixture", Commit: gitOutput(t, repo, "rev-parse", "HEAD")}
	ontology, _ := os.ReadFile(filepath.Join(repo, "ontology.yaml"))
	spec, _ := os.ReadFile(filepath.Join(repo, "spec.yaml"))
	p, _, err := ExtractOntology(s, "ontology.yaml", ontology)
	if err != nil {
		t.Fatal(err)
	}
	r, _, err := ExtractSpec(s, "spec.yaml", spec)
	if err != nil {
		t.Fatal(err)
	}
	p.Evidence = append(p.Evidence, r.Evidence...)
	p.Requirements = r.Requirements
	if err := p.Validate(); err != nil {
		t.Fatal(err)
	}
	if err := p.ValidateSources(context.Background(), repo); err != nil {
		t.Fatal(err)
	}
	if len(p.Concepts) != 4 || len(p.Requirements) != 4 || len(p.Evidence) != 13 || len(p.Assertions) != 0 {
		t.Fatal("draft lost source spans or acquired inferred edges")
	}
	for _, c := range p.Concepts {
		if c.Status != StatusProposed || c.ReviewedBy != "" {
			t.Fatal("historical entry review became ontology approval")
		}
	}
	for _, r := range p.Requirements {
		if r.Status != StatusProposed || r.ReviewedBy != "" {
			t.Fatal("draft specification became approved")
		}
	}
	queue, err := p.Review(8, "draft-boundary")
	if err != nil || len(queue.ReviewQueue) != 8 {
		t.Fatal(queue, err)
	}
	for _, item := range queue.ReviewQueue {
		if item.Status != StatusProposed || item.ReviewedBy != "" || item.HoldReason != "awaiting_human_review" {
			t.Fatal("review boundary lost", item)
		}
	}
	trace, err := p.Trace()
	if err != nil || len(trace.Requirements) != 4 {
		t.Fatal(trace, err)
	}
	for _, r := range trace.Requirements {
		if r.State != "spec_unapproved" || len(r.Paths) != 0 {
			t.Fatal("draft implied implemented/verified criteria", r)
		}
	}
	matches, err := p.MatchConcepts("쿼럼", 8)
	if err != nil || len(matches) != 1 || matches[0].Status != StatusProposed {
		t.Fatal(matches, err)
	}
	foreign := s
	foreign.ProjectID = "knowledge-system"
	if _, _, err := ExtractOntology(foreign, "ontology.yaml", ontology); err == nil {
		t.Fatal("foreign project accepted")
	}
}

func TestApprovedStablenetContentDoesNotImplyImplementationOrTests(t *testing.T) {
	dir := filepath.Join("..", "..", "..", "projects", "stablenet", "domain-knowledge", "meaning-approved")
	raw, err := os.ReadFile(filepath.Join(dir, "approval.json"))
	if err != nil {
		t.Fatal(err)
	}
	var approval struct {
		ReviewedBy string `json:"reviewed_by"`
		Binding    struct {
			ProjectID string `json:"actual_project_id"`
		} `json:"project_binding"`
	}
	if err := json.Unmarshal(raw, &approval); err != nil {
		t.Fatal(err)
	}
	if approval.ReviewedBy == "" || approval.Binding.ProjectID == "" {
		t.Fatal("approval identity missing")
	}
	repo := t.TempDir()
	gitOutput(t, repo, "init", "-q")
	for _, name := range []string{"ontology.yaml", "spec.yaml"} {
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(repo, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	gitOutput(t, repo, "add", ".")
	gitOutput(t, repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "-qm", "approved content fixture")
	s := Snapshot{ProjectID: approval.Binding.ProjectID, DatasetID: "approved-parser-fixture", Commit: gitOutput(t, repo, "rev-parse", "HEAD")}
	ontology, _ := os.ReadFile(filepath.Join(repo, "ontology.yaml"))
	spec, _ := os.ReadFile(filepath.Join(repo, "spec.yaml"))
	p, _, err := ExtractOntology(s, "ontology.yaml", ontology)
	if err != nil {
		t.Fatal(err)
	}
	r, _, err := ExtractSpec(s, "spec.yaml", spec)
	if err != nil {
		t.Fatal(err)
	}
	p.Evidence = append(p.Evidence, r.Evidence...)
	p.Requirements = r.Requirements
	if err := p.ValidateSources(context.Background(), repo); err != nil {
		t.Fatal(err)
	}
	if len(p.Assertions) != 0 {
		t.Fatal("content approval invented relationships")
	}
	for _, c := range p.Concepts {
		if c.Status != StatusVerified || c.ReviewedBy != approval.ReviewedBy {
			t.Fatal("content approval lost")
		}
	}
	for _, r := range p.Requirements {
		if r.Status != StatusVerified || r.ReviewedBy != approval.ReviewedBy {
			t.Fatal("spec approval lost")
		}
	}
	trace, err := p.Trace()
	if err != nil || len(trace.Requirements) != 4 {
		t.Fatal(trace, err)
	}
	for _, r := range trace.Requirements {
		if r.State != TraceMissingCode || len(r.Paths) != 0 {
			t.Fatal("approval claimed code/test acceptance", r)
		}
		// The public read selector must preserve peer requirements/evidence
		// while validating the full projection, then return only this item.
		view, found, err := (ActiveProjection{projection: p}).RequirementView(r.RequirementID, 20)
		if err != nil || !found || view.Trace.RequirementID != r.RequirementID || view.Trace.State != TraceMissingCode {
			t.Fatal("multi-requirement read falsely stale", view, found, err)
		}
	}
}
