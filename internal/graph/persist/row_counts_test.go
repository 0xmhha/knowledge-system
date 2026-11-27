package persist

import (
	"path/filepath"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// TestRowCounts: the counts are the table rows, not the manifest's stats.
func TestRowCounts(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.Close() }()
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	if err := s.SetManifest(Manifest{Stats: map[string]int{"nodes": 99, "edges": 99}}); err != nil {
		t.Fatal(err)
	}
	n := func(id string) types.Node {
		return types.Node{ID: id, Type: types.NodeFunction, Name: id, QualifiedName: "p." + id, FilePath: "a.go",
			StartLine: 1, EndLine: 1, Language: "go", Confidence: types.ConfExtracted}
	}
	if err := s.InsertNodes([]types.Node{n("aaaaaaaaaaaaaaaa"), n("bbbbbbbbbbbbbbbb")}); err != nil {
		t.Fatal(err)
	}
	if err := s.InsertEdges([]types.Edge{{Src: "aaaaaaaaaaaaaaaa", Dst: "bbbbbbbbbbbbbbbb", Type: types.EdgeCalls, Count: 1, Confidence: types.ConfExtracted}}); err != nil {
		t.Fatal(err)
	}
	nodes, edges, err := s.RowCounts()
	if err != nil || nodes != 2 || edges != 1 {
		t.Errorf("RowCounts = %d, %d, %v; want 2, 1", nodes, edges, err)
	}
}
