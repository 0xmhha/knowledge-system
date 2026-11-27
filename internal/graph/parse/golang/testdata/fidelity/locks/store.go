// Package locks holds mutexes that are locked in another file than the one
// that declares them, and mutexes declared by a value instead of a type.
package locks

import "sync"

// Store guards n with mu.
type Store struct {
	mu sync.Mutex
	n  int
}

// Pipe has two mutexes declared by one field.
type Pipe struct {
	rmu, wmu sync.Mutex // one for each direction
}

// registry is declared with a composite literal, not with a type.
var registry = sync.RWMutex{}
