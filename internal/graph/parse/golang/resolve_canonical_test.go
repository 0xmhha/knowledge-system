package golang_test

import (
	"testing"

	gop "github.com/0xmhha/knowledge-system/internal/graph/parse/golang"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

const sameNamePkg = "example.com/samename"

// edgeSet indexes a resolved graph by (src canonical id, edge type, dst
// canonical id) for the nodes that have a canonical id.
func edgeSet(t *testing.T) (map[[3]string]bool, []types.Node) {
	t.Helper()
	g, err := gop.LoadAndResolve("testdata/samename")
	if err != nil {
		t.Fatalf("LoadAndResolve: %v", err)
	}
	byID := make(map[string]types.Node, len(g.Nodes))
	for _, n := range g.Nodes {
		byID[n.ID] = n
	}
	set := map[[3]string]bool{}
	for _, e := range g.Edges {
		src, dst := byID[e.Src], byID[e.Dst]
		if src.CanonicalID == "" || dst.CanonicalID == "" {
			continue
		}
		set[[3]string{src.CanonicalID, string(e.Type), dst.CanonicalID}] = true
	}
	return set, g.Nodes
}

// TestResolve_ByCanonicalID covers the defects ckg verify found on
// go-stablenet: two packages with the same short name, function-local types
// shadowing a package-level type, conversions and func variables bound to
// same-named methods by the suffix fallback, and calls to generic methods.
func TestResolve_ByCanonicalID(t *testing.T) {
	set, nodes := edgeSet(t)
	use := sameNamePkg + "/use."
	one := sameNamePkg + "/one/x."
	two := sameNamePkg + "/two/x."
	cases := []struct {
		name     string
		src, dst string
		edge     types.EdgeType
		want     bool
	}{
		{"call resolves to the imported package", use + "Call", one + "Do", types.EdgeCalls, true},
		{"call does not resolve to the same-named package", use + "Call", two + "Do", types.EdgeCalls, false},
		{"conversion uint(x) is not a call to S.uint", use + "Call", use + "(S).uint", types.EdgeCalls, false},
		{"func variable cancel() is not a call to S.cancel", use + "Call", use + "(S).cancel", types.EdgeCalls, false},
		{"generic method call resolves to its declaration", use + "G", use + "(*Box).Put", types.EdgeCalls, true},
		{"call inside a goroutine literal is recorded", use + "Spawn", one + "Do", types.EdgeCalls, true},
		{"error.Error() is not a call to an unrelated Error method", use + "Report", use + "(Fail).Error", types.EdgeInvokes, false},
		{"error.Error() is not a calls edge to an unrelated Error method", use + "Report", use + "(Fail).Error", types.EdgeCalls, false},
		{"param type resolves to the imported package", use + "Take", one + "T", types.EdgeUsesType, true},
		{"param type does not resolve to the same-named package", use + "Take", two + "T", types.EdgeUsesType, false},
		{"param type resolves to the package-level type", use + "Use", use + "Genesis", types.EdgeUsesType, true},
	}
	for _, c := range cases {
		if got := set[[3]string{c.src, string(c.edge), c.dst}]; got != c.want {
			t.Errorf("%s: edge %s -%s-> %s present = %v, want %v", c.name, c.src, c.edge, c.dst, got, c.want)
		}
	}
	for _, n := range nodes {
		if n.Type == types.NodeStruct && n.Name == "Genesis" && n.CanonicalID != "" && n.CanonicalID != use+"Genesis" {
			t.Errorf("function-local Genesis has canonical id %q, want none", n.CanonicalID)
		}
	}
	local := 0
	for _, n := range nodes {
		if n.Type == types.NodeStruct && n.Name == "Genesis" && n.CanonicalID == "" {
			local++
		}
	}
	if local != 1 {
		t.Errorf("function-local Genesis nodes without canonical id = %d, want 1", local)
	}
}

// TestResolve_CountsUnresolvedReasons: references Resolve drops are counted by
// reason instead of disappearing. testdata/samename converts with uint(3) and
// calls a func variable (targets cleared), so the reason below must show up.
// Its err.Error() call is not dropped: the builtin error method has an
// External node.
func TestResolve_CountsUnresolvedReasons(t *testing.T) {
	g, err := gop.LoadAndResolve("testdata/samename")
	if err != nil {
		t.Fatalf("LoadAndResolve: %v", err)
	}
	if g.Unresolved["no_target"] == 0 {
		t.Errorf("no_target = 0, want the conversion and the func-variable call: %v", g.Unresolved)
	}
	bound := false
	for _, n := range g.Nodes {
		bound = bound || (n.Type == types.NodeExternal && n.CanonicalID == gop.BuiltinErrorMethodID)
	}
	if !bound {
		t.Errorf("err.Error() has no External node for the builtin error method; unresolved: %v", g.Unresolved)
	}
	for reason, n := range g.Unresolved {
		if len(g.UnresolvedSamples[reason]) == 0 || len(g.UnresolvedSamples[reason]) > n {
			t.Errorf("reason %s: %d samples for %d drops", reason, len(g.UnresolvedSamples[reason]), n)
		}
	}
}
