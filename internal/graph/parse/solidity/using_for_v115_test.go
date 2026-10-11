package solidity_test

import (
	"testing"
)

// Single-return local-var receiver tests.
//
// Shape: `Type x = expr; x.method(...)` where x is a function-local
// variable_declaration. New localVarTypes index (funcID, varName →
// typeName) plugs into the receiver-type lookup chain after
// stateVarTypes → paramTypes. The struct-
// walker `obj` resolution shares the same three-tier fallback via the
// new `lookupReceiverType` helper.
//
// The this-prefixed walker intentionally bypasses the helper — `this`
// references the caller contract, so the named member must be a state
// variable, never a parameter or local.
//
// Not covered here:
//   - Multi-return tuple destructuring (variable_declaration_tuple).
//   - Local-var with RHS = function-call (use funcReturnTypes for
//     type inference instead of declared LHS — covered indirectly when
//     the LHS type is explicit).
//   - Block-scoped shadowing (locals are treated as function-scoped here).

// TestUsingForV115_LocalVarDirect — canonical local-var case. Local var
// `uint256 x` used as a bare receiver (`x.add(1)`). Tests that
// localVarTypes plugs into resolveUsingForCallRef after stateVar/param
// miss.
func TestUsingForV115_LocalVarDirect(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v115", "local_var_direct.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "Calculator.compute", target: "SafeMath.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (V1.15 direct local-var) mismatch:\n got=%v\nwant=%v", got, want)
	}
}

// TestUsingForV115_LocalVarStruct — local-var receiver feeding the
// struct-field walker. `UserData memory u = ...; u.balance.add(1)`.
// Confirms localVarTypes fallback works for obj resolution in the struct-field
// chain too.
func TestUsingForV115_LocalVarStruct(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v115", "local_var_struct.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "Vault.handle", target: "SafeMath.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (V1.15 + V1.10 struct walker) mismatch:\n got=%v\nwant=%v", got, want)
	}
}

// TestUsingForV115_LocalVarUnknownType — local var whose declared type
// has no using-for binding. Resolver must drop — no false-positive
// EdgeCalls.
func TestUsingForV115_LocalVarUnknownType(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v115", "local_var_unknown_type.sol")
	got := collectUsingForCalls(nodes, edges)
	if len(got) != 0 {
		t.Errorf("unexpected V1.15 EdgeCalls when local-var type has no binding: %v", got)
	}
}
