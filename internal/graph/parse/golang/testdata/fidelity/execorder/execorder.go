// Package execorder holds what an execution-order reading needs: closes and
// the receives they wake, select statements, the locks held at each site,
// and operations on sync primitives.
package execorder

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Hub stops its workers by closing quit.
type Hub struct {
	quit  chan struct{}
	mu    sync.Mutex
	wg    sync.WaitGroup
	once  sync.Once
	cond  *sync.Cond
	hits  int64
	count atomic.Int32
	seen  int
}

var global = make(chan int)

// Stop closes quit and global while holding mu, and flushes.
func (h *Hub) Stop() {
	h.mu.Lock()
	close(h.quit)
	h.flush()
	h.mu.Unlock()
	close(global)
}

func (h *Hub) flush() {
	h.cond.Broadcast()
	h.seen++
}

// Drain closes a local channel it alone receives from.
func Drain() int {
	done := make(chan struct{})
	go func() {
		close(done)
	}()
	<-done
	select {
	case <-done:
		return 1
	default:
	}
	return 0
}

// Serve starts workers and waits for them.
func (h *Hub) Serve(ctx context.Context) {
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			h.wg.Add(1)
			atomic.AddInt64(&h.hits, 1)
			h.count.Add(1)
		}()
	}
	wg.Wait()
	h.once.Do(h.flush)
	h.cond.Wait()
	cctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	select {
	case <-cctx.Done():
	case <-h.quit:
	}
	t := time.AfterFunc(time.Second, h.flush)
	defer t.Stop()
	<-time.After(time.Millisecond)
}

func with(f func()) { f() }

// Spawn passes with a literal that starts a goroutine: with starts it.
func (h *Hub) Spawn() {
	with(func() {
		go h.flush()
	})
}

// Deferring defers its Unlock, then a Broadcast that runs before it; a
// builtin and a conversion run no code.
func (h *Hub) Deferring() {
	h.mu.Lock()
	defer h.mu.Unlock()
	defer h.cond.Broadcast()
	n := len(h.quit)
	h.seen = int(int64(n))
}

// Early defers a Broadcast before it locks: it runs after the Unlock.
func (h *Hub) Early() {
	defer h.cond.Broadcast()
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.seen > 0 {
		return
	}
	h.seen++
}

func worker(wg *sync.WaitGroup) { defer wg.Done() }

// Workers passes worker its local WaitGroup and its field.
func (h *Hub) Workers() {
	var wg sync.WaitGroup
	wg.Add(1)
	go worker(&wg)
	h.wg.Add(1)
	go worker(&h.wg)
	wg.Wait()
}

// Later runs f when a timer fires; Arm passes it a method.
func (h *Hub) Later(f func()) *time.Timer { return time.AfterFunc(time.Second, f) }

// Arm arms a timer for flush.
func (h *Hub) Arm() { h.Later(h.flush) }
