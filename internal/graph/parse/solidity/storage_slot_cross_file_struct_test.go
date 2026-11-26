package solidity_test

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// A state variable typed as a struct of another file (`Ext.Pair` after
// `import "./types_lib.sol" as Ext`). Struct sizes are read from the member
// nodes of every file (typeSizesFromNodes), so `Pair` is known to be one slot
// (two uint128). A struct always starts a new slot and the variable after it
// starts another, so the layout is head = 0, pair = 1, tail = 2: the same
// numbers the earlier one-slot fallback for cross-file structs gave, which
// this test used to lock.
func TestStorageSlot_CrossFileStructConservative(t *testing.T) {
	nodes, _ := parseResolveMultiSol(t,
		"testdata/storage_slot_cross_file_struct",
		[]string{"types_lib.sol", "holder.sol"})

	want := map[string]int{
		"Holder.head": 0,
		"Holder.pair": 1, // a struct starts a new slot
		"Holder.tail": 2,
	}
	got := map[string]int{}
	for _, n := range nodes {
		if n.Type != types.NodeField {
			continue
		}
		if _, ok := want[n.QualifiedName]; ok {
			got[n.QualifiedName] = n.SlotIndex
		}
	}
	for qn, w := range want {
		g, present := got[qn]
		if !present {
			t.Errorf("missing NodeField %q", qn)
			continue
		}
		if g != w {
			t.Errorf("NodeField %q SlotIndex: got %d, want %d", qn, g, w)
		}
	}
}
