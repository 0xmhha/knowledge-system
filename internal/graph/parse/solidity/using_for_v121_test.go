package solidity_test

import (
	"testing"
)

// catch_clause named parameter capture.
//
// Solidity's `catch Type(Ta a, Tb b) { ... }` binds the catch
// parameters to the catch body. Tree-sitter exposes them as `parameter`
// direct children of `catch_clause` (alongside an optional identifier
// for the catch type, e.g. "Error" or "Panic"). The collect-local-
// var-meta-pending recurses into catch_clause's body for declarations
// but did not surface the catch's own parameter slot — false-negative
// when the parameter is used as a using-for receiver.
//
// The fix handles catch_clause directly in the
// recursive descent: each parameter child emits a localVar PendingRef
// via emitTryReturnsBinding (same idiom as try-returns).
//
// Not covered here:
//   - Block-scoped shadowing precision.
//   - Module/import handling.
//   - Grammar-blocked items.

// TestUsingForV121_CatchNamedParam — canonical catch-param case.
// `catch Panic(uint256 errorCode) { errorCode.add(1) }`. errorCode is
// a uint256 in catch scope; the parser must emit it into localVarTypes so
// the using-for receiver lookup picks it up.
func TestUsingForV121_CatchNamedParam(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v121", "catch_named_param.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "C.f", target: "SafeMath.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (V1.21 catch named param) mismatch:\n got=%v\nwant=%v", got, want)
	}
}

// TestUsingForV121_CatchAnonymous — anonymous / no-param catch
// (`catch { ... }`). The catch emit must skip; over-reach guard. The
// try-returns path still fires for `r.add(1)` in the success
// block, so EdgeCalls is non-empty but contributed entirely by try-returns.
func TestUsingForV121_CatchAnonymous(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v121", "catch_anonymous.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "C.f", target: "SafeMath.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (V1.21 anonymous catch baseline) mismatch:\n got=%v\nwant=%v", got, want)
	}
}

// TestUsingForV121_CatchMulti — multiple catch clauses with mixed
// named / anonymous params and types. The parser must emit per named
// param across every catch_clause sibling. `code` (Panic catch) is
// the only one bound to SafeMath; other catches have no addressable
// receiver bound to a using-for binding.
func TestUsingForV121_CatchMulti(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v121", "catch_multi.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "C.f", target: "SafeMath.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (V1.21 multi catch) mismatch:\n got=%v\nwant=%v", got, want)
	}
}
