package cluster_test

import (
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/cluster"
	"github.com/0xmhha/knowledge-system/internal/graph/graph"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// TestBuildTopicTree_RepeatedLabelsKeepEveryMember: two separate communities
// whose members have the same names get the same label. The store keys
// communities by label, so before the fix the second community's members
// replaced the first one's and half the nodes lost their topic.
func TestBuildTopicTree_RepeatedLabelsKeepEveryMember(t *testing.T) {
	fn := func(id, qname string) types.Node {
		return types.Node{ID: id, Type: types.NodeFunction, Name: qname, QualifiedName: qname,
			FilePath: "p/" + id + ".go", Language: "go", Confidence: types.ConfExtracted}
	}
	g := &graph.Graph{
		// Two copies of p.Run calling p.step, unconnected to each other.
		Nodes: []types.Node{fn("a1", "p.Run"), fn("a2", "p.step"), fn("b1", "p.Run"), fn("b2", "p.step")},
		Edges: []types.Edge{
			{Src: "a1", Dst: "a2", Type: types.EdgeCalls},
			{Src: "b1", Dst: "b2", Type: types.EdgeCalls},
		},
	}
	tt := cluster.BuildTopicTree(g, []float64{1.0}, 42)
	members := tt.ResolutionMembers(0)
	total := 0
	for _, ids := range members {
		total += len(ids)
	}
	if len(members) != 2 || total != 4 {
		t.Fatalf("persisted communities = %v, want 2 communities holding all 4 nodes", members)
	}
	labels := map[string]bool{}
	for _, c := range tt.Resolutions[0].Communities {
		if labels[c.Label] {
			t.Fatalf("label %q used twice", c.Label)
		}
		labels[c.Label] = true
	}
}
