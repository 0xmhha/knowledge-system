package contract

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// TestConfidence_RoundTrip: every stored label maps to a contract value and
// back to itself; an unknown label has no value.
func TestConfidence_RoundTrip(t *testing.T) {
	for _, c := range []types.Confidence{types.ConfExtracted, types.ConfInferred, types.ConfAmbiguous} {
		v, ok := ConfidenceOf(c)
		if !ok || v.Stored() != c {
			t.Errorf("%s -> %q (%v) -> %s", c, v, ok, v.Stored())
		}
	}
	if _, ok := ConfidenceOf("GUESSED"); ok {
		t.Error("an unknown label got a contract value")
	}
}

func TestSymbolID(t *testing.T) {
	id := SymbolID("go", "example.com/p.(*T).M")
	if id != "go:example.com/p.(*T).M" || Identity("go", id) != "example.com/p.(*T).M" {
		t.Errorf("id %q, identity %q", id, Identity("go", id))
	}
	if SymbolID("go", "") != "" || Identity("go", "") != "" {
		t.Error("an empty identity must give an empty id")
	}
}

func TestKindOf(t *testing.T) {
	cases := map[types.NodeType]Kind{
		types.NodeMethod: KindCallable, types.NodeStruct: KindType, types.NodeInterface: KindInterface,
		types.NodePackage: KindModule, types.NodeMutex: KindConcurrency, types.NodeCallSite: KindStatement,
		types.NodeCommit: KindOther,
	}
	for nt, want := range cases {
		if got := KindOf(nt); got != want {
			t.Errorf("KindOf(%s) = %s, want %s", nt, got, want)
		}
	}
}
