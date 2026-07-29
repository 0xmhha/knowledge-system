package stage2

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

func TestIsDocCitation(t *testing.T) {
	t.Parallel()
	cases := []struct {
		chunkKind, file string
		want            bool
	}{
		{"doc", "docs/graph/ARCHITECTURE.md", true},
		{"doc", "weird/extensionless", true}, // kind label wins
		{"", "docs/vector/adr/004.md", true}, // suffix fallback
		{"", "README.markdown", true},
		{"symbol", "internal/system/composer/composer.go", false},
		{"", "internal/system/composer/composer.go", false},
	}
	for _, c := range cases {
		if got := isDocCitation(c.chunkKind, c.file); got != c.want {
			t.Errorf("isDocCitation(%q, %q) = %v, want %v", c.chunkKind, c.file, got, c.want)
		}
	}
}

func TestIsArchivePath(t *testing.T) {
	t.Parallel()
	cases := []struct {
		file string
		want bool
	}{
		{"docs/vector/archive/featurelist.md", true},
		{"archive/old.md", true},
		{"system/docs/archive/system-review-2026-05-27.md", true},
		{"docs/vector/ARCHITECTURE.md", false},
		{"internal/system/composer/composer.go", false},
	}
	for _, c := range cases {
		if got := isArchivePath(c.file); got != c.want {
			t.Errorf("isArchivePath(%q) = %v, want %v", c.file, got, c.want)
		}
	}
}

func TestDemoteDocsFor(t *testing.T) {
	t.Parallel()
	keep := []contract.Intent{contract.IntentDocsUpdate, contract.IntentArchExplain, contract.IntentUnknown}
	for _, in := range keep {
		if demoteDocsFor(in) {
			t.Errorf("demoteDocsFor(%s) = true, want false", in)
		}
	}
	demote := []contract.Intent{contract.IntentBugFix, contract.IntentQAReview, contract.IntentSecurity, contract.IntentRefactor}
	for _, in := range demote {
		if !demoteDocsFor(in) {
			t.Errorf("demoteDocsFor(%s) = false, want true", in)
		}
	}
}

// TestResults_DemotesDocsForCodeIntents mirrors the test-file demotion
// regression: equal raw contributions, doc citation must rank below code
// when demoteDocs is on, and above-board when off.
func TestResults_DemotesDocsForCodeIntents(t *testing.T) {
	t.Parallel()
	build := func() *aggregator {
		a := newAggregator(DefaultRRFK, DefaultBMWeight, DefaultSymbolWeight, DefaultCkvWeight)
		// Two single-element ckv lists give identical contributions; the
		// doc hit carries ckv's "doc" chunk-kind label.
		code := contract.Hit{Citation: cit("internal/system/composer/composer.go", 55, 58), Source: contract.HitSourceCKV, ChunkKind: "symbol"}
		doc := contract.Hit{Citation: cit("system/docs/design-spec.md", 1, 40), Source: contract.HitSourceCKV, ChunkKind: "doc"}
		a.addCkvList([]contract.Hit{doc})
		a.addCkvList([]contract.Hit{code})
		return a
	}

	out := build().results(0, false, true /* demoteDocs */)
	if len(out) != 2 {
		t.Fatalf("results count = %d, want 2", len(out))
	}
	if out[0].Citation.File != "internal/system/composer/composer.go" {
		t.Errorf("rank-1 = %q, want the code citation", out[0].Citation.File)
	}
	if out[0].Score <= out[1].Score {
		t.Errorf("code score (%v) should exceed demoted doc score (%v)", out[0].Score, out[1].Score)
	}

	// Flag off: original tie order (deterministic file-path tiebreak).
	off := build().results(0, false, false)
	if off[0].Score != off[1].Score {
		t.Errorf("without demotion scores should tie: %v vs %v", off[0].Score, off[1].Score)
	}
}

// TestResults_ArchiveDemotedRegardlessOfFlags pins the always-on archive
// demotion: even with demoteDocs off (e.g. IntentArchExplain), archived
// material ranks below a live doc with an equal raw score.
func TestResults_ArchiveDemotedRegardlessOfFlags(t *testing.T) {
	t.Parallel()
	a := newAggregator(DefaultRRFK, DefaultBMWeight, DefaultSymbolWeight, DefaultCkvWeight)
	live := contract.Hit{Citation: cit("docs/vector/ARCHITECTURE.md", 10, 30), Source: contract.HitSourceCKV, ChunkKind: "doc"}
	archived := contract.Hit{Citation: cit("docs/vector/archive/featurelist.md", 264, 269), Source: contract.HitSourceCKV, ChunkKind: "doc"}
	a.addCkvList([]contract.Hit{archived})
	a.addCkvList([]contract.Hit{live})

	out := a.results(0, false, false)
	if len(out) != 2 {
		t.Fatalf("results count = %d, want 2", len(out))
	}
	if out[0].Citation.File != "docs/vector/ARCHITECTURE.md" {
		t.Errorf("rank-1 = %q, want the live doc above the archived one", out[0].Citation.File)
	}
	if out[0].Score <= out[1].Score {
		t.Errorf("live doc score (%v) should exceed archived score (%v)", out[0].Score, out[1].Score)
	}
}

// TestResults_DemotesHeaderChunksForCodeIntents pins the file_header
// demotion: with equal raw contributions, a symbol chunk must outrank
// the same file's header chunk when demoteDocs is on.
func TestResults_DemotesHeaderChunksForCodeIntents(t *testing.T) {
	t.Parallel()
	a := newAggregator(DefaultRRFK, DefaultBMWeight, DefaultSymbolWeight, DefaultCkvWeight)
	header := contract.Hit{Citation: cit("pkg/system/contract/intent.go", 1, 50), Source: contract.HitSourceCKV, ChunkKind: "file_header"}
	symbol := contract.Hit{Citation: cit("pkg/system/contract/intent.go", 35, 157), Source: contract.HitSourceCKV, ChunkKind: "symbol"}
	a.addCkvList([]contract.Hit{header})
	a.addCkvList([]contract.Hit{symbol})

	out := a.results(0, false, true /* demoteDocs */)
	if len(out) != 2 {
		t.Fatalf("results count = %d, want 2", len(out))
	}
	if out[0].ChunkKind != "symbol" {
		t.Errorf("rank-1 chunk kind = %q, want symbol above file_header", out[0].ChunkKind)
	}
	if out[0].Score <= out[1].Score {
		t.Errorf("symbol score (%v) should exceed demoted header score (%v)", out[0].Score, out[1].Score)
	}

	// Flag off: tie preserved.
	b := newAggregator(DefaultRRFK, DefaultBMWeight, DefaultSymbolWeight, DefaultCkvWeight)
	b.addCkvList([]contract.Hit{header})
	b.addCkvList([]contract.Hit{symbol})
	off := b.results(0, false, false)
	if off[0].Score != off[1].Score {
		t.Errorf("without demotion scores should tie: %v vs %v", off[0].Score, off[1].Score)
	}
}
