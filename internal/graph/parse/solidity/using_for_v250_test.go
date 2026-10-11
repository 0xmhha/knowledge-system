package solidity_test

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Operator-form using directive with free-function
// target.
//
// The file-level operator-form recovery walker
// parses the braced body of `using {mul as *} for uint256 global;`
// from raw ERROR text and emits PendingRefs. The original lock
// asserted those refs drop at resolution because resolveUsingForRef
// only looked up byName[NodeContract]. A NodeFunction
// fallback now lets the free-function form resolve to the free function
// itself, flipping the historic 0-edge lock to one EdgeUsesFor per
// non-library container.

// TestUsingForV250_OperatorFormFreeFunctionResolves — `using {mul
// as *} for uint256 global;` at file scope now resolves through to
// the free function `mul` (NodeFunction). Calc is the only non-
// library container in the file and gets the binding edge.
func TestUsingForV250_OperatorFormFreeFunctionResolves(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v250", "operator_form.sol")

	byID := map[string]types.Node{}
	for _, n := range nodes {
		byID[n.ID] = n
	}

	type uf struct{ src, dst string }
	var got []uf
	for _, e := range edges {
		if e.Type != types.EdgeUsesFor {
			continue
		}
		got = append(got, uf{src: byID[e.Src].Name, dst: byID[e.Dst].Name})
	}

	want := uf{src: "Calc", dst: "mul"}
	if len(got) != 1 || got[0] != want {
		t.Errorf("expected one EdgeUsesFor %+v, got %v", want, got)
	}

	wantNodes := []string{"mul", "Calc", "Calc.compute"}
	seen := map[string]bool{}
	for _, n := range nodes {
		seen[n.QualifiedName] = true
	}
	for _, qn := range wantNodes {
		if !seen[qn] {
			t.Errorf("surround-safety: %q not indexed", qn)
		}
	}
}
