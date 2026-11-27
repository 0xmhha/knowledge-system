package persist

import (
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// A state variable in storage slot 0 must be told apart from a node that has
// no slot. The slot index alone cannot do it (0 is omitted), so the stored
// attrs carry `"slot_index":0` when the node has a slot and no slot_index key
// when it has none.
func TestNodeAttrs_SlotZeroIsStored(t *testing.T) {
	cases := []struct {
		name     string
		node     types.Node
		wantBlob string
		wantSlot bool
		slot     int
	}{
		{"slot 0", types.Node{HasStorageSlot: true, SlotIndex: 0}, `"slot_index":0`, true, 0},
		{"slot 5", types.Node{HasStorageSlot: true, SlotIndex: 5}, `"slot_index":5`, true, 5},
		{"no slot", types.Node{}, "", false, 0},
		{"no slot, another marker", types.Node{HasAssembly: true}, `"has_assembly":true`, false, 0},
		{"slot index written without the marker", types.Node{SlotIndex: 3}, `"slot_index":3`, true, 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			blob := marshalNodeAttrs(&tc.node)
			if tc.wantBlob == "" && blob != "" {
				t.Fatalf("blob %q, want empty", blob)
			}
			if !strings.Contains(blob, tc.wantBlob) {
				t.Fatalf("blob %q, want it to contain %q", blob, tc.wantBlob)
			}
			if !tc.wantSlot && strings.Contains(blob, "slot_index") {
				t.Fatalf("blob %q has a slot_index for a node without a slot", blob)
			}
			var got types.Node
			unmarshalNodeAttrs(blob, &got)
			if got.HasStorageSlot != tc.wantSlot || got.SlotIndex != tc.slot {
				t.Errorf("read back has slot %v index %d, want %v %d", got.HasStorageSlot, got.SlotIndex, tc.wantSlot, tc.slot)
			}
		})
	}
}
