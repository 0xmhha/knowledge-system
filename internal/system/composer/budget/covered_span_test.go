package budget

import (
	"context"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/composer/stage3"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// spanNeighbor is a neighbor whose target span is given, for the cases where
// the graph span of a function differs from its vector span.
func spanNeighbor(tgt string, start, end int, score float64) stage3.ScoredNeighbor {
	n := neighbor("s00.go", tgt, contract.RelationCalls, 1, score)
	n.Edge.Target = cit(tgt, start, end)
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
