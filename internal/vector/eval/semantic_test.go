package eval

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/vector/query"
	"github.com/0xmhha/knowledge-system/pkg/vector/types"
)

// TestLoadSemanticFixtureJSON verifies LoadFixture recognizes the JSON
// semantic-validation format used by vector/scripts/build-knowledge.sh
// and maps it into the shared Fixture shape with substring match mode.
func TestLoadSemanticFixtureJSON(t *testing.T) {
	body := `{
  "k": 10,
  "queries": [
    {"query": "where is the http server started", "expect": "server.go", "note": "listen"},
    {"query": "how are entries evicted from the cache", "expect": "cache.go", "note": "evict"}
  ]
}`
	path := filepath.Join(t.TempDir(), "sem.json")
	if err := writeFile(t, path, body); err != nil {
		t.Fatal(err)
	}
	fx, err := LoadFixture(path)
	if err != nil {
		t.Fatalf("LoadFixture: %v", err)
	}
	if fx.MatchMode != MatchSubstring {
		t.Errorf("MatchMode = %q, want %q", fx.MatchMode, MatchSubstring)
	}
	if fx.K != 10 {
		t.Errorf("K = %d, want 10", fx.K)
	}
	if len(fx.Queries) != 2 {
		t.Fatalf("expected 2 queries, got %d", len(fx.Queries))
	}
	q1 := fx.Queries[0]
	if q1.ID != "q1" {
		t.Errorf("q1.ID = %q, want q1", q1.ID)
	}
	if q1.Intent != "where is the http server started" {
		t.Errorf("q1.Intent = %q", q1.Intent)
	}
	if q1.Expected.Substring != "server.go" {
		t.Errorf("q1.Expected.Substring = %q, want server.go", q1.Expected.Substring)
	}
	if q1.Notes != "listen" {
		t.Errorf("q1.Notes = %q, want listen", q1.Notes)
	}
}

// TestLoadSemanticFixtureByContent verifies detection also works when the
// file lacks a .json extension but the content is a JSON object.
func TestLoadSemanticFixtureByContent(t *testing.T) {
	body := `{"k": 3, "queries": [{"query": "q", "expect": "e"}]}`
	path := filepath.Join(t.TempDir(), "fixture.txt")
	if err := writeFile(t, path, body); err != nil {
		t.Fatal(err)
	}
	fx, err := LoadFixture(path)
	if err != nil {
		t.Fatalf("LoadFixture: %v", err)
	}
	if fx.MatchMode != MatchSubstring || fx.K != 3 || len(fx.Queries) != 1 {
		t.Errorf("unexpected fixture: %+v", fx)
	}
}

// TestLoadSemanticFixtureRejectsBad rejects empty query sets and entries
// missing either the query text or the expect substring.
func TestLoadSemanticFixtureRejectsBad(t *testing.T) {
	bad := []string{
		`{"k": 10, "queries": []}`,
		`{"k": 10, "queries": [{"query": "", "expect": "x"}]}`,
		`{"k": 10, "queries": [{"query": "x", "expect": ""}]}`,
		`{"k": 10, "queries": [`, // malformed JSON
	}
	for i, body := range bad {
		path := filepath.Join(t.TempDir(), "bad.json")
		if err := writeFile(t, path, body); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadFixture(path); err == nil {
			t.Errorf("case %d: expected error for %q", i, body)
		}
	}
}

// TestLoadRealSemanticFixture loads the on-disk fixture the shell script
// used (vector/scripts/semantic-validation-queries.json). It carries an
// unknown "_comment" key and Korean query text, both of which must be
// tolerated by the loader.
func TestLoadRealSemanticFixture(t *testing.T) {
	path, _ := filepath.Abs(filepath.Join("..", "..", "..", "vector", "scripts", "semantic-validation-queries.json"))
	fx, err := LoadFixture(path)
	if err != nil {
		t.Fatalf("LoadFixture: %v", err)
	}
	if fx.MatchMode != MatchSubstring {
		t.Errorf("MatchMode = %q, want %q", fx.MatchMode, MatchSubstring)
	}
	if fx.K != 10 {
		t.Errorf("K = %d, want 10", fx.K)
	}
	if len(fx.Queries) == 0 {
		t.Fatal("expected non-empty query set")
	}
	for _, q := range fx.Queries {
		if q.Intent == "" || q.Expected.Substring == "" {
			t.Errorf("query %q has empty intent or substring: %+v", q.ID, q)
		}
	}
}

