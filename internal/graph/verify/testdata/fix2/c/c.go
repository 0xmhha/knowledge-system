// Package c holds the cases where an absent or present edge is correct.
package c

import (
	ax "example.com/fix2/a/x"
	bx "example.com/fix2/b/x"
)

// Both refers to two types named K from two packages.
func Both(a ax.K, b bx.K) {}

// Reader is an interface with one method.
type Reader interface {
	Read()
}

// Conn embeds Reader, so Conn values have a Read method.
type Conn struct {
	Reader
}

// Pull calls the embedded interface method through a struct.
func Pull(c Conn) {
	c.Read()
}
