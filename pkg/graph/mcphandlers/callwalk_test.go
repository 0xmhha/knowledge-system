package mcphandlers

import (
	"errors"
	"slices"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// memReader is an in-memory call graph. Only the reads the call walk makes
// are implemented; any other read panics on the nil embedded Reader.
type memReader struct {
	store.Reader
	edges []types.Edge
}

func (m memReader) NodesByIDs(ids []string) ([]types.Node, error) {
	out := make([]types.Node, 0, len(ids))
	for _, id := range ids {
		out = append(out, types.Node{ID: id, QualifiedName: id, Type: types.NodeMethod})
	}
	return out, nil
}

func (m memReader) NeighborhoodByIDs(ids []string, depth int, reverse bool, edgeTypes ...string) ([]types.Node, []types.Edge, error) {
	if depth != 1 {
		return nil, nil, errors.New("the call walk reads one hop at a time")
	}
	var es []types.Edge
	reached := slices.Clone(ids)
	for _, e := range m.edges {
		near, far := e.Src, e.Dst
		if reverse {
			near, far = far, near
		}
		if slices.Contains(ids, near) && slices.Contains(edgeTypes, string(e.Type)) {
			es = append(es, e)
			reached = append(reached, far)
		}
	}
	slices.Sort(reached)
	ns, err := m.NodesByIDs(slices.Compact(reached))
	return ns, es, err
}

// failReader fails every read the call walk makes.
type failReader struct{ store.Reader }

var errRead = errors.New("read failed")

func (failReader) NodesByIDs([]string) ([]types.Node, error) { return nil, errRead }

func (failReader) NeighborhoodByIDs([]string, int, bool, ...string) ([]types.Node, []types.Edge, error) {
	return nil, nil, errRead
}

func call(src, dst string) types.Edge { return types.Edge{Src: src, Dst: dst, Type: types.EdgeCalls} }

func invoke(src, dst string) types.Edge {
	return types.Edge{Src: src, Dst: dst, Type: types.EdgeInvokes, DispatchKind: dispatchInterfaceMethod}
}

func dispatch(src, dst, kind string) types.Edge {
	return types.Edge{Src: src, Dst: dst, Type: types.EdgeDispatchesTo, DispatchKind: kind}
}

func nodeIDs(ns []types.Node) []string {
	out := make([]string, 0, len(ns))
	for _, n := range ns {
		out = append(out, n.ID)
	}
	return out
}

func TestWalkCalls_Hops(t *testing.T) {
	// main calls run; run calls I.M; I.M can run T.M; T.M calls leaf.
	chain := []types.Edge{call("main", "run"), invoke("run", "I.M"), dispatch("I.M", "T.M", dispatchInterfaceMethod), call("T.M", "leaf")}
	// A second interface behind the first.
	twice := append(slices.Clone(chain), invoke("leaf", "J.N"), dispatch("J.N", "U.N", dispatchTypeHierarchy), call("U.N", "end"))
	cases := []struct {
		name    string
		edges   []types.Edge
		seed    string
		depth   int
		reverse bool
		want    []string
	}{
		{"implementation shares the interface method's hop", chain, "run", 1, false, []string{"I.M", "T.M"}},
		{"its callee is one hop further", chain, "run", 2, false, []string{"I.M", "T.M", "leaf"}},
		{"a call counts, an interface does not", chain, "main", 2, false, []string{"I.M", "T.M", "run"}},
		{"two interfaces on the way", twice, "run", 4, false, []string{"I.M", "J.N", "T.M", "U.N", "end", "leaf"}},
		{"two interfaces, one hop short", twice, "run", 3, false, []string{"I.M", "J.N", "T.M", "U.N", "leaf"}},
		{"reverse through the interface", chain, "leaf", 2, true, []string{"I.M", "T.M", "run"}},
		{"reverse, one hop short", chain, "leaf", 1, true, []string{"I.M", "T.M"}},
		{"reverse through two interfaces", twice, "end", 4, true, []string{"I.M", "J.N", "T.M", "U.N", "leaf", "run"}},
		{"reverse through two interfaces, one hop short", twice, "end", 3, true, []string{"I.M", "J.N", "T.M", "U.N", "leaf"}},
		{"reverse from the method itself", chain, "T.M", 1, true, []string{"I.M", "run"}},
		{"a function value is a call", []types.Edge{dispatch("each", "cb", "func_value"), call("cb", "leaf")}, "each", 1, false, []string{"cb"}},
		{
			"value flow first, the type hierarchy a hop later",
			[]types.Edge{invoke("run", "I.M"), dispatch("I.M", "T.M", dispatchInterfaceMethod), dispatch("I.M", "Unused.M", dispatchTypeHierarchy)},
			"run", 1, false, []string{"I.M", "T.M"},
		},
		{
			"the hidden type hierarchy is still reached",
			[]types.Edge{invoke("run", "I.M"), dispatch("I.M", "T.M", dispatchInterfaceMethod), dispatch("I.M", "Unused.M", dispatchTypeHierarchy)},
			"run", 2, false, []string{"I.M", "T.M", "Unused.M"},
		},
		{"the type hierarchy alone", []types.Edge{invoke("run", "I.M"), dispatch("I.M", "T.M", dispatchTypeHierarchy)}, "run", 1, false, []string{"I.M", "T.M"}},
		{
			"recursion and a cycle through an interface end",
			[]types.Edge{call("a", "a"), invoke("a", "I.M"), dispatch("I.M", "T.M", dispatchInterfaceMethod), invoke("T.M", "I.M")},
			"a", 5, false, []string{"I.M", "T.M"},
		},
		{"depth 0 walks no call", chain, "run", 0, false, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			nodes, edges, err := walkCalls(memReader{edges: c.edges}, []string{c.seed}, c.depth, c.reverse)
			if err != nil {
				t.Fatal(err)
			}
			var got []string
			for _, id := range nodeIDs(nodes) {
				if id != c.seed {
					got = append(got, id)
				}
			}
			slices.Sort(got)
			if !slices.Equal(got, c.want) {
				t.Errorf("walk from %s depth %d reverse %v = %v, want %v", c.seed, c.depth, c.reverse, got, c.want)
			}
			seen := map[types.Edge]bool{}
			for _, e := range edges {
				if seen[e] {
					t.Errorf("edge %s -> %s listed twice", e.Src, e.Dst)
				}
				seen[e] = true
				if ids := nodeIDs(nodes); !slices.Contains(ids, e.Src) || !slices.Contains(ids, e.Dst) {
					t.Errorf("edge %s -> %s names a node that is not in the answer", e.Src, e.Dst)
				}
			}
		})
	}
}

// Nodes come in hop order, so the answer is the same on every run and names
// near nodes first.
func TestWalkCalls_Order(t *testing.T) {
	edges := []types.Edge{call("s", "z"), call("z", "a"), invoke("s", "I.M"), dispatch("I.M", "T.M", dispatchInterfaceMethod)}
	nodes, _, err := walkCalls(memReader{edges: edges}, []string{"s"}, 2, false)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := nodeIDs(nodes), []string{"s", "I.M", "T.M", "z", "a"}; !slices.Equal(got, want) {
		t.Errorf("order = %v, want %v", got, want)
	}
}

// The walk keeps the reader's error instead of answering with part of a graph.
func TestWalkCalls_Error(t *testing.T) {
	if _, _, err := walkCalls(failReader{}, []string{"s"}, 1, false); !errors.Is(err, errRead) {
		t.Errorf("walkCalls error = %v, want %v", err, errRead)
	}
}
