// Package ifcalls holds calls whose callee is not a plain package-level
// function or method: interface methods reached in several ways, a method on
// an alias receiver, and a closure called where it is written.
package ifcalls

import "fmt"

// Reader is embedded in Conn.
type Reader interface{ Read() int }

// Conn embeds an interface.
type Conn struct{ Reader }

// Wrap holds a Conn.
type Wrap struct{ c *Conn }

// Pull calls the embedded interface's method through the struct.
func (w *Wrap) Pull() int { return w.c.Read() }

type elem struct{ n int }

// E is an alias whose methods are declared on the alias name.
type E = elem

// One is declared on the alias receiver.
func (e *E) One() *E { e.n = 1; return e }

// NewE calls a method declared on the alias receiver.
func NewE() *E { return new(E).One() }

// Text calls the method of the builtin error interface.
func Text(err error) string { return err.Error() }

// Threads calls a method of an interface declared in its body.
func Threads(v any) {
	type threaded interface{ SetThreads(n int) }
	if th, ok := v.(threaded); ok {
		th.SetThreads(1)
	}
}

// Temporary calls a method of an interface type written without a name.
func Temporary(err error) bool {
	t, ok := err.(interface{ Temporary() bool })
	return ok && t.Temporary()
}

// Slot has a parameter of an interface type written without a name.
func Slot(b interface{ Bytes() []byte }) int { return len(b.Bytes()) }

// Set has a type parameter whose constraint is written without a name.
type Set[T interface{ Key() string }] struct{ items []T }

// Keys calls the method of the constraint.
func (s *Set[T]) Keys() []string {
	var out []string
	for _, item := range s.items {
		out = append(out, item.Key())
	}
	return out
}

// Version is initialised by a closure that is called where it is written.
var Version = func() string { return fmt.Sprint(1) }()
