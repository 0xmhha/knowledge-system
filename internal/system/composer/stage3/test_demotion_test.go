package stage3

import (
	"context"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/internal/system/composer/stage2"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// A test-file neighbor gets Stage 2's test demotion when Stage 2 demoted
// tests, so production callers rank above test functions that call the same
// seed. Measured 2026-10-27 on grpc-go pr-8792 (qwen3): 19 of the 20
// neighbors above the answer caller were test functions reached by called_by.
func TestExpand_DemotesTestNeighborsWhenStage2Did(t *testing.T) {
	t.Parallel()
	seeds := []stage2.ScoredCitation{seed("watch.go", 1, 10, 4.0)}
	ckg := &ckgclient.Fake{NeighborEdges: []contract.Neighbor{
		neighbor("watch.go", "watch_test.go", contract.RelationCalls, 1),
		neighbor("watch.go", "resolver.go", contract.RelationCalls, 1),
	}}
	e, _ := New(ckg)

	out, err := e.Expand(context.Background(), seeds, contract.IntentUnknown, DemoteTestNeighbors(true))
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Neighbors) != 2 || out.Neighbors[0].Edge.Target.File != "resolver.go" {
		t.Fatalf("Neighbors = %+v, want resolver.go first", out.Neighbors)
	}
	if got, want := out.Neighbors[1].Score, out.Neighbors[0].Score*stage2.TestDemotionFactor; got != want {
		t.Errorf("test neighbor score = %v, want %v", got, want)
	}

	// Without the option (a test_add intent, or a prompt about tests) the
	// scores stay equal.
	out, err = e.Expand(context.Background(), seeds, contract.IntentUnknown)
	if err != nil {
		t.Fatal(err)
	}
	if out.Neighbors[0].Score != out.Neighbors[1].Score {
		t.Errorf("scores = %v, %v, want equal without test demotion", out.Neighbors[0].Score, out.Neighbors[1].Score)
	}
}
