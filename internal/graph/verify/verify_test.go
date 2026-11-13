package verify

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

const (
	fixtureRoot = "testdata/fix"
	pkgA        = "example.com/fix/a/x"
	pkgB        = "example.com/fix/b/x"
	pkgC        = "example.com/fix/c"
)

// fakeGraph is an in-memory GraphReader.
type fakeGraph struct {
	nodes []types.Node
	edges []types.Edge
}

func (g *fakeGraph) AllNodes() ([]types.Node, error) { return g.nodes, nil }

func (g *fakeGraph) QueryEdgesByType(t string) ([]types.Edge, error) {
	var out []types.Edge
	for _, e := range g.edges {
		if string(e.Type) == t {
			out = append(out, e)
		}
	}
	return out, nil
}

// lineOf returns the 1-based line of the first line in file containing text.
func lineOf(t *testing.T, file, text string) int {
	t.Helper()
	f, err := os.Open(filepath.Join(fixtureRoot, file))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for n := 1; sc.Scan(); n++ {
		if strings.Contains(sc.Text(), text) {
			return n
		}
	}
	t.Fatalf("%s: %q not found", file, text)
	return 0
}

// fixtureGraph builds a graph for testdata/fix with one correct edge per
// relation and one defect of each kind verify reports:
//
//   - Use calls a/x.Do, but the graph points to b/x.Do (misresolved call);
//   - the call inside Use's closure is not recorded (deduplicated into the
//     same pair, so it does not add a second truth pair);
//   - Other refers to the package-level T, but the graph points to the type
//     declared inside Marshal (misresolved type ref);
//   - Marshal's local T is a declaration node with a package-level id;
//   - Marshal has a call edge to a/x.Do that go/types does not support.
//
// Misresolved edges are also unexpected edges: the graph edge has no truth pair.
func fixtureGraph(t *testing.T) *fakeGraph {
	t.Helper()
	node := func(id string, nt types.NodeType, name, cid, file, text string, span int) types.Node {
		l := lineOf(t, file, text)
		return types.Node{ID: id, Type: nt, Name: name, CanonicalID: cid, FilePath: file,
			StartLine: l, EndLine: l + span, Language: "go"}
	}
	g := &fakeGraph{}
	g.nodes = []types.Node{
		node("doA", types.NodeFunction, "Do", pkgA+".Do", "a/x/x.go", "func Do()", 0),
		node("doB", types.NodeFunction, "Do", pkgB+".Do", "b/x/x.go", "func Do()", 0),
		node("T", types.NodeStruct, "T", pkgC+".T", "c/c.go", "type T struct{}", 0),
		node("M", types.NodeMethod, "M", pkgC+".(T).M", "c/c.go", "func (T) M()", 0),
		node("I", types.NodeInterface, "I", pkgC+".I", "c/c.go", "type I interface", 2),
		node("IM", types.NodeMethod, "M", pkgC+".I.M", "c/c.go", "\tM()", 0),
		node("Use", types.NodeFunction, "Use", pkgC+".Use", "c/c.go", "func Use(", 5),
		node("Marshal", types.NodeFunction, "Marshal", pkgC+".Marshal", "c/c.go", "func Marshal()", 3),
		node("localT", types.NodeStruct, "T", pkgC+".T@30", "c/c.go", "type T struct{ A int }", 0),
		node("Other", types.NodeFunction, "Other", pkgC+".Other", "c/c.go", "func Other()", 0),
	}
	g.edges = []types.Edge{
		{Src: "Use", Dst: "doB", Type: types.EdgeCalls},
		{Src: "Use", Dst: "IM", Type: types.EdgeInvokes},
		{Src: "Use", Dst: "I", Type: types.EdgeUsesType},
		{Src: "M", Dst: "T", Type: types.EdgeUsesType},
		{Src: "Other", Dst: "localT", Type: types.EdgeUsesType},
		{Src: "Marshal", Dst: "doA", Type: types.EdgeCalls},
	}
	return g
}

func TestRunGo_ClassifiesEachDefect(t *testing.T) {
	r, err := RunGo(fixtureRoot, fixtureGraph(t), Options{})
	if err != nil {
		t.Fatalf("RunGo: %v", err)
	}
	cases := []struct {
		name      string
		got, want int
	}{
		{"files", r.Files, 3},
		{"declarations matched", r.Decls.Matched, 10 - 1}, // the local T is not a package-level declaration
		{"declarations missing", r.Decls.Missing, 0},
		{"static calls truth", r.StaticCalls.Truth, 1},
		{"static calls misresolved", r.StaticCalls.Misresolved, 1},
		{"static calls matched", r.StaticCalls.Matched, 0},
		{"interface calls matched", r.InterfaceCalls.Matched, 1},
		{"type refs truth", r.TypeRefs.Truth, 3}, // Use->I, M->T, Other->T
		{"type refs matched", r.TypeRefs.Matched, 2},
		{"type refs misresolved", r.TypeRefs.Misresolved, 1},
		{"dynamic call sites", r.DynamicCallSites, 1}, // f()
		// Marshal->a/x.Do, plus the misresolved Use->b/x.Do edge itself.
		{"unexpected call edges", r.Unexpected.Calls, 2},
		{"unmatched declaration nodes", r.LocalDeclNodes.Count, 1},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %d, want %d", c.name, c.got, c.want)
		}
	}
	if len(r.StaticCalls.MisresolvedSamples) != 1 || !strings.Contains(r.StaticCalls.MisresolvedSamples[0], pkgB+".Do") {
		t.Errorf("misresolved sample does not name the wrong target: %v", r.StaticCalls.MisresolvedSamples)
	}
}

func TestRunGo_MissingEdgeIsNotMisresolved(t *testing.T) {
	g := fixtureGraph(t)
	g.edges = g.edges[1:] // drop Use -> b/x.Do: now there is no same-name substitute
	r, err := RunGo(fixtureRoot, g, Options{})
	if err != nil {
		t.Fatalf("RunGo: %v", err)
	}
	if r.StaticCalls.Missing != 1 || r.StaticCalls.Misresolved != 0 {
		t.Errorf("missing %d misresolved %d, want 1 0", r.StaticCalls.Missing, r.StaticCalls.Misresolved)
	}
}

func TestRunGo_MissingDeclarationNode(t *testing.T) {
	g := fixtureGraph(t)
	// Keep a/x/x.go in scope through its File node, as a real graph would.
	kept := []types.Node{{ID: "fileA", Type: types.NodeFile, Name: "x.go", FilePath: "a/x/x.go", StartLine: 1, EndLine: 5, Language: "go"}}
	for _, n := range g.nodes {
		if n.ID != "doA" {
			kept = append(kept, n)
		}
	}
	g.nodes = kept
	r, err := RunGo(fixtureRoot, g, Options{})
	if err != nil {
		t.Fatalf("RunGo: %v", err)
	}
	if r.Decls.Missing != 1 || r.StaticCalls.EndpointMissing != 1 {
		t.Errorf("decl missing %d, call endpoint missing %d, want 1 1", r.Decls.Missing, r.StaticCalls.EndpointMissing)
	}
}

func TestBaseID(t *testing.T) {
	cases := map[string]string{
		"example.com/c.T@30":     "example.com/c.T",
		"example.com/c.T":        "example.com/c.T",
		"example.com/c.(*T).M":   "example.com/c.(*T).M",
		"example.com/v@2/c.Func": "example.com/v@2/c.Func", // "@" inside the import path is not a line suffix
	}
	for in, want := range cases {
		if got := baseID(in); got != want {
			t.Errorf("baseID(%q) = %q, want %q", in, got, want)
		}
	}
}
