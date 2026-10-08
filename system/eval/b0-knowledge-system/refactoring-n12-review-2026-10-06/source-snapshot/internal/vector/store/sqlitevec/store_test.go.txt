package sqlitevec

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/0xmhha/knowledge-system/pkg/vector/types"
)

const testDim = 4

// mkChunk is a tiny helper for fixture chunks. start/end are line numbers;
// text drives ContentSHA256 (chunk_id determinism is verified in pkg/types).
func mkChunk(id, file, text string, start, end int, lang string, kind types.SymbolKind) types.Chunk {
	return types.Chunk{
		ID:            id,
		File:          file,
		StartLine:     start,
		EndLine:       end,
		Language:      lang,
		SymbolName:    "fn",
		SymbolKind:    kind,
		ChunkKind:     types.ChunkSymbol,
		CommitHash:    "deadbeef",
		ContentSHA256: types.ContentSHA256(text),
		Text:          text,
	}
}

func TestStoreUpsertAndSearch(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "vector.db")
	s, err := Open(dbPath, testDim)
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer s.Close()
	ctx := context.Background()

	chunks := []types.Chunk{
		mkChunk("a", "x.go", "alpha", 1, 5, "go", types.KindFunction),
		mkChunk("b", "y.go", "beta", 1, 8, "go", types.KindMethod),
		mkChunk("c", "z.ts", "gamma", 1, 3, "typescript", types.KindFunction),
	}
	embs := [][]float32{
		{1, 0, 0, 0}, // closest to query {0.9, 0.1, 0, 0}
		{0, 1, 0, 0},
		{0, 0, 1, 0},
	}
	if err := s.Upsert(ctx, chunks, embs); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := s.Search(ctx, []float32{0.9, 0.1, 0, 0}, 3, types.Filter{})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 hits, got %d", len(got))
	}
	if got[0].Chunk.ID != "a" {
		t.Errorf("expected closest chunk 'a' first, got %s (distance %f)",
			got[0].Chunk.ID, got[0].Score.VectorDistance)
	}
	if got[0].Score.VectorRank != 1 || got[1].Score.VectorRank != 2 {
		t.Errorf("rank not 1-based monotonic: %+v", got)
	}
	if got[0].Score.Normalized <= got[2].Score.Normalized {
		t.Errorf("normalized score must rank-monotone: got %v then %v",
			got[0].Score.Normalized, got[2].Score.Normalized)
	}
}

func TestStoreFilterByLanguage(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "v.db")
	s, _ := Open(dbPath, testDim)
	defer s.Close()
	ctx := context.Background()

	chunks := []types.Chunk{
		mkChunk("a", "x.go", "alpha", 1, 5, "go", types.KindFunction),
		mkChunk("b", "y.ts", "beta", 1, 8, "typescript", types.KindFunction),
	}
	embs := [][]float32{{1, 0, 0, 0}, {0.9, 0.1, 0, 0}}
	if err := s.Upsert(ctx, chunks, embs); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	got, err := s.Search(ctx, []float32{1, 0, 0, 0}, 5, types.Filter{Language: "typescript"})
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(got) != 1 || got[0].Chunk.ID != "b" {
		t.Fatalf("expected only ts chunk 'b', got %+v", got)
	}
}

func TestFilteredSearchRejectsMissingEmbeddingInsteadOfReturningShortSuccess(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "missing-vector.db"), testDim)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	chunks := []types.Chunk{
		mkChunk("a", "a.go", "a", 1, 1, "go", types.KindFunction),
		mkChunk("b", "b.go", "b", 1, 1, "go", types.KindFunction),
	}
	if err := s.Upsert(ctx, chunks, [][]float32{{1, 0, 0, 0}, {0, 1, 0, 0}}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM chunk_vec WHERE chunk_id = ?", "b"); err != nil {
		t.Fatal(err)
	}
	hits, err := s.Search(ctx, []float32{1, 0, 0, 0}, 2, types.Filter{Language: "go"})
	if !errors.Is(err, ErrIncompleteIndex) || hits != nil {
		t.Fatalf("missing vector must fail closed, hits=%v err=%v", hits, err)
	}
}

