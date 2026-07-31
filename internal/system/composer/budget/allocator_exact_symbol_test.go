package budget

import (
	"context"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/composer/stage2"
)

// exactSymbolSeed is a seed whose citation came from the prompt-verbatim,
// unambiguous FindSymbol gate in stage2.
func exactSymbolSeed(file string, score float64) stage2.ScoredCitation {
	sc := seed(file, score)
	sc.Sources = []string{stage2.SourceExactSymbolPrefix + "EnableBM25Rerank@rank=1(+0.04918)"}
	return sc
}

// TestAllocate_ExactSymbolReserveRescuesLowRankedDeclaration pins the
// starvation this reserve exists for. Measured 2026-07-31 on
// bm25-rerank-option: FindSymbol resolved EnableBM25Rerank to exactly one
// node and the boost fired, but the declaration is a single line
// competing with whole bodies from files named after the same concept, so
// the greedy pass dropped it and the scenario scored R 0.00.
func TestAllocate_ExactSymbolReserveRescuesLowRankedDeclaration(t *testing.T) {
	t.Parallel()
	seeds := []stage2.ScoredCitation{
		seed("rerank.go", 9.0),
		seed("service_rerank.go", 8.0),
		seed("rerank_test.go", 7.0),
		seed("bench.go", 6.0),
		exactSymbolSeed("engine.go", 0.5),
	}
	fetcher := &FakeFetcher{Bodies: map[string]string{
		cit("rerank.go", 1, 10).Key():         bodyN(10),
		cit("service_rerank.go", 1, 10).Key(): bodyN(10),
		cit("rerank_test.go", 1, 10).Key():    bodyN(10),
		cit("bench.go", 1, 10).Key():          bodyN(10),
		cit("engine.go", 1, 10).Key():         bodyN(10),
	}}
	a, _ := New(fetcher, WithConfig(Config{
		MaxTokens: 100000, OverheadReserve: 0.10,
		MaxCitations: 3, ExactSymbolReserve: 1,
	}))
	out, err := a.Allocate(context.Background(), seeds, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Selected) != 3 {
		t.Fatalf("Selected = %d, want 3", len(out.Selected))
	}
	got := map[string]bool{}
	for _, s := range out.Selected {
		got[s.Citation.File] = true
	}
	if !got["engine.go"] {
		t.Fatalf("exact-symbol declaration not selected; got %v", got)
	}
	// The held slot displaces exactly one higher-scoring body.
	if got["rerank_test.go"] {
		t.Errorf("rerank_test.go selected despite the exact-symbol holdback; got %v", got)
	}
}

// TestAllocate_ExactSymbolReserveOffKeepsGreedyOrder pins that the
// reserve is opt-in: at 0 the allocator behaves exactly as before.
func TestAllocate_ExactSymbolReserveOffKeepsGreedyOrder(t *testing.T) {
	t.Parallel()
	seeds := []stage2.ScoredCitation{
		seed("rerank.go", 9.0),
		seed("service_rerank.go", 8.0),
		seed("rerank_test.go", 7.0),
		exactSymbolSeed("engine.go", 0.5),
	}
	fetcher := &FakeFetcher{Bodies: map[string]string{
		cit("rerank.go", 1, 10).Key():         bodyN(10),
		cit("service_rerank.go", 1, 10).Key(): bodyN(10),
		cit("rerank_test.go", 1, 10).Key():    bodyN(10),
		cit("engine.go", 1, 10).Key():         bodyN(10),
	}}
	a, _ := New(fetcher, WithConfig(Config{
		MaxTokens: 100000, OverheadReserve: 0.10,
		MaxCitations: 3, ExactSymbolReserve: 0,
	}))
	out, err := a.Allocate(context.Background(), seeds, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range out.Selected {
		if s.Citation.File == "engine.go" {
			t.Fatalf("engine.go selected with the reserve disabled; got %v", out.Selected)
		}
	}
}

// TestHasExactSymbolSource pins the provenance predicate the allocator
// routes on, including that an ordinary symbol hit does not match.
func TestHasExactSymbolSource(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		sources []string
		want    bool
	}{
		"exact":        {[]string{"symbol_exact:Foo@rank=1(+0.5)"}, true},
		"plain symbol": {[]string{"symbol:Foo@rank=1(+0.2)"}, false},
		"bm25 only":    {[]string{"bm25:foo@rank=3(+0.1)"}, false},
		"mixed":        {[]string{"bm25:foo@rank=3(+0.1)", "symbol_exact:Foo@rank=1(+0.5)"}, true},
		"empty":        {nil, false},
		"short string": {[]string{"sym"}, false},
		// A tag with no keyword after it carries no identity to reserve
		// per, so it does not count as an exact-symbol source.
		"prefix only": {[]string{"symbol_exact:"}, false},
		// The trailing colon is what keeps a look-alike tag from matching.
		"look-alike": {[]string{"symbol_exactly_not:Foo"}, false},
	}
	for name, tc := range cases {
		if got := stage2.HasExactSymbolSource(tc.sources); got != tc.want {
			t.Errorf("%s: HasExactSymbolSource(%v) = %v, want %v", name, tc.sources, got, tc.want)
		}
	}
}

// TestAllocate_ExactSymbolReservePerKeyword pins the per-keyword rule.
// Measured 2026-07-31: a prompt naming both EnableBM25Rerank and BM25
// produced two exact-symbol citations, and a single global slot let the
// higher-scoring one evict the other — bm25-rerank-option stayed at
// R 0.00 until the reserve became one slot per distinct symbol.
func TestAllocate_ExactSymbolReservePerKeyword(t *testing.T) {
	t.Parallel()
	withKeyword := func(file string, score float64, kw string) stage2.ScoredCitation {
		sc := seed(file, score)
		sc.Sources = []string{stage2.SourceExactSymbolPrefix + kw + "@rank=1(+0.04918)"}
		return sc
	}
	seeds := []stage2.ScoredCitation{
		seed("a.go", 9.0),
		seed("b.go", 8.0),
		withKeyword("rerank.go", 1.0, "BM25"),
		withKeyword("engine.go", 0.5, "EnableBM25Rerank"),
	}
	fetcher := &FakeFetcher{Bodies: map[string]string{
		cit("a.go", 1, 10).Key():      bodyN(10),
		cit("b.go", 1, 10).Key():      bodyN(10),
		cit("rerank.go", 1, 10).Key(): bodyN(10),
		cit("engine.go", 1, 10).Key(): bodyN(10),
	}}
	a, _ := New(fetcher, WithConfig(Config{
		MaxTokens: 100000, OverheadReserve: 0.10,
		MaxCitations: 3, ExactSymbolReserve: 2,
	}))
	out, err := a.Allocate(context.Background(), seeds, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, s := range out.Selected {
		got[s.Citation.File] = true
	}
	if !got["engine.go"] {
		t.Errorf("second verbatim symbol evicted by the first; got %v", got)
	}
}

func TestExactSymbolKeyword(t *testing.T) {
	t.Parallel()
	cases := map[string]string{
		"symbol_exact:EnableBM25Rerank@rank=1(+0.5)": "EnableBM25Rerank",
		"symbol_exact:BM25@rank=2(+0.1)":             "BM25",
		"symbol_exact:NoAtSign":                      "NoAtSign",
		"symbol:Plain@rank=1(+0.2)":                  "",
		"bm25:kw@rank=1(+0.1)":                       "",
	}
	for src, want := range cases {
		if got := stage2.ExactSymbolKeyword([]string{src}); got != want {
			t.Errorf("ExactSymbolKeyword(%q) = %q, want %q", src, got, want)
		}
	}
}
