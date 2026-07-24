package setup

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"
)

// Event is one machine-readable progress record. The setup CLI serializes
// these as one JSON object per line under --progress=json; the MCP job
// surface returns the tail of the same stream.
type Event struct {
	Time    time.Time `json:"time"`
	Step    string    `json:"step"`
	Type    string    `json:"type"` // "start" | "output" | "warning" | "done" | "error"
	Message string    `json:"message,omitempty"`
}

// Runner executes one step. The default is SubprocessRunner; tests substitute
// their own to exercise plan orchestration without real engine binaries.
type Runner interface {
	Run(ctx context.Context, s Step, emit func(Event)) error
}

// SubprocessRunner runs command steps as child processes, streaming combined
// output line-by-line as Events, and runs internal steps in-process.
type SubprocessRunner struct{}

func (SubprocessRunner) Run(ctx context.Context, s Step, emit func(Event)) error {
	if s.Cmd == nil {
		if s.Verify == nil {
			return fmt.Errorf("step %s: neither command nor verify", s.ID)
		}
		return s.Verify(emit)
	}
	cmd := exec.CommandContext(ctx, s.Cmd[0], s.Cmd[1:]...)
	if len(s.Env) > 0 {
		cmd.Env = append(cmd.Environ(), s.Env...)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("step %s: %w", s.ID, err)
	}
	cmd.Stderr = cmd.Stdout // combined stream, in order
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("step %s: start %s: %w", s.ID, s.Cmd[0], err)
	}
	sc := bufio.NewScanner(stdout)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		emit(Event{Time: time.Now().UTC(), Step: s.ID, Type: "output", Message: sc.Text()})
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("step %s: %s: %w", s.ID, s.Cmd[0], err)
	}
	return nil
}

// Execute runs the plan sequentially, emitting start/done/error markers
// around each step and stopping at the first failure.
func Execute(ctx context.Context, p Plan, r Runner, emit func(Event)) error {
	if emit == nil {
		emit = func(Event) {}
	}
	var mu sync.Mutex
	safeEmit := func(e Event) { mu.Lock(); defer mu.Unlock(); emit(e) }
	for _, s := range p.Steps {
		safeEmit(Event{Time: time.Now().UTC(), Step: s.ID, Type: "start", Message: s.Title})
		if err := r.Run(ctx, s, safeEmit); err != nil {
			safeEmit(Event{Time: time.Now().UTC(), Step: s.ID, Type: "error", Message: err.Error()})
			return err
		}
		safeEmit(Event{Time: time.Now().UTC(), Step: s.ID, Type: "done"})
	}
	return nil
}
