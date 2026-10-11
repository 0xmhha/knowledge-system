package solidity_test

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Interface receiver via using-for regression guard.
//
// Common DeFi pattern: a contract holds a state variable typed as an
// interface (e.g. IERC20, IToken), with a `using <Lib> for <Interface>`
// directive in scope. Method calls on the interface-typed receiver
// dispatch through the bound library, which takes the interface as
// its first parameter.
//
// The binding map keys typeName as a plain string, so interface
// types should work uniformly with primitive / struct types. byName
// indexing for the library lookup uses NodeContract (libraries are
// indexed as NodeContract with SubKind="library"); the
// interface receiver type doesn't change anything about that lookup.
//
// This test locks in this expected behavior so future refactors don't
// accidentally exclude interface-typed receivers.
//
// Not covered here:
//   - Multi-binding for same type (`using A for uint256; using B for
//     uint256;`) — covered by TestUsingForV210_MultiBindingResolves.
//   - Array / mapping as bound type.
//   - Byte-range precision (line refinement).
//   - Module/import handling additional patterns.

// TestUsingForV210_InterfaceReceiver — state-var typed as interface
// dispatches via library bound to that interface. Expected EdgeCalls:
// Vault.run → HelperLib.balanceOf.
func TestUsingForV210_InterfaceReceiver(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v210", "interface_receiver.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "Vault.run", target: "HelperLib.balanceOf"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (V2.1 interface receiver) mismatch:\n got=%v\nwant=%v", got, want)
	}
}

// TestUsingForV210_MultiBindingResolves — multi-binding resolution.
// `using A for uint256; using B for uint256;` both apply. uint256.tag()
// resolves to LibA.tag (LibA has a tag method, LibB does not —
// resolveBindingLib iterates until it hits).
//
// This was originally locked as a known limitation (single-value
// binding, second-wins overwrite). Bindings are now
// `map[typeName][]libraryName` so all directives' libraries are
// preserved and resolution tries each until method hit.
func TestUsingForV210_MultiBindingResolves(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v210", "multi_binding_same_type.sol")
	got := collectUsingForCalls(nodes, edges)
	want := []callWant{
		{caller: "Vault.run", target: "LibA.tag"},
	}
	if !equalCallWants(got, want) {
		t.Errorf("EdgeCalls (V2.2 multi-binding resolves) mismatch:\n got=%v\nwant=%v", got, want)
	}
	// Defensive: both EdgeUsesFor edges (Vault → LibA, Vault → LibB)
	// should land regardless of method-dispatch resolution.
	qnameByID := map[string]string{}
	for _, n := range nodes {
		qnameByID[n.ID] = n.QualifiedName
	}
	seen := map[string]bool{}
	for _, e := range edges {
		if e.Type == types.EdgeUsesFor && qnameByID[e.Src] == "Vault" {
			seen[qnameByID[e.Dst]] = true
		}
	}
	for _, lib := range []string{"LibA", "LibB"} {
		if !seen[lib] {
			t.Errorf("missing EdgeUsesFor Vault → %s (V2.2 multi-binding)", lib)
		}
	}
}
