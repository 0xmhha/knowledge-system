package coll1

// --- statement links fixture (cond_reads, cond_calls, body_writes, encloses, site_calls) ---

type env struct {
	root  int
	state int
	count int
	ready bool
}

func (e *env) fresh() bool { return e.ready }

func load(n int) int { return n * 2 }

// refresh writes state only when the root changed; count is written on
// every call; ready only in the else branch.
func (e *env) refresh(root int) {
	if e.root != root {
		e.root = root
		e.state = load(root)
	} else {
		e.ready = true
	}
	e.count++
}

// drain loops while fresh() holds and switches on state.
func (e *env) drain() {
	for e.fresh() {
		switch e.state {
		case 1:
			e.count = 0
		default:
			return
		}
	}
	go func() {
		e.state = load(e.count)
	}()
	go e.refresh(e.root)
}

// probe writes e.state in its condition: that is not a read.
func (e *env) probe() {
	if e.state, e.count = load(1), 2; e.ready {
		e.root = 0
	}
}

// embeds holds a Tipper; calling Tip through it calls the interface method.
type embeds struct{ Tipper }

func viaEmbed(x embeds) int { return x.Tip() }

// spin starts a goroutine whose literal has its own statements, and one
// whose argument is a call.
func (e *env) spin() {
	go func() {
		if e.ready {
			e.count = load(2)
		}
	}()
	go e.refresh(load(3))
}

func runWith(f func() int) int { return f() }

// handOff passes a literal to the function a goroutine runs: the literal's
// return is not inside the goroutine statement.
func handOff() {
	go runWith(func() int {
		return load(4)
	})
}
