package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/0xmhha/knowledge-system/internal/system/composer/sanitize"
	"github.com/0xmhha/knowledge-system/internal/system/semantic"
	mcpserver "github.com/mark3labs/mcp-go/server"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSemanticStatusIsRegistered(t *testing.T) {
	s := mcpserver.NewMCPServer("status-test", "test")
	if err := Register(s, newFixture(t, nil).deps); err != nil {
		t.Fatal(err)
	}
	tool := s.GetTool(toolName("semantic.status"))
	if tool == nil {
		t.Fatal("ontology preparation status is not discoverable through MCP tools/list")
	}
	for _, field := range []string{"state", "mode", "availability", "read_tools_ready", "retrieval_ready"} {
		if _, ok := tool.Tool.OutputSchema.Properties[field]; !ok {
			t.Errorf("status output schema lacks %s", field)
		}
	}
}

// Document-only fixtures use production capture/store/retained validators.
// They contain no code anchors, model vectors, real reviews or activation.
func semanticReadFixture(t *testing.T, reviewed bool) Deps {
	return semanticReadFixtureN(t, reviewed, false)
}
func semanticReadFixtureN(t *testing.T, reviewed, ambiguous bool) Deps {
	t.Helper()
	source, version := t.TempDir(), t.TempDir()
	statusLine := "status: proposed"
	if reviewed {
		statusLine = "status: verified\n    reviewed_by: owned-fixture-label"
	}
	body := []byte(fmt.Sprintf(`version: 1
project_id: status-fixture
domain: owned fixture
competency_questions: ["Which feature?"]
concepts:
  - id: feature
    kind: entity
    definition: Fixture feature.
    includes: [fixture]
    excludes: [quality]
    terms: [{lang: en, value: feature, preferred: true}]
    %s
`, statusLine))
	if ambiguous {
		body = append(body, []byte(`  - id: feature-alt
    kind: entity
    definition: Alternative fixture meaning.
    includes: [fixture]
    excludes: [quality]
    terms: [{lang: en, value: feature, preferred: true}]
    status: proposed
`)...)
	}
	specBody := []byte(`version: 1
project_id: status-fixture
requirements:
  - id: req-feature
    version: 3
    title: Read feature
    statement: The fixture feature can be inspected.
    concept_ids: [feature]
    status: proposed
    acceptance_criteria:
      - id: ac-feature
        given: Captured meaning
        when: It is read
        then: Review status is retained
`)
	if err := os.WriteFile(filepath.Join(source, "spec.yaml"), specBody, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "ontology.yaml"), body, 0600); err != nil {
		t.Fatal(err)
	}
	captured, err := setup.CaptureSource(setup.CaptureOptions{Root: source, Out: version, ProjectID: "status-fixture", SourceMode: "snapshot-only"})
	if err != nil {
		t.Fatal(err)
	}
	id, err := setup.NewDatasetIdentity(captured.Identity, json.RawMessage(`{"model":"mock","dim":8,"checksum":"mock-space"}`), "inputs")
	if err != nil {
		t.Fatal(err)
	}
	graphDigest := strings.Repeat("a", 64)
	for _, side := range []string{"graph", "vector"} {
		dir := filepath.Join(version, side)
		if err := os.Mkdir(dir, 0700); err != nil {
			t.Fatal(err)
		}
		m := map[string]any{"src_root": source, "graph_digest": graphDigest, "schema_version": "1.24", "embedding_model": "mock", "embedding_dim": 8, "embedding_checksum": "mock-space", "project_id": captured.Identity.ProjectID, "snapshot_id": captured.Identity.SnapshotID, "dataset_id": id.DatasetID, "source_mode": "snapshot-only", "file_manifest_digest": captured.Identity.FileManifestDigest, "capture_policy_digest": captured.Identity.CapturePolicyDigest, "sources": map[string]any{"ckg": map[string]any{"graph_digest": graphDigest, "src_root": source}}}
		raw, _ := json.Marshal(m)
		if err := os.WriteFile(filepath.Join(dir, "manifest.json"), raw, 0600); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := setup.PublishCandidateIdentity(version, captured.Identity, "inputs"); err != nil {
		t.Fatal(err)
	}
	snapshot := semantic.Snapshot{ProjectID: captured.Identity.ProjectID, DatasetID: id.DatasetID, SnapshotID: captured.Identity.SnapshotID, SourceMode: "snapshot-only"}
	p, _, err := semantic.ExtractOntology(snapshot, "ontology.yaml", body)
	if err != nil {
		t.Fatal(err)
	}
	spec, _, err := semantic.ExtractSpec(snapshot, "spec.yaml", specBody)
	if err != nil {
		t.Fatal(err)
	}
	p.Requirements = spec.Requirements
	p.Evidence = append(p.Evidence, spec.Evidence...)
	storePath := filepath.Join(t.TempDir(), "meaning.sqlite")
	store, err := semantic.OpenStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.PutAlignedRetained(context.Background(), p, source, filepath.Join(version, "graph"), filepath.Join(version, "vector"), version); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	f := newFixture(t, nil)
	d := f.deps
	cleaner, err := sanitize.New(f.ruleset)
	if err != nil {
		t.Fatal(err)
	}
	d.EvidenceSanitizer = cleaner
	d.EvidenceVersionDir = version
	d.EvidenceSourceRoot = source
	d.SemanticStorePath = storePath
	d.SemanticBudgetMS = 5000
	return d
}

