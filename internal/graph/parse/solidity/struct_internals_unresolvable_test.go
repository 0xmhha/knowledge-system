package solidity_test

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Slots of a state variable whose struct has a reference-type member.
//
// This test used to pin a known defect as a characterization: a struct with
// a mapping, a dynamic `bytes` or a dynamic `T[]` member had no computed
// size, so the state variable of that struct was laid out as one slot and
// the variable after it landed two slots too early (tail = 2 instead of 4).
// Because every struct of the fixture is named `S` and sizes are keyed by
// name, the all-primitive struct fell back with them.
//
// A dynamic array, a string or bytes takes one slot (isDynamicSlotType), so
// each `S` of the fixture is three slots and all four contracts lay out as
// the fixture's comments on PrimitiveOnly say: head = 0, inner = 1 (slots 1
// to 3), tail = 4. The same rule puts GovCouncil's two AddressSet variables
// of the system contracts at slots 0x32 and 0x34, as its source documents.
func TestW9V28_StructInternalsFallback_Characterization(t *testing.T) {
	nodes, _ := parseResolveOneSol(t, "testdata/storage_slot_packing",
		"struct_internals_unresolvable.sol")

	// All four shapes: head at slot 0, inner takes slots 1 to 3, tail at
	// slot 4.
	want := map[string]int{
		"PrimitiveOnly.head":  0,
		"PrimitiveOnly.inner": 1,
		"PrimitiveOnly.tail":  4,
		"WithMap.head":        0,
		"WithMap.inner":       1,
		"WithMap.tail":        4,
		"WithBytes.head":      0,
		"WithBytes.inner":     1,
		"WithBytes.tail":      4,
		"WithDynArray.head":   0,
		"WithDynArray.inner":  1,
		"WithDynArray.tail":   4,
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
			t.Errorf("missing field %q", qn)
			continue
		}
		if g != w {
			t.Errorf("%s SlotIndex: got %d, want %d", qn, g, w)
		}
	}
}
