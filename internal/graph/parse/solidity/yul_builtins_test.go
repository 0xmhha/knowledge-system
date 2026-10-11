package solidity_test

import (
	"reflect"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Yul EVM builtin detection inside assembly blocks.
//
// HasAssembly is only a presence flag on callables. YulBuiltins enumerates
// the security-relevant EVM opcodes that actually appear inside
// the block (delegatecall, sstore, sload, selfdestruct, call,
// staticcall). The walker filters to that critical set — common
// arithmetic / memory ops like add, calldatacopy, returndatacopy
// stay invisible to avoid noise on every assembly user.
//
// Surface contract:
//   Node.YulBuiltins []string  — sorted, deduped, lower-case names.
//                                Empty for callables with no
//                                qualifying assembly content.

func TestYulBuiltins_CriticalOps(t *testing.T) {
	nodes, _ := parseResolveOneSol(t, "testdata/yul_builtins", "critical_ops.sol")

	want := map[string][]string{
		"Proxy.delegate": {"delegatecall"},
		"Proxy.read":     {"sload"},
		"Proxy.write":    {"sstore"},
		"Proxy.kill":     {"selfdestruct"},
		"Proxy.multiOp":  {"call", "selfdestruct", "sload", "sstore"},
		"Proxy.safe":     nil,
		"Proxy.plain":    nil,
	}

	got := map[string][]string{}
	seen := map[string]bool{}
	for _, n := range nodes {
		if n.Type != types.NodeFunction {
			continue
		}
		if _, ok := want[n.QualifiedName]; !ok {
			continue
		}
		seen[n.QualifiedName] = true
		got[n.QualifiedName] = n.YulBuiltins
	}

	for qn, w := range want {
		if !seen[qn] {
			t.Errorf("W10 V1.1 missing function %q", qn)
			continue
		}
		g := got[qn]
		if !reflect.DeepEqual(g, w) {
			t.Errorf("W10 V1.1 %q YulBuiltins: got %v, want %v", qn, g, w)
		}
	}
}
