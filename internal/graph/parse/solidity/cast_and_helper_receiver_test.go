package solidity_test

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Cast-receiver / helper-return chain negative lock.
//
// chained_cross_contract.sol covers the `s.getCb()(x)` shape
// where the outer callee is itself a call_expression.
// cross_contract_fn_pointer.go documents two more shapes as
// deferred:
//
//   - Cast-receiver: `Hub(addr).onAction(x)` — receiver is a
//     contract-cast call_expression.
//   - Helper-return-receiver: `getHub().onAction(x)` — receiver is
//     an internal helper's return value.
//
// Both fall through matchStateVarMethodCall's bare-identifier
// receiver requirement. This test pins all three rows in one fixture:
//
//   - bareInvoke (reference)   : HasFunctionPointerCall=true   (covered)
//   - castInvoke   (cell A)    : HasFunctionPointerCall=false  (dropped)
//   - helperInvoke (cell B)    : HasFunctionPointerCall=false  (dropped)
//
// The reference row is essential: it pins the cross-contract walker's positive baseline
// inside the same fixture so the negative cells are *specifically*
// about receiver-shape variance from the bare-identifier case, not
// about cross-contract propagation being broken generally. If it ever regresses
// on the bare path, bareInvoke fails and the diagnosis is
// "general regression," not "negative locks unexpectedly flipped."
//
// Cross-flip protocol: if any of the three rows flips, both this
// test value and WALKER_SYMMETRY drift row 5 must move together —
// the catalogue lists the chained-callee and cast/helper-receiver
// shapes as the *same* limitation family.
func TestW8V27_CastAndHelperReceiver_NegativeLock(t *testing.T) {
	nodes, _ := parseResolveOneSol(t, "testdata/function_typed_var",
		"cast_and_helper_receiver.sol")

	type cell struct {
		ptrCall bool
		ptrProp bool
	}
	want := map[string]cell{
		// Reference row: bare-identifier positive baseline.
		"Caller.bareInvoke": {ptrCall: true, ptrProp: false},
		// Cell A: cast-receiver shape.
		"Caller.castInvoke": {ptrCall: false, ptrProp: false},
		// Cell B: helper-return-receiver shape.
		"Caller.helperInvoke": {ptrCall: false, ptrProp: false},
	}
	got := map[string]cell{}
	for _, n := range nodes {
		if n.Type != types.NodeFunction {
			continue
		}
		if _, ok := want[n.QualifiedName]; !ok {
			continue
		}
		got[n.QualifiedName] = cell{
			ptrCall: n.HasFunctionPointerCall,
			ptrProp: n.HasFunctionPointerPropagation,
		}
	}
	for qn, w := range want {
		g, present := got[qn]
		if !present {
			t.Errorf("missing function %q", qn)
			continue
		}
		if g != w {
			t.Errorf("%s: got %+v, want %+v\n  (if a walker change flipped this row on purpose, flip the want value AND update WALKER_SYMMETRY drift row 5 — V26 and V27 are the same limitation family and must move together)",
				qn, g, w)
		}
	}
}
