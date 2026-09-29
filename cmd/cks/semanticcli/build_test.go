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
	spec := []byte(`version: 1
project_id: example
requirements:
  - id: req-source
    version: 1
    title: Source citation
    statement: Answers cite committed source lines.
    status: proposed
    concept_ids: [source]
    acceptance_criteria:
      - id: ac-source
        given: A committed document
        when: Its content is retrieved
        then: The original lines are cited
`)
	if err := os.WriteFile(filepath.Join(repo, "requirements.yaml"), spec, 0o644); err != nil {
		t.Fatal(err)
	}
	testGit(t, repo, "add", "spec.md", "ontology.yaml", "requirements.yaml")
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
		"--docs", "spec.md", "--ontology", "ontology.yaml", "--spec", "requirements.yaml", "--activate"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	var result struct {
		Sections     int  `json:"sections"`
		Concepts     int  `json:"concepts"`
		Requirements int  `json:"requirements"`
		Activated    bool `json:"activated"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Sections == 0 || result.Concepts != 1 || result.Requirements != 1 || !result.Activated {
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
	if p.Snapshot.Commit != commit || p.Sections[0].Heading != "Expected" || p.Concepts[0].ID != "source" || p.Requirements[0].ID != "req-source" {
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
	trace := NewCmd()
	var traceOut bytes.Buffer
	trace.SetOut(&traceOut)
	trace.SetArgs([]string{"trace", "--project-id", "example", "--repo", repo,
		"--graph", graph, "--vector", vector, "--store", storePath})
	if err := trace.Execute(); err != nil {
		t.Fatalf("trace active dataset: %v", err)
	}
	var traceReport semantic.TraceReport
	if err := json.Unmarshal(traceOut.Bytes(), &traceReport); err != nil {
		t.Fatal(err)
	}
	if len(traceReport.Requirements) != 1 || traceReport.Requirements[0].State != semantic.TraceSpecUnapproved {
		t.Fatalf("proposed spec was treated as implemented: %+v", traceReport)
	}
	extract := NewCmd()
	extract.SetArgs([]string{"build", "--repo", repo, "--project-id", "example", "--dataset-id", "cut-2",
		"--graph", graph, "--vector", vector, "--store", storePath, "--out", output,
		"--docs", "spec.md", "--ontology", "ontology.yaml", "--spec", "requirements.yaml", "--extract-only"})
	if err := extract.Execute(); err != nil {
		t.Fatalf("extract-only: %v", err)
	}
	if _, err := store.Load(context.Background(), "example", "cut-2"); err == nil {
		t.Fatal("extract-only stored a dataset")
	}
	promote := NewCmd()
	promote.SetArgs([]string{"promote", "--input", output, "--repo", repo,
		"--graph", graph, "--vector", vector, "--store", storePath, "--activate"})
	if err := promote.Execute(); err != nil {
		t.Fatalf("promote extracted projection: %v", err)
	}
	current, err = store.Current(context.Background(), "example")
	if err != nil || current.Snapshot.DatasetID != "cut-2" {
		t.Fatalf("promoted projection = %+v, %v", current.Snapshot, err)
	}
	bad := NewCmd()
	bad.SetArgs([]string{"build", "--repo", repo, "--project-id", "example", "--dataset-id", "cut-3",
		"--graph", graph, "--vector", vector, "--store", storePath, "--out", output,
		"--docs", "../secret.md", "--activate"})
	if err := bad.Execute(); err == nil || !strings.Contains(err.Error(), "unsafe document path") {
		t.Fatalf("unsafe document path allowed: %v", err)
	}
}
