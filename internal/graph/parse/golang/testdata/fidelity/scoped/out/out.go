// Package out belongs to the module but is left out of the build scope by
// the test that reads it.
package out

// T is a type the build uses but does not contain.
type T struct{ N int }

// Helper is called from inside the build.
func Helper() int { return 1 }

// Twice is a method called from inside the build.
func (t T) Twice() int { return t.N * 2 }
