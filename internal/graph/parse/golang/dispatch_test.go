package golang_test

import (
	"testing"

	gop "github.com/0xmhha/knowledge-system/internal/graph/parse/golang"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// TestDispatchEdges checks the runtime links of testdata/resolve/coll1/dispatch.go:
// an interface method links to the implementations whose values reach one of
// its calls, and a call through a function value links to the function passed.
func TestDispatchEdges(t *testing.T) {
	g, err := gop.LoadAndResolve("testdata/resolve")
	if err != nil {
		t.Fatalf("LoadAndResolve: %v", err)
	}
	qn := map[string]string{}
	for _, n := range g.Nodes {
		qn[n.ID] = n.QualifiedName
	}
	type key struct{ src, dst, kind string }
	got := map[key]types.Edge{}
	for _, e := range g.Edges {
		if e.Type == types.EdgeDispatchesTo {
			got[key{qn[e.Src], qn[e.Dst], e.DispatchKind}] = e
		}
	}
	for _, w := range []key{
		{"coll1.Tipper.Tip", "coll1.govTip.Tip", "interface_method"},
		{"coll1.Tipper.Tip", "coll1.fixedTip.Tip", "interface_method"},
		{"coll1.eachBlock", "coll1.onBlock", "func_value"},
	} {
		e, ok := got[w]
		if !ok {
			t.Errorf("missing dispatches_to %v; have %v", w, got)
			continue
		}
		if e.Confidence != types.ConfInferred {
			t.Errorf("%v: confidence %s, want %s", w, e.Confidence, types.ConfInferred)
		}
	}
	if w := (key{"coll1.eachBlock", "coll1.onBlock", "func_value"}); got[w].Line != 36 {
		t.Errorf("%v: line %d, want 36 (the call f(1))", w, got[w].Line)
	}
	// unusedTip never reaches a Tipper and Thing never reaches a Hasher: no
	// link from the value flow, only the weaker one from the type hierarchy.
	for _, k := range []key{{"coll1.Tipper.Tip", "coll1.unusedTip.Tip", ""}, {"coll1.Hasher.Hash", "coll1.Thing.Hash", ""}} {
		if _, ok := got[key{k.src, k.dst, "interface_method"}]; ok {
			t.Errorf("value-flow dispatches_to %s -> %s: the type never flows into the interface", k.src, k.dst)
		}
		e, ok := got[key{k.src, k.dst, "type_hierarchy"}]
		if !ok {
			t.Errorf("missing type_hierarchy dispatches_to %s -> %s", k.src, k.dst)
		} else if e.Confidence != types.ConfInferred {
			t.Errorf("type_hierarchy confidence %s", e.Confidence)
		}
	}
	// A type-hierarchy edge is not added where the value flow already links.
	if _, ok := got[key{"coll1.Tipper.Tip", "coll1.govTip.Tip", "type_hierarchy"}]; ok {
		t.Errorf("type_hierarchy edge duplicates a value-flow edge")
	}
	// Static calls stay calls edges only.
	for k := range got {
		if k.src == "coll1.runTips" {
			t.Errorf("static call recorded as dispatch: %v", k)
		}
	}
}

// TestDispatchEdgesLocalAndAbstract: an interface declared in a function or
// written inline in a type assertion dispatches like a package-level one,
// and no dispatch ends at an interface method (a struct embedding Pricer
// selects Pricer.Price, which has no body). Before, the 31 dispatches of
// such interfaces in go-stablenet were missing, and one type_hierarchy edge
// ended at an interface method.
func TestDispatchEdgesLocalAndAbstract(t *testing.T) {
	g, err := gop.LoadAndResolve("testdata/dispatch_local")
	if err != nil {
		t.Fatalf("LoadAndResolve: %v", err)
	}
	byID := map[string]types.Node{}
	for _, n := range g.Nodes {
		byID[n.ID] = n
	}
	got := map[[2]string]bool{}
	for _, e := range g.Edges {
		if e.Type != types.EdgeDispatchesTo {
			continue
		}
		src, dst := byID[e.Src], byID[e.Dst]
		got[[2]string{src.QualifiedName, dst.QualifiedName}] = true
		if dst.Type == types.NodeMethod && (dst.QualifiedName == "local.Pricer.Price" || dst.QualifiedName == "local.Backend.Price") {
			t.Errorf("dispatch %s -> %s ends at an interface method (%s)", src.QualifiedName, dst.QualifiedName, e.DispatchKind)
		}
	}
	var inline, declared string
	for _, n := range g.Nodes {
		if n.Type == types.NodeMethod && n.Name == "Bytes" && n.CanonicalID == "" {
			if n.QualifiedName == "local.Count.sized.Bytes" {
				declared = n.QualifiedName
			} else {
				inline = n.QualifiedName
			}
		}
	}
	if inline == "" || declared == "" {
		t.Fatalf("no node of the local interface methods: inline %q, declared %q", inline, declared)
	}
	for _, w := range [][2]string{
		{inline, "local.word.Bytes"}, {inline, "local.pair.Bytes"},
		{declared, "local.word.Bytes"}, {declared, "local.pair.Bytes"},
		{"local.Backend.Price", "local.fixed.Price"},
	} {
		if !got[w] {
			t.Errorf("missing dispatches_to %s -> %s; have %v", w[0], w[1], got)
		}
	}
}
