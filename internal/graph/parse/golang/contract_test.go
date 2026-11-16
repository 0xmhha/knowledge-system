package golang_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/contract"
	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	gop "github.com/0xmhha/knowledge-system/internal/graph/parse/golang"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

func productionOnly(string) contract.Layer { return contract.LayerProduction }

// TestContract_RoundTrip: the resolved graph of each fixture module comes back
// unchanged from the records, so the records hold everything the Go pipeline
// produces.
func TestContract_RoundTrip(t *testing.T) {
	for _, dir := range []string{"concurrency", "context_paths", "distributed", "identity", "implements_xpkg", "samename"} {
		t.Run(dir, func(t *testing.T) {
			rg, err := gop.LoadAndResolve("testdata/" + dir)
			if err != nil {
				t.Fatalf("LoadAndResolve: %v", err)
			}
			if len(rg.Nodes) == 0 || len(rg.Edges) == 0 {
				t.Fatalf("fixture produced %d nodes, %d edges", len(rg.Nodes), len(rg.Edges))
			}
			u, err := gop.ToUnit(rg, productionOnly)
			if err != nil {
				t.Fatalf("ToUnit: %v", err)
			}
			back := gop.FromUnit(u)
			if !reflect.DeepEqual(back.Nodes, rg.Nodes) {
				t.Errorf("nodes differ after the round trip")
			}
			if !reflect.DeepEqual(back.Edges, rg.Edges) {
				t.Errorf("edges differ after the round trip")
			}
			if !reflect.DeepEqual(back.Unresolved, rg.Unresolved) {
				t.Errorf("unresolved counts differ: %v vs %v", back.Unresolved, rg.Unresolved)
			}
		})
	}
}

// TestContract_Records: the records carry the symbol id, the common kind, the
// layer and the mapped confidence.
func TestContract_Records(t *testing.T) {
	rg, err := gop.LoadAndResolve("testdata/identity")
	if err != nil {
		t.Fatal(err)
	}
	layer := func(f string) contract.Layer {
		if strings.HasSuffix(f, "_test.go") {
			return contract.LayerTest
		}
		return contract.LayerProduction
	}
	u, err := gop.ToUnit(rg, layer)
	if err != nil {
		t.Fatal(err)
	}
	var sawCallable bool
	for i, d := range u.Decls {
		n := rg.Nodes[i]
		if n.CanonicalID != "" && d.SymbolID != "go:"+n.CanonicalID {
			t.Errorf("%s: symbol id %q, want go:%s", n.QualifiedName, d.SymbolID, n.CanonicalID)
		}
		if n.CanonicalID == "" && d.SymbolID != "" {
			t.Errorf("%s: symbol id %q without a canonical id", n.QualifiedName, d.SymbolID)
		}
		if d.Layer != layer(n.FilePath) {
			t.Errorf("%s: layer %q", n.QualifiedName, d.Layer)
		}
		if n.Type == types.NodeFunction {
			sawCallable = true
			if d.Kind != contract.KindCallable {
				t.Errorf("%s: kind %q, want callable", n.QualifiedName, d.Kind)
			}
		}
	}
	if !sawCallable {
		t.Error("fixture has no function")
	}
	for i, r := range u.Refs {
		want := map[types.Confidence]contract.Confidence{types.ConfExtracted: contract.Certain, types.ConfInferred: contract.StaticMay, types.ConfAmbiguous: contract.Unbound}[rg.Edges[i].Confidence]
		if r.Confidence != want {
			t.Errorf("edge %d: confidence %q from %q", i, r.Confidence, rg.Edges[i].Confidence)
		}
	}
}

// TestContract_RefusesFieldsItCannotHold: a node with a field outside the
// records, or a confidence without a contract value, is an error instead of a
// silent loss.
func TestContract_RefusesFieldsItCannotHold(t *testing.T) {
	base := types.Node{ID: "0123456789abcdef", Type: types.NodeFunction, Name: "f", QualifiedName: "p.f",
		FilePath: "p/f.go", StartLine: 1, EndLine: 2, EndByte: 10, Language: "go", Confidence: types.ConfExtracted}
	withMarker := base
	withMarker.HasAssembly = true
	unknown := types.Edge{Src: base.ID, Dst: base.ID, Type: types.EdgeCalls, Count: 1, Confidence: "GUESSED"}
	cases := map[string]struct {
		nodes []types.Node
		edges []types.Edge
	}{
		"solidity marker":   {nodes: []types.Node{withMarker}},
		"unknown edge conf": {nodes: []types.Node{base}, edges: []types.Edge{unknown}},
		"empty node conf":   {nodes: []types.Node{{ID: base.ID, Type: types.NodeFunction, Language: "go"}}},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			rg := &parse.ResolvedGraph{Nodes: c.nodes, Edges: c.edges}
			if _, err := gop.ToUnit(rg, productionOnly); err == nil {
				t.Error("want an error")
			}
		})
	}
	if _, err := gop.ToUnit(&parse.ResolvedGraph{Nodes: []types.Node{base}}, productionOnly); err != nil {
		t.Errorf("a plain node: %v", err)
	}
}

// TestContract_DropsScores: scores a reloaded node keeps from the previous
// build are not carried (score.Compute sets them again), and empty lists read
// back from the store count as absent.
func TestContract_DropsScores(t *testing.T) {
	n := types.Node{ID: "0123456789abcdef", Type: types.NodeFunction, Name: "f", QualifiedName: "p.f",
		FilePath: "p/f.go", StartLine: 1, EndLine: 2, EndByte: 10, Language: "go", Confidence: types.ConfExtracted,
		InDegree: 3, OutDegree: 2, PageRank: 0.25, UsageScore: 1, YulBuiltins: []string{}, RecentPRs: []types.PRRef{}}
	u, err := gop.ToUnit(&parse.ResolvedGraph{Nodes: []types.Node{n}}, productionOnly)
	if err != nil {
		t.Fatal(err)
	}
	got := gop.FromUnit(u).Nodes[0]
	if got.InDegree != 0 || got.OutDegree != 0 || got.PageRank != 0 || got.UsageScore != 0 {
		t.Errorf("scores carried: %+v", got)
	}
	if got.QualifiedName != n.QualifiedName || got.EndByte != n.EndByte {
		t.Errorf("declaration fields lost: %+v", got)
	}
}
