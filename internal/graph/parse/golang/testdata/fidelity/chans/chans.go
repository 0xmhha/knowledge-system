// Package chans holds channels made in several ways, a range over a channel
// and statements nested in function literals and go statements.
package chans

type signal chan struct{}

// Work makes three channels and nests statements.
func Work(in chan int, items [][]byte) int {
	named := make(signal)
	raw := make(chan []byte, 1)
	go func() {
		inner := make(chan *int)
		close(inner)
	}()
	total := 0
	for v := range in { // ranges over a channel
		if v > 0 {
			in <- v // sends inside an if
		}
		total += v
	}
	for i := range items {
		run(func() {
			if i > 0 { // inside a literal inside a loop
				total++
			}
		})
	}
	go func() {
		raw <- nil // sends on a local channel inside a go literal
	}()
	go func(raw chan []byte) {
		raw <- nil // sends on the parameter that hides the local channel
	}(raw)
	go add(next(total)) // next is called by the go statement
	close(named)
	close(raw)
	return total
}

// ceiling is read in a loop header, inside a function literal.
var ceiling = 3

// Each ranges over function literals written in its header.
func Each() int {
	n := 0
	for _, f := range []func() int{func() int { return ceiling }} { // ceiling is read in the header
		n += f()
	}
	return n
}

func run(f func()) { f() }

func add(int) {}

func next(n int) int { return n + 1 }
