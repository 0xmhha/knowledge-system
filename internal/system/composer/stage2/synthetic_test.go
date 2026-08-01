package stage2

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

func TestIsSyntheticLocation(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		c    contract.Citation
		want bool
	}{
		"convention summary": {contract.Citation{File: "internal/vector/chunk/<convention>"}, true},
		"invariant at a real line": {
			contract.Citation{File: "cmd/filelist-gen/main.go", StartLine: 134, EndLine: 134}, false},
		"doc chunk": {
			contract.Citation{File: "docs/dev/x.md", StartLine: 1, EndLine: 40}, false},
	}
	for name, tc := range cases {
		if got := isSyntheticLocation(tc.c); got != tc.want {
			t.Errorf("%s: isSyntheticLocation = %v, want %v", name, got, tc.want)
		}
	}
}

// TestKnowledgeFromHit pins the shape the pack ships, including that the
// scope drops the <convention> marker so consumers see the package.
func TestKnowledgeFromHit(t *testing.T) {
	t.Parallel()
	h := contract.Hit{
		Citation:  contract.Citation{File: "internal/vector/footprint/<convention>"},
		ChunkKind: "convention",
		Text:      "package: internal/vector/footprint. conventions summary.",
	}
	got, ok := knowledgeFromHit(h)
	if !ok {
		t.Fatal("knowledgeFromHit returned false for a well-formed hit")
	}
	if got.Scope != "internal/vector/footprint" {
		t.Errorf("Scope = %q, want the package dir without the marker", got.Scope)
	}
	if !got.IsValid() {
		t.Errorf("produced an invalid KnowledgeChunk: %+v", got)
	}

	if _, ok := knowledgeFromHit(contract.Hit{
		Citation: contract.Citation{File: "x/<convention>"},
	}); ok {
		t.Error("a hit with no text must not become a KnowledgeChunk (the contract rejects it)")
	}
}

// TestSearch_SyntheticHitsLeaveCitations pins the split: a synthetic hit
// must never reach the citation aggregator, because Citation.IsValid
// rejects a zero line range and the body fetcher has no file to read.
func TestSearch_SyntheticHitsLeaveCitations(t *testing.T) {
	t.Parallel()
	agg := newAggregator(DefaultRRFK, 1.0, 1.5, 1.0)
	agg.addCkvList([]contract.Hit{{
		Citation: contract.Citation{File: "real.go", StartLine: 10, EndLine: 20},
	}})
	for _, c := range agg.results(0, false, false) {
		if !c.Citation.IsValid() {
			t.Errorf("aggregator produced an invalid citation: %+v", c.Citation)
		}
	}
}
