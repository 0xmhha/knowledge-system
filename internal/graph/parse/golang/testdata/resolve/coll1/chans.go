package coll1

// --- channel links fixture (sends_to, recvs_from by channel identity) ---

type pump struct{ out chan int }

func newPump() *pump { return &pump{out: make(chan int, 1)} }

// push and pull use the same field channel.
func (p *pump) push(v int) { p.out <- v }

func (p *pump) pull() int { return <-p.out }

// events is a package-level channel.
var events = make(chan string)

func emit(s string) { events <- s }

func listen() string {
	select {
	case s := <-events:
		return s
	}
}

// relay receives on in and sends on out; runRelay passes two local channels
// to it in a goroutine, so relay's operations are on runRelay's channels.
func relay(in <-chan int, out chan<- int) { out <- <-in }

func runRelay() int {
	a := make(chan int)
	b := make(chan int)
	go relay(a, b)
	a <- 1
	return <-b
}

// seal sends on its parameter from a goroutine; the closure captures the
// parameter by reference. runSeal passes its own channel.
func seal(results chan<- int, stop <-chan struct{}) {
	go func() {
		select {
		case <-stop:
			results <- 0
			return
		}
	}()
	results = nil
}

func runSeal() int {
	out := make(chan int, 1)
	seal(out, nil)
	return <-out
}
