package ckgclient

import (
	"context"
	"fmt"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// depthStore grows a call fan-out tenfold per level and records the depths
// it was asked for.
type depthStore struct {
	*mockStoreReader
	depths []int
	factor int
	typ    types.EdgeType
}

func (d *depthStore) SubgraphByQname(qname string, depth int) ([]types.Node, []types.Edge, error) {
	d.depths = append(d.depths, depth)
	seed := node("seed000000000000", "pkg.Seed", "a.go", 1, 2, types.NodeFunction, "go")
	nodes := []types.Node{seed}
	var edges []types.Edge
	width := 1
	for level := 1; level <= depth; level++ {
		width *= d.factor
		for i := 0; i < width; i++ {
			id := fmt.Sprintf("n%02d%013d", level, i)
			nodes = append(nodes, node(id, fmt.Sprintf("pkg.N%d_%d", level, i), "b.go", i+1, i+1, types.NodeFunction, "go"))
			edges = append(edges, types.Edge{Src: seed.ID, Dst: id, Type: d.typ})
		}
	}
	return nodes, edges, nil
}

// get_subgraph keeps the edges nearest the seed, so it stops deepening once
// it holds max_total usable edges, or subgraphTraversalBudget edges in all:
// on a production-size graph depth 5 allocated up to 10 GB to return 80 edges.
func TestGetSubgraphStopsDeepeningOnceTheBudgetIsReached(t *testing.T) {
	ds := &depthStore{factor: 10, typ: types.EdgeCalls, mockStoreReader: &mockStoreReader{
		symbolByName: map[string][]types.Node{"pkg.Seed": {node("seed000000000000", "pkg.Seed", "a.go", 1, 2, types.NodeFunction, "go")}},
		manifest:     ManifestSnapshot{SrcCommit: "abc"},
	}}
	r := newRealWithStore(ds)
	cits, nbs, err := r.GetSubgraph(context.Background(), "pkg.Seed", SubgraphOpts{Depth: 5, MaxTotal: 80})
	if err != nil {
		t.Fatal(err)
	}
	if len(nbs) != 80 || len(cits) == 0 {
		t.Fatalf("neighbors=%d citations=%d", len(nbs), len(cits))
	}
	if fmt.Sprint(ds.depths) != "[1 2]" {
		t.Fatalf("depths walked = %v, want [1 2] (stop at the first level with 80 usable edges)", ds.depths)
	}
	ds.depths = nil
	if _, _, err := r.GetSubgraph(context.Background(), "pkg.Seed", SubgraphOpts{Depth: 2, MaxTotal: 80}); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprint(ds.depths) != "[1 2]" {
		t.Fatalf("shallow request walked %v", ds.depths)
	}
}

// Edges the response cannot use (here changed_in) do not count toward
// max_total, so the walk goes deeper, until the total budget stops it.
func TestGetSubgraphTotalBudgetBoundsUnusableEdges(t *testing.T) {
	ds := &depthStore{factor: 40, typ: types.EdgeChangedIn, mockStoreReader: &mockStoreReader{
		symbolByName: map[string][]types.Node{"pkg.Seed": {node("seed000000000000", "pkg.Seed", "a.go", 1, 2, types.NodeFunction, "go")}},
		manifest:     ManifestSnapshot{SrcCommit: "abc"},
	}}
	r := newRealWithStore(ds)
	if _, _, err := r.GetSubgraph(context.Background(), "pkg.Seed", SubgraphOpts{Depth: 5, MaxTotal: 80}); err != nil {
		t.Fatal(err)
	}
	// 40, 1,600, then 64,000 edges: the third level passes the budget.
	if fmt.Sprint(ds.depths) != "[1 2 3]" {
		t.Fatalf("depths walked = %v, want [1 2 3]", ds.depths)
	}
}
