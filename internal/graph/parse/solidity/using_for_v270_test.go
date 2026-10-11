package solidity_test

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Contract-scope operator-form using directive probe.
//
// The original file-level operator-form behavior:
//   `using {mul as *} for uint256 global;`  → 0 EdgeUsesFor
//   Reason: queryUsingFor only matches inside contract/library/
//   interface bodies; file-level scope is intentionally excluded.
//
// using_for_v260_test.go shows that contract-scope free-function form is
// incidentally captured:
//   `contract C { using {Math.add, Math.sub} for uint256; }`
//                                                → 1 EdgeUsesFor
//   Reason: tree-sitter v1.2.13 wraps the alias-list entry's
//   qualifier identifier inside a `type_alias` node, and the
//   query `(type_alias (identifier) @lib)` matches it as @lib.
//
// This test mirrors that one with the operator-form variant — same scope
// (contract body), different alias structure (`as +` adds a
// `user_definable_operator` child). The empirical question: does
// the operator suffix break the type_alias wrapping, or does the
// query still incidentally match?
//
// The test below executes the parser+resolver pipeline and locks
// whichever behavior emerges so future grammar bumps / query
// edits don't silently flip the result.

// TestUsingForV270_ContractScopeOperatorForm — `contract Calc {
// using {Math.add as +} for uint256; }`. Locks empirical
// behavior. Whether 0 or 1 edges, the test pins the count and
// (if any) the contract → library pair.
//
// Query-only behaviour (tree-sitter-solidity v1.2.13):
//   - 0 EdgeUsesFor — operator-form's `user_definable_operator`
//     child changes the alias-entry AST shape enough that
//     `(type_alias (identifier) @lib)` no longer matches. The
//     operator suffix is the discriminator vs. the free-function form.
//
// Contrast table (query only, before recovery walkers):
//
//	file-level     + operator-form     → 0 edges (scope)
//	contract-scope + free-function     → 1 edge  (incidental)
//	contract-scope + operator-form     → 0 edges (AST shape)
//
// Surround-safety: the function `Math.add`, library `Math`, and
// contract `Calc` (with `Calc.compute`) must all still index;
// the using directive's parse shape shouldn't cascade.
func TestUsingForV270_ContractScopeOperatorForm(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v270", "probe_operator_form.sol")

	// (a) Lock: 1 EdgeUsesFor for
	// contract-scope operator-form. A recovery walker
	// that pattern-matches the misparsed `state_variable_declaration`
	// shape and emits the binding pair runUsingFor produces.
	edgeCount := 0
	for _, e := range edges {
		if e.Type == types.EdgeUsesFor {
			edgeCount++
		}
	}
	if edgeCount != 1 {
		t.Errorf("expected 1 EdgeUsesFor for V2.7 operator-form (contract scope, post-V2.20 recovery), got %d", edgeCount)
		for _, e := range edges {
			if e.Type == types.EdgeUsesFor {
				t.Logf("  edge: %+v", e)
			}
		}
	}

	// (b) Surround-safety: declarations still indexed.
	seenLib := false
	seenAdd := false
	seenCalc := false
	seenCompute := false
	for _, n := range nodes {
		switch n.QualifiedName {
		case "Math":
			seenLib = true
		case "Math.add":
			seenAdd = true
		case "Calc":
			seenCalc = true
		case "Calc.compute":
			seenCompute = true
		}
	}
	if !seenLib {
		t.Errorf("library `Math` not indexed (V2.7 surround-safety)")
	}
	if !seenAdd {
		t.Errorf("function `Math.add` not indexed (V2.7 surround-safety)")
	}
	if !seenCalc {
		t.Errorf("contract `Calc` not indexed (V2.7 surround-safety)")
	}
	if !seenCompute {
		t.Errorf("function `Calc.compute` not indexed (V2.7 surround-safety)")
	}
}
