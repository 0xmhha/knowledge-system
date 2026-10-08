package setup

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyTextCorpusRejectsTamperAndMixedSources(t *testing.T) {
	root := t.TempDir()
	graph := t.TempDir()
	commit := strings.Repeat("a", 40)
	writeManifest(t, graph, map[string]any{"src_root": root, "src_commit": commit})
	corpus := t.TempDir()
	body := []byte("# Reviewed concept\n")
	if err := os.WriteFile(filepath.Join(corpus, "concept.md"), body, 0600); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(body)
	writeManifest(t, corpus, map[string]any{"schema_version": 1,
		"snapshot": map[string]any{"commit": commit}, "source_root": root,
		"projection_sha256": strings.Repeat("b", 64),
		"files":             []map[string]any{{"path": "concept.md", "sha256": hex.EncodeToString(sum[:])}}})
	if err := VerifyTextCorpus(corpus, graph); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(corpus, "concept.md"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyTextCorpus(corpus, graph); err == nil || !strings.Contains(err.Error(), "digest mismatch") {
		t.Fatalf("changed content accepted: %v", err)
	}
	if err := os.WriteFile(filepath.Join(corpus, "concept.md"), body, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(corpus, "extra.md"), body, 0600); err != nil {
		t.Fatal(err)
	}
	if err := VerifyTextCorpus(corpus, graph); err == nil || !strings.Contains(err.Error(), "unmanifested") {
		t.Fatalf("extra Markdown accepted: %v", err)
	}
	if err := os.Remove(filepath.Join(corpus, "extra.md")); err != nil {
		t.Fatal(err)
	}
	writeManifest(t, graph, map[string]any{"src_root": t.TempDir(), "src_commit": commit})
	if err := VerifyTextCorpus(corpus, graph); err == nil || !strings.Contains(err.Error(), "different source root") {
		t.Fatalf("foreign root accepted: %v", err)
	}
}

func TestBuildPlanVerifiesSemanticCorpusBeforeVectorEmbedding(t *testing.T) {
	plan, err := BuildPlan(Options{Src: "/src", Out: "/out", Embedder: "mock", SemanticCorpus: "/corpus"})
	if err != nil || len(plan.Steps) != 4 || plan.Steps[1].ID != "semantic-corpus-verify" ||
		plan.Steps[2].ID != "vector-build" || !strings.Contains(strings.Join(plan.Steps[2].Cmd, " "), "--docs /corpus") {
		t.Fatalf("semantic corpus escaped verification: %+v, %v", plan, err)
	}
	if _, err := BuildPlan(Options{Src: "/src", Out: "/out", SemanticCorpus: "/corpus", SkipVector: true}); err == nil {
		t.Fatal("semantic corpus silently ignored in graph-only build")
	}
}
