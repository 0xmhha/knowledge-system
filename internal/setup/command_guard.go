package setup

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

type mutationLeaseKey struct{}
type commandGuardianKey struct{}

// Internal-only protocol. Caller argv comes from the trusted setup plan, never
// a public tool argument. The owner socket is mandatory; no PID polling.
type guardedCommand struct {
	SchemaVersion    int      `json:"schema_version"`
	Argv             []string `json:"argv"`
	DeadlineUnixNano int64    `json:"deadline_unix_nano"`
	MutationLease    bool     `json:"mutation_lease"`
}

func setupCommandContext(ctx context.Context, name string, args ...string) *managedCommand {
	guardian, _ := ctx.Value(commandGuardianKey{}).(string)
	lease, _ := ctx.Value(mutationLeaseKey{}).(*os.File)
	if guardian == "" {
		cmd := buildCommandContext(ctx, name, args...)
		if lease != nil {
			cmd.ExtraFiles = []*os.File{lease}
		}
		return &managedCommand{Cmd: cmd}
	}
	payload := guardedCommand{SchemaVersion: 1, Argv: append([]string{name}, args...), MutationLease: lease != nil}
	deadline, ok := ctx.Deadline()
	if !ok {
		cmd := exec.CommandContext(ctx, guardian)
		cmd.Err = fmt.Errorf("guarded command requires a deadline")
		return &managedCommand{Cmd: cmd}
	}
	payload.DeadlineUnixNano = deadline.UnixNano()
	raw, err := json.Marshal(payload)
	if err != nil || len(raw) > 64<<10 {
		cmd := exec.CommandContext(ctx, guardian)
		cmd.Err = fmt.Errorf("guarded command exceeds protocol limit")
		return &managedCommand{Cmd: cmd}
	}
	pair, err := unix.Socketpair(unix.AF_UNIX, unix.SOCK_STREAM, 0)
	if err != nil {
		cmd := exec.CommandContext(ctx, guardian)
		cmd.Err = err
		return &managedCommand{Cmd: cmd}
	}
	for _, fd := range pair {
		unix.CloseOnExec(fd)
		if err := unix.SetNonblock(fd, true); err != nil {
			_ = unix.Close(pair[0])
			_ = unix.Close(pair[1])
			cmd := exec.CommandContext(ctx, guardian)
			cmd.Err = err
			return &managedCommand{Cmd: cmd}
		}
	}
	owner := os.NewFile(uintptr(pair[0]), "dataset-owner")
	child := os.NewFile(uintptr(pair[1]), "dataset-guardian")
	cmd := exec.CommandContext(ctx, guardian, "__dataset-step", base64.RawURLEncoding.EncodeToString(raw))
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.ExtraFiles = []*os.File{child}
	if lease != nil {
		cmd.ExtraFiles = append(cmd.ExtraFiles, lease)
	}
	closeOwner := func() { _ = owner.Close() }
	stop := context.AfterFunc(ctx, closeOwner)
	go func() {
		defer closeOwner()
		var ready [1]byte
		if _, err := io.ReadFull(owner, ready[:]); err != nil || ready[0] != 1 {
			return
		}
		_, _ = io.Copy(io.Discard, owner)
	}()
	// Notify the live guardian first. If it cannot stop, WaitDelay kills only
	// the guardian; inherited mutation ownership keeps unconfirmed writers busy.
	cmd.Cancel = func() error { closeOwner(); return nil }
	cmd.WaitDelay = 5 * time.Second
	return &managedCommand{Cmd: cmd, afterStart: func() { _ = child.Close() }, cleanup: func() { stop(); closeOwner(); _ = child.Close() }}
}

// Start/Wait own the ExtraFiles lifecycle. Closing an inherited os.File from
// a readiness goroutine races os.StartProcess's final FD inspection.
type managedCommand struct {
	*exec.Cmd
	afterStart func()
	cleanup    func()
}

