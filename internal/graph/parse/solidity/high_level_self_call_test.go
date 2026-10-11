package solidity_test

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// High-level self-call marker audit.
//
// The self-cast tests cover the low-level self-call surface
// (`payable(this).call`/`delegatecall`/`transfer`/`send` and the
// `.call{...}` chained syntax). This test covers the *parallel* high-level
// surface — typed dispatch through the same EVM message-call
// boundary. The two are independent markers because security
// tooling routinely characterises them differently (low-level
// allows arbitrary signatures, high-level is type-safe but still
// re-entrant), and a single boolean would force consumers to merge
// distinguishable signals.
//
// Three fixture contracts exercise the three recognised shapes:
//
//   - ThisCall.fire       : this.target()
//     bare-this dispatch (no cast wrapper).
//   - ContractCast.fire   : ThisCall(address(this)).target()
//     contract-type cast wrapping address(this) — the recursive
//     branch in isSelfRef unwinds two casts to reach `this`.
//   - InterfaceCast.fire  : IFoo(address(this)).foo()
//     same pattern through an interface type cast.
//
// All three should end up HasHighLevelSelfCall=true. The low-level
// markers (HasSelfReentrantCall, HasExternalCall) must stay false —
// the low-level walker doesn't fire for typed method names, and the high-level one doesn't
// poison the low-level surface for the same reason.
func TestHighLevel_SelfCallMarker(t *testing.T) {
	nodes, _ := parseResolveOneSol(t, "testdata/yul_receiver",
		"sol_high_level_self_call.sol")

	want := map[string]bool{
		"ThisCall.fire":      true,
		"ContractCast.fire":  true,
		"InterfaceCast.fire": true,
	}
	got := map[string]bool{}
	for _, n := range nodes {
		if n.Type != types.NodeFunction {
			continue
		}
		if _, ok := want[n.QualifiedName]; !ok {
			continue
		}
		got[n.QualifiedName] = n.HasHighLevelSelfCall
		// The high-level marker must not leak into the low-level markers — those stay
		// reserved for the self-cast surface so cks can keep the two
		// axes distinct in its rerank weights.
		if n.HasSelfReentrantCall {
			t.Errorf("%s HasSelfReentrantCall: got true (V19 expects false; high-level call is a separate marker)",
				n.QualifiedName)
		}
		if n.HasExternalCall {
			t.Errorf("%s HasExternalCall: got true (V19 expects false; high-level self-call is not arbitrary-address dispatch)",
				n.QualifiedName)
		}
	}
	for qn, w := range want {
		g, present := got[qn]
		if !present {
			t.Errorf("missing function %q", qn)
			continue
		}
		if g != w {
			t.Errorf("%s HasHighLevelSelfCall: got %v, want %v", qn, g, w)
		}
	}
}
