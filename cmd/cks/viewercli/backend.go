package viewercli

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"syscall"
	"time"
)

// graphBackend owns exactly one started graph API process. The done close
// publishes err and ProcessState after its sole Wait call; callers never poll
// a recovered PID or kill the user's independently managed API/browser.
type graphBackend struct {
	cmd    *exec.Cmd
	cancel context.CancelFunc
	done   chan struct{}
	err    error
}

func startGraphBackend(parent context.Context, binary string, args []string, stderr io.Writer, grace time.Duration) (*graphBackend, error) {
	ctx, cancel := context.WithCancel(parent)
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Stderr = stderr
	cmd.Cancel = func() error { return cmd.Process.Signal(syscall.SIGTERM) }
	cmd.WaitDelay = grace
	if err := cmd.Start(); err != nil {
		cancel()
		return nil, err
	}
	backend := &graphBackend{cmd: cmd, cancel: cancel, done: make(chan struct{})}
	go func() { backend.err = cmd.Wait(); close(backend.done) }()
	return backend, nil
}
func (b *graphBackend) stop() { b.cancel(); <-b.done }
func (b *graphBackend) exited() error {
	if b == nil {
		return nil
	}
	select {
	case <-b.done:
		if b.err != nil {
			return fmt.Errorf("graph API exited before readiness: %w", b.err)
		}
		return fmt.Errorf("graph API exited before readiness")
	default:
		return nil
	}
}

func (b *graphBackend) stopError() error {
	b.stop()
	if b.cmd.ProcessState == nil || !b.cmd.ProcessState.Success() {
		return fmt.Errorf("graph API shutdown failed: %v", b.err)
	}
	return nil
}
