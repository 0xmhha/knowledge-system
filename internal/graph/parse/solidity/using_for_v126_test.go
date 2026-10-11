package solidity_test

import (
	"testing"
)

// Abstract contract body scope regression guard.
//
// Sol's `abstract contract Base { ... }` shares the
// contract_declaration AST kind with regular contracts; the `abstract`
// keyword is captured as a SubKind on NodeContract
// (abstract_library.go). All using-for indexing
// (binding map, paramTypes, localVarTypes, containerIDByFuncID) keys
// off contract_declaration name, so abstract contracts should be
// indistinguishable from regular contracts for dispatch purposes.
//
// This test locks this in with a minimal fixture so future refactors
// don't accidentally exclude abstract contracts.
//
// Not covered here:
//   - Inherited modifier `using` baseline regression guard.
//   - Block-scoped shadowing precision.
//   - Module/import handling.

// TestUsingForV126_AbstractContract — abstract contract with using-for
// binding and method-call dispatch in body. Asserts the abstract
// keyword doesn't disrupt the state-var, local-var, or shadowing paths.
func TestUsingForV126_AbstractContract(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v126", "abstract_using.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "Base.computeBase", target: "SafeMath.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (V1.26 abstract contract) mismatch:\n got=%v\nwant=%v", got, want)
	}
}
