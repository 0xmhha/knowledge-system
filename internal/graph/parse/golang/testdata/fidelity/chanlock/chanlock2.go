package chanlock

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

// proto writes under a lock it gets through a parameter.
type proto struct {
	wstart <-chan struct{}
	w      int
}

// run makes the lock, fills it and hands it to start.
func run() {
	ws := make(chan struct{}, 1)
	ws <- struct{}{}
	start(ws)
}

// start keeps the lock in a field.
func start(ws <-chan struct{}) {
	p := &proto{wstart: ws}
	p.write()
}

// write takes the lock through the field.
func (p *proto) write() {
	<-p.wstart
	p.w++
}
