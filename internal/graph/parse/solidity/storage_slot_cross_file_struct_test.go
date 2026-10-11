package solidity_test

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Cross-file struct size propagation audit.
// Mirrors the cross-file enum lock for the struct case. A
// per-file structSizes index routes same-file
// struct-typed state vars through advanceForArrayField with the
// computed sum-of-field-bytes. Cross-file struct references
// (`Ext.Pair` after `import "./types_lib.sol" as Ext`) stay
// outside that map and fall through to solTypeSize's
// conservative 32-byte fallback (advanceForField with size=32,
// which forces a fresh slot when the current one is partially
// used).
//
// This test locks the current conservative behaviour. Promoting to
// actual cross-file size propagation needs a globalStructSizes
// index aggregated across ParseResults and a Pass-2 post-pass
// that updates SlotIndex retroactively — same scope as the
// enum-side carry-over.
//
// A future regression that loosens the per-file lookup (e.g. by
// stripping namespace prefixes blindly so `Ext.Pair` accidentally
// hits the local `Pair` entry) would silently rewrite slot
// indices and fail this lock.
func TestStorageSlot_CrossFileStructConservative(t *testing.T) {
	nodes, _ := parseResolveMultiSol(t,
		"testdata/storage_slot_cross_file_struct",
		[]string{"types_lib.sol", "holder.sol"})

	want := map[string]int{
		"Holder.head": 0,
		"Holder.pair": 1, // locked: cross-file struct → full slot
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
			t.Errorf("NodeField %q SlotIndex: got %d, want %d (V16 conservative lock)",
				qn, g, w)
		}
	}
}
