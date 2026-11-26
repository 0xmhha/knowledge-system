// Package calls holds call shapes whose graph sites and edges the fidelity
// tests check.
package calls

// T counts.
type T struct{ n int }

// New returns a counter.
func New() *T { return &T{} }

// Inc adds one and returns the counter.
func (t *T) Inc() *T { t.n++; return t }

// Val returns the count.
func (t *T) Val() int { return t.n }

// Chain makes three calls that start at the same byte.
func Chain() int {
	return New().Inc().Val()
}

// Default is initialised by two calls outside any function.
var Default = New().Inc()

// Hook is a function literal in a package-level initialiser.
var Hook = func() int {
	if Default != nil {
		return Default.Val()
	}
	return 0
}

// Eager is initialised by a function literal that is called on the spot.
var Eager = func() int { return New().Val() }()

// Scoped declares a local variable.
func Scoped() int {
	var buf int
	return buf
}
