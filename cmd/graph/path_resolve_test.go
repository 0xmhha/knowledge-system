package main

import (
	"bytes"
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

// codeSpans returns the text in backticks in out.
func codeSpans(out string) []string {
	var spans []string
	for i, part := range strings.Split(out, "`") {
		if i%2 == 1 {
			spans = append(spans, part)
		}
	}
	return spans
}

// cliStore holds a function that declares x twice and calls a helper of a
// name another package declares too, so short names and plain qualified
// names would not open one node.
func cliStore(t *testing.T) (persist.Store, []types.Node, []types.Edge) {
	t.Helper()
	node := func(id string, nt types.NodeType, name, qname, file string, line int) types.Node {
		return types.Node{ID: id, Type: nt, Name: name, QualifiedName: qname, CanonicalID: map[bool]string{true: "ex/" + qname + "@" + file}[nt != types.NodeLocalVariable],
			FilePath: file, StartLine: line, EndLine: line, PageRank: 0.1}
	}
	nodes := []types.Node{
		node("cc00000000000001", types.NodeFunction, "Run", "p.Run", "p/run.go", 1),
		node("cc00000000000002", types.NodeLocalVariable, "x", "p.Run.x", "p/run.go", 3),
		node("cc00000000000003", types.NodeLocalVariable, "x", "p.Run.x", "p/run.go", 6),
		node("cc00000000000004", types.NodeFunction, "help", "util.help", "a/util/h.go", 2),
		node("cc00000000000005", types.NodeFunction, "help", "util.help", "b/util/h.go", 2),
	}
	edges := []types.Edge{
		{Src: "cc00000000000001", Dst: "cc00000000000002", Type: types.EdgeDefines},
		{Src: "cc00000000000001", Dst: "cc00000000000003", Type: types.EdgeDefines},
		{Src: "cc00000000000001", Dst: "cc00000000000004", Type: types.EdgeCalls},
		{Src: "cc00000000000001", Dst: "cc00000000000005", Type: types.EdgeCalls},
	}
	return pathStore(t, nodes), nodes, edges
}

// TestCLINamesRoundTrip: every node name ckg query, ckg path and ckg report
// print in code spans opens exactly one node. The query's neighbourhood
// printed short names (299 of 300 sampled did not resolve back), and the
// report printed short and plain qualified names.
func TestCLINamesRoundTrip(t *testing.T) {
	s, nodes, edges := cliStore(t)
	name := func(n types.Node) string { return persist.PrintedName(s, n) }
	var q bytes.Buffer
	renderQueryAnswer(&q, "run", nodes[:1], nodes, buildAdjacency(edges), buildEdgeIndex(edges), 2, 2000, name)
	var p strings.Builder
	path, hops := bfsShortestPath(nodes, edges, "cc00000000000003", "cc00000000000005", 4)
	if err := printPath(&p, s, nodes, path, hops); err != nil {
		t.Fatal(err)
	}
	for _, out := range []string{q.String(), p.String()} {
		spans := codeSpans(out)
		if len(spans) == 0 {
			t.Fatalf("no names in:\n%s", out)
		}
		for _, sp := range spans {
			res, err := persist.ResolveName(s, sp, "")
			if err != nil || len(res.Nodes) != 1 {
				t.Errorf("%q printed in\n%s\nresolves to %d nodes (candidates %v)", sp, out, len(res.Nodes), res.Candidates)
			}
		}
	}
	// The report prints in code spans the names it is given, never a short
	// name: mark each printed name to find any that bypasses it.
	report := buildReport(persist.Manifest{}, nodes, edges, nil, 5, func(n types.Node) string { return "@" + n.ID })
	for _, sp := range codeSpans(report) {
		for _, n := range nodes {
			if sp == n.Name || sp == n.QualifiedName {
				t.Errorf("report prints %q, the short or plain name of %s", sp, n.ID)
			}
		}
	}
}
