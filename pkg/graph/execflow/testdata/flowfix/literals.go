package flowfix

import (
	"context"
	"sync"
)

// Jobs passes callbacks to functions that use them in each way.
type Jobs struct {
	mu   sync.Mutex
	kept func()
}

// Run passes one literal to each kind of callee, all under mu.
func (j *Jobs) Run(items []int) {
	j.mu.Lock()
	defer j.mu.Unlock()
	callNow(func() {
		j.touch(1)
	})
	startLater(func() {
		j.touch(2)
	})
	j.keep(func() {
		j.touch(3)
	})
	deferIt(func() {
		j.touch(4)
	})
	handOn(func() {
		j.touch(5)
	})
	maybe(func() {
		j.touch(6)
	}, len(items) > 0)
	relay(func() {
		j.touch(7)
	})
}

// touch records v.
func (j *Jobs) touch(v int) { _ = v }

// callNow calls f before it returns.
func callNow(f func()) { f() }

// startLater runs f in a goroutine.
func startLater(f func()) { go f() }

// keep stores f for later.
func (j *Jobs) keep(f func()) { j.kept = f }

// deferIt calls f as it returns.
func deferIt(f func()) { defer f() }

// handOn hands f to a goroutine that calls it.
func handOn(f func()) {
	go func() {
		f()
	}()
}

// relay hands f on to another function.
func relay(f func()) { callNow(f) }

// maybe calls f on one branch only.
func maybe(f func(), ok bool) {
	if ok {
		f()
	}
}

// Skips leaves a loop iteration early, a loop, and a switch clause.
func Skips(xs []int, mode int) int {
	total := 0
	for _, x := range xs {
		if x < 0 {
			continue
		}
		if x > 100 {
			break
		}
		total += sideA(x)
	}
	switch mode {
	case 1:
		if total == 0 {
			return 0
		}
		total += sideA(1)
	case 2:
		total += sideA(2)
	}
	return total + sideA(3)
}

// sideA returns x.
func sideA(x int) int { return x }

// Svc keeps its context and the cancel of it in fields.
type Svc struct {
	ctx    context.Context
	cancel context.CancelFunc
}

// NewSvc makes the context and stores both in the service.
func NewSvc() *Svc {
	ctx, cancel := context.WithCancel(context.Background())
	return &Svc{
		ctx:    ctx,
		cancel: cancel,
	}
}

// Close cancels the context through the field.
func (s *Svc) Close() { s.cancel() }

// Wait waits until the context is cancelled.
func (s *Svc) Wait() { <-s.ctx.Done() }

// Poster queues callbacks for a worker.
type Poster struct {
	mu   sync.Mutex
	work chan func()
}

// Post queues a callback under mu.
func (p *Poster) Post() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.queue(func() {
		sideA(8)
	})
}

// queue sends f to the worker.
func (p *Poster) queue(f func()) { p.work <- f }

// Loop starts workers from inside select cases.
func (p *Poster) Loop(tick, flush, quit chan struct{}) {
	for {
		select {
		case <-tick:
			go p.drain()
		case <-flush:
			go func() {
				sideA(10)
			}()
			defer func() {
				sideA(11)
			}()
		case <-quit:
			return
		}
	}
}

// Local keeps literals in variables and calls them under mu.
func (p *Poster) Local() {
	visit := func(x int) {
		sideA(x)
	}
	var again func(n int)
	again = func(n int) {
		if n > 0 {
			again(n - 1)
		}
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	visit(12)
	again(3)
}

// drain does one unit of work.
func (p *Poster) drain() { sideA(9) }
