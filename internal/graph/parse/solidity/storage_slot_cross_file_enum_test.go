package solidity_test

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// A state variable typed as an enum of another file (`Ext.Status` after
// `import "./status_lib.sol" as Ext`) packs like the enum it is: one byte for
// up to 256 values.
//
// This test used to lock a known defect: enum sizes were kept per file, so a
// cross-file enum fell back to a full slot (status1 = 1, tail = 2). Sizes are
// now read from the nodes of every file (typeSizesFromNodes), so `head` and
// `status1` share slot 0 and `tail` is in slot 1.
func TestStorageSlot_CrossFileEnum(t *testing.T) {
	nodes, _ := parseResolveMultiSol(t,
		"testdata/storage_slot_cross_file_enum",
		[]string{"status_lib.sol", "holder.sol"})

	want := map[string]int{
		"Holder.head":    0,
		"Holder.status1": 0, // one byte, shares the slot of head
		"Holder.tail":    1, // uint256 starts the next slot
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
