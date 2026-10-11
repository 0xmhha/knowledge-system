package solidity_test

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// High-level walker × callable shape cross-axis lock.
//
// WALKER_SYMMETRY.md (internal/parse/solidity/) marked three cells
// '?': the high-level self-call walker was only locked on
// function bodies (plain and try-wrapped). Constructor, fallback/receive, and
// modifier scopes were assumed to propagate because the walker
// shares nearestFunctionQnameAndStart with the cast walker — but
// 'assumed to propagate' is not 'tested', and the call-options and Yul self-call fixes already
// proved that what's assumed and what's tested can drift.
//
// This test closes all three cells in one fixture so each callable shape
// the cast walker recognises (fallback/receive, constructor,
// modifier) is independently asserted on the high-level marker
// side.
//
// Expected outcome (0-line walker change required):
//
//   - CtorHighSelf.constructor    : NodeFunction (SubKind=constructor)
//     HasHighLevelSelfCall = true.
//   - FallbackHighSelf.receive    : NodeFunction (SubKind=receive)
//     HasHighLevelSelfCall = true.
//   - ModifierHighSelf.guard      : NodeModifier
//     HasHighLevelSelfCall = true.
//
// Cross-axis assertion: HasSelfReentrantCall stays false on all
// three — the high-level marker is its own axis and the bodies contain no low-level
// .call/.delegatecall/.transfer/.send.
//
// What this catches that the function-body tests alone don't:
//
//	A refactor that introduces a scope window on the high-level
//	walker (e.g. 'only walk function_definition bodies') would
//	pass the function-body and try-wrap tests and silently drop
//	the marker on constructors, receives, and modifiers — exactly
//	the shapes the self-cast audits already proved are reachable
//	through the cast walker's parent-chain helper.
func TestHighLevel_ShapeCrossAxis(t *testing.T) {
	nodes, _ := parseResolveOneSol(t, "testdata/yul_receiver",
		"sol_high_level_shape_cross.sol")

	type spec struct {
		kind    types.NodeType
		subKind string // empty = don't require a specific subkind
	}
	want := map[string]spec{
		"CtorHighSelf.constructor": {kind: types.NodeFunction, subKind: "constructor"},
		"FallbackHighSelf.receive": {kind: types.NodeFunction, subKind: "receive"},
		"ModifierHighSelf.guard":   {kind: types.NodeModifier},
	}
	got := map[string]bool{}
	for _, n := range nodes {
		s, ok := want[n.QualifiedName]
		if !ok {
			continue
		}
		if n.Type != s.kind {
			continue
		}
		if s.subKind != "" && n.SubKind != s.subKind {
			continue
		}
		got[n.QualifiedName] = n.HasHighLevelSelfCall
		// Low-level axis must stay quiet — the bodies contain no
		// .call / .delegatecall / .transfer / .send, so the self-cast walker
		// should not fire either.
		if n.HasSelfReentrantCall {
			t.Errorf("%s HasSelfReentrantCall: got true (V24 expects false; high-level marker only)",
				n.QualifiedName)
		}
	}
	for qn := range want {
		g, present := got[qn]
		if !present {
			t.Errorf("missing %q (with expected node kind / subkind)", qn)
			continue
		}
		if !g {
			t.Errorf("%s HasHighLevelSelfCall: got false, want true", qn)
		}
	}
}
