package budget

import (
	"context"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/composer/stage2"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// knowledgeSeed is a seed whose hit came from the kind-scoped knowledge
// pass (invariant/convention chunk).
func knowledgeSeed(file string, score float64, kind string) stage2.ScoredCitation {
	sc := seed(file, score)
	sc.ChunkKind = kind
	return sc
}

func TestAllocate_KnowledgeReserveRescuesLowRankedInvariant(t *testing.T) {
	t.Parallel()
	// 4 code seeds outscore 1 invariant seed; MaxCitations=3 with
	// KnowledgeReserve=1. Without the reserve the total cap fires at the
	// three code seeds and the invariant never gets processed — the exact
	// starvation shape the neighbor reserve fixed one level down.
	seeds := []stage2.ScoredCitation{
		seed("a.go", 9.0),
		seed("b.go", 8.0),
		seed("c.go", 7.0),
		seed("d.go", 6.0),
		knowledgeSeed("rules.md", 0.5, "invariant"),
	}
	fetcher := &FakeFetcher{Bodies: map[string]string{
		cit("a.go", 1, 10).Key():     bodyN(10),
		cit("b.go", 1, 10).Key():     bodyN(10),
		cit("c.go", 1, 10).Key():     bodyN(10),
		cit("d.go", 1, 10).Key():     bodyN(10),
		cit("rules.md", 1, 10).Key(): bodyN(10),
	}}
	a, _ := New(fetcher, WithConfig(Config{
		MaxTokens: 100000, OverheadReserve: 0.10,
		MaxCitations: 3, KnowledgeReserve: 1,
	}))
	out, err := a.Allocate(context.Background(), seeds, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Selected) != 3 {
		t.Fatalf("Selected = %d, want 3", len(out.Selected))
	}
	got := map[string]string{}
	for _, s := range out.Selected {
		got[s.Citation.File] = s.ChunkKind
	}
	if _, ok := got["rules.md"]; !ok {
		t.Fatalf("invariant chunk not selected; got %v", got)
	}
	if got["rules.md"] != "invariant" {
		t.Errorf("SelectedItem.ChunkKind = %q, want invariant (kind must survive into Stage4Output)", got["rules.md"])
	}
	// The held slot displaces exactly one code seed: a.go + b.go stay, c.go loses.
	if _, ok := got["c.go"]; ok {
		t.Errorf("c.go selected despite knowledge holdback; got %v", got)
	}
}

func TestAllocate_KnowledgeReserveExemptsSeedCap(t *testing.T) {
	t.Parallel()
	// seedCap = MaxCitations - NeighborReserve = 1. The invariant seed is
	// past the seed quota but must still enter via the reserve exemption —
	// domain rules must not lose their slot to yet another code body.
	//
	// The fixture used to be a "convention" chunk. Those cannot reach the
	// allocator since #72 (built at line 0-0, split into the pack's
	// knowledge section before the citation aggregator), so the case is
	// pinned with the kind that actually fills the reserve: invariant
	// chunks took 16 body slots across the fifteen scenarios when sampled
	// on 2026-08-03.
	seeds := []stage2.ScoredCitation{
		seed("a.go", 9.0),
		seed("b.go", 8.0),
		knowledgeSeed("rules.md", 0.5, "invariant"),
	}
	fetcher := &FakeFetcher{Bodies: map[string]string{
		cit("a.go", 1, 10).Key():     bodyN(10),
		cit("b.go", 1, 10).Key():     bodyN(10),
		cit("rules.md", 1, 10).Key(): bodyN(10),
	}}
	a, _ := New(fetcher, WithConfig(Config{
		MaxTokens: 100000, OverheadReserve: 0.10,
		MaxCitations: 2, NeighborReserve: 1, KnowledgeReserve: 1,
	}))
	out, err := a.Allocate(context.Background(), seeds, nil)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]bool{}
	for _, s := range out.Selected {
		files[s.Citation.File] = true
	}
	if !files["rules.md"] {
		t.Fatalf("convention chunk lost to seed cap; selected %v", files)
	}
}

