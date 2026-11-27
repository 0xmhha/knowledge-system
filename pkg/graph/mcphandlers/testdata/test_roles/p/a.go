// Package p holds a field used by files whose build roles differ from their
// paths.
package p

// T holds v.
type T struct{ v int }

// Prod reads v in production code.
func Prod(t *T) int { return t.v }
