package ckgclient

import (
	"context"
	"reflect"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// chain: S calls A, A calls B, B calls C; S also calls D directly.
func chainStore() *mockStoreReader {
	s := node("S", "pkg.S", "s.go", 1, 9, types.NodeFunction, "go")
	return &mockStoreReader{
		manifest:  ManifestSnapshot{SrcCommit: "h"},
		pathNodes: []types.Node{s},
		neighOut: []types.Node{s,
			node("A", "pkg.A", "a.go", 1, 9, types.NodeFunction, "go"),
			node("B", "pkg.B", "b.go", 1, 9, types.NodeFunction, "go"),
			node("C", "pkg.C", "c.go", 1, 9, types.NodeFunction, "go"),
			node("D", "pkg.D", "d.go", 1, 9, types.NodeFunction, "go"),
		},
		neighEdges: []types.Edge{
			edge("S", "A", types.EdgeCalls), edge("A", "B", types.EdgeCalls),
			edge("B", "C", types.EdgeCalls), edge("S", "D", types.EdgeCalls),
			edge("C", "S", types.EdgeCalls), // a cycle back to the seed
		},
	}
}

func targets(out []contract.Neighbor) map[string]int {
	m := map[string]int{}
	for _, n := range out {
		m[n.Target.File] = n.Distance
	}
	return m
}

// TestReal_Neighbors_DistanceIsTheHop: every neighbour carries the hop it was
// reached at. Before the fix every edge said Distance 1, so the composer
// scored a 3-hop callee like a direct one.
func TestReal_Neighbors_DistanceIsTheHop(t *testing.T) {
	t.Parallel()
	r := newRealWithStore(chainStore())
	out, err := r.Neighbors(context.Background(), contract.Citation{File: "s.go", StartLine: 1, EndLine: 9, CommitHash: "h"},
		NeighborsOpts{Hops: 3, Relations: []contract.Relation{contract.RelationCalls}})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{"a.go": 1, "d.go": 1, "b.go": 2, "c.go": 3}
	if got := targets(out); !reflect.DeepEqual(got, want) {
		t.Fatalf("distances = %v, want %v (the cycle back to the seed is not a neighbour)", got, want)
	}
	for i := 1; i < len(out); i++ {
		if out[i].Distance < out[i-1].Distance {
			t.Fatalf("neighbours not ordered by hop: %+v", out)
		}
	}
}

// TestReal_Neighbors_CapIsSharedBetweenHops: with a cap, direct neighbours
// do not crowd out the farther hops.
func TestReal_Neighbors_CapIsSharedBetweenHops(t *testing.T) {
	t.Parallel()
	s := node("S", "pkg.S", "s.go", 1, 9, types.NodeFunction, "go")
	m := &mockStoreReader{manifest: ManifestSnapshot{SrcCommit: "h"}, pathNodes: []types.Node{s}, neighOut: []types.Node{s}}
	// S calls H1..H4; H1 calls F1..F4.
	for _, id := range []string{"H1", "H2", "H3", "H4"} {
		m.neighOut = append(m.neighOut, node(id, "pkg."+id, id+".go", 1, 9, types.NodeFunction, "go"))
		m.neighEdges = append(m.neighEdges, edge("S", id, types.EdgeCalls))
	}
	for _, id := range []string{"F1", "F2", "F3", "F4"} {
		m.neighOut = append(m.neighOut, node(id, "pkg."+id, id+".go", 1, 9, types.NodeFunction, "go"))
		m.neighEdges = append(m.neighEdges, edge("H1", id, types.EdgeCalls))
	}
	r := newRealWithStore(m)
	out, err := r.Neighbors(context.Background(), contract.Citation{File: "s.go", StartLine: 1, EndLine: 9, CommitHash: "h"},
		NeighborsOpts{Hops: 3, MaxTotal: 4, Relations: []contract.Relation{contract.RelationCalls}})
	if err != nil {
		t.Fatal(err)
	}
	perHop := map[int]int{}
	for _, n := range out {
		perHop[n.Distance]++
	}
	if len(out) != 4 || perHop[1] != 2 || perHop[2] != 2 {
		t.Fatalf("per hop = %v (%d neighbours), want 2 direct and 2 at hop 2", perHop, len(out))
	}
}

// TestReal_Neighbors_StartsFromTheMatchedNode: another node with the same
// qualified name (two packages named pkg) is not a start of the walk.
func TestReal_Neighbors_StartsFromTheMatchedNode(t *testing.T) {
	t.Parallel()
	mine := node("M", "pkg.F", "a/f.go", 1, 9, types.NodeFunction, "go")
	other := node("O", "pkg.F", "b/f.go", 1, 9, types.NodeFunction, "go")
	m := &mockStoreReader{
		manifest:  ManifestSnapshot{SrcCommit: "h"},
		pathNodes: []types.Node{mine},
		neighOut: []types.Node{mine, other,
			node("X", "pkg.X", "x.go", 1, 9, types.NodeFunction, "go"),
			node("Y", "pkg.Y", "y.go", 1, 9, types.NodeFunction, "go")},
		neighEdges: []types.Edge{edge("M", "X", types.EdgeCalls), edge("O", "Y", types.EdgeCalls)},
	}
	r := newRealWithStore(m)
	out, err := r.Neighbors(context.Background(), contract.Citation{File: "a/f.go", StartLine: 1, EndLine: 9, CommitHash: "h"},
		NeighborsOpts{Hops: 2, Relations: []contract.Relation{contract.RelationCalls}})
	if err != nil {
		t.Fatal(err)
	}
	if got := targets(out); !reflect.DeepEqual(got, map[string]int{"x.go": 1}) {
		t.Fatalf("neighbours = %v, want only the matched node's callee", got)
	}
}

func TestShareCap(t *testing.T) {
	cases := []struct {
		counts []int
		limit  int
		want   []int
	}{
		{[]int{10, 10, 10}, 9, []int{3, 3, 3}},
		{[]int{1, 10, 10}, 9, []int{1, 4, 4}}, // the small hop's share goes to the others
		{[]int{10, 0, 0}, 5, []int{5, 0, 0}},
		{[]int{2, 3}, 0, []int{2, 3}}, // no cap
		{[]int{5, 5, 5}, 4, []int{2, 1, 1}},
	}
	for _, c := range cases {
		if got := shareCap(c.counts, c.limit); !reflect.DeepEqual(got, c.want) {
			t.Errorf("shareCap(%v, %d) = %v, want %v", c.counts, c.limit, got, c.want)
		}
	}
}
