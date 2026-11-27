package lockprop

import "sync"

// Ordered calls helpers before it takes mu, while it holds mu, and through a
// helper that releases mu before part of its work.
type Ordered struct {
	mu     sync.Mutex
	before int
	inner  int
	after  int
	late   int
}

// Run calls touchBefore with mu not held, then unlockThenTouch while it
// holds mu.
func (o *Ordered) Run() {
	o.touchBefore()
	o.mu.Lock()
	o.unlockThenTouch()
}

// touchBefore runs before Run takes mu.
func (o *Ordered) touchBefore() { o.before++ }

// unlockThenTouch uses inner while its caller's mu is held, releases mu, and
// then uses after.
func (o *Ordered) unlockThenTouch() {
	o.inner++
	o.mu.Unlock()
	o.after++
}

// Later calls touchLate after it released mu.
func (o *Ordered) Later() {
	o.mu.Lock()
	o.mu.Unlock()
	o.touchLate()
}

// touchLate runs after Later released mu.
func (o *Ordered) touchLate() { o.late++ }
