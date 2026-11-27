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
