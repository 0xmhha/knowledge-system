package semanticcli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/semantic"
)

func writeTestManifest(t *testing.T, dir string, v any) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	buf, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), buf, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestBuildCommandUsesCommittedMarkdownAndAlignedDatasets(t *testing.T) {
	repo := t.TempDir()
	testGit(t, repo, "init", "-q")
	file := filepath.Join(repo, "spec.md")
	if err := os.WriteFile(file, []byte("# Expected\nThe committed behavior.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ontology := []byte(`version: 1
project_id: example
domain: fixture
competency_questions: ['What is the source?']
concepts:
  - id: source
    kind: artifact
    definition: A committed source.
    includes: [Code]
    excludes: [Uncommitted edits]
    terms: [{lang: en, value: source, preferred: true}]
    status: proposed
`)
	if err := os.WriteFile(filepath.Join(repo, "ontology.yaml"), ontology, 0o644); err != nil {
		t.Fatal(err)
	}
	testGit(t, repo, "add", "spec.md", "ontology.yaml")
	testGit(t, repo, "-c", "user.name=Test", "-c", "user.email=test@example.com", "-c", "commit.gpgsign=false", "commit", "-qm", "spec")
	commit := testGit(t, repo, "rev-parse", "HEAD")
	// Building while a developer has local edits must still use the recorded
	// Git object; working-tree mode is a separate future contract.
	if err := os.WriteFile(file, []byte("# Changed\nUncommitted behavior.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	data := t.TempDir()
	graph, vector := filepath.Join(data, "graph"), filepath.Join(data, "vector")
	digest := strings.Repeat("a", 64)
	writeTestManifest(t, graph, map[string]any{"schema_version": "1.23", "src_root": repo, "src_commit": commit, "graph_digest": digest})
	writeTestManifest(t, vector, map[string]any{"src_root": repo, "src_commit": commit,
		"sources": map[string]any{"ckg": map[string]any{"src_commit": commit, "graph_digest": digest}}})
	storePath, output := filepath.Join(data, "semantic.db"), filepath.Join(data, "projection.json")
	cmd := NewCmd()
	var stdout bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetArgs([]string{"build", "--repo", repo, "--project-id", "example", "--dataset-id", "cut-1",
		"--graph", graph, "--vector", vector, "--store", storePath, "--out", output,
		"--docs", "spec.md", "--ontology", "ontology.yaml", "--activate"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Sections  int  `json:"sections"`
		Concepts  int  `json:"concepts"`
		Activated bool `json:"activated"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Sections == 0 || result.Concepts != 1 || !result.Activated {
		t.Fatalf("build result %+v", result)
	}
	buf, err := os.ReadFile(output)
	if err != nil {
		t.Fatal(err)
	}
	var p semantic.Projection
	if err := json.Unmarshal(buf, &p); err != nil {
		t.Fatal(err)
	}
	if p.Snapshot.Commit != commit || p.Sections[0].Heading != "Expected" || p.Concepts[0].ID != "source" {
		t.Fatalf("projection followed dirty working tree: %+v", p)
	}
	store, err := semantic.OpenStore(storePath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	current, err := store.Current(context.Background(), "example")
	if err != nil || current.Snapshot.DatasetID != "cut-1" {
		t.Fatalf("active projection = %+v, %v", current.Snapshot, err)
	}
	bad := NewCmd()
	bad.SetArgs([]string{"build", "--repo", repo, "--project-id", "example", "--dataset-id", "cut-2",
		"--graph", graph, "--vector", vector, "--store", storePath, "--out", output,
		"--docs", "../secret.md", "--activate"})
	if err := bad.Execute(); err == nil || !strings.Contains(err.Error(), "unsafe document path") {
		t.Fatalf("unsafe document path allowed: %v", err)
	}
}
