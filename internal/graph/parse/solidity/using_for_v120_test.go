package solidity_test

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Additional name-scope captures.
//
// Two gaps the local-var / named-return captures didn't cover:
//
//  1. for-loop init variable: `for (uint256 i = 0; ...)` declares `i`
//     inside the for_statement's init slot. The local-var recursive descent
//     through function body should reach this variable_declaration_
//     statement.
//
//  2. try/catch returns clause: `try foo() returns (uint256 r) { ... }`
//     binds `r` to the success block. Tree-sitter exposes the returns
//     clause as `parameter` children of try_statement — distinct from
//     function_definition's `return_type` field, which the named-return walker covers.
//
// Not covered here:
//   - catch_clause's named parameter (`catch Error(string memory s)`).
//   - Block-scoped shadowing precision.
//   - Module/import handling.
//   - Grammar-blocked items.

// TestUsingForV120_ForLoopInit — for-loop init variable as using-for
// receiver. The local-var recursive descent already reaches variable_declaration
// _statement inside for_statement init slot — this test is a regression
// guard that the descent stays in place after future refactors.
func TestUsingForV120_ForLoopInit(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v120", "for_loop_init.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "C.f", target: "SafeMath.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (V1.20 for-loop init) mismatch:\n got=%v\nwant=%v", got, want)
	}
}

// TestUsingForV120_TryCatchReturns — try/catch returns clause named
// parameter. Previously a false negative: only function_
// definition.return_type was walked. The fix walks try_statement's direct
// parameter children too.
func TestUsingForV120_TryCatchReturns(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v120", "try_catch_returns.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "C.f", target: "SafeMath.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (V1.20 try-catch returns) mismatch:\n got=%v\nwant=%v", got, want)
	}
}

// TestUsingForV120_TryCatchAnonymousReturns — try/catch with anonymous
// returns slot. The emit must skip (no name field → no addressable
// receiver). Over-reach guard.
func TestUsingForV120_TryCatchAnonymousReturns(t *testing.T) {
	_, edges := parseResolveOneSol(t, "testdata/using_for_v120", "try_catch_anonymous_returns.sol")
	for _, e := range edges {
		if e.Type == types.EdgeCalls {
			// The parser shouldn't surface an EdgeCalls for try-returns
			// here — there's no `<receiver>.method()` for using-for to
			// dispatch on (the success block returns a literal).
			t.Errorf("unexpected EdgeCalls in anonymous-returns body: %+v", e)
		}
	}
}