// TestScoreSubstringMatch checks the substring-in-top-k scoring path: a
// query passes when a top-k hit's citation file CONTAINS the expect
// substring, and the found rank is the first such hit.
func TestScoreSubstringMatch(t *testing.T) {
	q := Query{ID: "q1", Intent: "x", Expected: Expected{Substring: "server.go"}}
	resp := &query.Response{
		Hits: []query.Hit{
			{Citation: types.Citation{File: "cache.go"}},
			{Citation: types.Citation{File: "pkg/net/server.go"}},
		},
	}
	got := Score(q, resp, 10, "")
	if got.FoundRank != 2 {
		t.Errorf("FoundRank = %d, want 2", got.FoundRank)
	}
	if !got.CitationCorrect {
		t.Error("expected CitationCorrect=true (pkg/net/server.go contains server.go)")
	}
	if got.ReciprocalRank < 0.49 || got.ReciprocalRank > 0.51 {
		t.Errorf("ReciprocalRank = %f, want ~0.5", got.ReciprocalRank)
	}

	// Miss: no top-k file contains the substring.
	miss := Query{ID: "q2", Intent: "x", Expected: Expected{Substring: "does-not-exist.go"}}
	gotMiss := Score(miss, resp, 10, "")
	if gotMiss.FoundRank != 0 {
		t.Errorf("miss FoundRank = %d, want 0", gotMiss.FoundRank)
	}
}

// TestRunSemanticAgainstSample runs a small JSON semantic fixture against
// the built sample index and asserts the aggregate pass count / rate. One
// query targets a substring guaranteed to appear (".go" — the corpus is
// Go-heavy); one targets a substring that cannot match any file.
func TestRunSemanticAgainstSample(t *testing.T) {
	eng, _ := newSampleEngine(t)
	body := `{
  "k": 10,
  "queries": [
    {"query": "start the http server and listen for connections", "expect": ".go", "note": "any go file"},
    {"query": "totally unrelated content zzzzz", "expect": "no-such-file-xyz.go", "note": "guaranteed miss"}
  ]
}`
	path := filepath.Join(t.TempDir(), "sem.json")
	if err := writeFile(t, path, body); err != nil {
		t.Fatal(err)
	}
	fx, err := LoadFixture(path)
	if err != nil {
		t.Fatalf("LoadFixture: %v", err)
	}
	res, err := Run(context.Background(), eng, fx, Options{K: fx.K, Threshold: -1})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if res.Aggregate.Total != 2 {
		t.Fatalf("Total = %d, want 2", res.Aggregate.Total)
	}

	byID := map[string]PerQuery{}
	for _, p := range res.PerQuery {
		byID[p.QueryID] = p
	}
	if byID["q1"].FoundRank == 0 {
		t.Errorf("q1 (.go substring) should match a Go file in top-10; got %+v", byID["q1"])
	}
	if byID["q2"].FoundRank != 0 {
		t.Errorf("q2 (no-such-file substring) must miss; got FoundRank=%d", byID["q2"].FoundRank)
	}
	// Pass count is queries with a substring hit in top-k.
	if res.Aggregate.Found != 1 {
		t.Errorf("Found = %d, want 1", res.Aggregate.Found)
	}
	passRate := float64(res.Aggregate.Found) / float64(res.Aggregate.Total)
	if passRate != 0.5 {
		t.Errorf("pass rate = %.3f, want 0.5", passRate)
	}
}
