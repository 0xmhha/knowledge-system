package flowfix

import (
	"context"
	"sync"
	"time"
)

// Ticker waits for a timer, then polls a channel without blocking.
func Ticker(ch chan int) {
	t := time.NewTimer(time.Second)
	select {
	case <-t.C:
		Depth(5)
	case v := <-ch:
		Depth(v)
	}
	select {
	case v := <-ch:
		Depth(v)
	default:
	}
}

// Cancels makes a context, cancels it in a goroutine and waits for it.
func Cancels() {
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		cancel()
	}()
	<-ctx.Done()
}

// once guards setup.
var once sync.Once

// Init runs setup once.
func Init() {
	once.Do(setup)
}

// setup runs once.
func setup() {
	Depth(6)
}

// ErrText calls the builtin error interface.
func ErrText(err error) string {
	return err.Error()
}

// Spawner computes the receiver of its goroutine under the mutex.
func (h *Hooks) Spawner() {
	h.mu.Lock()
	defer h.mu.Unlock()
	go h.pick().Run()
}

// pick is evaluated by Spawner before the goroutine starts.
func (h *Hooks) pick() *Hooks {
	return h
}

// Run is what the goroutine runs.
func (h *Hooks) Run() {}

// Polls has a select whose first case receives in its body.
func Polls(requests chan chan error, term chan struct{}) error {
	reply := make(chan error)
	select {
	case requests <- reply:
		return <-reply
	case <-term:
		return nil
	}
}

// Watch waits in a goroutine on a ticker and a context.
func Watch(ctx context.Context, wg *sync.WaitGroup) {
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		select {
		case <-ticker.C:
			Depth(7)
		case <-ctx.Done():
			return
		}
	}()
}

// Branches returns on one branch of an else-if chain, then ranges over a
// call that runs once.
func Branches(a, b bool) int {
	if a {
		if b {
			return 1
		}
		Depth(8)
	} else if b {
		Depth(9)
	}
	for _, v := range rangeOf() {
		Depth(v)
	}
	return 0
}

// rangeOf is the range expression of Branches.
func rangeOf() []int {
	return nil
}

// LitIf passes a literal that branches inside.
func LitIf() {
	withEach(func(v int) {
		if v > 0 {
			Depth(v)
		}
	}, 1)
}

// Pick2 returns early, then stores a literal whose code runs later.
func Pick2(v int) func() {
	if v < 0 {
		return nil
	}
	f := func() {
		Depth(11)
	}
	return f
}
