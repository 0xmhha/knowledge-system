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
func TestResolveNodeQuery(t *testing.T) {
	s, err := persist.Open(filepath.Join(t.TempDir(), "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	if err := s.Migrate(); err != nil {
		t.Fatal(err)
	}
	local := func(id string, line int) types.Node {
		return types.Node{ID: id, Type: types.NodeLocalVariable, Name: "x", QualifiedName: "p.F.x", FilePath: "p/f.go",
			StartLine: line, EndLine: line, Language: "go", Confidence: types.ConfExtracted}
	}
	nodes := []types.Node{local("aa00000000000001", 3), local("aa00000000000002", 9)}
	if err := s.InsertNodes(nodes); err != nil {
		t.Fatal(err)
	}
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
