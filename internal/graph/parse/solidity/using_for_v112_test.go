package solidity_test

import (
	"testing"
)

// Generic member-chain walker tests.
//
// Shape: `<obj>.<f1>.<f2>...<fN>.<method>(...)` for N ≥ 3 (depth-3+
// pure member access chain). Dedicated predicates catch N=1 and N=2;
// this fallback handles arbitrary depth via iterative walker through
// structFieldTypes.
//
// Not covered here: this-prefixed variants, multi-return
// tuple destructuring, cross-file struct validation.

// TestUsingForV112_Depth3MemberChain — canonical generic-walker case. depth-3
// chain `org.user.account.balance.add(1)`. The depth-2 predicate rejects;
// the generic walker fires.
func TestUsingForV112_Depth3MemberChain(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v112", "depth3_member_chain.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "Vault.run", target: "DeepLib.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (using-for V1.12 depth-3) mismatch:\n got=%v\nwant=%v", got, want)
	}
}

// TestUsingForV112_Depth4MemberChain — arbitrary depth (N=4). Confirms
// the walker isn't capped at any specific depth.
func TestUsingForV112_Depth4MemberChain(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v112", "depth4_member_chain.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "Top.run", target: "MoreLib.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (using-for V1.12 depth-4) mismatch:\n got=%v\nwant=%v", got, want)
	}
}

// TestUsingForV112_Depth3MidUnknown — depth-3 chain but middle hop's
// field doesn't exist on the parent struct. The walker drops.
func TestUsingForV112_Depth3MidUnknown(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v112", "depth3_mid_unknown.sol")
	got := collectUsingForCalls(nodes, edges)
	if len(got) != 0 {
		t.Errorf("unexpected V1.12 EdgeCalls when middle hop unknown: %v", got)
	}
}
