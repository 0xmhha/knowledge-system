package solidity_test

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Receive/fallback self-cast value-transfer audit.
// The cast walker sets HasSelfReentrantCall for value-
// transfer methods (send / transfer); this test locks that the same
// signal reaches receive() and fallback() functions (which
// are emitted as NodeFunction with SubKind="receive" /
// "fallback"). A regression that excludes
// fallback_receive_definition from nearestFunctionQnameAndStart
// — or that splits receive/fallback into a non-function node
// type — would drop the reentrancy-loop signal and fail this
// lock.
func TestReceiveFallback_SelfReentrantValueTransfer(t *testing.T) {
	nodes, _ := parseResolveOneSol(t, "testdata/yul_receiver",
		"sol_receive_fallback_self_reentrant.sol")

	want := map[string]bool{
		"LoopHolder.receive":  true,
		"LoopHolder.fallback": true,
	}
	got := map[string]bool{}
	for _, n := range nodes {
		if n.Type != types.NodeFunction {
			continue
		}
		if _, ok := want[n.QualifiedName]; ok {
			got[n.QualifiedName] = n.HasSelfReentrantCall
			// Self-cast value-transfer must NOT also light up the
			// arbitrary-address HasExternalCall path (the walker routes
			// self-casts to selfAffected, not affected).
			if n.HasExternalCall {
				t.Errorf("%s HasExternalCall: got true (V14 expects false; self-cast routes to reentrant)",
					n.QualifiedName)
			}
		}
	}
	for qn, w := range want {
		g, present := got[qn]
		if !present {
			t.Errorf("missing NodeFunction %q", qn)
			continue
		}
		if g != w {
			t.Errorf("%s HasSelfReentrantCall: got %v, want %v", qn, g, w)
		}
	}
}
