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
	t.Helper()
	root, version := t.TempDir(), t.TempDir()
	packRoot := filepath.Join(root, "vendor", "decisions")
	overlay := filepath.Join(root, ".cks", "knowledge")
	for _, dir := range []string{packRoot, filepath.Join(overlay, "policies"), filepath.Join(overlay, "decisions")} {
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
		body, err := json.Marshal(map[string]any{"src_root": root, "graph_digest": "graph", "schema_version": "1.23",
			"embedding_model": "mock", "embedding_dim": 8, "embedding_checksum": "mock-space",
			"project_id": captured.Identity.ProjectID, "snapshot_id": captured.Identity.SnapshotID,
			"dataset_id": identity.DatasetID, "source_mode": captured.Identity.SourceMode,
			"file_manifest_digest":  captured.Identity.FileManifestDigest,
			"capture_policy_digest": captured.Identity.CapturePolicyDigest})
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
