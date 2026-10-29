package budget

import (
	"context"
	"fmt"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/composer/stage2"
	"github.com/0xmhha/knowledge-system/internal/system/composer/stage3"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

func seedsAndBodies(n int) ([]stage2.ScoredCitation, *FakeFetcher) {
	f := &FakeFetcher{Bodies: map[string]string{}}
	seeds := make([]stage2.ScoredCitation, 0, n)
	for i := range n {
		s := seed(fmt.Sprintf("s%02d.go", i), float64(100-i))
		f.Bodies[s.Citation.Key()] = bodyN(40)
		seeds = append(seeds, s)
	}
	return seeds, f
}

// With the default reserves and no neighbor, knowledge or exact-symbol
// candidate anywhere in the list, every slot goes to seeds: a slot held for a
// kind that never arrives only cuts the next seed. Measured 2026-10-27 on
// grpc-go: all 48 answers Stage 4 cut were cut from packs of 8 to 11 bodies.
func TestAllocate_ReservesReleaseWhenNothingEligibleIsAhead(t *testing.T) {
	t.Parallel()
	seeds, f := seedsAndBodies(16)
	a, err := New(f)
	if err != nil {
		t.Fatal(err)
	}
	out, err := a.Allocate(context.Background(), seeds, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Selected) != DefaultMaxCitations {
		t.Fatalf("Selected = %d, want %d (no eligible candidate for any reserve)", len(out.Selected), DefaultMaxCitations)
	}
}

// A reserve still holds a slot for a candidate of its kind that is ahead in
// the list, but only as many slots as there are such candidates.
func TestAllocate_NeighborReserveHoldsOnlyForNeighborsAhead(t *testing.T) {
	t.Parallel()
	seeds, f := seedsAndBodies(16)
	n := neighbor("s00.go", "n.go", contract.RelationCalls, 1, 0.5)
	f.Bodies[n.Edge.Target.Key()] = bodyN(40)
	a, err := New(f)
	if err != nil {
		t.Fatal(err)
	}
	out, err := a.Allocate(context.Background(), seeds, []stage3.ScoredNeighbor{n})
	if err != nil {
		t.Fatal(err)
	}
	neighbors := 0
	for _, s := range out.Selected {
		if s.Origin == OriginNeighbor {
			neighbors++
		}
	}
	if len(out.Selected) != DefaultMaxCitations || neighbors != 1 {
		t.Fatalf("Selected = %d with %d neighbors, want %d with 1", len(out.Selected), neighbors, DefaultMaxCitations)
	}
}
