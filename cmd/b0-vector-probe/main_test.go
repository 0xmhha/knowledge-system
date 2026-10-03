package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/vector/embed/mock"
	"github.com/0xmhha/knowledge-system/internal/vector/manifest"
	"github.com/0xmhha/knowledge-system/internal/vector/store/sqlitevec"
	"github.com/0xmhha/knowledge-system/pkg/vector/types"
)

func frozenFixture(t *testing.T) (string, types.Embedder, types.Filter) {
	t.Helper()
	root := t.TempDir()
	embedding := mock.Default()
	ctx := context.Background()
	store, err := sqlitevec.Open(filepath.Join(root, "vector.db"), embedding.Dimension())
	if err != nil {
		t.Fatal(err)
	}
	q, err := embedding.Embed(ctx, []string{"Alpha route"})
	if err != nil {
		t.Fatal(err)
	}
	var chunks []types.Chunk
	var vectors [][]float32
	for _, entry := range []struct {
		id, path string
		eligible bool
	}{
		{"e1", "src/one.go", true}, {"e2", "src/two.go", true}, {"e3", "src/three.go", true},
		{"d1", "src/distractor/one.go", false}, {"d2", "src/distractor/two.go", false}, {"d3", "src/distractor/three.go", false},
		{"t1", "src/helper_test.go", false},
	} {
		chunk := types.Chunk{ID: entry.id, File: entry.path, StartLine: 3, EndLine: 3, Language: "go", SymbolKind: types.KindFunction, ChunkKind: types.ChunkSymbol, CommitHash: strings.Repeat("1", 40), Text: "synthetic source", ContentSHA256: types.ContentSHA256("synthetic source")}
		vector := append([]float32(nil), q[0]...)
		if entry.eligible {
			for i := range vector {
				vector[i] = -vector[i]
			}
		}
		chunks = append(chunks, chunk)
		vectors = append(vectors, vector)
	}
	if err := store.Upsert(ctx, chunks, vectors); err != nil {
		t.Fatal(err)
	}
	if err := store.SetManifest(ctx, map[string]string{"embedding_checksum": embedding.Identity().Checksum()}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	digest, err := fileHash(filepath.Join(root, "vector.db"))
	if err != nil {
		t.Fatal(err)
	}
	metadata := manifest.Manifest{EmbeddingChecksum: embedding.Identity().Checksum(), EmbeddingDim: embedding.Dimension(), DBSHA256: digest}
	raw, _ := json.Marshal(metadata)
	if err := os.WriteFile(filepath.Join(root, "manifest.json"), raw, 0600); err != nil {
		t.Fatal(err)
	}
	return root, embedding, types.Filter{Language: "go", PathGlob: "src/*.go", SymbolKinds: []types.SymbolKind{types.KindFunction}, ChunkKinds: []types.ChunkKind{types.ChunkSymbol}, ExcludeTests: true}
}

func TestProbeUsesSameStoredVectorsForFilterOracleAndBudget(t *testing.T) {
	root, embedding, filter := frozenFixture(t)
	report, err := probe(context.Background(), root, "Alpha route", filter, 3, 2, embedding)
	if err != nil {
		t.Fatal(err)
	}
	if !report.ExactAgreement || report.Oracle.EligibleCount != 3 || len(report.Filtered.Hits) != 3 {
		t.Fatalf("wrong filter/oracle: %+v", report)
	}
	if report.Budget.Status != sqlitevec.SearchIncomplete || report.Budget.Reason != "candidate_limit" {
		t.Fatalf("budget masqueraded as success: %+v", report.Budget)
	}
	if !report.SparseFixtureQualified {
		t.Fatal("unfiltered nearest distractors did not qualify the sparse fixture")
	}
	for i, id := range []string{"e1", "e2", "e3"} {
		if report.Oracle.Hits[i].ID != id || report.Filtered.Hits[i].Chunk.ID != id {
			t.Fatal("exact tie order differs")
		}
	}
	if report.DatabaseSHA256Before != report.DatabaseSHA256After || !report.IdentityVerifiedAfter {
		t.Fatal("input/model guard missing")
	}
	if report.QualityMetrics != nil {
		t.Fatal("probe invented B0 quality metrics")
	}
}

func TestProbeRejectsUnsealedWalOutsidePublishedHash(t *testing.T) {
	root, embedding, filter := frozenFixture(t)
	if err := os.WriteFile(filepath.Join(root, "vector.db-wal"), []byte("unsealed WAL bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := probe(context.Background(), root, "Alpha route", filter, 3, 2, embedding)
	if err == nil || !strings.Contains(report.Error, "unsealed") {
		t.Fatalf("unhashed WAL accepted: %+v %v", report, err)
	}
}

func TestFilterRefusesUnknownAndTrailingInput(t *testing.T) {
	for _, raw := range []string{`{"langauge":"go"}`, `{"language":"go"} {}`} {
		if _, err := parseFilter([]byte(raw)); err == nil {
			t.Fatalf("invalid filter accepted: %s", raw)
		}
	}
	if filter, err := parseFilter([]byte(`{"language":"go","path":"src/*.go","exclude_tests":true}`)); err != nil || filter.PathGlob != "src/*.go" || !filter.ExcludeTests {
		t.Fatalf("valid filter rejected: %+v %v", filter, err)
	}
}

func TestOracleRejectsMissingEligibleVector(t *testing.T) {
	root, embedding, filter := frozenFixture(t)
	db, err := sql.Open("sqlite3", filepath.Join(root, "vector.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec("DELETE FROM chunk_vec WHERE chunk_id='e1'"); err != nil {
		t.Fatal(err)
	}
	db.Close()
	q, _ := embedding.Embed(context.Background(), []string{"Alpha route"})
	if _, err := exactOracle(context.Background(), filepath.Join(root, "vector.db"), q[0], filter, 3); err == nil {
		t.Fatal("oracle silently omitted missing vector")
	}
	report, err := probe(context.Background(), root, "Alpha route", filter, 3, 2, embedding)
	if err == nil || report.State != "error" || report.Error == "" {
		t.Fatal("damaged published hash was not rejected")
	}
}

func TestProbeRejectsSidecarCoordinateNotRecordedInDatabase(t *testing.T) {
	root, embedding, filter := frozenFixture(t)
	path := filepath.Join(root, "manifest.json")
	raw, _ := os.ReadFile(path)
	var metadata manifest.Manifest
	if err := json.Unmarshal(raw, &metadata); err != nil {
		t.Fatal(err)
	}
	metadata.ProjectID = "wrong-project"
	raw, _ = json.Marshal(metadata)
	if err := os.WriteFile(path, raw, 0600); err != nil {
		t.Fatal(err)
	}
	report, err := probe(context.Background(), root, "Alpha route", filter, 3, 2, embedding)
	if err == nil || report.State != "error" || report.QueryVector != nil {
		t.Fatal("coordinate mismatch reached query embedding")
	}
}
