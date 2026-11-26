package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// TestResolveNodeQuery: ckg path resolves its arguments with the resolver
// every tool shares. A qualified name that two declarations carry is an
// error listing both site names, and a site name resolves to its own
// declaration; ckg path used to take the first node silently.
// pathStore is a graph store holding nodes.
func pathStore(t *testing.T, nodes []types.Node) persist.Store {
	t.Helper()
	s, err := persist.Open(filepath.Join(t.TempDir(), "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	for i := range nodes {
		if nodes[i].Language == "" {
			nodes[i].Language = "go"
		}
		if nodes[i].Confidence == "" {
			nodes[i].Confidence = types.ConfExtracted
		}
	}
	if err := s.InsertNodes(nodes); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestResolveNodeQuery(t *testing.T) {
	local := func(id string, line int) types.Node {
		return types.Node{ID: id, Type: types.NodeLocalVariable, Name: "x", QualifiedName: "p.F.x", FilePath: "p/f.go",
			StartLine: line, EndLine: line, Language: "go", Confidence: types.ConfExtracted}
	}
	nodes := []types.Node{local("aa00000000000001", 3), local("aa00000000000002", 9)}
	s := pathStore(t, nodes)
	if _, err := resolveNodeQuery(s, nodes, "p.F.x"); err == nil || !strings.Contains(err.Error(), "p.F.x@p/f.go:3") || !strings.Contains(err.Error(), "p.F.x@p/f.go:9") {
		t.Errorf("ambiguous name: err %v, want both site names", err)
	}
	if id, err := resolveNodeQuery(s, nodes, "p.F.x@p/f.go:9"); err != nil || id != "aa00000000000002" {
		t.Errorf("site name: %q, %v", id, err)
	}
	if id, err := resolveNodeQuery(s, nodes, "aa00000000000001"); err != nil || id != "aa00000000000001" {
		t.Errorf("id prefix: %q, %v", id, err)
	}
}

// TestPrintPath_SiteNames: a node whose qualified name several declarations
// carry is printed by its site name, so two locals of one name in one
// function can be told apart in the path, and the printed name can be given
// back to ckg path.
func TestPrintPath_SiteNames(t *testing.T) {
	nodes := []types.Node{
		{ID: "aa00000000000001", Type: types.NodeLocalVariable, Name: "x", QualifiedName: "p.F.x", FilePath: "p/f.go", StartLine: 3, EndLine: 3},
		{ID: "aa00000000000002", Type: types.NodeLocalVariable, Name: "x", QualifiedName: "p.F.x", FilePath: "p/f.go", StartLine: 9, EndLine: 9},
		{ID: "aa00000000000003", Type: types.NodeFunction, Name: "F", QualifiedName: "p.F", CanonicalID: "ex/p.F", FilePath: "p/f.go", StartLine: 1, EndLine: 12},
	}
	s := pathStore(t, nodes)
	edges := []types.Edge{{Src: "aa00000000000003", Dst: "aa00000000000002", Type: types.EdgeDefines}}
	path, hops := bfsShortestPath(nodes, edges, "aa00000000000003", "aa00000000000002", 3)
	var out strings.Builder
	if err := printPath(&out, s, nodes, path, hops); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "`p.F.x@p/f.go:9`") || !strings.Contains(out.String(), "`p.F`") {
		t.Errorf("path output:\n%s", out.String())
	}
}
