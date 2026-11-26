package solidity

// enumByteWidth returns the byte footprint Sol assigns to an enum
// with the given variant count. The mapping follows EVM packing:
//
//	  1..256 variants -> 1 byte  (uint8)
//	257..65536 variants -> 2 bytes (uint16)
//	65537..2^32 variants -> 4 bytes (uint32)
//	larger -> 8 bytes (uint64; Sol's compiler rejects this in
//	         practice but the lookup stays safe for any input)
//
// Zero or negative variant counts default to 1 byte — the parser
// always sees at least one variant for a well-formed enum, and
// the conservative-but-small default keeps regression risk near
// zero.
func enumByteWidth(variants int) int {
	switch {
	case variants <= 256:
		return 1
	case variants <= 65536:
		return 2
	case variants <= (1 << 32):
		return 4
	default:
		return 8
	}
}