func TestSearchRejectsCancelledContextEvenForEmptyResult(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "cancelled.db"), testDim)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, filter := range []types.Filter{{}, {Language: "go"}} {
		hits, err := s.Search(ctx, []float32{1, 0, 0, 0}, 0, filter)
		if !errors.Is(err, context.Canceled) || hits != nil {
			t.Fatalf("cancelled search must not look complete, filter=%+v hits=%v err=%v", filter, hits, err)
		}
	}
}

func TestSearchDetailedReportsCandidateLimitWithoutCompleteHits(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "limited.db"), testDim)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	chunks := []types.Chunk{
		mkChunk("a", "a.go", "a", 1, 1, "go", types.KindFunction),
		mkChunk("b", "b.go", "b", 1, 1, "go", types.KindFunction),
		mkChunk("c", "c.go", "c", 1, 1, "go", types.KindFunction),
	}
	if err := s.Upsert(context.Background(), chunks, [][]float32{{1, 0, 0, 0}, {0.9, 0.1, 0, 0}, {0, 1, 0, 0}}); err != nil {
		t.Fatal(err)
	}
	result, err := s.SearchDetailed(context.Background(), []float32{1, 0, 0, 0}, 2,
		types.Filter{Language: "go"}, SearchOptions{MaxExactCandidates: 2})
	if err != nil {
		t.Fatal(err)
	}
	if result.Status != SearchIncomplete || result.Reason != "candidate_limit" || len(result.Hits) != 0 {
		t.Fatalf("candidate limit must be explicit: %+v", result)
	}
	if result.CandidateCount != 3 || result.SearchedCount != 0 {
		t.Fatalf("candidate accounting wrong: %+v", result)
	}
	result, err = s.SearchDetailed(context.Background(), []float32{1, 0, 0, 0}, 2,
		types.Filter{Language: "go"}, SearchOptions{MaxExactCandidates: 3})
	if err != nil || result.Status != SearchComplete || len(result.Hits) != 2 || result.Hits[0].Chunk.ID != "a" {
		t.Fatalf("exact search at limit must complete: result=%+v err=%v", result, err)
	}
	if result.EligibleCount == nil || *result.EligibleCount != 3 || result.SearchedCount != 3 {
		t.Fatalf("exact search accounting wrong: %+v", result)
	}
}

func TestSearchDetailedCancellationIsNotEmptySuccess(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "cancel-status.db"), testDim)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := s.SearchDetailed(ctx, []float32{1, 0, 0, 0}, 2, types.Filter{}, SearchOptions{})
	if err != nil || result.Status != SearchCancelled || result.Reason != "context_cancelled" || len(result.Hits) != 0 {
		t.Fatalf("cancelled search looked complete: result=%+v err=%v", result, err)
	}
	deadlineCtx, stop := context.WithDeadline(context.Background(), time.Unix(0, 0))
	defer stop()
	result, err = s.SearchDetailed(deadlineCtx, []float32{1, 0, 0, 0}, 2, types.Filter{}, SearchOptions{})
	if err != nil || result.Status != SearchIncomplete || result.Reason != "deadline_exceeded" || len(result.Hits) != 0 {
		t.Fatalf("expired search looked complete: result=%+v err=%v", result, err)
	}
}

func TestSearchDetailedEmptyAndContradictoryFiltersAreComplete(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "empty-filter.db"), testDim)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	query := []float32{1, 0, 0, 0}
	for _, filter := range []types.Filter{{}, {Language: "typescript", CommitHash: "missing"}} {
		result, err := s.SearchDetailed(context.Background(), query, 5, filter, SearchOptions{})
		if err != nil || result.Status != SearchComplete || len(result.Hits) != 0 {
			t.Fatalf("empty eligible set must complete: filter=%+v result=%+v err=%v", filter, result, err)
		}
		if result.EligibleCount == nil || *result.EligibleCount != 0 {
			t.Fatalf("eligible count must be zero: filter=%+v result=%+v", filter, result)
		}
	}
}

