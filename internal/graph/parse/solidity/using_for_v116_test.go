package solidity_test

import (
	"testing"
)

// Multi-return tuple destructuring tests.
//
// Shape: `(Ta a, Tb b) = foo(); a.method(...)` — variable_declaration_
// statement wraps a variable_declaration_tuple whose children are mixed
// `variable_declaration` (typed slot — covered here) and `identifier`
// (pre-declared slot — not covered, dropped here without RHS multi-
// slot fnReturnTypes inference).
//
// This reuses the localVarTypes infrastructure: each typed tuple
// slot emits the same (varName, typeName) PendingRef as a single-var
// declaration, so the existing receiver resolvers automatically pick
// it up via the existing lookupReceiverType chain.
//
// Not covered here:
//   - Pre-declared identifier slots: need multi-slot funcReturnTypes
//     (extend funcReturnTypes to (funcID, slotIndex) → typeName).
//   - Wildcard slots (`(uint256 a, ) = pair()` — the empty slot is
//     skipped here without ceremony; no PendingRef emitted, no impact).

// TestUsingForV116_TupleBasic — canonical tuple case. Tuple LHS with
// two typed slots; first slot used as a bare receiver
// (`a.add(1)`). Confirms the parser fans out PendingRefs through
// variable_declaration_tuple's variable_declaration children.
func TestUsingForV116_TupleBasic(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v116", "tuple_basic.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "Vault.run", target: "SafeMath.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (V1.16 tuple basic) mismatch:\n got=%v\nwant=%v", got, want)
	}
}

// TestUsingForV116_TupleStructSlot — tuple slot is a struct, feeds
// into the struct-field walker. Confirms that the localVarTypes
// emitted from tuple slots interoperate with the struct-field obj resolution
// chain via lookupReceiverType.
func TestUsingForV116_TupleStructSlot(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v116", "tuple_struct_slot.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "Handler.process", target: "SafeMath.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (V1.16 + V1.10 struct slot) mismatch:\n got=%v\nwant=%v", got, want)
	}
}

// TestUsingForV116_TupleUnboundSlot — typed tuple slot whose type has
// no using-for binding (bytes32 here, with only `using SafeMath for
// uint256`). Negative guard: only the bound slot's receiver emits an
// EdgeCalls; the unbound slot's receiver would drop in resolution.
func TestUsingForV116_TupleUnboundSlot(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v116", "tuple_unbound_slot.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "Caller.run", target: "SafeMath.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (V1.16 unbound-slot guard) mismatch:\n got=%v\nwant=%v", got, want)
	}
}