func (c *managedCommand) Start() error {
	err := c.Cmd.Start()
	if c.afterStart != nil {
		c.afterStart()
	}
	if err != nil && c.cleanup != nil {
		c.cleanup()
	}
	return err
}
func (c *managedCommand) Wait() error {
	err := c.Cmd.Wait()
	if c.cleanup != nil {
		c.cleanup()
	}
	return err
}
func (c *managedCommand) Run() error {
	if err := c.Start(); err != nil {
		return err
	}
	return c.Wait()
}
func (c *managedCommand) Output() ([]byte, error) {
	if c.Stdout != nil {
		return nil, fmt.Errorf("stdout already set")
	}
	var output, diagnostic bytes.Buffer
	c.Stdout = &output
	capture := c.Stderr == nil
	if capture {
		c.Stderr = &diagnostic
	}
	err := c.Run()
	if exit, ok := err.(*exec.ExitError); ok && capture {
		exit.Stderr = diagnostic.Bytes()
	}
	return output.Bytes(), err
}
func (c *managedCommand) CombinedOutput() ([]byte, error) {
	if c.Stdout != nil || c.Stderr != nil {
		return nil, fmt.Errorf("command output already set")
	}
	var output bytes.Buffer
	c.Stdout, c.Stderr = &output, &output
	err := c.Run()
	return output.Bytes(), err
}

// RunGuardedDatasetStep is called only by the hidden cks child entrypoint.
// It supervises its own newly spawned process group, never a recovered PID.
func RunGuardedDatasetStep(encoded string) error {
	raw, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(raw) > 64<<10 {
		return fmt.Errorf("invalid dataset step protocol")
	}
	var payload guardedCommand
	if err := json.Unmarshal(raw, &payload); err != nil || payload.SchemaVersion != 1 || len(payload.Argv) == 0 || payload.Argv[0] == "" || payload.DeadlineUnixNano <= 0 {
		return fmt.Errorf("invalid dataset step payload")
	}
	control := os.NewFile(uintptr(3), "dataset-owner")
	if control == nil {
		return fmt.Errorf("dataset owner channel missing")
	}
	defer func() { _ = control.Close() }()
	var stat unix.Stat_t
	if unix.Fstat(3, &stat) != nil || stat.Mode&unix.S_IFMT != unix.S_IFSOCK {
		return fmt.Errorf("dataset owner channel invalid")
	}
	ctx, cancel := context.WithDeadline(context.Background(), time.Unix(0, payload.DeadlineUnixNano))
	defer cancel()
	if _, err := control.Write([]byte{1}); err != nil {
		return err
	}
	go func() { _, _ = io.Copy(io.Discard, control); cancel() }()
	cmd := buildCommandContext(ctx, payload.Argv[0], payload.Argv[1:]...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	var lease *os.File
	if payload.MutationLease {
		lease = os.NewFile(uintptr(4), "dataset-mutation-lease")
		if lease == nil || unix.Fstat(4, &stat) != nil || stat.Mode&unix.S_IFMT != unix.S_IFREG || stat.Nlink != 1 {
			return fmt.Errorf("dataset mutation lease invalid")
		}
		defer func() { _ = lease.Close() }()
		cmd.ExtraFiles = append(cmd.ExtraFiles, lease)
	}
	quiescent, writer, err := os.Pipe()
	if err != nil {
		return err
	}
	defer func() { _ = quiescent.Close() }()
	cmd.ExtraFiles = append(cmd.ExtraFiles, writer)
	if err := cmd.Start(); err != nil {
		_ = writer.Close()
		return err
	}
	_ = writer.Close()
	closed := make(chan struct{})
	go func() { _, _ = io.Copy(io.Discard, quiescent); close(closed) }()
	err = cmd.Wait()
	// The command has exited. Stop any same-group descendants that still hold
	// its execution lease; escaped writers keep the inherited dataset lock.
	select {
	case <-closed:
	default:
		killErr := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		if killErr != nil && !errors.Is(killErr, syscall.ESRCH) {
			return killErr
		}
		<-closed
	}
	if err != nil {
		return err
	}
	return ctx.Err()
}
