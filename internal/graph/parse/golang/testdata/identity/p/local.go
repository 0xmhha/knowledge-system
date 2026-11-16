package p

import "sync"

// pair is the package-level type.
type pair struct{ k string }

// Lit declares its own pair and builds a value of it.
func Lit() int {
	type pair struct{ a int }
	v := []pair{{1}}
	return len(v)
}

// Pkg uses the package-level pair.
func Pkg(p pair) string { return p.k }

// mu is a package-level mutex: one Variable node and one Mutex node.
var mu sync.Mutex
