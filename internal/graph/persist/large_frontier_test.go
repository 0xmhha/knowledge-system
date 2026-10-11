package persist

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Traversals and ID lookups on real graphs reach tens of thousands of IDs in
// one frontier; one IN (...) per call exceeded SQLite's variable limit.
func TestLargeIDListsAreQueriedInChunks(t *testing.T) {
	st, err := Open(filepath.Join(t.TempDir(), "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(); err != nil {
		t.Fatal(err)
	}
	const fanout = 40000
	node := func(id, q string) types.Node {
		return types.Node{ID: id, Type: types.NodeFunction, Name: q, QualifiedName: q, FilePath: "a.go", StartLine: 1, EndLine: 1, Language: "go", Confidence: types.ConfExtracted}
	}
	nodes := []types.Node{node("root000000000000", "pkg.Root")}
	var edges []types.Edge
	ids := make([]string, 0, fanout)
	for i := 0; i < fanout; i++ {
		id := fmt.Sprintf("c%015d", i)
		nodes = append(nodes, node(id, fmt.Sprintf("pkg.C%d", i)))
		edges = append(edges, types.Edge{Src: "root000000000000", Dst: id, Type: types.EdgeCalls, Count: 1, Confidence: types.ConfExtracted})
		ids = append(ids, id)
	}
	// One more level so the 40,000-ID frontier is expanded.
	nodes = append(nodes, node("leaf000000000000", "pkg.Leaf"))
	edges = append(edges, types.Edge{Src: ids[fanout-1], Dst: "leaf000000000000", Type: types.EdgeCalls, Count: 1, Confidence: types.ConfExtracted})
	if err := st.InsertNodes(nodes); err != nil {
		t.Fatal(err)
	}
	if err := st.InsertEdges(edges); err != nil {
		t.Fatal(err)
	}
	got, err := st.NodesByIDs(ids)
	if err != nil || len(got) != fanout {
		t.Fatalf("NodesByIDs: %d nodes, %v", len(got), err)
	}
	ns, es, err := st.NeighborhoodByQname("pkg.Root", 3, false)
	if err != nil || len(ns) != fanout+2 || len(es) != fanout+1 {
		t.Fatalf("forward: %d nodes %d edges %v", len(ns), len(es), err)
	}
	ns, es, err = st.NeighborhoodByQname("pkg.Leaf", 3, true)
	if err != nil || len(ns) != 3 || len(es) != 2 {
		t.Fatalf("reverse: %d nodes %d edges %v", len(ns), len(es), err)
	}
	if _, _, err := st.NeighborhoodByQname("pkg.C0", 3, true); err != nil {
		t.Fatalf("reverse from a child: %v", err)
	}
}
