package mcphandlers

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// sameNameStore holds two functions with one qualified name ("p.F", as two
// packages named p produce) but different canonical ids, each called by its
// own caller.
func sameNameStore(t *testing.T) persist.Store {
	t.Helper()
	s, err := persist.Open(filepath.Join(t.TempDir(), "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	fn := func(id, qname, canonical, file string) types.Node {
		return types.Node{ID: id, Type: types.NodeFunction, Name: qname[len(qname)-1:], QualifiedName: qname,
			CanonicalID: canonical, FilePath: file, Language: "go", StartLine: 1, EndLine: 2, Confidence: types.ConfExtracted}
	}
	nodes := []types.Node{
		fn("a000000000000001", "p.F", "example.invalid/a/p.F", "a/p/f.go"),
		fn("b000000000000001", "p.F", "example.invalid/b/p.F", "b/p/f.go"),
		fn("a000000000000002", "a.CallA", "example.invalid/a.CallA", "a/call.go"),
		fn("b000000000000002", "b.CallB", "example.invalid/b.CallB", "b/call.go"),
	}
	if err := s.InsertNodes(nodes); err != nil {
		t.Fatal(err)
	}
	edges := []types.Edge{
		{Src: "a000000000000002", Dst: "a000000000000001", Type: types.EdgeCalls, Count: 1, Confidence: types.ConfExtracted},
		{Src: "b000000000000002", Dst: "b000000000000001", Type: types.EdgeCalls, Count: 1, Confidence: types.ConfExtracted},
	}
	if err := s.InsertEdges(edges); err != nil {
		t.Fatal(err)
	}
	return s
}

// TestCanonicalSeedWalksOneNode: a canonical id names one node, so
// find_callers from it reaches only that node's caller. Before the fix the
// id was turned into its qualified name and both callers came back.
func TestCanonicalSeedWalksOneNode(t *testing.T) {
	s := sameNameStore(t)
	seed, _, _, ok := resolveSeed(s, "example.invalid/a/p.F", "")
	if !ok || seed.QName != "p.F" || len(seed.IDs) != 1 || seed.IDs[0] != "a000000000000001" {
		t.Fatalf("seed = %+v, ok %v", seed, ok)
	}
	nodes, _, err := reverseCallersUnion(s, seed, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got := qnames(nodes); !equalQnames(got, "a.CallA", "p.F") {
		t.Fatalf("callers of one p.F: %v", got)
	}

	// A qualified-name seed still walks every node of that name.
	byName, _, _, ok := resolveSeed(s, "p.F", "")
	if !ok || len(byName.IDs) != 0 {
		t.Fatalf("qname seed = %+v, ok %v", byName, ok)
	}
	nodes, _, err = reverseCallersUnion(s, byName, 2)
	if err != nil {
		t.Fatal(err)
	}
	if got := qnames(nodes); !equalQnames(got, "a.CallA", "b.CallB", "p.F", "p.F") {
		t.Fatalf("callers of every p.F: %v", got)
	}

	// get_subgraph and change_history read the same seed.
	sub, _, err := seed.subgraph(s, 1)
	if err != nil {
		t.Fatal(err)
	}
	if got := qnames(sub); !equalQnames(got, "a.CallA", "p.F") {
		t.Fatalf("subgraph of one p.F: %v", got)
	}
	own, err := seed.nodes(s)
	if err != nil || len(own) != 1 || own[0].ID != "a000000000000001" {
		t.Fatalf("seed nodes = %v, %v", own, err)
	}
}

func equalQnames(got []nodeQname, want ...string) bool {
	g := make([]string, len(got))
	for i, q := range got {
		g[i] = string(q)
	}
	sort.Strings(g)
	sort.Strings(want)
	if len(g) != len(want) {
		return false
	}
	for i := range g {
		if g[i] != want[i] {
			return false
		}
	}
	return true
}
