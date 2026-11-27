package flowfix

import "sync"

// Hooks calls through a function-typed field, defers a literal that
// unlocks, takes a lock inside a goroutine and waits on a channel that is
// only closed.
type Hooks struct {
	mu     sync.Mutex
	height func() int
	done   chan struct{}
}

// NewHooks stores a method value in the function-typed field.
func NewHooks(c *Counter) *Hooks {
	h := &Hooks{done: make(chan struct{})}
	h.height = c.value
	return h
}

// Height calls through the function-typed field.
func (h *Hooks) Height() int {
	return h.height()
}

// Defers takes the mutex and releases it in a deferred literal.
func (h *Hooks) Defers() {
	h.mu.Lock()
	defer func() {
		h.mu.Unlock()
	}()
	Depth(1)
}

// Spawn takes the mutex inside its goroutine.
func (h *Hooks) Spawn() {
	go func() {
		h.mu.Lock()
		Depth(2)
		h.mu.Unlock()
	}()
}

// Unregister releases the mutex on a branch that returns and still holds
// it after the branch.
func (h *Hooks) Unregister(ok bool) {
	h.mu.Lock()
	if !ok {
		h.mu.Unlock()
		return
	}
	Depth(3)
	h.mu.Unlock()
}

// Snapshot takes and releases the mutex on a branch that returns, and on
// a branch whose literal returns; the code after them holds nothing.
func (h *Hooks) Snapshot(empty bool, fs []func() int) []func() int {
	if empty {
		h.mu.Lock()
		h.mu.Unlock()
		return nil
	}
	if len(fs) > 0 {
		h.mu.Lock()
		h.mu.Unlock()
		fs = append(fs, func() int { return 1 })
	}
	Depth(4)
	return fs
}

// Wait waits for done, which Close closes.
func (h *Hooks) Wait() {
	<-h.done
}

// Close closes done.
func (h *Hooks) Close() {
	close(h.done)
}

// value is what the field holds.
func (c *Counter) value() int {
	return c.last
}

// Circle implements Shape but no Circle reaches Sum.
type Circle struct {
	r int
}

// Area of a circle.
func (c Circle) Area() int {
	return 3 * c.r * c.r
}

// viaTestsOnly is called from test code only.
func viaTestsOnly() {
	testOnly()
}

// testOnly is called by viaTestsOnly.
func testOnly() {}
