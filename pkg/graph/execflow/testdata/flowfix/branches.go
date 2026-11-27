package flowfix

import (
	"sync"
	"sync/atomic"
	"time"
)

// Counter counts with an atomic, waits for its workers and keeps the last
// value under a mutex that one method takes and another releases.
type Counter struct {
	n    atomic.Int64
	wg   sync.WaitGroup
	mu   sync.Mutex
	last int
}

// Fan starts a worker per item, sends each item and waits for the workers.
func (c *Counter) Fan(items []int, out chan int) {
	for _, it := range items {
		c.wg.Add(1)
		go c.count(it)
		out <- it
	}
	c.wg.Wait()
	time.Sleep(time.Millisecond)
}

// count adds one item atomically.
func (c *Counter) count(it int) {
	defer c.wg.Done()
	c.n.Add(int64(it))
}

// Pick returns early, then takes one branch or the other.
func (c *Counter) Pick(v int) int {
	if v < 0 {
		return Depth(0)
	}
	if v > 10 {
		c.hold()
	} else {
		c.store(v)
	}
	return Depth(v)
}

// hold takes the mutex; release releases it.
func (c *Counter) hold() {
	c.mu.Lock()
}

// release releases the mutex hold takes.
func (c *Counter) release() {
	c.mu.Unlock()
}

// Drain stores every value received until the channel is closed.
func (c *Counter) Drain(ch chan int) {
	for v := range ch {
		c.store(v)
	}
}

// Arm arms a timer under the mutex; its callback runs later, without it.
func (c *Counter) Arm() {
	c.mu.Lock()
	defer c.mu.Unlock()
	time.AfterFunc(time.Second, func() {
		c.store(1)
	})
	schedule(func() {
		c.store(2)
	})
}

// schedule runs f later, in a goroutine of its own.
func schedule(f func()) {
	go f()
}

// store keeps the last value.
func (c *Counter) store(v int) {
	c.last = v
}

// Locked runs a callback inside withEach while it holds the mutex.
func (c *Counter) Locked() {
	c.mu.Lock()
	defer c.mu.Unlock()
	withEach(func(v int) {
		c.store(v)
	}, 3)
}

// withEach calls f before it returns.
func withEach(f func(int), v int) {
	f(v)
}

// Apply has a parameter of function type, which is not a literal.
func (c *Counter) Apply(v int,
	f func(int) int) {
	c.store(f(v))
}
