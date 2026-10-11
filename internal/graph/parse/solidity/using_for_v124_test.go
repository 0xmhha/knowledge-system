package solidity_test

import (
	"testing"
)

// fallback_receive_definition graph node + meta walker.
//
// Solidity's `fallback() { ... }` and `receive() external payable { ... }`
// are both parsed by tree-sitter-solidity v1.2.13 as the same node kind
// `fallback_receive_definition`. The grammar has no field that
// disambiguates them — the parser reads the source-text leading token to
// pick the synthetic name ("fallback" or "receive").
//
// Previously there was no graph node for either, so any using-for
// receiver in their body dropped. The fix adds:
//   1. queryFallbackReceive — matches both forms.
//   2. runFallbackReceiveDecl — synthesises name from source text,
//      emits NodeFunction with SubKind="fallback" or "receive", qname
//      "<Container>.fallback" / "<Container>.receive".
//   3. emitParameterMetaPending + emitLocalVarMetaPending called per
//      definition (fallback can carry params in 0.6+; receive cannot
//      by language rule, so its parameter walk is naturally empty).
//   4. nearestFunctionQnameAndStart recognises fallback_receive_definition.
//
// Not covered here:
//   - Free function parameters (file-level function_definition in
//     0.7.4+; runFunctionDecl may already cover — to verify).
//   - Block-scoped shadowing precision.
//   - Module/import handling.

// TestUsingForV124_ReceiveLocal — receive() body local var as receiver.
// receive() takes no parameters by language rule, so the test focuses
// on local-var capture.
func TestUsingForV124_ReceiveLocal(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v124", "receive_local.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "C.receive", target: "SafeMath.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (V1.24 receive local) mismatch:\n got=%v\nwant=%v", got, want)
	}
}

// TestUsingForV124_FallbackLocal — fallback() body local var as
// receiver. Confirms the parser synthesises the right name for the fallback
// half of the union.
func TestUsingForV124_FallbackLocal(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v124", "fallback_param.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "C.fallback", target: "SafeMath.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (V1.24 fallback local) mismatch:\n got=%v\nwant=%v", got, want)
	}
}
