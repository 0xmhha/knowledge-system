// Package flowfix is the execution flow fixture: a worker pool fed by a
// channel, a select with a done signal, workers started with a named
// method, a call chain under a mutex, an interface call with two
// implementations and a recursive function.
package flowfix

import "sync"

// Job is one unit of work.
type Job struct {
	n int
}

// Pool feeds jobs to its workers over a channel.
type Pool struct {
	mu    sync.Mutex
	total int
	jobs  chan Job
	done  chan struct{}
}

// NewPool makes a pool with its channels.
func NewPool() *Pool {
	return &Pool{jobs: make(chan Job), done: make(chan struct{})}
}

// Start starts the workers, each a goroutine that runs a named method.
func (p *Pool) Start(workers int) {
	for i := 0; i < workers; i++ {
		go p.work()
	}
}

// work is the worker loop: it handles each job and stops on done.
func (p *Pool) work() {
	for {
		select {
		case j := <-p.jobs:
			p.handle(j)
		case <-p.done:
			return
		}
	}
}

// handle adds a job to the total under the mutex.
func (p *Pool) handle(j Job) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.add(j.n)
}

// add writes the total; its caller holds mu.
func (p *Pool) add(n int) {
	p.total += n
}

// Submit sends a job to the workers.
func (p *Pool) Submit(n int) {
	p.jobs <- Job{n: n}
}

// Stop signals the workers to stop.
func (p *Pool) Stop() {
	p.done <- struct{}{}
}

// Run drives the pool: start the workers, submit a job, stop.
func Run() {
	p := NewPool()
	p.Start(2)
	p.Submit(1)
	p.Stop()
}

// Flush holds the mutex while it starts a goroutine whose literal calls
// add: the goroutine does not hold the mutex.
func (p *Pool) Flush() {
	p.mu.Lock()
	defer p.mu.Unlock()
	go func() {
		p.add(0)
	}()
}