func TestSearchDetailedRejectsOversizedKBeforeAllocation(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "oversized-k.db"), testDim)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	result, err := s.SearchDetailed(context.Background(), []float32{1, 0, 0, 0},
		DefaultMaxSearchK+1, types.Filter{}, SearchOptions{})
	if err != nil || result.Status != SearchIncomplete || result.Reason != "requested_k_exceeds_limit" || len(result.Hits) != 0 {
		t.Fatalf("oversized K looked complete: result=%+v err=%v", result, err)
	}
	if hits, err := s.Search(context.Background(), []float32{1, 0, 0, 0},
		DefaultMaxSearchK+1, types.Filter{}); !errors.Is(err, ErrSearchIncomplete) || hits != nil {
		t.Fatalf("legacy search must fail closed: hits=%v err=%v", hits, err)
	}
}

func TestStoreFilterPathPrefixKeepsExactGlobSemantics(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "path.db"), testDim)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	chunks := []types.Chunk{
		mkChunk("near-excluded", "internal/vector/other.go", "near", 1, 1, "go", types.KindFunction),
		mkChunk("target", "internal/vector/store/sqlitevec/store.go", "target", 1, 1, "go", types.KindFunction),
		mkChunk("nested-excluded", "internal/vector/store/sqlitevec/nested/store.go", "nested", 1, 1, "go", types.KindFunction),
	}
	if err := s.Upsert(context.Background(), chunks, [][]float32{{1, 0, 0, 0}, {0, 1, 0, 0}, {0.9, 0.1, 0, 0}}); err != nil {
		t.Fatal(err)
	}
	hits, err := s.Search(context.Background(), []float32{1, 0, 0, 0}, 1,
		types.Filter{PathGlob: "internal/vector/store/sqlitevec/*.go"})
	if err != nil || len(hits) != 1 || hits[0].Chunk.ID != "target" {
		t.Fatalf("path filter lost target or admitted nested path: hits=%+v err=%v", hits, err)
	}
	if literalGlobPrefix("internal/vector/[ab]/*.go") != "internal/vector/" || literalGlobPrefix("internal/vector/\\*.go") != "internal/vector/" {
		t.Fatal("glob prefix scanner narrowed an escaped or bracket pattern")
	}
}

func TestStoreCombinedFilterMatchesExactOracle(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "combined.db"), testDim)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	var chunks []types.Chunk
	var vectors [][]float32
	for i := 0; i < 60; i++ {
		file := fmt.Sprintf("src/f%02d.go", i)
		if i%7 == 0 {
			file = fmt.Sprintf("src/f%02d_test.go", i)
		}
		kind := types.KindFunction
		if i%5 == 0 {
			kind = types.KindMethod
		}
		lang := "go"
		if i%3 == 0 {
			lang = "typescript"
		}
		chunk := mkChunk(fmt.Sprintf("id-%02d", i), file, "body", i+1, i+1, lang, kind)
		chunk.IsTest = i%7 == 0
		if i%11 == 0 {
			chunk.CommitHash = "other"
		}
		chunks = append(chunks, chunk)
		vectors = append(vectors, []float32{float32(i) / 60, 0, 0, 0})
	}
	if err := s.Upsert(context.Background(), chunks, vectors); err != nil {
		t.Fatal(err)
	}
	filter := types.Filter{Language: "go", PathGlob: "src/*.go", SymbolKinds: []types.SymbolKind{types.KindFunction},
		ChunkKinds: []types.ChunkKind{types.ChunkSymbol}, CommitHash: "deadbeef", ExcludeTests: true}
	var eligible []types.Chunk
	for _, chunk := range chunks {
		if filter.Matches(chunk) {
			eligible = append(eligible, chunk)
		}
	}
	sort.Slice(eligible, func(i, j int) bool { return eligible[i].StartLine > eligible[j].StartLine })
	const k = 5
	if len(eligible) < k {
		t.Fatalf("fixture has only %d eligible chunks", len(eligible))
	}
	hits, err := s.Search(context.Background(), []float32{1, 0, 0, 0}, k, filter)
	if err != nil || len(hits) != k {
		t.Fatalf("filtered search: hits=%d err=%v", len(hits), err)
	}
	for i, hit := range hits {
		if hit.Chunk.ID != eligible[i].ID {
			t.Errorf("rank %d = %s, exact oracle = %s", i+1, hit.Chunk.ID, eligible[i].ID)
		}
	}
	filter.Language = "solidity"
	hits, err = s.Search(context.Background(), []float32{1, 0, 0, 0}, k, filter)
	if err != nil || len(hits) != 0 {
		t.Fatalf("contradictory filter returned hits=%d err=%v", len(hits), err)
	}
}

