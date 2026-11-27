package mutex_fixture

// Resetter runs at most one reset: busy is a lock taken by a send.
type Resetter struct {
	runs int
}

// Loop tries to take busy before a reset and returns it when one is done.
func (r *Resetter) Loop(done chan int, heads chan int) {
	busy := make(chan struct{}, 1)
	for {
		select {
		case busy <- struct{}{}:
			r.runs++
		default:
		}
		select {
		case <-heads:
		case <-done:
			<-busy
		}
	}
}

// Indexer answers a quit request with nil on the channel it receives.
type Indexer struct {
	quit  chan chan error
	state int
}

// Stop receives the reply channel and answers on it.
func (c *Indexer) Stop() {
	<-c.quit <- nil
	c.state = 1
}

// Sub is unsubscribed in a deferred call.
type Sub interface{ Unsubscribe() }

// Watch runs a literal that defers an interface call and calls a closure
// stored in a variable.
func Watch(s Sub, p Doer) {
	shutdown := func() {
		record(1)
	}
	go func() {
		defer s.Unsubscribe()
		shutdown()
	}()
	go p.Do()
}

// Named is received by value; its ID is promoted from Inner.
type Named struct{ Inner }

// Inner has ID.
type Inner struct{ id int }

// ID returns id.
func (i *Inner) ID() int { return i.id }

// Recv calls the promoted method on what it receives.
func Recv(ch chan Named) {
	pd := <-ch
	record(pd.ID())
}

func pong() {}

// Pinger receives what its goroutine's literal sends, then calls pong.
func Pinger() {
	ch := make(chan int)
	go func() { ch <- 1 }()
	<-ch
	pong()
}

// Reg runs a function for each entry.
type Reg struct{}

// Each calls f.
func (Reg) Each(f func(int)) { f(1) }

func snap(int) {}

// LogAll calls snap in a literal passed to Each, on every tick.
func LogAll(r Reg, tick chan struct{}) {
	for range tick {
		r.Each(func(i int) {
			snap(i)
		})
	}
}

func multi() {}

// Two receives twice, then calls multi: both receives handle that line.
func Two(a, b chan int) {
	<-a
	<-b
	multi(); multi()
}

// RunTwo sends to both from one goroutine and from itself, on one line.
func RunTwo(r Reg) {
	a, b := make(chan int, 2), make(chan int, 2)
	go func() { a <- 1; b <- 1; multi(); multi() }()
	a <- 2; b <- 2
	Two(a, b)
	r.Each(func(int) { multi(); multi() })
	go func() { multi() }(); go func() { multi() }()
}
