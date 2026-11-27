package chanlock

// ClosableMutex is a mutex that can be closed: a channel of capacity one
// that holds its token when unlocked.
type ClosableMutex struct {
	ch chan struct{}
}

// NewClosableMutex makes the lock and puts its token in.
func NewClosableMutex() *ClosableMutex {
	ch := make(chan struct{}, 1)
	ch <- struct{}{}
	return &ClosableMutex{ch}
}

// TryLock takes the token if the lock is not closed.
func (cm *ClosableMutex) TryLock() bool {
	_, ok := <-cm.ch
	return ok
}

// MustLock takes the token or panics.
func (cm *ClosableMutex) MustLock() {
	_, ok := <-cm.ch
	if !ok {
		panic("mutex closed")
	}
}

// Unlock returns the token.
func (cm *ClosableMutex) Unlock() {
	select {
	case cm.ch <- struct{}{}:
	default:
		panic("Unlock of already-unlocked ClosableMutex")
	}
}

// Close takes the token and closes the lock.
func (cm *ClosableMutex) Close() {
	_, ok := <-cm.ch
	if !ok {
		panic("Close of already-closed ClosableMutex")
	}
	close(cm.ch)
}

// Device guards count with a channel lock taken in a select.
type Device struct {
	comms chan struct{}
	count int
	last  int
}

// NewDevice makes the lock and fills it.
func NewDevice() *Device {
	d := &Device{comms: make(chan struct{}, 1)}
	d.comms <- struct{}{}
	return d
}

// Bump takes the lock unless cancelled: count is used under it, last only
// on the cancel path.
func (d *Device) Bump(cancel chan struct{}) {
	select {
	case <-d.comms:
		d.count++
		d.comms <- struct{}{}
	case <-cancel:
		d.last = 1
	}
}

// Pool is a semaphore of capacity two: a channel, not a mutex.
type Pool struct {
	slots chan struct{}
	used  int
}

// NewPool fills the semaphore.
func NewPool() *Pool {
	p := &Pool{slots: make(chan struct{}, 2)}
	p.slots <- struct{}{}
	p.slots <- struct{}{}
	return p
}

// Use takes a slot.
func (p *Pool) Use() {
	<-p.slots
	p.used++
	p.slots <- struct{}{}
}

// Gate is a struct{} channel of capacity one filled in another function: a
// channel, not a mutex.
type Gate struct {
	open  chan struct{}
	state int
}

// NewGate makes the channel and leaves it empty.
func NewGate() *Gate { return &Gate{open: make(chan struct{}, 1)} }

// Open fills it.
func (g *Gate) Open() { g.open <- struct{}{} }

// Pass waits for it.
func (g *Gate) Pass() {
	<-g.open
	g.state++
}
