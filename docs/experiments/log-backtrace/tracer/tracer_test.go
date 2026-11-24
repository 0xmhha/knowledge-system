package main

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

func TestFragments(t *testing.T) {
	cases := map[string][]string{
		"message too long":                  {"message too long"},
		"cannot read %s from the disk: %v":  {"from the disk"},
		"outer failed: inner %d of %d left": {"outer failed", "inner"},
		"%v":                                nil,
	}
	for in, want := range cases {
		if got := fragments(in); !slices.Equal(got, want) {
			t.Errorf("fragments(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestLiteralLines(t *testing.T) {
	dir := t.TempDir()
	for name, src := range map[string]string{
		"a.go":      "package a\n// too long in a comment\nvar e = errors.New(\"msg too long\")\n",
		"a_test.go": "package a\nvar t = \"msg too long\"\n",
	} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got, err := literalLines(dir, "too long")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(got, []position{{"a.go", 3}}) {
		t.Errorf("got %v", got)
	}
}

func node(id string, k types.NodeType, qname, file string, s, e int) types.Node {
	return types.Node{ID: id, Type: k, Name: qname, QualifiedName: qname, FilePath: file, StartLine: s, EndLine: e, CanonicalID: id}
}

// TestTrace follows one message through every rule: emit fails when a field is
// zero, limit is compared in the same condition, load returns the checked
// value, build leaves the field out, set writes it, top calls emit through an
// interface.
func TestTrace(t *testing.T) {
	const fn, f = types.NodeFunction, "a.go"
	nodes := []types.Node{
		node("emit", fn, "a.emit", f, 10, 20), node("limit", types.NodeConstant, "a.limit", f, 1, 1),
		node("local", types.NodeVariable, "a.local", f, 12, 12),
		node("field", types.NodeField, "a.T.F", f, 3, 3), node("other", types.NodeField, "a.T.G", f, 4, 4),
		node("load", fn, "a.load", f, 30, 40), node("build", fn, "a.build", f, 50, 60), node("set", fn, "a.set", f, 70, 80),
		node("iface", types.NodeMethod, "a.I.Emit", f, 5, 5), node("top", fn, "a.top", f, 90, 95),
		node("inTest", fn, "a.inTest", "a_test.go", 1, 9),
		{ID: "ext", Type: types.NodeExternal, QualifiedName: "fmt.Errorf", FilePath: "(external)"},
	}
	edge := func(src, dst string, k types.EdgeType, line int) types.Edge {
		return types.Edge{Src: src, Dst: dst, Type: k, Line: line}
	}
	ix := newIndex(nodes, []types.Edge{
		edge("emit", "limit", types.EdgeReferences, 14), edge("emit", "local", types.EdgeReferences, 14),
		edge("emit", "field", types.EdgeReadsField, 14), edge("emit", "load", types.EdgeCalls, 13),
		edge("emit", "ext", types.EdgeCalls, 15), edge("build", "other", types.EdgeWritesField, 55),
		edge("set", "field", types.EdgeWritesField, 75), edge("set", "other", types.EdgeWritesField, 76),
		edge("iface", "emit", types.EdgeDispatchesTo, 0), edge("top", "iface", types.EdgeInvokes, 92),
		edge("inTest", "emit", types.EdgeCalls, 3),
	})
	flow := func(_ context.Context, id string, _ []int) flowLines {
		if id == "emit" {
			return flowLines{guards: []int{14}, defs: []int{13}}
		}
		return flowLines{failed: true}
	}
	got := newTracer(ix, flow).run(context.Background(), [][]position{{{f, 15}}})
	want := map[string]float64{"emit": 1, "limit": 0.8, "build": 0.8, "set": 0.6, "load": 0.6, "top": 0.5}
	if len(got) != len(want) {
		t.Fatalf("got %d candidates %+v, want %d", len(got), got, len(want))
	}
	for _, r := range got {
		if w, ok := want[r.ID]; !ok || r.Score != w {
			t.Errorf("%s: score %v, want %v (why %v)", r.ID, r.Score, w, r.Why)
		}
	}
	if got[0].ID != "emit" || got[0].Rank != 1 {
		t.Errorf("first is %+v", got[0])
	}
}

func TestSitesThroughErrorValue(t *testing.T) {
	ix := newIndex([]types.Node{
		node("err", types.NodeVariable, "a.errX", "a.go", 3, 3), node("u", types.NodeFunction, "a.use", "b.go", 1, 9),
	}, []types.Edge{{Src: "u", Dst: "err", Type: types.EdgeReferences, Line: 5}})
	got := newTracer(ix, nil).sites([]position{{"a.go", 3}})
	if len(got) != 1 || got[0].fn != "u" || !slices.Equal(got[0].lines, []int{5}) {
		t.Errorf("got %+v", got)
	}
}
