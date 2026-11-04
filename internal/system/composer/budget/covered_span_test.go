package budget

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/composer/stage2"
	"github.com/0xmhha/knowledge-system/internal/system/composer/stage3"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// spanNeighbor is a neighbor whose target span is given, for the cases where
// the graph span of a function differs from its vector span.
func spanNeighbor(tgt string, start, end int, score float64) stage3.ScoredNeighbor {
	n := neighbor("s00.go", tgt, contract.RelationCalls, 1, score)
	n.Edge.Target = cit(tgt, start, end)
	// The graph cites the scenario commit; vector seeds keep the commit
	// their chunk was first indexed at.
	n.Edge.Target.CommitHash = "def"
	return n
}

// A neighbor inside a selected seed's span repeats code already in the pack
// and must not take a neighbor slot. One inside a seed the quota cut is new
// code and keeps its chance (go-stablenet pr-75: the owning function was a
// cut seed and a neighbor at once).
func TestAllocate_SkipsNeighborCoveredBySelectedBody(t *testing.T) {
	t.Parallel()
	seeds, f := seedsAndBodies(16) // s00..s09 fill the seed quota, s10..s15 are cut
	inSelected := spanNeighbor("s01.go", 3, 10, 0.05)
	inCut := spanNeighbor("s15.go", 3, 10, 0.045)
	wider := spanNeighbor("s02.go", 1, 20, 0.044)
	for _, n := range []stage3.ScoredNeighbor{inSelected, inCut, wider} {
		f.Bodies[n.Edge.Target.Key()] = bodyN(40)
	}
	a, err := New(f)
	if err != nil {
		t.Fatal(err)
	}
	out, err := a.Allocate(context.Background(), seeds, []stage3.ScoredNeighbor{inSelected, inCut, wider})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, s := range out.Selected {
		if s.Origin == OriginNeighbor {
			got[s.Citation.Key()] = true
		}
	}
	if got[inSelected.Edge.Target.Key()] {
		t.Errorf("neighbor inside selected s01.go body took a slot; neighbors %v", got)
	}
	if !got[inCut.Edge.Target.Key()] || !got[wider.Edge.Target.Key()] {
		t.Errorf("want the cut seed's span and the wider span as neighbors; got %v", got)
	}
}

// A seed below the quota that is also a neighbor of a stronger seed (same
// function, vector span from the doc comment, graph span from the func line)
// gains the neighbor's score and enters the pack (go-stablenet pr-75: the
// owning function was seed 16 and a direct callee of seeds 1 and 2).
func TestAllocate_SeedThatIsAlsoNeighborGainsGraphScore(t *testing.T) {
	t.Parallel()
	f := &FakeFetcher{Bodies: map[string]string{}}
	var seeds []stage2.ScoredCitation
	for i := range 14 {
		s := seed(fmt.Sprintf("s%02d.go", i), 0.08-float64(i)*0.001)
		f.Bodies[s.Citation.Key()] = bodyN(40)
		seeds = append(seeds, s)
	}
	same := spanNeighbor("s13.go", 2, 10, 0.04) // s13.go 1-10 under its graph span
	f.Bodies[same.Edge.Target.Key()] = bodyN(40)
	a, err := New(f)
	if err != nil {
		t.Fatal(err)
	}
	out, err := a.Allocate(context.Background(), seeds, []stage3.ScoredNeighbor{same})
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]SelectedItem{}
	for _, s := range out.Selected {
		got[s.Citation.Key()] = s
	}
	s13, ok := got[cit("s13.go", 1, 10).Key()]
	if !ok || s13.Origin != OriginSeed {
		t.Fatalf("seed s13.go not selected as a seed; selected %v", keys(got))
	}
	if !strings.HasPrefix(s13.Sources[len(s13.Sources)-1], "graph:same-function") {
		t.Errorf("s13.go sources %v, want a graph:same-function entry", s13.Sources)
	}
	if _, dup := got[same.Edge.Target.Key()]; dup {
		t.Errorf("the neighbor copy of s13.go was selected too; selected %v", keys(got))
	}
}

func keys(m map[string]SelectedItem) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
