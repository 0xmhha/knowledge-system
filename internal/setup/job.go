package setup

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Job states. A job is one asynchronous plan execution.
const (
	JobRunning = "running"
	JobDone    = "done"
	JobFailed  = "failed"
)

// Retention bounds keep a long-lived server's memory flat: cap the events
// stored per job and the number of finished jobs kept for status polling.
const (
	maxJobEvents    = 1000
	maxRetainedJobs = 64
)

// job is the mutable record behind one asynchronous run.
type job struct {
	id       string
	mu       sync.Mutex
	state    string
	events   []Event
	errMsg   string
	started  time.Time
	finished time.Time
	cancel   context.CancelFunc // cancels this job's context (idempotent)
}

// JobSnapshot is the read-only view returned to status callers.
type JobSnapshot struct {
	ID       string    `json:"id"`
	State    string    `json:"state"`
	Error    string    `json:"error,omitempty"`
	Started  time.Time `json:"started"`
	Finished time.Time `json:"finished,omitzero"`
	// Events is the tail of the progress stream (most recent last).
	Events []Event `json:"events"`
}

// Jobs is an in-memory registry of asynchronous setup runs. MCP tools are
// request/response, so long-running work is exposed as start + status-poll;
// this registry is the state between the two calls. In-memory is deliberate:
// a job's lifetime is bounded by the server process that spawned it.
type Jobs struct {
	mu     sync.Mutex
	seq    int
	byID   map[string]*job
	runner Runner
	// baseCtx is the parent of every job's context; cancelling it (Shutdown)
	// stops all in-flight jobs so a server exit does not orphan build subprocesses.
	baseCtx    context.Context
	baseCancel context.CancelFunc
}

// NewJobs returns a registry executing plans with r (nil → SubprocessRunner).
func NewJobs(r Runner) *Jobs {
	if r == nil {
		r = SubprocessRunner{}
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &Jobs{byID: map[string]*job{}, runner: r, baseCtx: ctx, baseCancel: cancel}
}

// StartFunc launches an arbitrary long-running operation in the background and
// returns the job ID. It is the generic core behind every async ops tool: kind
// labels the job in its ID, fn streams progress via the emit callback and
// returns an error on failure. The job detaches from the caller's context — an
// MCP request finishing must not kill work it started.
func (js *Jobs) StartFunc(kind string, fn func(ctx context.Context, emit func(Event)) error) string {
	jobCtx, cancel := context.WithCancel(js.baseCtx)
	js.mu.Lock()
	js.seq++
	j := &job{
		id:      fmt.Sprintf("%s-%d-%d", kind, js.seq, time.Now().UTC().Unix()),
		state:   JobRunning,
		started: time.Now().UTC(),
		cancel:  cancel,
	}
	js.byID[j.id] = j
	js.evictLocked()
	js.mu.Unlock()

	go func() {
		defer cancel()
		err := fn(jobCtx, func(e Event) {
			j.mu.Lock()
			j.events = append(j.events, e)
			if len(j.events) > maxJobEvents { // retain only the most recent tail
				j.events = j.events[len(j.events)-maxJobEvents:]
			}
			j.mu.Unlock()
		})
		j.mu.Lock()
		defer j.mu.Unlock()
		j.finished = time.Now().UTC()
		if err != nil {
			j.state = JobFailed
			j.errMsg = err.Error()
		} else {
			j.state = JobDone
		}
	}()
	return j.id
}

// evictLocked drops the oldest terminal (done/failed) jobs so at most
// maxRetainedJobs finished jobs are kept; running jobs are never evicted.
// The caller holds js.mu.
func (js *Jobs) evictLocked() {
	var terminal []*job
	for _, j := range js.byID {
		j.mu.Lock()
		done := j.state != JobRunning
		j.mu.Unlock()
		if done {
			terminal = append(terminal, j)
		}
	}
	if len(terminal) <= maxRetainedJobs {
		return
	}
	// started is set once at creation and never mutated, so it is safe to read.
	sort.Slice(terminal, func(a, b int) bool { return terminal[a].started.Before(terminal[b].started) })
	for _, j := range terminal[:len(terminal)-maxRetainedJobs] {
		delete(js.byID, j.id)
	}
}

// Shutdown cancels every in-flight job so build subprocesses do not outlive the
// server. Safe to call more than once.
func (js *Jobs) Shutdown() { js.baseCancel() }

// Cancel stops one running job by id, returning whether it was found. The job's
// function observes the cancelled context and finishes in the failed state.
func (js *Jobs) Cancel(id string) bool {
	js.mu.Lock()
	j, ok := js.byID[id]
	js.mu.Unlock()
	if !ok {
		return false
	}
	j.cancel()
	return true
}

// Start launches plan execution in the background and returns the job ID.
func (js *Jobs) Start(p Plan) string {
	return js.StartFunc("setup", func(ctx context.Context, emit func(Event)) error {
		return Execute(ctx, p, js.runner, emit)
	})
}

// StartReindex launches a blue-green Reindex (build o.Out/<version> → gate →
// promote current) as an async job and returns the job ID. current is left
// unchanged when the gate fails, so a bad rebuild never breaks the live dataset.
func (js *Jobs) StartReindex(o Options, version string, gopt GateOptions) string {
	return js.StartFunc("reindex", func(ctx context.Context, emit func(Event)) error {
		return Reindex(ctx, o, version, gopt, js.runner, emit)
	})
}

// Get returns a snapshot of the job, with at most tail trailing events
// (tail <= 0 returns all).
func (js *Jobs) Get(id string, tail int) (JobSnapshot, bool) {
	js.mu.Lock()
	j, ok := js.byID[id]
	js.mu.Unlock()
	if !ok {
		return JobSnapshot{}, false
	}
	j.mu.Lock()
	defer j.mu.Unlock()
	ev := j.events
	if tail > 0 && len(ev) > tail {
		ev = ev[len(ev)-tail:]
	}
	out := make([]Event, len(ev))
	copy(out, ev)
	return JobSnapshot{
		ID: j.id, State: j.state, Error: j.errMsg,
		Started: j.started, Finished: j.finished, Events: out,
	}, true
}
