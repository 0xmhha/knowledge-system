package solidity_test

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Modifier-scope self-cast audit. Modifiers are emitted
// via runDecl(NodeModifier) and the cast walker keys
// the HasSelfReentrantCall marker on the (qname, startByte) pair that
// nearestFunctionQnameAndStart returns. Since that
// helper recognises modifier_definition as an enclosing-callable
// shape, the marker should land on the NodeModifier row when the
// body contains payable(this).call(...) or .transfer(...).
//
// This test locks the coverage. Modifiers wrapping other functions are a
// classic re-entrancy vector: a self-call inside `_; payable(this).call(...)`
// re-enters the wrapped function before its state-mutation hooks
// settle. The marker must surface that surface independently of the
// wrapped function's own markers.
//
// Two contracts exercise the two cast-walker admission paths:
//
//   - GuardSelf.reentrantGuard   : property=="call",
//     inner=payable(this) → selfAffected.
//   - GuardTransfer.refundGuard  : property=="transfer", admitted by
//     the value-transfer branch alongside the low-level-call branch.
//
// Both modifier rows should end up with HasSelfReentrantCall=true and
// HasExternalCall=false — the self-cast precedence rule still applies on
// NodeModifier (the walker doesn't gate on node type).
func TestModifier_SelfReentrantCast(t *testing.T) {
	nodes, _ := parseResolveOneSol(t, "testdata/yul_receiver",
		"sol_modifier_self_cast.sol")

	want := map[string]bool{
		"GuardSelf.reentrantGuard":  true,
		"GuardTransfer.refundGuard": true,
	}
	got := map[string]bool{}
	for _, n := range nodes {
		if n.Type != types.NodeModifier {
			continue
		}
		if _, ok := want[n.QualifiedName]; !ok {
			continue
		}
		got[n.QualifiedName] = n.HasSelfReentrantCall
		if n.HasExternalCall {
			t.Errorf("%s HasExternalCall: got true (V17 expects false; self-cast routes to reentrant)",
				n.QualifiedName)
		}
	}
	for qn, w := range want {
		g, present := got[qn]
		if !present {
			t.Errorf("missing modifier %q", qn)
			continue
		}
		if g != w {
			t.Errorf("%s HasSelfReentrantCall: got %v, want %v", qn, g, w)
		}
	}
}
