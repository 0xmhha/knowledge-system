package mutex_fixture

import "fmt"

// Doer is called through an interface by a goroutine.
type Doer interface{ Do() }

// Base is embedded by Worker; Log is promoted.
type Base struct{ name string }

// Log reads name.
func (b *Base) Log() string { return b.name }

// Worker runs a loop in a goroutine.
type Worker struct {
	Base
	d     Doer
	f     func()
	inbox chan int
	n     int
}

// NewWorker makes inbox and keeps it in the field.
func NewWorker() *Worker {
	ch := make(chan int, 1)
	w := &Worker{inbox: ch}
	ch <- 1 // sent on the make site's name, received through the field
	return w
}

// hits is a package-level variable a goroutine's literal assigns.
var hits int

// Start runs loop as a goroutine and a literal that calls through an
// interface, a function value and a promoted method.
func (w *Worker) Start() {
	go w.loop()
	go func() {
		w.d.Do()
		w.f()
		_ = w.Log()
		hits++
	}()
}

// loop is the entry of a goroutine; the literal it calls in place is its
// own code, reached through loop's own edges.
func (w *Worker) loop() {
	func() {
		w.n++
	}()
	v := <-w.inbox
	fmt.Println("got", v)
	w.reply(v)
}
