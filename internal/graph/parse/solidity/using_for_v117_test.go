package solidity_test

import (
	"testing"
)

// Solidity shadowing precedence fix.
//
// Per Solidity scoping rules, function-scope (locals + parameters)
// shadows contract-scope (state variables). The inner scope wins on
// identifier resolution. `lookupReceiverType` used to walk
// state-var → param → local-var, which is the REVERSE of Solidity
// semantics — when a local shadows a state-var with the same name,
// the old chain would resolve to the state-var type and drop
// (or misroute) the dispatch.
//
// The fix corrects the precedence to local-var → param → state-var.
// The this-prefixed walker (`this.<state-var>.<method>`) is unaffected — `this.` is an
// explicit contract reference that bypasses the function scope.
//
// Not covered here:
//   - Pre-declared identifier-slot tuple (`(a, b) = pair()` with
//     `var` keyword) — practically irrelevant in modern Solidity
//     (`var` deprecated in 0.5.0+).
//   - Cross-file tuple validation (see using_for_v118_test.go).
//   - Block-scoped shadowing precision.
//   - Grammar-blocked items.

// TestUsingForV117_LocalShadowsState — canonical shadowing case. State var
// `x` is a struct (no SafeMath binding); local var `x` in the function
// is `uint256` (bound to SafeMath). The receiver `x.add(1)` must
// resolve to the local (uint256), not the state (UserData). The old
// state-var-first order produces a false negative; the fix produces the
// correct EdgeCalls.
func TestUsingForV117_LocalShadowsState(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v117", "local_shadows_state.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "C.f", target: "SafeMath.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (V1.17 local shadows state) mismatch:\n got=%v\nwant=%v", got, want)
	}
}

// TestUsingForV117_ParamShadowsState — function parameter shadows
// state-variable with the same name. param `uint256 x` must win over
// state `UserData x`. Locks the parameter path's precedence position.
func TestUsingForV117_ParamShadowsState(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v117", "param_shadows_state.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "C.f", target: "SafeMath.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (V1.17 param shadows state) mismatch:\n got=%v\nwant=%v", got, want)
	}
}

// TestUsingForV117_StateOnlyNoShadow — baseline regression guard: with
// no local/param of the same name, state-var resolution still works
// after the precedence flip. Catches accidental over-reach of
// the fix.
func TestUsingForV117_StateOnlyNoShadow(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v117", "state_only_no_shadow.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "C.f", target: "SafeMath.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (V1.17 no-shadow baseline) mismatch:\n got=%v\nwant=%v", got, want)
	}
}
