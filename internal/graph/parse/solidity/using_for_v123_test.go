package solidity_test

import (
	"testing"
)

// constructor_definition graph node + meta walker.
//
// Previously there was no graph node for constructor_definition at all
// (queryConstructor missing). Constructors with using-for receivers in
// their body dropped silently — no NodeFunction to anchor the
// PendingRef, and no entry in containerIDByFuncID for the resolver.
//
// The fix adds:
//   1. queryConstructor matching constructor_definition.
//   2. runConstructorDecl creating a NodeFunction with synthetic name
//      "constructor" and qname "<Container>.constructor". Solidity
//      grammar has no `name` field on constructor_definition, so we
//      use the literal "constructor" keyword as the identifier.
//   3. emitParameterMetaPending + emitLocalVarMetaPending called per
//      constructor.
//   4. nearestFunctionQnameAndStart extended to recognise
//      constructor_definition.
//
// Not covered here:
//   - fallback_receive_definition (same AST family).
//   - Free function parameters (file-level function_definition).
//   - Block-scoped shadowing precision.
//   - Module/import handling.

// TestUsingForV123_ConstructorParam — constructor parameter as receiver.
// Previously a false negative: no constructor node, no paramTypes entry.
// The fix surfaces C.constructor → SafeMath.add.
func TestUsingForV123_ConstructorParam(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v123", "constructor_param.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "C.constructor", target: "SafeMath.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (V1.23 constructor param) mismatch:\n got=%v\nwant=%v", got, want)
	}
}

// TestUsingForV123_ConstructorLocal — constructor body local variable
// as receiver. Confirms emitLocalVarMetaPending is wired in for
// constructor_definition's function_body too.
func TestUsingForV123_ConstructorLocal(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v123", "constructor_local.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "C.constructor", target: "SafeMath.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (V1.23 constructor local) mismatch:\n got=%v\nwant=%v", got, want)
	}
}
