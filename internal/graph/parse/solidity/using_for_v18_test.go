package solidity_test

import (
	"testing"
)

// Generic iterative walker tests.
//
// The generic walker handles chains the hardcoded predicates don't catch:
//   - depth ≥ 4 same-contract chains
//   - depth ≥ 3 cross-contract chains
//
// The hardcoded predicates still catch the shallower shapes via
// earlier caller-dispatch branches; the generic walker only fires when those reject.
//
// Not covered here: multi-return tuple slot selection,
// member-of-member receivers (`a.b.c.foo()`), grammar-blocked items.

// TestUsingForV18_Depth4SameContract — 4 same-contract chain links.
// `mkA().mkB().mkC().mkD().add(1)` — depth-3 predicate rejects, generic walker fires.
func TestUsingForV18_Depth4SameContract(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v18", "depth4_same_contract.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "Vault.run", target: "GenLib.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (using-for generic) mismatch:\n got=%v\nwant=%v", got, want)
	}
}

// TestUsingForV18_Depth3CrossContract — 3 cross-contract chain links.
// `obj.foo().bar().baz().add(1)` — depth-2 cross predicate rejects, generic walker fires.
func TestUsingForV18_Depth3CrossContract(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v18", "depth3_cross_contract.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "Caller.run", target: "CrossLib.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (using-for generic cross) mismatch:\n got=%v\nwant=%v", got, want)
	}
}

// TestUsingForV18_Depth5SameContract — arbitrary depth (5 links).
// Confirms generic walker scales without hardcoded ceiling.
func TestUsingForV18_Depth5SameContract(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v18", "depth5_same_contract.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "Deep.run", target: "DeepLib.tag"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (using-for generic depth-5) mismatch:\n got=%v\nwant=%v", got, want)
	}
}
