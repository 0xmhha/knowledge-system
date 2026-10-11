package solidity_test

import (
	"testing"
)

// Library body using-for regression guard.
//
// The queryUsingFor pattern matches library_declaration bodies
// uniformly with contract_declaration / interface_declaration bodies
// (verified by reading queries.go). Pass 1.5 containerIDByFuncID maps
// every NodeFunction inside a library to that library's ID. bindings
// keyed by library ID get populated by the binding sweep. lookupReceiverType
// + resolveBindingLib pipeline therefore should work uniformly inside
// library bodies as well.
//
// This test locks this expectation in with a fixture so future refactors
// don't accidentally exclude library-body using-for from the
// dispatch pipeline.
//
// Not covered here:
//   - Byte-range precision over the line-based fallback.
//   - Module/import additional patterns.
//   - Grammar-blocked items.

// TestUsingForV230_LibraryBodyUsing — library declares its own using-
// for binding and dispatches via that binding in its own function.
// WrapperLib.compute → InnerLib.add via `using InnerLib for uint256`
// inside WrapperLib.
func TestUsingForV230_LibraryBodyUsing(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v230", "library_body_using.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "WrapperLib.compute", target: "InnerLib.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (V2.3 library body using-for) mismatch:\n got=%v\nwant=%v", got, want)
	}
}