func TestStorePreservesDocumentParentMetadata(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "docs.db"), testDim)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	doc := mkChunk("doc-child", "README.md", "tail paragraph", 40, 42, "markdown", types.KindDocSection)
	doc.ChunkKind = types.ChunkDoc
	doc.ParentID, doc.ParentStartLine, doc.ParentEndLine = "parent-id", 10, 50
	doc.PartOrdinal = 3
	doc.HeadingPath = []string{"System", "Search"}
	if err := s.Upsert(ctx, []types.Chunk{doc}, [][]float32{{1, 0, 0, 0}}); err != nil {
		t.Fatal(err)
	}
	got, err := s.LookupByIDs(ctx, []string{doc.ID})
	if err != nil || len(got) != 1 {
		t.Fatalf("lookup: chunks=%d err=%v", len(got), err)
	}
	if got[0].ParentID != doc.ParentID || got[0].ParentStartLine != 10 || got[0].ParentEndLine != 50 || got[0].PartOrdinal != 3 || len(got[0].HeadingPath) != 2 {
		t.Fatalf("parent metadata not persisted: %+v", got[0])
	}
	hits, err := s.Search(ctx, []float32{1, 0, 0, 0}, 1, types.Filter{})
	if err != nil || len(hits) != 1 || hits[0].Chunk.ParentID != doc.ParentID {
		t.Fatalf("search lost parent metadata: hits=%+v err=%v", hits, err)
	}
}

