package buildpipe

import (
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/graph"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// TestEmitDerivedPasses_TopicLabelUsesPageRank checks that topic labels are
// named after the member with the highest PageRank. LabelCommunity picks that
// member, but it can only do so once score.Compute has filled PageRank; when
// scores ran after the topic tree every PageRank was zero and the label fell
// back to the first member. The hub is placed last so the first member and
// the highest-PageRank member differ.
func TestEmitDerivedPasses_TopicLabelUsesPageRank(t *testing.T) {
	const file = "p/p.go"
	fn := func(id, name string, line int) types.Node {
		return types.Node{
			ID: id, Type: types.NodeFunction, Name: name, QualifiedName: "p." + name,
			FilePath: file, StartLine: line, EndLine: line, StartByte: line * 10, EndByte: line*10 + 5,
			Language: "go", Confidence: types.ConfExtracted,
		}
	}
	callers := []string{"alpha", "bravo", "charlie", "delta"}
	g := &graph.Graph{}
	for i, name := range callers {
		g.Nodes = append(g.Nodes, fn("fn-"+name, name, i+1))
	}
	g.Nodes = append(g.Nodes, fn("fn-hub", "Hub", 10))
	for _, name := range callers {
		g.Edges = append(g.Edges, types.Edge{
			Src: "fn-" + name, Dst: "fn-hub", Type: types.EdgeCalls,
			FilePath: file, Count: 1, Confidence: types.ConfExtracted,
		})
	}

	// t.TempDir is not a git checkout, so the temporal pass adds nothing.
	_, tt, _, err := emitDerivedPasses(g, t.TempDir(), nil, discardLog(), true, HistoryOptions{}, nil)
	if err != nil {
		t.Fatalf("emitDerivedPasses: %v", err)
	}

	checked := 0
	for _, res := range tt.Resolutions {
		for _, c := range res.Communities {
			hasHub := false
			for _, id := range c.Members {
				hasHub = hasHub || id == "fn-hub"
			}
			if !hasHub || len(c.Members) < 2 {
				continue
			}
			checked++
			if !strings.HasSuffix(c.Label, "Hub") {
				t.Errorf("gamma %v community %d label = %q, want it named after the highest-PageRank member Hub",
					res.Gamma, c.ID, c.Label)
			}
		}
	}
	if checked == 0 {
		t.Fatal("no community holds the hub together with a caller; the test graph no longer exercises the label")
	}
}
