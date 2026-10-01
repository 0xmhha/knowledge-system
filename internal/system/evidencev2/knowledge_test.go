package evidencev2

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/knowledgepack"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

func knowledgeVersion(t *testing.T, visibility, statement string, decisionVisibility ...string) (string, string) {
	return buildKnowledgeVersion(t, visibility, statement, false, decisionVisibility...)
}

func buildKnowledgeVersion(t *testing.T, visibility, statement string, withRelation bool, decisionVisibility ...string) (string, string) {
	t.Helper()
	root, version := t.TempDir(), t.TempDir()
	packRoot := filepath.Join(root, "vendor", "decisions")
	overlay := filepath.Join(root, ".cks", "knowledge")
	for _, dir := range []string{packRoot, filepath.Join(overlay, "policies"), filepath.Join(overlay, "decisions"), filepath.Join(overlay, "relations"), filepath.Join(overlay, "trace-links")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	packYAML := `pack_schema_version: 1
pack_id: engineering.decisions
version: 1.0.0
owner: project
scope: fixture
requires: []
concepts: [{id: business-policy, kind: rule, definition: A rule.}, {id: design-decision, kind: entity, definition: A reviewed design choice.}]
relation_types: []
constraints: []
competency_questions: ["Which policy applies?"]
`
	if withRelation {
		packYAML = strings.Replace(packYAML, "relation_types: []", `relation_types:
  - predicate: motivates
    subject_type: {pack_id: engineering.decisions, local_id: business-policy}
    object_type: {pack_id: engineering.decisions, local_id: design-decision}
    direction: forward
    cardinality: many-to-many
    required_evidence: true
    review_rule: human`, 1)
	}
	if err := os.WriteFile(filepath.Join(packRoot, "pack.yaml"), []byte(packYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	digest, err := knowledgepack.DigestTree(packRoot)
	if err != nil {
		t.Fatal(err)
	}
	manifest := `schema_version: 1
project_id: p
selected_packs:
  - pack_id: engineering.decisions
    version: 1.0.0
    source: ./vendor/decisions
    sha256: ` + digest + `
overlay_root: .cks/knowledge
`
	if err := os.WriteFile(filepath.Join(overlay, "manifest.yaml"), []byte(manifest), 0o600); err != nil {
		t.Fatal(err)
	}
	policy := `id: BR-1
type: {pack_id: engineering.decisions, local_id: business-policy}
statement: ` + statement + `
owner: team
scope: {subsystem: transfers}
effective_from: "2026-01-01"
status: verified
reviewed_by: reviewer
review_reason: Compared with the source.
visibility: ` + visibility + `
conflicts_with: []
source_ref: {origin_id: repo, path: .cks/knowledge/policies/BR-1.yaml}
`
	if err := os.WriteFile(filepath.Join(overlay, "policies", "BR-1.yaml"), []byte(policy), 0o600); err != nil {
		t.Fatal(err)
	}
	if len(decisionVisibility) > 0 {
		decision := `---
id: ADR-1
type: {pack_id: engineering.decisions, local_id: design-decision}
problem: Which transfer approval path is justified?
decision: Use independent approval.
rationale: Preserve separation of duties.
alternatives: [Single approver]
assumptions: [Approval service is available]
scope: {subsystem: transfers}
date: "2026-01-01"
status: verified
reviewed_by: reviewer
review_reason: Compared with the policy and alternatives.
visibility: ` + decisionVisibility[0] + `
requirement_ids: [REQ-1]
source_ref: {origin_id: repo, path: .cks/knowledge/decisions/ADR-1.md}
---
# Decision
Independent approval was selected.
`
		if err := os.WriteFile(filepath.Join(overlay, "decisions", "ADR-1.md"), []byte(decision), 0o600); err != nil {
			t.Fatal(err)
		}
		link := `id: TRACE-1
decision_id: ADR-1
requirement_id: REQ-1
criterion_id: AC-1
code_canonical_id: pkg.Alpha
test_canonical_id: pkg.TestAlpha
status: verified
reviewed_by: reviewer
review_reason: Compared with the specification, code, and test.
visibility: public
source_ref: {origin_id: repo, path: .cks/knowledge/trace-links/TRACE-1.yaml}
evidence_refs:
  - {origin_id: repo, path: .cks/knowledge/decisions/ADR-1.md}
`
		if err := os.WriteFile(filepath.Join(overlay, "trace-links", "TRACE-1.yaml"), []byte(link), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if withRelation {
		relation := `id: REL-1
type: {pack_id: engineering.decisions, local_id: motivates}
subject: {id: BR-1, type: {pack_id: engineering.decisions, local_id: business-policy}}
object: {id: ADR-1, type: {pack_id: engineering.decisions, local_id: design-decision}}
status: verified
reviewed_by: reviewer
review_reason: Compared with both source files.
visibility: public
source_ref: {origin_id: repo, path: .cks/knowledge/relations/REL-1.yaml}
evidence_refs:
  - {origin_id: repo, path: .cks/knowledge/policies/BR-1.yaml}
  - {origin_id: repo, path: .cks/knowledge/decisions/ADR-1.md}
`
		if err := os.WriteFile(filepath.Join(overlay, "relations", "REL-1.yaml"), []byte(relation), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	lock, err := knowledgepack.BuildLock(root)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(lock)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(overlay, "knowledge.lock.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("code evidence\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"ontology.yaml": "version: 1\nproject_id: p\ndomain: transfers\ncompetency_questions: [\"How does Alpha work?\"]\nconcepts:\n  - id: alpha\n    kind: entity\n    definition: Alpha behavior\n    includes: [Alpha]\n    excludes: [Beta]\n    terms: [{lang: en, value: Alpha, preferred: true}]\n    status: verified\n    reviewed_by: reviewer\n",
		"spec.yaml":     "version: 1\nproject_id: p\nrequirements:\n  - id: REQ-1\n    version: 1\n    title: Alpha works\n    statement: Alpha responds.\n    status: verified\n    reviewed_by: reviewer\n    concept_ids: [alpha]\n    acceptance_criteria:\n      - id: AC-1\n        given: Alpha exists\n        when: called\n        then: it responds\n",
		"main.go":       "package p\nfunc Alpha() {}\n",
		"main_test.go":  "package p\nfunc TestAlpha() {}\n",
	} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	captured, err := setup.CaptureSource(setup.CaptureOptions{Root: root, Out: version, ProjectID: "p", SourceMode: "snapshot-only",
		ExternalOrigins: []setup.CaptureOrigin{{ID: "knowledge:engineering.decisions", Root: packRoot}}})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := setup.NewDatasetIdentity(captured.Identity, json.RawMessage(`{"model":"mock","dim":8,"checksum":"mock-space"}`), "inputs")
	if err != nil {
		t.Fatal(err)
	}
	for _, side := range []string{"graph", "vector"} {
		dir := filepath.Join(version, side)
		if err := os.Mkdir(dir, 0o700); err != nil {
			t.Fatal(err)
		}
		body, err := json.Marshal(map[string]any{"src_root": root, "graph_digest": strings.Repeat("a", 64), "schema_version": "1.23",
			"embedding_model": "mock", "embedding_dim": 8, "embedding_checksum": "mock-space",
			"project_id": captured.Identity.ProjectID, "snapshot_id": captured.Identity.SnapshotID,
			"dataset_id": identity.DatasetID, "source_mode": captured.Identity.SourceMode,
			"file_manifest_digest":  captured.Identity.FileManifestDigest,
			"capture_policy_digest": captured.Identity.CapturePolicyDigest,
			"sources":               map[string]any{"ckg": map[string]any{"graph_digest": strings.Repeat("a", 64), "src_commit": ""}}})
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "manifest.json"), body, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := setup.PublishCandidateIdentity(version, captured.Identity, "inputs"); err != nil {
		t.Fatal(err)
	}
	return root, version
}

func TestAttachKnowledgeIncludesReviewedArchivedDecision(t *testing.T) {
	root, version := knowledgeVersion(t, "public", "Separate approval is required.", "public")
	base, err := Build(context.Background(), version, "why", []contract.Citation{{File: "README.md", StartLine: 1, EndLine: 1}}, testCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".cks", "knowledge", "decisions", "ADR-1.md"), []byte("changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := AttachKnowledge(context.Background(), base, version, "2026-06-01", "transfers", testCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	k := got.Semantic.(contract.KnowledgeSemanticV2).KnowledgeContext
	if k.State != "partial" || len(k.Decisions) != 1 || k.Decisions[0].ID != "ADR-1" ||
		len(k.Decisions[0].RequirementIDs) != 0 || len(k.Unknowns) == 0 ||
		len(got.Citations) != 3 || !strings.Contains(got.Bodies[2].Text, "Preserve separation of duties") {
		t.Fatalf("reviewed archive ADR missing: %+v", got)
	}
	if err := Verify(got); err != nil {
		t.Fatal(err)
	}
}

func TestAttachKnowledgeCitesReviewedLocalRelationWithoutInventingExternalLinks(t *testing.T) {
	_, version := buildKnowledgeVersion(t, "public", "Separate approval is required.", true, "public")
	base, err := Build(context.Background(), version, "why", []contract.Citation{{File: "README.md", StartLine: 1, EndLine: 1}}, testCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	got, err := AttachKnowledge(context.Background(), base, version, "2026-06-01", "transfers", testCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	k := got.Semantic.(contract.KnowledgeSemanticV2).KnowledgeContext
	coding := got.Semantic.(contract.KnowledgeSemanticV2).CodingContext
	if k.State != "partial" || len(k.Relations) != 1 || k.Relations[0].Predicate != "motivates" ||
		k.Relations[0].SubjectID != "BR-1" || k.Relations[0].ObjectID != "ADR-1" ||
		len(k.RelatedRequirements) != 0 || len(k.TestLinks) != 0 || len(got.Citations) != 4 ||
		k.Relations[0].Citation.File != ".cks/knowledge/relations/REL-1.yaml" {
		t.Fatalf("reviewed relation/citation missing or external link invented: %+v", k)
	}
	if len(coding.ImplementedBehavior) != 0 || len(coding.RequiredBehavior) != 1 ||
		coding.RequiredBehavior[0].ID != "BR-1" || len(coding.Rationale) != 1 ||
		coding.Rationale[0].ID != "ADR-1" || len(coding.Evidence) != len(got.Citations) ||
		!containsString(coding.Unknowns, "implementation_link_unverified") {
		t.Fatalf("coding context conflated source and required behavior: %+v", coding)
	}
	if err := Verify(got); err != nil {
		t.Fatal(err)
	}
	wire, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	var decoded contract.EvidencePackV2
	if err := json.Unmarshal(wire, &decoded); err != nil {
		t.Fatal(err)
	}
	if err := Verify(decoded); err != nil {
		t.Fatalf("v2 wire consumer could not replay coding and knowledge evidence: %v", err)
	}
	legacy := decoded
	legacySemantic := decoded.Semantic.(map[string]any)
	delete(legacySemantic, "coding_context")
	legacy.Semantic = legacySemantic
	if err := Stamp(&legacy); err != nil {
		t.Fatal(err)
	}
	if err := Verify(legacy); err != nil {
		t.Fatalf("pre-coding-context v2 pack lost read compatibility: %v", err)
	}
	tampered := got
	modified := k
	modified.Relations = append([]contract.KnowledgeRelationV2(nil), k.Relations...)
	modified.Relations[0].Predicate = "forged"
	tampered.Semantic = contract.KnowledgeSemanticV2{KnowledgeContext: modified, CodingContext: coding}
	if err := Verify(tampered); err == nil {
		t.Fatal("modified relation kept the v2 integrity hash")
	}
}

func TestAttachKnowledgeUsesReviewedArchivedPolicyWithoutReplacingBase(t *testing.T) {
	root, version := knowledgeVersion(t, "public", "Separate approval is required.")
	base, err := Build(context.Background(), version, "why", []contract.Citation{{File: "README.md", StartLine: 1, EndLine: 1}}, testCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".cks", "knowledge", "policies", "BR-1.yaml"), []byte("changed\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got, err := AttachKnowledge(context.Background(), base, version, "2026-06-01", "transfers", testCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	semantic, ok := got.Semantic.(contract.KnowledgeSemanticV2)
	if !ok || semantic.KnowledgeContext.State != "complete" || len(semantic.KnowledgeContext.ApplicablePolicies) != 1 ||
		len(got.Citations) != 2 || got.Citations[0] != base.Citations[0] || !strings.Contains(got.Bodies[1].Text, "Separate approval") {
		t.Fatalf("archived policy context or base candidate missing: %+v", got)
	}
	if err := Verify(got); err != nil {
		t.Fatal(err)
	}
	got.Semantic = nil
	if err := Verify(got); err == nil {
		t.Fatal("semantic overlay excluded from v2 hash")
	}
}

func TestAttachKnowledgeHidesRestrictedPolicyAndPreservesBase(t *testing.T) {
	_, version := knowledgeVersion(t, "restricted", "Private approval rule.")
	base, err := Build(context.Background(), version, "why", []contract.Citation{{File: "README.md", StartLine: 1, EndLine: 1}}, testCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	got, err := AttachKnowledge(context.Background(), base, version, "2026-06-01", "transfers", testCleaner(t))
	if err != nil {
		t.Fatal(err)
	}
	k := got.Semantic.(contract.KnowledgeSemanticV2).KnowledgeContext
	if k.State != "restricted" || len(k.ApplicablePolicies) != 0 || len(got.Citations) != 1 || len(got.Bodies) != 1 {
		t.Fatalf("restricted policy leaked or base changed: %+v", got)
	}
	if err := Verify(got); err != nil {
		t.Fatal(err)
	}
}
