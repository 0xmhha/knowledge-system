package locks

import "sync"

// Ledger has fields that its methods use with and without its mutexes held.
type Ledger struct {
	mu        sync.Mutex
	other     sync.Mutex
	rw        sync.RWMutex
	before    int
	inside    int
	after     int
	branch    int
	deferred  int
	literal   int
	inPlace   int
	looped    int
	relocked  int
	early     int
	rlocked   int
	jumped    int
	switched  int
	litOuter  int
	afterCall int
}

// Sequence uses before ahead of the lock and after behind the unlock.
func (l *Ledger) Sequence() {
	l.before++
	l.mu.Lock()
	l.inside++
	l.mu.Unlock()
	l.after++
}

// Deferred holds mu to its end.
func (l *Ledger) Deferred() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.deferred++
}

// Branch takes other in one branch only and releases it there.
func (l *Ledger) Branch(ok bool) {
	if ok {
		l.other.Lock()
		l.other.Unlock()
	}
	l.branch++
}

// Literal holds mu, but the goroutine's literal runs apart from it, while
// the literal called in place runs under it.
func (l *Ledger) Literal() {
	l.mu.Lock()
	defer l.mu.Unlock()
	go func() { l.literal++ }()
	func() { l.inPlace++ }()
}

// Loop locks and unlocks in every round, and Relock unlocks and locks again
// inside a loop that runs under mu.
func (l *Ledger) Loop() {
	for i := 0; i < 3; i++ {
		l.mu.Lock()
		l.looped++
		l.mu.Unlock()
	}
	l.mu.Lock()
	for i := 0; i < 3; i++ {
		l.mu.Unlock()
		l.mu.Lock()
	}
	l.relocked++
	l.mu.Unlock()
}

// Early returns from one branch after unlocking; the rest runs under mu.
func (l *Ledger) Early(stop bool) {
	l.mu.Lock()
	if stop {
		l.mu.Unlock()
		return
	}
	l.early++
	l.mu.Unlock()
}

// Read holds the read lock of rw.
func (l *Ledger) Read() int {
	l.rw.RLock()
	defer l.rw.RUnlock()
	return l.rlocked
}

// Jump holds mu at jumped, but a goto puts the function out of the rule.
func (l *Ledger) Jump() {
	l.mu.Lock()
	l.jumped++
	goto done
done:
	l.mu.Unlock()
}

// Switch unlocks mu in one case, so it is not held after the switch.
func (l *Ledger) Switch(v int) {
	l.mu.Lock()
	switch v {
	case 1:
		l.mu.Unlock()
		return
	case 2:
		l.mu.Unlock()
	}
	l.switched++
}

// Outer takes other inside a goroutine's literal, which does not cover the
// code around it, and a literal called in place that releases mu leaves it
// released.
func (l *Ledger) Outer() {
	go func() {
		l.other.Lock()
		l.other.Unlock()
	}()
	l.litOuter++
	l.mu.Lock()
	func() { l.mu.Unlock() }()
	l.afterCall++
}

// Order takes other while it holds mu: the field other is used under mu,
// and mu is not used under other (it is unlocked after other is released).
func (l *Ledger) Order() {
	l.mu.Lock()
	l.other.Lock()
	l.other.Unlock()
	l.mu.Unlock()
}

// entry is set by a composite literal while a mutex is held.
type entry struct {
	key, value int
}

// Fill sets both fields of an entry, by key and by place, while it holds mu.
func (l *Ledger) Fill() (entry, entry) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return entry{key: 1}, entry{2, 3}
}

// Outer reaches a Ledger's mutex through its field inner.
type Outer struct {
	inner *Ledger
}

// Hold reads inner again where the deferred Unlock stands, under mu.
func (o *Outer) Hold() {
	o.inner.mu.Lock()
	defer o.inner.mu.Unlock()
}

// Counter is used by functions that Ledger.Locked calls.
type Counter struct {
	viaCall, afterCall, deep int
}

// Locked calls bump while it holds mu and settle after it released mu.
func (l *Ledger) Locked(c *Counter) {
	l.mu.Lock()
	bump(c)
	l.mu.Unlock()
	settle(c)
}

// bump uses viaCall and calls deeper, both while its caller holds mu.
func bump(c *Counter) {
	c.viaCall++
	deeper(c)
}

// deeper is two calls below the lock.
func deeper(c *Counter) { c.deep++ }

// settle runs after the lock was released.
func settle(c *Counter) { c.afterCall++ }
