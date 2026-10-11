package solidity_test

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Diamond inheritance + multi-parent using-for
// binding union.
//
// BFS-based inheritance propagation for binding
// maps so a child contract picks up its parents' `using` directives
// transparently. Bindings are also multi-value to
// support `contract Foo { using A for T; using B for T; }` within
// a single contract.
//
// This test probes the intersection: when a child inherits from two
// parents A and B that BOTH bind the same type T to different
// libraries, do both bindings survive in the child's binding map?
//
// Earlier code: the BFS "don't clobber" branch in resolve.go
// wrote `bindings[childID][typeName] = cloned` only when
// the slot didn't already exist. After the first ancestor's
// binding set the slot, subsequent ancestors' bindings for the
// same type were dropped — child-shadows-inherited semantics
// applied incorrectly to peer-ancestors at the SAME inheritance
// depth.
//
// Expected dispatch surface:
//   - Child.callAdd → SafeMath.add (inherited from A)
//   - Child.callMul → OtherMath.mul (inherited from B)
//
// Both should fire because the child's receiver `x` is uint256
// and both libraries contribute methods on uint256.

// TestUsingForV2130_DiamondInheritedBinding — multi-parent
// inherited binding union check.
func TestUsingForV2130_DiamondInheritedBinding(t *testing.T) {
	nodes, edges := parseResolveOneSol(t, "testdata/using_for_v2130", "diamond_using.sol")

	qnameByID := map[string]string{}
	for _, n := range nodes {
		qnameByID[n.ID] = n.QualifiedName
	}

	// (a) Direct EdgeUsesFor on each parent.
	wantUF := map[string]bool{
		"A->SafeMath":  false,
		"B->OtherMath": false,
	}
	for _, e := range edges {
		if e.Type != types.EdgeUsesFor {
			continue
		}
		key := qnameByID[e.Src] + "->" + qnameByID[e.Dst]
		if _, ok := wantUF[key]; ok {
			wantUF[key] = true
		}
	}
	for k, hit := range wantUF {
		if !hit {
			t.Errorf("missing EdgeUsesFor %s (V2.13 parent direct binding)", k)
		}
	}

	// (b) Inherited dispatch: child's receiver method calls must
	// resolve through BOTH parents' bindings, not just one.
	gotAdd := false
	gotMul := false
	for _, e := range edges {
		if e.Type != types.EdgeCalls {
			continue
		}
		src := qnameByID[e.Src]
		dst := qnameByID[e.Dst]
		if src == "Child.callAdd" && dst == "SafeMath.add" {
			gotAdd = true
		}
		if src == "Child.callMul" && dst == "OtherMath.mul" {
			gotMul = true
		}
	}
	if !gotAdd {
		t.Errorf("missing EdgeCalls Child.callAdd → SafeMath.add (V2.13 inherited from A)")
	}
	if !gotMul {
		t.Errorf("missing EdgeCalls Child.callMul → OtherMath.mul (V2.13 inherited from B — multi-parent union)")
	}
}
