package solidity_test

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Using-alias free-function form regression guard.
//
// Empirical finding: the base queryUsingFor incidentally captures the
// `Math` identifier from `using {Math.add, Math.sub} for uint256;`
// (contract-scoped free-function form). Tree-sitter v1.2.13's
// parse is clean enough that the query's
// `(using_directive (type_alias (identifier) @lib) ...)` shape
// matches against the grammar's actual structure for this case.
//
// This invalidates part of the earlier grammar-blocked assumption: the
// claim "`{Math.add, Math.sub}` brace shape parses to ERROR" is
// false for v1.2.13. The original 0-edges result for `using {mul as *}
// for uint256 global;` was due to file-level scope being excluded
// by queryUsingFor (only matches contract / library / interface
// bodies), not because using_alias handling is broken.
//
// This test locks the rediscovered behavior:
//   (a) Contract-scoped `using {Lib.f1, Lib.f2} for T;` emits 1
//       EdgeUsesFor (Caller → Lib) — the query captures Lib.
//   (b) The using-alias binding successfully feeds method-call dispatch:
//       receivers of type T resolve through Lib for method-call
//       dispatch.
//
// Not covered here:
//   - File-level using directives (still excluded from queryUsingFor).
//   - Operator-form `using {f as +}` at contract scope (see
//     using_for_v270_test.go).
//   - Byte-range precision / additional module-import patterns /
//     grammar-blocked items.

// TestUsingForV260_FreeFunctionFormBinding — `using {Math.add,
// Math.sub} for uint256;` at contract scope. The query incidentally
// matches `Math` as @lib token. Verifies Caller → Math EdgeUsesFor.
func TestUsingForV260_FreeFunctionFormBinding(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v260", "probe_using_alias.sol")
	qnameByID := map[string]string{}
	for _, n := range nodes {
		qnameByID[n.ID] = n.QualifiedName
	}
	seen := false
	for _, e := range edges {
		if e.Type == types.EdgeUsesFor &&
			qnameByID[e.Src] == "Calc" && qnameByID[e.Dst] == "Math" {
			seen = true
		}
	}
	if !seen {
		t.Errorf("missing EdgeUsesFor Calc → Math (V2.6 free-function form binding)")
	}
}

// TestUsingForV260_FreeFunctionFormDispatch — same form + actual
// receiver method-call dispatch. Verifies the method-call dispatch chain works
// through the using-alias-emitted binding.
func TestUsingForV260_FreeFunctionFormDispatch(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v260", "free_function_dispatch.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "Vault.run", target: "Math.add"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (V2.6 using-alias + V1.0 dispatch) mismatch:\n got=%v\nwant=%v", got, want)
	}
}
