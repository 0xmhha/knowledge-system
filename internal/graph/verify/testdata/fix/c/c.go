// Package c exercises the relations verify checks.
package c

import ax "example.com/fix/a/x"

// T is the package-level type.
type T struct{}

// M is a method of T.
func (T) M() {}

// I is an interface with one method.
type I interface {
	M()
}

// Use makes a static call, an interface call, a call inside a closure and a
// call through a function value.
func Use(i I) {
	ax.Do()
	i.M()
	f := func() { ax.Do() }
	f()
}

// Marshal declares a local type with the same name as T.
func Marshal() {
	type T struct{ A int }
	_ = T{A: 1}
}

// Other refers to the package-level T.
func Other() T { return T{} }
