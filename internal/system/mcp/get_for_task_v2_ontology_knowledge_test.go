package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/composer/sanitize"
	"github.com/0xmhha/knowledge-system/internal/system/composer/stage2"
	"github.com/0xmhha/knowledge-system/internal/system/evidencev2"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

type unavailableOntologyProvider struct{}

func (unavailableOntologyProvider) Resolve(context.Context) (stage2.OntologyResolver, error) {
	return nil, stage2.ErrOntologyUnavailable
}

func TestV2KnowledgeRequestAcceptsStampedOntologyDiagnostic(t *testing.T) {
	// A core-only retained snapshot isolates the composition boundary: knowledge
	// checks integrity even when it will add no policies. The optional diagnostic
	// must therefore be stamped before entering that layer.
	root, version := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte("fixture source\n"), 0600); err != nil {
		t.Fatal(err)
	}
	captured, err := setup.CaptureSource(setup.CaptureOptions{Root: root, Out: version, ProjectID: "p", SourceMode: "snapshot-only"})
	if err != nil {
		t.Fatal(err)
	}
	identity, err := setup.NewDatasetIdentity(captured.Identity, json.RawMessage(`{"model":"mock","dim":8,"checksum":"mock-space"}`), "inputs")
	if err != nil {
		t.Fatal(err)
	}
	for _, side := range []string{"graph", "vector"} {
		dir := filepath.Join(version, side)
		if err := os.Mkdir(dir, 0755); err != nil {
			t.Fatal(err)
		}
		manifest := map[string]any{"src_root": root, "graph_digest": "graph", "schema_version": "1.23", "embedding_model": "mock", "embedding_dim": 8, "embedding_checksum": "mock-space", "project_id": "p", "snapshot_id": captured.Identity.SnapshotID, "dataset_id": identity.DatasetID, "source_mode": captured.Identity.SourceMode, "file_manifest_digest": captured.Identity.FileManifestDigest, "capture_policy_digest": captured.Identity.CapturePolicyDigest}
		raw, _ := json.Marshal(manifest)
		if err := os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0644); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := setup.PublishCandidateIdentity(version, captured.Identity, "inputs"); err != nil {
		t.Fatal(err)
	}
	f := newFixture(t, nil, stage2.WithOntologyProvider(unavailableOntologyProvider{}, .2, time.Second))
	cleaner, err := sanitize.New(f.ruleset)
	if err != nil {
		t.Fatal(err)
	}
	f.deps.EvidenceVersionDir = version
	f.deps.EvidenceSanitizer = cleaner
	for _, knowledge := range []bool{false, true} {
		result, err := handleGetForTaskV2(context.Background(), f.deps, callToolReq(map[string]any{"prompt": "Alpha", "include_knowledge": knowledge, "knowledge_as_of": "2026-10-03", "knowledge_subsystem": "refund"}))
		if err != nil || result.IsError {
			t.Fatalf("ontology+knowledge=%v failed: %+v %v", knowledge, result, err)
		}
		pack, ok := result.StructuredContent.(contract.EvidencePackV2)
		if !ok || pack.Metadata.Ontology == nil || pack.Metadata.Ontology.Mode != "relations" {
			t.Fatalf("diagnostic lost: %+v", result)
		}
		if err := evidencev2.Verify(pack); err != nil {
			t.Fatal(err)
		}
		pack.Metadata.Ontology.State = "tampered"
		if err := evidencev2.Verify(pack); err == nil {
			t.Fatal("combined-layer diagnostic escaped integrity")
		}
	}
}
