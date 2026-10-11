package solidity_test

import (
	"testing"
)

// This-prefixed nested member-chain walker tests.
//
// Shape: `this.<stateVar>.<f1>.<f2>...<fN>.<method>(...)` for N ≥ 1
// struct-field hops after the stateVar. A separate predicate catches depth-0
// (`this.<stateVar>.<method>`); this walker fires for N ≥ 1.
//
// It is the this-prefixed cousin of the struct-field walkers (which
// explicitly reject `this` as innermost object). Resolver uses
// callerContainerID as the implicit `this` target and looks up
// stateVar in stateVarTypes only (paramTypes excluded — `this` is a
// contract reference, never a parameter).

// TestUsingForV113_Depth1ThisNested — canonical this-chain case. depth-1
// chain `this.user.balance.add(1)`. The depth-0 predicate rejects; the
// this-chain walker fires.
func TestUsingForV113_Depth1ThisNested(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v113", "depth1_this_nested.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "Vault.run", target: "SafeMath.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (using-for V1.13 depth-1) mismatch:\n got=%v\nwant=%v", got, want)
	}
}

// TestUsingForV113_Depth3ThisNested — arbitrary depth (N=3). Confirms
// the walker isn't capped at depth-1.
func TestUsingForV113_Depth3ThisNested(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v113", "depth3_this_nested.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "Top.run", target: "DeepLib.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (using-for V1.13 depth-3) mismatch:\n got=%v\nwant=%v", got, want)
	}
}

// TestUsingForV113_Depth1ThisUnknownField — depth-1 this-chain but the
// field doesn't exist on the stateVar's struct. The walker drops.
func TestUsingForV113_Depth1ThisUnknownField(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v113", "depth1_this_unknown_field.sol")
	got := collectUsingForCalls(nodes, edges)
	if len(got) != 0 {
		t.Errorf("unexpected V1.13 EdgeCalls when struct field unknown: %v", got)
	}
}
