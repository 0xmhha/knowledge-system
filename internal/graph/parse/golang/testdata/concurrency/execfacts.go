package mutex_fixture

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Crew closes quit to stop its workers; a close wakes every receive.
type Crew struct {
	quit  chan struct{}
	mu    sync.Mutex
	wg    sync.WaitGroup
	once  sync.Once
	cond  *sync.Cond
	hits  int64
	count atomic.Int64
	n     int
}

// Stop closes quit while holding mu.
func (p *Crew) Stop() {
	p.mu.Lock()
	close(p.quit)
	p.flush()
	p.mu.Unlock()
}

// Work waits for quit or a tick, and has a default.
func (p *Crew) Work(ticks chan int) {
	select {
	case <-p.quit:
		p.n++
	case v := <-ticks:
		p.n += v
	default:
	}
}

// Wait waits for quit only.
func (p *Crew) Wait() {
	select {
	case <-p.quit:
	}
	<-p.quit
}

// Run starts workers and waits for them.
func (p *Crew) Run() {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		p.wg.Add(1)
	}()
	wg.Wait()
	p.once.Do(p.init)
	p.cond.Signal()
	atomic.AddInt64(&p.hits, 1)
	p.count.Add(1)
}

func (p *Crew) init() {}

// flush wakes every waiter; Stop calls it holding mu.
func (p *Crew) flush() {
	p.cond.Broadcast()
}

// Cancel makes a context and cancels it.
func (p *Crew) Cancel(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	<-ctx.Done()
	t := time.AfterFunc(time.Second, p.init)
	t.Stop()
	<-time.After(time.Second)
	tk := time.NewTicker(time.Second)
	<-tk.C
	tk.Stop()
}

func run(f func()) { f() }

// Spawn passes run a literal that starts a goroutine: run starts it.
func (p *Crew) Spawn() {
	run(func() {
		go p.init()
	})
}

// Finish closes quit when it returns; its constant runs nothing under mu.
func (p *Crew) Finish() {
	defer close(p.quit)
	p.mu.Lock()
	const limit = int64(1 << 4)
	p.n = int(limit)
	p.mu.Unlock()
}

// Reorg makes next before its loop and again inside it: close(next) closes
// either.
func (p *Crew) Reorg(stop chan int, run bool) {
	var (
		cur  chan struct{}
		next = make(chan struct{})
	)
	for {
		if cur == nil && run {
			go p.wait(next)
			cur, next = next, make(chan struct{})
		}
		select {
		case <-cur:
			cur = nil
		case <-stop:
			close(next)
			return
		}
	}
}

func (p *Crew) wait(ch chan struct{}) { <-ch }

// Deferring defers its Unlock, then a Signal that runs before it; a
// builtin and a conversion run no code.
func (p *Crew) Deferring() {
	p.mu.Lock()
	defer p.mu.Unlock()
	defer p.cond.Signal()
	n := len(p.quit)
	p.n = int(int64(n))
}

// Early defers a Signal before it locks: it runs after the Unlock.
func (p *Crew) Early() {
	defer p.cond.Broadcast()
	p.mu.Lock()
	defer p.mu.Unlock()
}

func crewWorker(wg *sync.WaitGroup) { defer wg.Done() }

// Workers passes worker its local WaitGroup and its field.
func (p *Crew) Workers() {
	var wg sync.WaitGroup
	wg.Add(1)
	go crewWorker(&wg)
	p.wg.Add(1)
	go crewWorker(&p.wg)
	wg.Wait()
}

func crewWaitCtx(ctx context.Context) { <-ctx.Done() }

// Later runs f when a timer fires; Arm passes it a method.
func (p *Crew) Later(f func()) *time.Timer { return time.AfterFunc(time.Second, f) }

// Arm arms a timer for init and waits on a context no declaration holds.
func (p *Crew) Arm(parent context.Context) {
	p.Later(p.init)
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	crewWaitCtx(ctx)
}

// Tick waits for a timer or for a cancel a goroutine makes.
func (p *Crew) Tick(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	go func() { cancel() }()
	t := time.NewTimer(time.Second)
	select {
	case <-t.C:
		p.init()
	case <-ctx.Done():
		p.flush()
	}
}

// DeferLit defers its Unlock, then work that runs before it, with mu held:
// a literal (its call and the calls in it), a close. The call of new is no
// site; the method called on its result, from the same byte, is one.
func (p *Crew) DeferLit() {
	p.mu.Lock()
	defer p.mu.Unlock()
	defer func(start time.Time) {
		p.flush()
	}(time.Now())
	defer close(p.quit)
	new(Crew).flush()
}

var crewMu sync.Mutex

// DeferNested defers a literal that takes crewMu while mu is still held.
func (p *Crew) DeferNested() {
	p.mu.Lock()
	defer p.mu.Unlock()
	defer func() {
		crewMu.Lock()
		p.n--
		crewMu.Unlock()
	}()
}

// DeferRelease defers a literal that unlocks mu; a call deferred before it
// runs after it, without mu.
func (p *Crew) DeferRelease() {
	defer p.flush()
	p.mu.Lock()
	defer func() { p.mu.Unlock() }()
}

// Inline receives from a timer made where it is received.
func (p *Crew) Inline() {
	<-time.NewTimer(time.Second).C
}

// crewSleep waits on a context its callers pass: Rest passes one a local
// variable holds, Nap the value a function returns (no declaration).
func crewSleep(ctx context.Context) {
	select {
	case <-ctx.Done():
	default:
	}
}

func (p *Crew) Rest() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	crewSleep(ctx)
}

func (p *Crew) Nap() { crewSleep(crewContext()) }

func crewContext() context.Context { return context.Background() }

// crewRearm resets the timer it is passed; Rearm passes a local one.
func crewRearm(t *time.Timer) { t.Reset(time.Second) }

func (p *Crew) Rearm() {
	t := time.NewTimer(time.Second)
	crewRearm(t)
}

// crewCount adds to the counter it is passed; Count passes a field.
func crewCount(n *int64) { atomic.AddInt64(n, 1) }

func (p *Crew) Count() { crewCount(&p.hits) }

// crewIter keeps a context made from the one it is passed: the made one is
// held by the field it is stored in, not by the parameter it overwrites.
type crewIter struct {
	ctx    context.Context
	cancel context.CancelFunc
}

func newCrewIter(ctx context.Context) *crewIter {
	ctx, cancel := context.WithCancel(ctx)
	return &crewIter{ctx: ctx, cancel: cancel}
}
