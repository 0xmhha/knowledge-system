package a

import "time"

// Tipper has two implementations whose values reach use.
type Tipper interface{ Tip() int }

type gov struct{ v int }

func (g *gov) Tip() int { return g.v }

type fixed struct{}

func (fixed) Tip() int { return 1 }

func use(t Tipper) int { return t.Tip() }

func each(f func(int) int) int { return f(1) }

func inc(n int) int { return n + 1 }

type pump struct{ out chan int }

func newPump() *pump { return &pump{out: make(chan int, 1)} }

func (p *pump) push(v int) { p.out <- v }

func (p *pump) pull() int {
	select {
	case v := <-p.out:
		return v
	}
}

// Run makes every kind of call once.
func Run() int {
	p := newPump()
	p.push(2)
	return use(&gov{v: 3}) + use(fixed{}) + each(inc) + each(func(n int) int { return n * 2 }) + p.pull()
}

// hub keeps the channel newHub makes; Hub sends on the local name and
// receives on the field.
type hub struct{ in chan int }

func newHub() (*hub, chan int) {
	c := make(chan int, 1)
	return &hub{in: c}, c
}

func Hub() int {
	h, c := newHub()
	c <- 5
	return <-h.in
}

// Shutter is called only by a deferred call.
type Shutter interface{ Shut() }

func (g *gov) Shut() { g.v = 0 }

// Deferred makes an interface call and a function-value call in defer
// statements, which run at its end, and waits on a timer, whose channel
// the runtime sends on.
func Deferred(f func(int) int) int {
	var s Shutter = &gov{v: 1}
	defer s.Shut()
	defer f(1)
	t := time.NewTimer(time.Millisecond)
	<-t.C
	return 0
}
