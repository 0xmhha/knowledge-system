package service

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// launchctlPath is the absolute path to the service manager CLI; a job that
// installs launchd agents cannot rely on inheriting a PATH.
const launchctlPath = "/bin/launchctl"

// runnerTimeout bounds every launchctl call. launchctl can block indefinitely
// against a wedged domain, and an operator waiting on `service status` deserves
// an error instead of a hang.
const runnerTimeout = 15 * time.Second

// Runner executes an external command and returns its combined output. It is
// the single seam between this package and the host, so tests substitute a
// recorder and never touch launchd.
type Runner interface {
	Run(ctx context.Context, name string, args ...string) (string, error)
}

// ExecRunner is the production Runner: it actually runs the command.
type ExecRunner struct{}

// Run executes name with args and returns its combined output. The returned
// error wraps the exit status; output is returned either way, because
// launchctl's diagnosis is on stdout even when it exits non-zero.
func (ExecRunner) Run(ctx context.Context, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, runnerTimeout)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("run %s %s: %w", name, strings.Join(args, " "), err)
	}
	return string(out), nil
}

// Controller drives launchctl within one user's GUI domain (gui/<uid>), the
// domain a login session's agents live in.
type Controller struct {
	Runner Runner
	UID    int
	// Sleep paces the retries in Replace. Defaults to time.Sleep; tests
	// substitute a no-op so the wait is not real.
	Sleep func(time.Duration)
	// UnloadTimeout bounds how long Replace waits for a teardown. Zero picks
	// unloadTimeout.
	UnloadTimeout time.Duration
}

const (
	// unloadTimeout bounds the wait for launchd to finish tearing a label down.
	unloadTimeout = 10 * time.Second
	// unloadInterval is how often that teardown is re-checked.
	unloadInterval = 250 * time.Millisecond
	// bootstrapAttempts is how many times a load is retried. launchd can still
	// answer "Input/output error" for a moment after the label has stopped
	// reporting as loaded.
	bootstrapAttempts = 5
	// bootstrapRetryDelay spaces those attempts.
	bootstrapRetryDelay = time.Second
)

// sleep waits, through the injected seam when there is one.
func (c Controller) sleep(d time.Duration) {
	if c.Sleep != nil {
		c.Sleep(d)
		return
	}
	time.Sleep(d)
}

// Replace loads the definition at plistPath as label, replacing any job already
// running under it — the operation "install, and reinstall over an existing
// install" needs.
//
// The waiting is the substance. bootout returns before launchd has finished
// tearing the job down, and bootstrapping into a half-removed label fails with
// a bare "Bootstrap failed: 5: Input/output error" — which, done blindly during
// a reinstall, leaves the service unloaded and the server down.
func (c Controller) Replace(ctx context.Context, label, plistPath string) error {
	if err := c.Bootout(ctx, label); err != nil {
		return err
	}
	if err := c.waitUnloaded(ctx, label); err != nil {
		return err
	}
	var err error
	for attempt := 0; attempt < bootstrapAttempts; attempt++ {
		if attempt > 0 {
			c.sleep(bootstrapRetryDelay)
		}
		if err = c.Bootstrap(ctx, plistPath); err == nil {
			return nil
		}
	}
	return fmt.Errorf("load %s after %d attempts: %w", label, bootstrapAttempts, err)
}

// waitUnloaded blocks until label is gone from the domain, or reports what is
// still holding it after unloadTimeout.
func (c Controller) waitUnloaded(ctx context.Context, label string) error {
	timeout := c.UnloadTimeout
	if timeout <= 0 {
		timeout = unloadTimeout
	}
	deadline := time.Now().Add(timeout)
	for {
		loaded, err := c.Loaded(ctx, label)
		if err != nil {
			return err
		}
		if !loaded {
			return nil
		}
		if ctx.Err() != nil || !time.Now().Before(deadline) {
			return fmt.Errorf("%s is still loaded %s after bootout", label, timeout)
		}
		c.sleep(unloadInterval)
	}
}

// Domain is the launchctl domain target for this controller's user.
func (c Controller) Domain() string { return "gui/" + strconv.Itoa(c.UID) }

// target is the service target (domain plus label) launchctl verbs address.
func (c Controller) target(label string) string { return c.Domain() + "/" + label }

// Bootstrap loads the agent defined at plistPath into the user domain. An
// already-loaded label is reported as an error by launchctl; callers that want
// idempotence call Bootout first.
func (c Controller) Bootstrap(ctx context.Context, plistPath string) error {
	out, err := c.Runner.Run(ctx, launchctlPath, "bootstrap", c.Domain(), plistPath)
	if err != nil {
		return fmt.Errorf("bootstrap %s: %w: %s", plistPath, err, strings.TrimSpace(out))
	}
	return nil
}

// Bootout unloads a label. Unloading a label that is not loaded is treated as
// success, so uninstall and reinstall are both idempotent.
func (c Controller) Bootout(ctx context.Context, label string) error {
	out, err := c.Runner.Run(ctx, launchctlPath, "bootout", c.target(label))
	if err == nil || isNotLoaded(out) {
		return nil
	}
	return fmt.Errorf("bootout %s: %w: %s", label, err, strings.TrimSpace(out))
}

// Kickstart restarts a loaded job (-k kills the running instance first). This
// is how recovery restarts the server: launchd owns the process, so signalling
// the pid directly would race with the KeepAlive respawn.
func (c Controller) Kickstart(ctx context.Context, label string) error {
	out, err := c.Runner.Run(ctx, launchctlPath, "kickstart", "-k", c.target(label))
	if err != nil {
		return fmt.Errorf("kickstart %s: %w: %s", label, err, strings.TrimSpace(out))
	}
	return nil
}

// Loaded reports whether a label is currently loaded in the user domain. A
// label launchctl does not know is (false, nil) — that is an answer, not a
// failure; anything else is returned as an error rather than read as "absent".
func (c Controller) Loaded(ctx context.Context, label string) (bool, error) {
	out, err := c.Runner.Run(ctx, launchctlPath, "print", c.target(label))
	if err == nil {
		return true, nil
	}
	if isNotLoaded(out) {
		return false, nil
	}
	return false, fmt.Errorf("print %s: %w: %s", label, err, strings.TrimSpace(out))
}

// isNotLoaded reports whether launchctl output means "this label is not loaded"
// as opposed to a real failure. The wording varies by macOS release, so all
// known spellings are matched.
func isNotLoaded(out string) bool {
	markers := []string{
		"could not find service",
		"no such process",
		"service not found",
	}
	low := strings.ToLower(out)
	for _, m := range markers {
		if strings.Contains(low, m) {
			return true
		}
	}
	return false
}
