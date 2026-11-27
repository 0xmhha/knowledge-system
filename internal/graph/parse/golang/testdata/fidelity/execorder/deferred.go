package execorder

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Pool holds a lock its deferred work runs under.
type Pool struct {
	mu    sync.Mutex
	inner sync.Mutex
	quit  chan struct{}
	n     int64
	spent int
}

func (p *Pool) note() {}

// Add defers its Unlock, then work that runs before it with mu held: a
// literal, the calls in it, a close. A method on new(T) starts at the
// byte the builtin new starts at; only the method's call is a site.
func (p *Pool) Add() {
	p.mu.Lock()
	defer p.mu.Unlock()
	defer func(start time.Time) {
		p.note()
	}(time.Now())
	defer close(p.quit)
	new(Pool).note()
}

// Nested defers a literal that takes inner while mu is still held.
func (p *Pool) Nested() {
	p.mu.Lock()
	defer p.mu.Unlock()
	defer func() {
		p.inner.Lock()
		p.spent--
		p.inner.Unlock()
	}()
}

// Release defers a literal that unlocks mu: what is deferred before it
// runs after it, without mu.
func (p *Pool) Release() {
	defer p.note()
	p.mu.Lock()
	defer func() { p.mu.Unlock() }()
}

// Inline receives from a timer made where it is received.
func Inline() {
	<-time.NewTimer(time.Second).C
}

// sleep waits on a context its callers pass: one held by a local, one a
// function returns, so the set is incomplete and the site has no edge.
func sleep(ctx context.Context) {
	select {
	case <-ctx.Done():
	default:
	}
}

func rest() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	sleep(ctx)
}

func nap() { sleep(made()) }

func made() context.Context { return context.Background() }

// wait waits on a context every caller holds in a declaration, through a
// caller's own parameter.
func wait(ctx context.Context) { <-ctx.Done() }

func relay(ctx context.Context) { wait(ctx) }

func serve() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	relay(ctx)
	wait(context.Background())
}

// rearm resets the timer its caller assigns to a local.
func rearm(t *time.Timer) { t.Reset(time.Second) }

func arm() {
	t := time.NewTimer(time.Second)
	rearm(t)
}

// count adds to the counter its caller passes as a field.
func count(n *int64) { atomic.AddInt64(n, 1) }

func (p *Pool) Count() { count(&p.n) }

// Await is exported: code the program does not hold can call it, so its
// context's sources are never all known, though its one caller here
// passes a local.
func Await(ctx context.Context) { <-ctx.Done() }

func awaitLocal() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	Await(ctx)
}

// iter keeps a context made from the one it is passed: the made context is
// held by the field it is stored in.
type iter struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func newIter(ctx context.Context) *iter {
	ctx, cancel := context.WithCancel(ctx)
	return &iter{ctx: ctx, cancel: cancel}
}
