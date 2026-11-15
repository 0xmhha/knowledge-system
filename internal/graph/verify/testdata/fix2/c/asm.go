package c

// Word is the type the bodyless declaration below refers to.
type Word uint64

// add is implemented in assembly; only its signature is Go.
func add(a, b *Word)
