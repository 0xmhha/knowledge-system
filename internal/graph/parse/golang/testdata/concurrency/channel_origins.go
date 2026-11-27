package mutex_fixture

// Channel origins a goroutine literal uses: a channel declared with var,
// one ranged over, two made by one :=, and one made inside the literal.
func ChannelOrigins() {
	var done = make(chan struct{})
	items := make(chan int)
	errs, outs := make(chan error, 1), make(chan string, 1)
	go func() {
		for v := range items {
			_ = v
		}
		done <- struct{}{}
		errs <- nil
		outs <- ""
		inner := make(chan bool, 1)
		inner <- true
	}()
	<-done
}

// Works sends on a channel made by := in the goroutine's caller.
func Works() {
	ch := make(chan int, 1)
	go func() { ch <- 1 }()
}

// InsideFor starts the goroutine in a loop.
func InsideFor(n int) {
	ch := make(chan int, n)
	for i := 0; i < n; i++ {
		go func() { ch <- i }()
	}
}

// InsideIf starts the goroutine in a branch.
func InsideIf(ok bool) {
	ch := make(chan int, 1)
	if ok {
		go func() { ch <- 1 }()
	}
}

// GroupedVar declares its channel in a grouped var.
func GroupedVar() {
	var (
		fail = make(chan error, 1)
	)
	go func() { fail <- nil }()
	<-fail
}

// CallArg passes its channel to a call before the goroutine ranges over it.
func CallArg() {
	ev := make(chan int)
	register(ev)
	go func() {
		for range ev {
		}
	}()
}

func register(chan int) {}

// Job is worked on by goroutines.
type Job struct {
	id   int
	done bool
}

func worker(n int) {}

// StartsNamed starts a declared function.
func StartsNamed(j *Job) {
	go worker(j.id)
}

// StartsLiteral starts a literal that calls worker and uses j's fields.
func StartsLiteral(j *Job) {
	go func() {
		worker(j.id)
		j.done = true
	}()
}
