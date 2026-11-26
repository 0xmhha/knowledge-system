// Package shapes holds declarations whose graph nodes the fidelity tests check.
package shapes

// Key is embedded by Unlocked.
type Key struct {
	ID int
}

// Name reports a fixed name.
func (k *Key) Name() string { return "k" }

// Unlocked embeds *Key: the embedded field is used as x.Key.
type Unlocked struct {
	*Key
	Abort chan struct{}
}

// Wire has a field of an anonymous struct type.
type Wire struct {
	Sig struct {
		R int
		S int
	}
}

// Box is generic, so its methods have a generic receiver.
type Box[T any] struct {
	v T
}

// Reset clears the box.
func (b *Box[T]) Reset() {
	var zero T
	b.v = zero
}