func TestStoreMigratesOlderChunkTableWithoutParentColumns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	initial, err := Open(path, testDim)
	if err != nil {
		t.Fatal(err)
	}
	if err := initial.Close(); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite3", path)
	if err != nil {
		t.Fatal(err)
	}
	_, err = db.Exec(`ALTER TABLE chunks DROP COLUMN parent_id;
		ALTER TABLE chunks DROP COLUMN parent_start_line;
		ALTER TABLE chunks DROP COLUMN parent_end_line;
		ALTER TABLE chunks DROP COLUMN part_ordinal;
		ALTER TABLE chunks DROP COLUMN heading_path;
		INSERT INTO chunks (id, file, start_line, end_line, language, chunk_kind, commit_hash, content_sha256, text)
		VALUES ('legacy', 'README.md', 1, 2, 'markdown', 'doc', 'abc', 'sha', 'old text');
		UPDATE chunks SET symbol_name='', symbol_kind='', canonical_id='', recent_prs='', category='',
			guidance='', invariants='', convention_stats='', flow_meta='', enforced_at='', provenance=''
		WHERE id='legacy';`)
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		store, err := Open(path, testDim)
		if err != nil {
			t.Fatalf("open %d after additive migration: %v", i, err)
		}
		got, err := store.LookupByIDs(context.Background(), []string{"legacy"})
		if err != nil || len(got) != 1 || got[0].Text != "old text" || got[0].ParentID != "" {
			t.Fatalf("legacy row after migration: chunks=%+v err=%v", got, err)
		}
		if err := store.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestStoreFilterReturnsKWhenGlobalTopCandidatesAreExcluded(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "selective.db")
	s, err := Open(dbPath, testDim)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	var chunks []types.Chunk
	var embs [][]float32
	for i := 0; i < 12; i++ {
		chunks = append(chunks, mkChunk(string(rune('a'+i)), "near.go", "near", i+1, i+1, "go", types.KindFunction))
		embs = append(embs, []float32{1, 0, 0, 0})
	}
	chunks = append(chunks,
		mkChunk("target-one", "target.ts", "one", 1, 1, "typescript", types.KindFunction),
		mkChunk("target-two", "target.ts", "two", 2, 2, "typescript", types.KindFunction),
	)
	embs = append(embs, []float32{0, 1, 0, 0}, []float32{0, 0.8, 0.2, 0})
	if err := s.Upsert(ctx, chunks, embs); err != nil {
		t.Fatal(err)
	}
	hits, err := s.Search(ctx, []float32{1, 0, 0, 0}, 2, types.Filter{Language: "typescript"})
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 2 {
		t.Fatalf("filtered KNN returned %d hits, want 2", len(hits))
	}
	seen := map[string]bool{}
	for _, h := range hits {
		seen[h.Chunk.ID] = true
	}
	if !seen["target-one"] || !seen["target-two"] {
		t.Fatalf("wrong filtered hits: %+v", hits)
	}
}

func TestStoreFilterLargeCandidateSetFallsBackToExact(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "large-filter.db"), testDim)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	chunks := make([]types.Chunk, 0, 2050)
	embs := make([][]float32, 0, 2050)
	for i := 0; i < 2049; i++ {
		chunks = append(chunks, mkChunk(fmt.Sprintf("near-%d", i), "near/a.go", "near", i+1, i+1, "go", types.KindFunction))
		embs = append(embs, []float32{1, 0, 0, 0})
	}
	chunks = append(chunks, mkChunk("target", "target.go", "target", 1, 1, "go", types.KindFunction))
	embs = append(embs, []float32{0, 1, 0, 0})
	if err := s.Upsert(ctx, chunks, embs); err != nil {
		t.Fatal(err)
	}
	// The bracket glob has no pushdown prefix. All 2,050 metadata rows are
	// candidates, but the first vec0 neighbors are all rejected by Matches.
	filter := types.Filter{PathGlob: "[t]*.go"}
	limited, err := s.SearchDetailed(ctx, []float32{1, 0, 0, 0}, 1, filter,
		SearchOptions{MaxExactCandidates: 2048})
	if err != nil || limited.Status != SearchIncomplete || limited.Reason != "candidate_limit" ||
		limited.CandidateCount != 2050 || len(limited.Hits) != 0 {
		t.Fatalf("bounded fallback must disclose incomplete search: result=%+v err=%v", limited, err)
	}
	hits, err := s.Search(ctx, []float32{1, 0, 0, 0}, 1, filter)
	if err != nil {
		t.Fatal(err)
	}
	if len(hits) != 1 || hits[0].Chunk.ID != "target" {
		t.Fatalf("large-set fallback missed target: %+v", hits)
	}
}

