package messages

// Messages: what a send on a channel makes run where it is received.

// Store keeps what the workers handle.
type Store struct {
	total int
	last  string
	jobs  chan int // a channel kept in a field
}

func record(n int)   {}
func audit(s string) {}
func finish()        {}

// WorkerLoop handles each job received by a range over the channel.
func WorkerLoop(s *Store, jobs chan int) {
	for j := range jobs {
		s.total += j
		record(j)
	}
}

// Produce sends jobs to the worker through a parameter.
func Produce(jobs chan int) {
	jobs <- 1
}

// RunWorker makes an unbuffered channel, starts the worker and sends to it
// through Produce.
func RunWorker(s *Store) {
	jobs := make(chan int)
	go WorkerLoop(s, jobs)
	Produce(jobs)
}

// Serve handles two channels and a done signal in one select.
func Serve(s *Store, nums chan int, names chan string, done chan struct{}) {
	for {
		select {
		case n := <-nums:
			record(n)
		case name := <-names:
			s.last = name
			audit(name)
		case <-done:
			finish()
			return
		}
	}
}

// RunServe starts Serve and sends on each of its channels.
func RunServe(s *Store) {
	nums := make(chan int, 1)
	names := make(chan string, 1)
	done := make(chan struct{})
	go Serve(s, nums, names, done)
	nums <- 2
	names <- "a"
	close(done)
	done <- struct{}{}
}

// Drain receives one job from the field channel and handles the rest of
// its block.
func (s *Store) Drain() {
	j := <-s.jobs
	record(j)
	s.total = j
}

// Feed sends on the field channel.
func (s *Store) Feed() {
	s.jobs <- 3
}

// Local sends on a local channel that a goroutine's literal receives from.
func Local() {
	ch := make(chan int)
	go func() {
		v := <-ch
		record(v)
	}()
	ch <- 4
}
