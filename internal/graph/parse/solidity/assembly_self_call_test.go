package solidity_test

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Yul self-call marker symmetry audit. The Yul delegatecall walker marked
// `delegatecall(gas(), address(), …)` as HasSelfDelegatecallDead
// but left the sibling shapes `call(gas(), address(), …)` and
// `staticcall(gas(), address(), …)` unmarked — a left/right
// asymmetry inside yul_low_level_calls.go itself. The fix mirrors the
// Sol self-cast convention: a Yul self-call (excluding delegatecall, which
// keeps its dead-weight marker) sets HasSelfReentrantCall on the
// enclosing callable.
//
// The asymmetry was real and noticeable in practice — a contract
// using inline assembly to issue a self-`call` had the YulBuiltins
// list populated and HasAssembly=true, but neither the self-cast
// walker (no Sol-level call expression) nor the Yul delegatecall walker (delegatecall-
// only) flagged the reentrancy surface.
//
// Two fixtures lock the post-fix behaviour:
//
//   - AsmSelf.fire         : call(gas(), address(), 0, 0, 0, 0, 0)
//     → HasSelfReentrantCall=true.
//   - AsmSelfDelegate.fire : delegatecall(gas(), address(), 0, 0, 0, 0)
//     → HasSelfDelegatecallDead=true (unchanged) and
//     HasSelfReentrantCall stays false because delegatecall has
//     its dedicated marker rather than the reentrant one.
//
// The cross-axis assertion (delegatecall NOT setting
// HasSelfReentrantCall) keeps the delegatecall / self-call separation explicit —
// a future refactor that merged the two markers would silently
// change the cks-side signal mix.
func TestAssembly_SelfCallMarker(t *testing.T) {
	nodes, _ := parseResolveOneSol(t, "testdata/yul_receiver",
		"sol_assembly_self_call.sol")

	type expect struct {
		reentrant    bool
		delegateDead bool
	}
	want := map[string]expect{
		"AsmSelf.fire":         {reentrant: true, delegateDead: false},
		"AsmSelfDelegate.fire": {reentrant: false, delegateDead: true},
	}
	got := map[string]expect{}
	for _, n := range nodes {
		if n.Type != types.NodeFunction {
			continue
		}
		if _, ok := want[n.QualifiedName]; !ok {
			continue
		}
		got[n.QualifiedName] = expect{
			reentrant:    n.HasSelfReentrantCall,
			delegateDead: n.HasSelfDelegatecallDead,
		}
		// HasAssembly should still fire for both (assembly_marker
		// runs independently). Pin it so a regression that
		// disabled the assembly walker would surface here too.
		if !n.HasAssembly {
			t.Errorf("%s HasAssembly: got false, want true", n.QualifiedName)
		}
	}
	for qn, w := range want {
		g, present := got[qn]
		if !present {
			t.Errorf("missing function %q", qn)
			continue
		}
		if g.reentrant != w.reentrant {
			t.Errorf("%s HasSelfReentrantCall: got %v, want %v",
				qn, g.reentrant, w.reentrant)
		}
		if g.delegateDead != w.delegateDead {
			t.Errorf("%s HasSelfDelegatecallDead: got %v, want %v",
				qn, g.delegateDead, w.delegateDead)
		}
	}
}
