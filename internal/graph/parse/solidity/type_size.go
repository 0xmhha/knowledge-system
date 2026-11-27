package solidity

import (
	"strconv"
	"strings"
)

// Storage byte sizes of the elementary Solidity types, for slot packing.
//
//	bool                             → 1
//	address, address payable         → 20
//	uintN  (N ∈ 8..256, N % 8 == 0)  → N/8
//	intN   (N ∈ 8..256, N % 8 == 0)  → N/8
//	uint, int                        → 32 (aliases for uint256 / int256)
//	bytesN (N ∈ 1..32)               → N
//
// Every other type (dynamic bytes, string, arrays, structs, mappings,
// enums, contracts) is sized by typeSizes.place, which knows the
// user-defined types of the graph.

// solValueTypeSize returns the storage size in bytes of an elementary value
// type, and whether typeName is one.
func solValueTypeSize(sig string) (int, bool) {
	sig = strings.TrimSpace(sig)
	switch sig {
	case "bool":
		return 1, true
	case "address", "address payable":
		return 20, true
	case "uint", "int":
		return 32, true
	}
	if strings.HasPrefix(sig, "uint") {
		if rest := sig[len("uint"):]; rest != "" {
			n, err := strconv.Atoi(rest)
			if err == nil && n >= 8 && n <= 256 && n%8 == 0 {
				return n / 8, true
			}
		}
	}
	if strings.HasPrefix(sig, "int") {
		if rest := sig[len("int"):]; rest != "" {
			n, err := strconv.Atoi(rest)
			if err == nil && n >= 8 && n <= 256 && n%8 == 0 {
				return n / 8, true
			}
		}
	}
	if strings.HasPrefix(sig, "bytes") {
		if rest := sig[len("bytes"):]; rest != "" {
			n, err := strconv.Atoi(rest)
			if err == nil && n >= 1 && n <= 32 {
				return n, true
			}
		}
	}
	return 0, false
}

// solFixedArrayBytes parses a Sol fixed-size value-type array
// signature (`T[N]`, `T[N][M]`, …) and returns the total byte
// footprint = element_size * N * M * … . Returns ok=false for
// dynamic arrays (`T[]`), non-array signatures, and arrays whose
// element type isn't in the recognised value-type set (callers fall
// back to the conservative full-slot path).
//
// Sol §11.1: fixed-size value-type arrays pack their elements
// tightly into consecutive storage slots. Compound shapes nest:
// `uint8[4][2]` is two arrays of `uint8[4]`, so the total is
// 2 × 4 × 1 = 8 bytes. Elements span slot boundaries — a 33-byte
// array (`uint8[33]`) consumes two slots with one byte spilling
// into the second.
func solFixedArrayBytes(sig string) (int, bool) {
	sig = strings.TrimSpace(sig)
	if !strings.HasSuffix(sig, "]") {
		return 0, false
	}
	openIdx := strings.LastIndex(sig, "[")
	if openIdx < 0 {
		return 0, false
	}
	closeIdx := len(sig) - 1
	countStr := strings.TrimSpace(sig[openIdx+1 : closeIdx])
	if countStr == "" {
		return 0, false
	}
	n, err := strconv.Atoi(countStr)
	if err != nil || n <= 0 {
		return 0, false
	}
	inner := strings.TrimSpace(sig[:openIdx])
	var elementSize int
	if strings.HasSuffix(inner, "]") {
		innerBytes, ok := solFixedArrayBytes(inner)
		if !ok {
			return 0, false
		}
		elementSize = innerBytes
	} else {
		size, ok := solValueTypeSize(inner)
		if !ok {
			return 0, false
		}
		elementSize = size
	}
	return elementSize * n, true
}

// slotState carries the per-contract packing counter used by
// runStateVarDecl. `slot` is the current 0-indexed slot; `used` is
// the byte offset within the slot already consumed (0..32).
type slotState struct {
	slot int
	used int
}

// advanceForField returns the slot index this field occupies and
// the updated state.
//
// Packing rules (per Sol §11.1):
//
//   - A size >= 32 field always starts on a slot boundary and fills
//     it entirely (advance state.slot afterwards, reset used).
//
//   - A smaller field shares the current slot if there's room
//     (state.used + size <= 32); otherwise it advances to a fresh
//     slot. After placement, if the slot is now exactly full, the
//     next field starts on a new slot.
func advanceForField(state slotState, size int) (int, slotState) {
	// >= 32-byte types always sit on a slot boundary.
	if size >= 32 {
		if state.used > 0 {
			state.slot++
			state.used = 0
		}
		slot := state.slot
		state.slot++
		state.used = 0
		return slot, state
	}
	if state.used+size > 32 {
		state.slot++
		state.used = 0
	}
	slot := state.slot
	state.used += size
	if state.used >= 32 {
		state.slot++
		state.used = 0
	}
	return slot, state
}

// advanceForMapping consumes a full slot for a mapping state-var
// and returns the slot the mapping occupies. It also produces a
// SlotIndex so NodeMapping
// rows can be located by storage slot the same way NodeField rows
// can. The per-key data still lives at keccak256(key, slot) at
// runtime; this slot is just the declaration slot.
func advanceForMapping(state slotState) (int, slotState) {
	if state.used > 0 {
		state.slot++
		state.used = 0
	}
	slot := state.slot
	state.slot++
	return slot, state
}

// advanceForArrayField places a fixed-size
// value-type array. Sol §11.1 says struct- and array-shaped state
// variables always start a new slot, and the next variable after
// them also starts a new slot — so we pre-align, occupy
// ceil(totalBytes / 32) slots, and post-align by clearing used. The
// returned slot index is the first slot of the array.
func advanceForArrayField(state slotState, totalBytes int) (int, slotState) {
	if state.used > 0 {
		state.slot++
		state.used = 0
	}
	startSlot := state.slot
	slotsNeeded := max((totalBytes+31)/32, 1)
	state.slot += slotsNeeded
	state.used = 0
	return startSlot, state
}

// slotBytes is the size of one storage slot.
const slotBytes = 32

// dynamicArraySuffix ends the type of a dynamic array (`address[]`).
const dynamicArraySuffix = "[]"

// isDynamicSlotType reports whether a type is a dynamic array, a string or
// bytes: a value of unknown length that takes exactly one slot where it is
// declared. Without it a struct with such a member had no known size, and a
// state variable of that struct was laid out as one slot.
func isDynamicSlotType(sig string) bool {
	sig = strings.TrimSpace(sig)
	return sig == "string" || sig == "bytes" || strings.HasSuffix(sig, dynamicArraySuffix)
}
