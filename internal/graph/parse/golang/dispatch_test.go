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
