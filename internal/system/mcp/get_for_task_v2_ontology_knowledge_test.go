package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
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
	version := newPinnedV2ErrorFixture(t)
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

// newPinnedV2ErrorFixture publishes a valid archive without live engine databases.
// Handler preflight tests use it to isolate source and coordinate rejection.
func newPinnedV2ErrorFixture(t *testing.T) string {
	return newPinnedV2ContentFixture(t, "fixture source\n")
}

func newPinnedV2ContentFixture(t *testing.T, content string) string {
	t.Helper()
	root, version := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "README.md"), []byte(content), 0600); err != nil {
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
	return version
}

func TestV2UsesBudgetedBodiesWithoutExpandingEdgeOnlyCitations(t *testing.T) {
	version := newPinnedV2ContentFixture(t, strings.Repeat("Alpha fixture source\n", 30))
	f := newFixture(t, func(f *fixture) {
		seed := contract.Citation{File: "README.md", StartLine: 1, EndLine: 1}
		f.ckv.SearchHits = []contract.Hit{{Citation: seed, Score: 1, Rank: 1, Symbol: "Alpha", Source: contract.HitSourceCKV}}
		f.ckg.SymbolCitations = []contract.Citation{seed}
		f.fetcher.Bodies[seed.Key()] = "Alpha fixture source\n"
		for i := 2; i <= 25; i++ {
			target := contract.Citation{File: "README.md", StartLine: i, EndLine: i}
			f.ckg.NeighborEdges = append(f.ckg.NeighborEdges, contract.Neighbor{Source: seed, Target: target, Relation: contract.RelationCalls, Distance: 1})
			f.fetcher.Bodies[target.Key()] = fmt.Sprintf("Alpha neighbor %d\n", i)
		}
	})
	legacy, err := f.deps.Composer.Compose(context.Background(), "Alpha")
	if err != nil || len(legacy.Citations) <= 12 || len(legacy.Bodies) > 12 {
		t.Fatalf("fixture did not reproduce edge overflow: refs=%d bodies=%d err=%v", len(legacy.Citations), len(legacy.Bodies), err)
	}
	cleaner, err := sanitize.New(f.ruleset)
	if err != nil {
		t.Fatal(err)
	}
	f.deps.EvidenceVersionDir, f.deps.EvidenceSanitizer = version, cleaner
	for _, knowledge := range []bool{false, true} {
		result, err := handleGetForTaskV2(context.Background(), f.deps, callToolReq(map[string]any{"prompt": "Alpha", "include_knowledge": knowledge, "knowledge_as_of": "2026-10-03", "knowledge_subsystem": "refund"}))
		if err != nil || result.IsError {
			t.Fatalf("edge-only refs broke bounded v2 response (knowledge=%v): %+v %v", knowledge, result, err)
		}
		pack := result.StructuredContent.(contract.EvidencePackV2)
		if len(pack.Citations) != len(legacy.Bodies) || len(pack.Bodies) != len(legacy.Bodies) || len(pack.GraphNeighbors) != 0 {
			t.Fatalf("v2 expanded unbudgeted graph bodies: %+v", pack)
		}
		for i, body := range legacy.Bodies {
			got := pack.Citations[i]
			if got.File != body.Citation.File || got.StartLine != body.Citation.StartLine || got.EndLine != body.Citation.EndLine {
				t.Fatalf("body selection/rank changed at %d", i)
			}
		}
		if err := evidencev2.Verify(pack); err != nil {
			t.Fatal(err)
		}
	}
}