func TestDeleteDocsChunks(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "v.db")
	s, _ := Open(dbPath, testDim)
	defer s.Close()
	ctx := context.Background()

	// Curated docs layer (chunk_kind=doc, category=domain) alongside a
	// symbol chunk and an in-tree doc chunk (category=""). Only the curated
	// docs layer must be deleted.
	doc1 := mkChunk("d1", "corpus/a.md", "domain doc one", 1, 3, "markdown", types.KindFunction)
	doc1.ChunkKind, doc1.Category = types.ChunkDoc, "domain"
	doc2 := mkChunk("d2", "corpus/b.md", "domain doc two", 1, 3, "markdown", types.KindFunction)
	doc2.ChunkKind, doc2.Category = types.ChunkDoc, "domain"
	intreeDoc := mkChunk("d3", "README.md", "in-tree doc", 1, 3, "markdown", types.KindFunction)
	intreeDoc.ChunkKind, intreeDoc.Category = types.ChunkDoc, "" // NOT domain → keep
	sym := mkChunk("s1", "x.go", "code", 1, 5, "go", types.KindFunction)

	embs := [][]float32{{1, 0, 0, 0}, {0, 1, 0, 0}, {0, 0, 1, 0}, {0, 0, 0, 1}}
	if err := s.Upsert(ctx, []types.Chunk{doc1, doc2, intreeDoc, sym}, embs); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	n, err := s.DeleteDocsChunks(ctx)
	if err != nil {
		t.Fatalf("DeleteDocsChunks: %v", err)
	}
	if n != 2 {
		t.Fatalf("deleted %d docs chunks, want 2 (curated domain only)", n)
	}

	remaining, _ := s.DocsChunks(ctx)
	if len(remaining) != 0 {
		t.Fatalf("DocsChunks after delete = %d, want 0", len(remaining))
	}
	// The symbol and in-tree doc chunks must survive.
	st, _ := s.Stats(ctx)
	if st.ChunkCount != 2 {
		t.Fatalf("remaining chunks = %d, want 2 (symbol + in-tree doc)", st.ChunkCount)
	}
}

func TestStoreDeleteByFile(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "v.db")
	s, _ := Open(dbPath, testDim)
	defer s.Close()
	ctx := context.Background()

	chunks := []types.Chunk{
		mkChunk("a1", "x.go", "alpha1", 1, 5, "go", types.KindFunction),
		mkChunk("a2", "x.go", "alpha2", 6, 10, "go", types.KindFunction),
		mkChunk("b", "y.go", "beta", 1, 8, "go", types.KindMethod),
	}
	embs := [][]float32{{1, 0, 0, 0}, {0.7, 0.7, 0, 0}, {0, 1, 0, 0}}
	if err := s.Upsert(ctx, chunks, embs); err != nil {
		t.Fatalf("Upsert: %v", err)
	}

	if err := s.DeleteByFile(ctx, "x.go"); err != nil {
		t.Fatalf("DeleteByFile: %v", err)
	}
	st, _ := s.Stats(ctx)
	if st.ChunkCount != 1 {
		t.Errorf("expected 1 remaining chunk, got %d", st.ChunkCount)
	}
	got, _ := s.Search(ctx, []float32{1, 0, 0, 0}, 5, types.Filter{})
	for _, h := range got {
		if h.Chunk.File == "x.go" {
			t.Errorf("chunk from deleted file leaked: %s", h.Chunk.ID)
		}
	}
}

func TestReopenRejectsDimMismatch(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "v.db")
	s, err := Open(dbPath, 4)
	if err != nil {
		t.Fatalf("first open: %v", err)
	}
	s.Close()

	if _, err := Open(dbPath, 8); err == nil {
		t.Fatal("expected dim-mismatch error, got nil")
	}
}

func TestStatsReflectsManifest(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "v.db")
	s, _ := Open(dbPath, testDim)
	defer s.Close()
	ctx := context.Background()

	_ = s.SetManifest(ctx, map[string]string{
		"embedding_model": "bge-large-en-v1.5",
		"indexed_head":    "abc123",
		"built_at":        "2026-05-08T12:00:00Z",
	})
	st, err := s.Stats(ctx)
	if err != nil {
		t.Fatalf("Stats: %v", err)
	}
	if st.EmbeddingModel != "bge-large-en-v1.5" || st.IndexedHead != "abc123" || st.EmbeddingDim != testDim {
		t.Errorf("manifest not surfaced: %+v", st)
	}
}
