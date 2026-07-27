package setup

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Job states. A job is one asynchronous plan execution.
const (
	JobRunning = "running"
	JobDone    = "done"
	JobFailed  = "failed"
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
}

// NewJobs returns a registry executing plans with r (nil → SubprocessRunner).
func NewJobs(r Runner) *Jobs {
	if r == nil {
		r = SubprocessRunner{}
	}
	return &Jobs{byID: map[string]*job{}, runner: r}
}

// StartFunc launches an arbitrary long-running operation in the background and
// returns the job ID. It is the generic core behind every async ops tool: kind
// labels the job in its ID, fn streams progress via the emit callback and
// returns an error on failure. The job detaches from the caller's context — an
// MCP request finishing must not kill work it started.
func (js *Jobs) StartFunc(kind string, fn func(ctx context.Context, emit func(Event)) error) string {
	js.mu.Lock()
	js.seq++
	j := &job{
		id:      fmt.Sprintf("%s-%d-%d", kind, js.seq, time.Now().UTC().Unix()),
		state:   JobRunning,
		started: time.Now().UTC(),
	}
	js.byID[j.id] = j
	js.mu.Unlock()

	go func() {
		err := fn(context.Background(), func(e Event) {
			j.mu.Lock()
			j.events = append(j.events, e)
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