func TestAllocate_NoKnowledgeCandidatesLeavesReserveUnused(t *testing.T) {
	t.Parallel()
	// The holdback is unconditional: it does not look ahead to see
	// whether a knowledge candidate is still coming, so with none in the
	// list the reserve goes unfilled and selection stops one short of the
	// cap — the same semantics as the neighbor reserve with no neighbors.
	//
	// The prose here used to open by claiming the opposite ("only while
	// knowledge candidates could still appear") while asserting this, so
	// it was measured both ways on 2026-08-03. Adding the lookahead, and
	// separately dropping KnowledgeReserve to 0, both recover the slot
	// and both cost composer-pipeline-flow the identical 0.0038 MRR,
	// because the extra body pushes an edge-only citation down. Recall
	// does not move either way. The unconditional form is what ships.
	seeds := []stage2.ScoredCitation{
		seed("a.go", 9.0),
		seed("b.go", 8.0),
		seed("c.go", 7.0),
	}
	fetcher := &FakeFetcher{Bodies: map[string]string{
		cit("a.go", 1, 10).Key(): bodyN(10),
		cit("b.go", 1, 10).Key(): bodyN(10),
		cit("c.go", 1, 10).Key(): bodyN(10),
	}}
	a, _ := New(fetcher, WithConfig(Config{
		MaxTokens: 100000, OverheadReserve: 0.10,
		MaxCitations: 3, KnowledgeReserve: 1,
	}))
	out, err := a.Allocate(context.Background(), seeds, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Selected) != 2 {
		t.Fatalf("Selected = %d, want 2 (one slot held for absent knowledge)", len(out.Selected))
	}
}

func TestDefaultConfig_KnowledgeReserve(t *testing.T) {
	t.Parallel()
	if DefaultConfig().KnowledgeReserve != DefaultKnowledgeReserve || DefaultKnowledgeReserve != 2 {
		t.Errorf("DefaultConfig.KnowledgeReserve = %d, want %d", DefaultConfig().KnowledgeReserve, DefaultKnowledgeReserve)
	}
}

// A domain-knowledge corpus chunk is indexed by ckv as chunk_kind "doc" with an
// empty commit hash (out-of-tree markdown), not "invariant"/"convention". The
// reserve must still rescue it, otherwise real domain rules never reach the pack
// under the citation cap — the production symptom this fix addresses (live
// get_for_task shipped 0 invariants for a generic prompt).
func TestAllocate_KnowledgeReserveRescuesDocKindCorpusChunk(t *testing.T) {
	t.Parallel()
	docCit := contract.Citation{File: "A6.rules.md", StartLine: 1, EndLine: 10, CommitHash: ""}
	docSeed := stage2.ScoredCitation{Citation: docCit, Score: 0.5, ChunkKind: "doc"}
	seeds := []stage2.ScoredCitation{
		seed("a.go", 9.0),
		seed("b.go", 8.0),
		seed("c.go", 7.0),
		seed("d.go", 6.0),
		docSeed,
	}
	fetcher := &FakeFetcher{Bodies: map[string]string{
		cit("a.go", 1, 10).Key(): bodyN(10),
		cit("b.go", 1, 10).Key(): bodyN(10),
		cit("c.go", 1, 10).Key(): bodyN(10),
		cit("d.go", 1, 10).Key(): bodyN(10),
		docCit.Key():             bodyN(10),
	}}
	a, _ := New(fetcher, WithConfig(Config{
		MaxTokens: 100000, OverheadReserve: 0.10,
		MaxCitations: 3, KnowledgeReserve: 1,
	}))
	out, err := a.Allocate(context.Background(), seeds, nil)
	if err != nil {
		t.Fatal(err)
	}
	files := map[string]bool{}
	for _, s := range out.Selected {
		files[s.Citation.File] = true
	}
	if !files["A6.rules.md"] {
		t.Fatalf("out-of-tree doc corpus chunk not rescued by knowledge reserve; selected %v", files)
	}
}

// Guard against over-inclusion: in-tree markdown (a README) is also chunk_kind
// "doc" but carries a real commit hash, so it must NOT be treated as knowledge —
// otherwise generic docs would steal the reserve. With no true knowledge
// candidate present the held slot goes unused and the README is not selected.
func TestAllocate_InTreeDocIsNotKnowledge(t *testing.T) {
	t.Parallel()
	readmeCit := contract.Citation{File: "README.md", StartLine: 1, EndLine: 10, CommitHash: "abc"}
	readmeSeed := stage2.ScoredCitation{Citation: readmeCit, Score: 0.5, ChunkKind: "doc"}
	seeds := []stage2.ScoredCitation{
		seed("a.go", 9.0),
		seed("b.go", 8.0),
		seed("c.go", 7.0),
		readmeSeed,
	}
	fetcher := &FakeFetcher{Bodies: map[string]string{
		cit("a.go", 1, 10).Key(): bodyN(10),
		cit("b.go", 1, 10).Key(): bodyN(10),
		cit("c.go", 1, 10).Key(): bodyN(10),
		readmeCit.Key():          bodyN(10),
	}}
	a, _ := New(fetcher, WithConfig(Config{
		MaxTokens: 100000, OverheadReserve: 0.10,
		MaxCitations: 3, KnowledgeReserve: 1,
	}))
	out, err := a.Allocate(context.Background(), seeds, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range out.Selected {
		if s.Citation.File == "README.md" {
			t.Fatalf("in-tree doc (real commit) wrongly rescued as knowledge; selected %v", out.Selected)
		}
	}
}