func TestSemanticStatusDistinguishesReadinessFromMode(t *testing.T) {
	for _, reviewed := range []bool{false, true} {
		t.Run(fmt.Sprint(reviewed), func(t *testing.T) {
			d := semanticReadFixture(t, reviewed)
			before, err := os.ReadFile(d.SemanticStorePath)
			if err != nil {
				t.Fatal(err)
			}
			off := inspectSemanticStatus(context.Background(), d)
			if off.State != "off" || off.Availability != "ready" || !off.ReadToolsReady || off.RetrievalReady || off.Counts == nil || off.Counts.Concepts != 1 {
				t.Fatalf("off loses readable data: %+v", off)
			}
			d.SemanticOntologyMode = "combined"
			on := inspectSemanticStatus(context.Background(), d)
			if on.RetrievalReady != reviewed || on.ReadToolsReady != true || on.Snapshot == nil {
				t.Fatalf("review readiness: %+v", on)
			}
			d.SemanticOntologyMode = "relations"
			relations := inspectSemanticStatus(context.Background(), d)
			if relations.RetrievalReady {
				t.Fatalf("no implementations: %+v", relations)
			}
			after, _ := os.ReadFile(d.SemanticStorePath)
			if sha256.Sum256(before) != sha256.Sum256(after) {
				t.Fatal("status changed SQLite bytes")
			}
			store, err := semantic.OpenStoreReadOnly(context.Background(), d.SemanticStorePath)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := store.Close(); err != nil {
					t.Errorf("close read-only semantic store: %v", err)
				}
			})
			if _, err := store.Current(context.Background(), on.Snapshot.ProjectID); err == nil {
				t.Fatal("status activated a candidate")
			}
		})
	}
}

func TestSemanticStatusUnavailableReasonsArePathSafe(t *testing.T) {
	for _, tc := range []struct {
		name, mode, want string
		mutate           func(*Deps)
	}{
		{"unconfigured", "combined", "unconfigured", func(d *Deps) { d.SemanticStorePath = "" }},
		{"missing-store", "combined", "store_missing", func(d *Deps) { d.SemanticStorePath = filepath.Join(t.TempDir(), "must-not-create.sqlite") }},
		{"missing-dataset", "combined", "dataset_missing", func(d *Deps) { d.EvidenceVersionDir = "" }},
		{"legacy-dataset", "combined", "unsupported_dataset", func(d *Deps) { d.EvidenceVersionDir = t.TempDir() }},
		{"stale", "combined", "stale", func(d *Deps) {
			if err := os.WriteFile(filepath.Join(d.EvidenceVersionDir, "graph", "manifest.json"), []byte(`{"source":"private-secret-path"}`), 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{"projection-missing", "combined", "projection_missing", func(d *Deps) {
			d.SemanticStorePath = filepath.Join(t.TempDir(), "empty.sqlite")
			s, err := semantic.OpenStore(d.SemanticStorePath)
			if err != nil {
				t.Fatal(err)
			}
			if err := s.Close(); err != nil {
				t.Fatal(err)
			}
		}},
		{"store-unavailable", "combined", "store_unavailable", func(d *Deps) {
			if err := os.WriteFile(d.SemanticStorePath, []byte("invalid db private-secret-path"), 0600); err != nil {
				t.Fatal(err)
			}
		}},
		{"wrong-mode", "not-a-mode", "unsupported", func(d *Deps) {}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := semanticReadFixture(t, true)
			tc.mutate(&d)
			d.SemanticOntologyMode = tc.mode
			out := inspectSemanticStatus(context.Background(), d)
			if out.Availability != tc.want || out.ReadToolsReady || out.RetrievalReady || out.Counts != nil {
				t.Fatalf("unexpected: %+v", out)
			}
			raw, _ := json.Marshal(out)
			if strings.Contains(string(raw), d.EvidenceSourceRoot) || strings.Contains(string(raw), "private-secret-path") {
				t.Fatal("path leaked")
			}
			if tc.name == "missing-store" {
				if _, err := os.Stat(d.SemanticStorePath); !os.IsNotExist(err) {
					t.Fatal("missing store created")
				}
			}
		})
	}
	d := semanticReadFixture(t, true)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if got := inspectSemanticStatus(ctx, d); got.Availability != "budget_exceeded" || got.Counts != nil {
		t.Fatalf("cancelled result: %+v", got)
	}
}
