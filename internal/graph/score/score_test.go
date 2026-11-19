package score_test

import (
	"math"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/graph"
	"github.com/0xmhha/knowledge-system/internal/graph/score"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Triangle graph: A -> B -> C -> A
func triangle() *graph.Graph {
	mk := func(id string) types.Node {
		return types.Node{ID: id, Type: types.NodeFunction, Name: id, QualifiedName: id,
			FilePath: "f.go", StartLine: 1, EndLine: 1, EndByte: 1,
			Language: "go", Confidence: types.ConfExtracted}
	}
	a, b, c := mk("aaaaaaaaaaaaaaaa"), mk("bbbbbbbbbbbbbbbb"), mk("cccccccccccccccc")
	mke := func(s, d string) types.Edge {
		return types.Edge{Src: s, Dst: d, Type: types.EdgeCalls, Count: 1, Confidence: types.ConfExtracted}
	}
	return &graph.Graph{
		Nodes: []types.Node{a, b, c},
		Edges: []types.Edge{mke(a.ID, b.ID), mke(b.ID, c.ID), mke(c.ID, a.ID)},
	}
}

func TestDegreeAndUsage(t *testing.T) {
	g := triangle()
	score.Compute(g)
	for _, n := range g.Nodes {
		if n.InDegree != 1 || n.OutDegree != 1 {
			t.Errorf("%s: in=%d out=%d, want 1/1", n.ID, n.InDegree, n.OutDegree)
		}
		if n.UsageScore != 1 {
			t.Errorf("%s: usage=%.2f, want 1", n.ID, n.UsageScore)
		}
	}
}

func TestPageRankSumsToOne(t *testing.T) {
	g := triangle()
	score.Compute(g)
	sum := 0.0
	for _, n := range g.Nodes {
		sum += n.PageRank
	}
	if math.Abs(sum-1.0) > 1e-3 {
		t.Errorf("PageRank sum = %.6f, want ~1.0", sum)
	}
}

// Statement links (schema 1.32) describe the inside of a function; the
// calls they repeat are already function-level edges. They do not move
// PageRank.
func TestPageRankIgnoresStatementLinks(t *testing.T) {
	base := triangle()
	score.Compute(base)
	g := triangle()
	a, b, c := g.Nodes[0].ID, g.Nodes[1].ID, g.Nodes[2].ID
	for _, e := range []struct {
		s, d string
		t    types.EdgeType
	}{{a, c, types.EdgeSiteCalls}, {b, a, types.EdgeEncloses}, {c, b, types.EdgeCondReads},
		{a, b, types.EdgeCondCalls}, {b, c, types.EdgeBodyWrites}} {
		g.Edges = append(g.Edges, types.Edge{Src: e.s, Dst: e.d, Type: e.t, Count: 1, Confidence: types.ConfExtracted})
	}
	score.Compute(g)
	for i := range g.Nodes {
		if math.Abs(g.Nodes[i].PageRank-base.Nodes[i].PageRank) > 1e-12 {
			t.Errorf("%s: PageRank %.6f with statement links, %.6f without", g.Nodes[i].ID, g.Nodes[i].PageRank, base.Nodes[i].PageRank)
		}
	}
}

// Type-hierarchy dispatch candidates (schema 1.34) say what go/types
// allows, not what the value flow found; they do not move PageRank.
func TestPageRankIgnoresTypeHierarchy(t *testing.T) {
	base := triangle()
	score.Compute(base)
	g := triangle()
	a, c := g.Nodes[0].ID, g.Nodes[2].ID
	g.Edges = append(g.Edges, types.Edge{Src: a, Dst: c, Type: types.EdgeDispatchesTo, DispatchKind: "type_hierarchy",
		Count: 1, Confidence: types.ConfInferred})
	score.Compute(g)
	for i := range g.Nodes {
		if math.Abs(g.Nodes[i].PageRank-base.Nodes[i].PageRank) > 1e-12 {
			t.Errorf("%s: PageRank %.6f with a type-hierarchy edge, %.6f without", g.Nodes[i].ID, g.Nodes[i].PageRank, base.Nodes[i].PageRank)
		}
	}
}
