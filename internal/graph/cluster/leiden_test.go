package cluster_test

import (
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/cluster"
)

// Two cliques connected by a single bridge edge — Leiden should detect
// exactly two communities at γ=1.0.
func TestLeidenTwoClusters(t *testing.T) {
	// Build edge list. Nodes 0..3 are clique A, 4..7 are clique B,
	// edge (3,4) is the bridge.
	edges := [][2]int{
		{0, 1}, {0, 2}, {0, 3}, {1, 2}, {1, 3}, {2, 3}, // A
		{4, 5}, {4, 6}, {4, 7}, {5, 6}, {5, 7}, {6, 7}, // B
		{3, 4}, // bridge
	}
	parts := cluster.RunLeiden(8, edges, cluster.LeidenOpts{Resolution: 1.0, Seed: 42, MaxIters: 50})
	if got := distinct(parts); got != 2 {
		t.Errorf("Leiden communities = %d, want 2", got)
	}
}

func distinct(p []int) int {
	m := map[int]struct{}{}
	for _, x := range p {
		m[x] = struct{}{}
	}
	return len(m)
}

// TestRunLeiden_Deterministic locks the run-to-run reproducibility of the
// seeded clustering: Go map iteration order must not leak into tie-breaking.
// The fixture is deliberately tie-heavy — two symmetric triangles bridged by
// one edge — so equal-gain candidates occur and order-sensitive tie-breaks
// would diverge across repetitions.
func TestRunLeiden_Deterministic(t *testing.T) {
	edges := [][2]int{
		{0, 1}, {1, 2}, {2, 0}, // triangle A
		{3, 4}, {4, 5}, {5, 3}, // triangle B (symmetric twin)
		{2, 3}, // bridge
	}
	first := cluster.RunLeiden(6, edges, cluster.LeidenOpts{Resolution: 1.0, Seed: 42, MaxIters: 50})
	for i := 0; i < 50; i++ {
		got := cluster.RunLeiden(6, edges, cluster.LeidenOpts{Resolution: 1.0, Seed: 42, MaxIters: 50})
		for j := range got {
			if got[j] != first[j] {
				t.Fatalf("run %d diverged at node %d: %v vs %v", i, j, got, first)
			}
		}
	}
}
