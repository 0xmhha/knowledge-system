package budget

import (
	"context"
	"fmt"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/composer/stage2"
	"github.com/0xmhha/knowledge-system/internal/system/composer/stage3"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// Neighbors that tie on score keep Stage 3's order (score, then larger span)
// instead of being re-sorted by file and line. Measured 2026-10-27 on grpc-go
// pr-8875 (bge-m3): the answer was Stage 3 neighbor 2, a 30-line function, and
// lost the four neighbor slots to shorter spans of the same file that start on
// lower lines.
func TestMergeCandidates_TiesKeepStage3Order(t *testing.T) {
	t.Parallel()
	nb := func(file string, start, end int) stage3.ScoredNeighbor {
		return stage3.ScoredNeighbor{
			Edge:  contract.Neighbor{Source: cit("seed.go", 1, 10), Target: cit(file, start, end), Relation: contract.RelationCalls, Distance: 1},
			Score: 0.5,
		}
	}
	// Stage 3 order: the long body first, then the short helpers.
	neighbors := []stage3.ScoredNeighbor{nb("z.go", 259, 288), nb("a.go", 45, 53), nb("z.go", 85, 87)}
	got := mergeCandidates([]stage2.ScoredCitation{seed("s.go", 0.5)}, neighbors)
	want := []string{"s.go:1", "z.go:259", "a.go:45", "z.go:85"}
	if len(got) != len(want) {
		t.Fatalf("candidates = %d, want %d", len(got), len(want))
	}
	for i, c := range got {
		if k := fmt.Sprintf("%s:%d", c.Citation.File, c.Citation.StartLine); k != want[i] {
			t.Errorf("candidate %d = %s, want %s (seed first on a tie, then Stage 3 order)", i, k, want[i])
		}
	}

	f := &FakeFetcher{Bodies: map[string]string{}}
	for _, c := range got {
		f.Bodies[c.Citation.Key()] = bodyN(40)
	}
	a, err := New(f, WithConfig(Config{MaxTokens: 10000, MaxCitations: 2}))
	if err != nil {
		t.Fatal(err)
	}
	out, err := a.Allocate(context.Background(), []stage2.ScoredCitation{seed("s.go", 0.5)}, neighbors)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Selected) != 2 || out.Selected[1].Citation.File != "z.go" || out.Selected[1].Citation.StartLine != 259 {
		t.Fatalf("second selected = %+v, want z.go:259 (Stage 3's first neighbor)", out.Selected)
	}
}
