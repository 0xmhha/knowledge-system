package solidity_test

import (
	"testing"
)

// Deep cross-contract chained dispatch tests.
//
// Shape: `<obj>.<innerFn1>().<innerFn2>().<method>(...)`. Combines
// the cross-contract receiver-type lookup with the depth-2 return-type chain.
//
// Not covered here: depth >= 3 generic chains
// (`f().g().h().i()`), multi-return tuple slot selection.

// TestUsingForV16_DeepCrossChainBasic — canonical deep cross-contract case.
// `factory.makeStage().computeNumber().add(1)`. Receiver factory is a
// Factory state var; makeStage returns Stage; computeNumber returns
// uint256; .add(1) resolves through using ChainLib for uint256.
func TestUsingForV16_DeepCrossChainBasic(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v16", "deep_cross_chain_basic.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "Vault.run", target: "ChainLib.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (using-for deep-cross-chain) mismatch:\n got=%v\nwant=%v", got, want)
	}
}

// TestUsingForV16_DeepCrossChainUnknownMiddle — middle step's method
// doesn't exist on the inner function's return type. The resolver must drop
// without false-positive emission.
func TestUsingForV16_DeepCrossChainUnknownMiddle(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v16", "deep_cross_chain_unknown_middle.sol")
	got := collectUsingForCalls(nodes, edges)
	if len(got) != 0 {
		t.Errorf("unexpected V1.6 EdgeCalls when middle step unknown: %v", got)
	}
}

// TestUsingForV16_DeepCrossChainNoBinding — chain links all resolve
// but the final return type has no binding declaration. The resolver must drop
// at the binding lookup step.
func TestUsingForV16_DeepCrossChainNoBinding(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v16", "deep_cross_chain_no_binding.sol")
	got := collectUsingForCalls(nodes, edges)
	if len(got) != 0 {
		t.Errorf("unexpected V1.6 EdgeCalls when binding misses: %v", got)
	}
}
