package flowfix

import (
	"context"
	"sync"
	"sync/atomic"
)

// Box receives values under its own lock.
type Box struct {
	mu  sync.Mutex
	in  chan int
	on  atomic.Bool
	sem chan struct{}
}

// NewBox makes a box whose sem is a channel used as a lock.
func NewBox() *Box {
	b := &Box{in: make(chan int), sem: make(chan struct{}, 1)}
	b.sem <- struct{}{}
	return b
}

// Put sends v to the box's receiver.
func (b *Box) Put(v int) { b.in <- v }

// Take receives in a loop while it holds mu.
func (b *Box) Take() {
	b.mu.Lock()
	defer b.mu.Unlock()
	for {
		v := <-b.in
		sideA(v)
	}
}

// Flag stores a flag under mu; a builtin call shares the line.
func (b *Box) Flag(xs []int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.on.Store(len(xs) != 0)
}

// Enter takes the channel lock in a select case and gives it back.
func (b *Box) Enter(quit chan struct{}) {
	select {
	case <-b.sem:
		sideA(13)
		b.sem <- struct{}{}
	case <-quit:
	}
}

// Measure calls an interface method under mu.
func (b *Box) Measure(s Shape) int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return s.Area()
}

// Timed cancels its context as it returns.
func Timed() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	<-ctx.Done()
}
