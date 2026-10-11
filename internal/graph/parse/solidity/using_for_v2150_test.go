package solidity_test

import (
	"testing"
)

// Same-line shadow disambiguation via byte-offset
// scope containment.
//
// Line-range scope-aware localVar lookup fixes the
// first-decl-wins trade-off for multi-line shadowing, but leaves
// ">1 stmt per line edge cases" open — when an outer decl, an
// inner-block shadow decl, and use sites for both all sit on the
// same physical line, a line-only filter accepts both decls for
// either use site, and the tiebreak (`declLine > bestDeclLine`, strict
// `>`) keeps the first-appended (outer). Inner use sites lose to the
// outer scope — a return of the first-decl-wins false-negative, scoped to
// same-line code.
//
// The fix closes the gap by tracking byte-offset scope ranges
// (declStartByte, scopeEndByte) alongside line ranges, threading
// useSiteByte through the lookup chain, and tie-breaking on max
// declStartByte (rightmost = innermost source position).
//
// Fixture: shadow_inner_use.sol with the function body
// compressed to a single line.

// TestUsingForV2150_SameLineShadow — outer `uint256 x` + inner-block
// `bytes32 x` + both use sites all on one line. Asserts both edges
// resolve correctly (inner → Other.tag, outer → SafeMath.add). With
// line-only scoping the inner use drops because the outer decl
// wins the tiebreak.
func TestUsingForV2150_SameLineShadow(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v2150", "shadow_same_line.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "C.f", target: "Other.tag"},
		{caller: "C.f", target: "SafeMath.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (V2.15 same-line shadow) mismatch:\n got=%v\nwant=%v", got, want)
	}
}
