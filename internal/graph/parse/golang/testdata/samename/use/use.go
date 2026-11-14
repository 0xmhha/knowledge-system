// Package use calls into one of the two x packages.
package use

import "example.com/samename/one/x"

// Genesis is the package-level type.
type Genesis struct{ A int }

// S has methods whose names match a conversion and a func variable below.
type S struct{}

func (S) cancel() {}

func (S) uint() {}

// Box is generic.
type Box[T any] struct{ v T }

// Put is a method of a generic type.
func (b *Box[T]) Put(v T) { b.v = v }

// Call calls one/x.Do, converts with uint and calls a func variable.
func Call() uint {
	x.Do()
	cancel := func() {}
	cancel()
	return uint(3)
}

// Take refers to one/x.T.
func Take(t x.T) x.T { return t }

// Use refers to the package-level Genesis.
func Use(g Genesis) Genesis { return g }

// G calls a method of an instantiated generic type.
func G() {
	var b Box[int]
	b.Put(1)
}

// Spawn calls one/x.Do inside a goroutine literal.
func Spawn() {
	go func() {
		x.Do()
	}()
}

// Fail is an error with its own Error method.
type Fail struct{}

func (Fail) Error() string { return "fail" }

// Report calls Error through the builtin error interface.
func Report(err error) string { return err.Error() }
