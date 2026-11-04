package budget

import (
	"context"
	"fmt"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/composer/stage2"
	"github.com/0xmhha/knowledge-system/internal/system/composer/stage3"
)

// Seeds the quota cuts are reported in rank order with their symbol, so the
// pack can list them (go-stablenet pr-67 and pr-73, bge-m3: the owning
// functions were seeds 19 and 16 and appeared nowhere in the pack). A cut
// seed whose function got a body under another span is not reported.
func TestAllocate_ReportsCutSeeds(t *testing.T) {
	t.Parallel()
	f := &FakeFetcher{Bodies: map[string]string{}}
	var seeds []stage2.ScoredCitation
	for i := range 14 {
		s := seed(fmt.Sprintf("s%02d.go", i), 1.0-float64(i)*0.01)
		s.Symbol = fmt.Sprintf("Fn%02d", i)
		f.Bodies[s.Citation.Key()] = bodyN(40)
		seeds = append(seeds, s)
	}
	inv := knowledgeSeed("rules.md", 0.001, "invariant")
	f.Bodies[inv.Citation.Key()] = bodyN(5)
	// s12.go is cut as a seed but its function enters as a neighbor.
	same := spanNeighbor("s12.go", 2, 10, 0.5)
	f.Bodies[same.Edge.Target.Key()] = bodyN(40)
	a, err := New(f)
	if err != nil {
		t.Fatal(err)
	}
	out, err := a.Allocate(context.Background(), append(seeds, inv), []stage3.ScoredNeighbor{same})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, cs := range out.CutSeeds {
		got = append(got, fmt.Sprintf("%s#%d:%s", cs.Symbol, cs.Rank, cs.Reason))
	}
	want := []string{"Fn10#11:seed_quota", "Fn11#12:seed_quota", "Fn13#14:seed_quota"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("CutSeeds = %v, want %v", got, want)
	}
}
