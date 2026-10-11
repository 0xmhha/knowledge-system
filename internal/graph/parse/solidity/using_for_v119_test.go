package solidity_test

import (
	"testing"
)

// Named return parameters → paramTypes.
//
// Solidity allows function return declarations to carry names:
// `function f() returns (uint256 result)`. `result` is then a function-
// scope variable initialised to zero, assignable and readable inside
// the function body. The parser used to index (paramName, type)
// only for direct parameter children of function_definition — return_
// type's parameter children were captured for their type
// (funcReturnTypes) but their names were never paired into paramTypes.
//
// The fix extends the return_type walker to also
// emit dispatchKindUsingForParamType PendingRefs whenever a return
// parameter has a name field. Resolution path: lookupReceiverType
// (ordered: local-var → param → state-var) picks up the named
// return param via paramTypes.
//
// Not covered here:
//   - Block-scoped shadowing precision.
//   - Grammar-blocked items (free-function form, file-level using).
//   - Module/import handling.

// TestUsingForV119_NamedReturnParam — canonical named-return case. Function
// `f() returns (uint256 result)`. Inside f(), `result.add(1)` must
// resolve via paramTypes → uint256 → SafeMath.add.
// Without the fix this is a false negative.
func TestUsingForV119_NamedReturnParam(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v119", "named_return.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "C.f", target: "SafeMath.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (V1.19 named return param) mismatch:\n got=%v\nwant=%v", got, want)
	}
}

// TestUsingForV119_NamedReturnMulti — multiple named return parameters.
// Function `f() returns (uint256 a, uint256 b)`. Inside f(), `a.add(b)`
// uses both — the parser must emit a paramType PendingRef for each named
// slot, not just the first.
func TestUsingForV119_NamedReturnMulti(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v119", "named_return_multi.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "C.f", target: "SafeMath.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (V1.19 multi named return param) mismatch:\n got=%v\nwant=%v", got, want)
	}
}

// TestUsingForV119_AnonymousReturn — baseline guard: a return slot
// without a name field emits nothing through this path (anonymous slot
// has no addressable identifier). funcReturnTypes still fires
// for chain-call dispatch separately. Confirms over-reach 0.
func TestUsingForV119_AnonymousReturn(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v119", "anonymous_return.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "C.f", target: "SafeMath.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (V1.19 anonymous return baseline) mismatch:\n got=%v\nwant=%v", got, want)
	}
}
