package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// defaultRecoverTimeout bounds how long recovery waits for a restarted
	// instance to serve. Loading the dataset dominates it.
	defaultRecoverTimeout = 90 * time.Second
	// defaultRecoverInterval is the health poll period while waiting.
	defaultRecoverInterval = 2 * time.Second
	// defaultCooldown is the minimum gap between restarts. An instance that
	// cannot come back is restarted on a slow clock instead of every tick, so
	// the log reads as a standing failure rather than a stream of attempts.
	defaultCooldown = 5 * time.Minute
	// defaultDependencyWait bounds the wait for a restored dependency.
	defaultDependencyWait = 60 * time.Second
	// recoverStateFile carries the last restart across watchdog processes,
	// under the deployment's run dir. Each tick is a fresh process, so a
	// cooldown that lived in memory would be no cooldown at all.
	recoverStateFile = "recover-state.json"
)

// Outcome is what one recovery attempt concluded.
type Outcome string

const (
	// OutcomeHealthy means the instance was already serving and was left alone.
	OutcomeHealthy Outcome = "healthy"
	// OutcomeRecovered means the instance was restarted and came back serving.
	OutcomeRecovered Outcome = "recovered"
	// OutcomeFailed means the instance was restarted and did not come back
	// within the timeout. The operator has to look at the log.
	OutcomeFailed Outcome = "failed"
	// OutcomeSuppressed means the instance is not serving and was left alone
	// because it was restarted too recently. It is reported as a failure —
	// nothing is serving — but named apart from OutcomeFailed, because "we
	// chose not to try" and "we tried and it did not work" call for different
	// next moves.
	OutcomeSuppressed Outcome = "suppressed"
)

// Dependency is a process the instance needs but does not own, and that a
// restart of the instance therefore cannot fix.
//
// The one this exists for is the embedding daemon. serviceable() requires the
// model to be reachable, so a daemon that is down makes /healthz report 503
// while the server itself is fine — and on a laptop that daemon does not
// survive a sleep. Restarting the server there is pure downtime: it comes back
// and reports 503 again. Restoring the dependency is what recovers the
// instance, and the server needs no bounce afterwards because it reconnects on
// the next call.
type Dependency struct {
	// Name appears in the report, so an operator reading a log knows what was
	// restarted without consulting the code.
	Name string
	// Probe reports whether the dependency is answering.
	Probe func(ctx context.Context) bool
	// Start brings it back. It should return once the start was requested;
	// waiting for it to answer is Probe's job.
	Start func(ctx context.Context) error
	// Wait bounds the poll for it to answer after Start (default 60s).
	Wait time.Duration
}

// Report is one recovery attempt's result. Actions lists what was actually
// done, in order, so a watchdog log or an SSH caller can see whether a restart
// happened at all.
type Report struct {
	Instance string   `json:"instance"`
	Addr     string   `json:"addr"`
	Outcome  Outcome  `json:"outcome"`
	Actions  []string `json:"actions,omitempty"`
	Detail   string   `json:"detail,omitempty"`
}

// Recoverer restarts an instance that is not serving and confirms it came back.
// Every dependency is injected: Probe answers "is it serving", Restart puts a
// fresh process there, and Sleep is the clock, so the whole decision is
// exercisable without launchd or a network.
type Recoverer struct {
	// Probe reports whether the instance at addr answers /healthz.
	Probe func(addr string) bool
	// Restart replaces the running instance (launchctl kickstart -k).
	Restart func(ctx context.Context) error
	// Timeout bounds the wait for a restarted instance (default 90s);
	// Interval is the poll period (default 2s).
	Timeout  time.Duration
	Interval time.Duration
	// Dependency, when set, is tried before the instance is touched: a
	// dependency that is down is restored and health re-probed, and the
	// instance is bounced only if that was not the fault. Nil skips the rung.
	Dependency *Dependency
	// Cooldown is the minimum gap between restarts (default 5m). It needs
	// StatePath to mean anything across watchdog processes.
	Cooldown time.Duration
	// StatePath persists the last restart. Empty disables persistence, which
	// disables the cooldown with it — say so rather than pretending to rate
	// limit across processes that share no memory.
	StatePath string
	// Now and Sleep default to time.Now and time.Sleep.
	Now   func() time.Time
	Sleep func(time.Duration)
}

// Recover probes addr and, when it is not serving (or force is set), restarts
// the instance and waits for it to serve again. A healthy instance with force
// unset is left untouched — this is the routine a watchdog runs on a timer, so
// doing nothing has to be the normal case.
//
// The returned error is non-nil when recovery did not end with a serving
// instance, so a caller can exit non-zero on it; the Report is always usable.
func (r Recoverer) Recover(ctx context.Context, instance, addr string, force bool) (Report, error) {
	if r.Probe == nil || r.Restart == nil {
		return Report{}, fmt.Errorf("service: recoverer needs both Probe and Restart")
	}
	rep := Report{Instance: instance, Addr: addr}

	if r.Probe(addr) && !force {
		rep.Outcome = OutcomeHealthy
		rep.Detail = "instance is serving; no action taken"
		return rep, nil
	}
	if force {
		rep.Actions = append(rep.Actions, "restart requested with --force")
	} else {
		rep.Actions = append(rep.Actions, "health probe failed at "+addr)
	}

	// Dependency first, and only when the operator did not ask for a bounce:
	// --force means "restart the instance", not "diagnose it".
	if !force {
		if done, err := r.tryDependency(ctx, addr, &rep); done {
			return rep, err
		}
	}

	// A restart too soon after the last one buys nothing but downtime.
	if !force {
		if since, blocked := r.inCooldown(); blocked {
			rep.Outcome = OutcomeSuppressed
			rep.Detail = fmt.Sprintf("not serving, and the last restart was %s ago (cooldown %s)",
				since.Round(time.Second), r.cooldown())
			return rep, fmt.Errorf("recover %s: %s", instance, rep.Detail)
		}
	}

	if err := r.Restart(ctx); err != nil {
		rep.Outcome = OutcomeFailed
		rep.Detail = err.Error()
		return rep, fmt.Errorf("recover %s: %w", instance, err)
	}
	rep.Actions = append(rep.Actions, "restarted the launchd job")
	r.recordRestart(&rep)

	timeout, interval := r.Timeout, r.Interval
	if timeout <= 0 {
		timeout = defaultRecoverTimeout
	}
	if interval <= 0 {
		interval = defaultRecoverInterval
	}
	if !r.waitServing(ctx, addr, timeout, interval) {
		rep.Outcome = OutcomeFailed
		rep.Detail = fmt.Sprintf("still not serving %s after %s", addr, timeout)
		return rep, fmt.Errorf("recover %s: %s", instance, rep.Detail)
	}
	rep.Outcome = OutcomeRecovered
	rep.Detail = "instance is serving again"
	return rep, nil
}

// tryDependency walks the dependency rung. It reports done=true when the
// attempt settled the whole recovery — the dependency was down and restoring
// it brought the instance back — so the caller returns without a restart the
// instance does not need.
//
// A dependency that is already answering is not the fault, and a dependency
// that comes back without the instance following is recorded and fallen
// through: something else is wrong, and that is what the restart is for.
func (r Recoverer) tryDependency(ctx context.Context, addr string, rep *Report) (bool, error) {
	d := r.Dependency
	if d == nil || d.Probe == nil || d.Start == nil {
		return false, nil
	}
	if d.Probe(ctx) {
		return false, nil
	}
	rep.Actions = append(rep.Actions, d.Name+" is down")
	if err := d.Start(ctx); err != nil {
		rep.Actions = append(rep.Actions, "could not start "+d.Name+": "+err.Error())
		return false, nil
	}
	wait := d.Wait
	if wait <= 0 {
		wait = defaultDependencyWait
	}
	if !r.waitFor(ctx, func() bool { return d.Probe(ctx) }, wait) {
		rep.Actions = append(rep.Actions, d.Name+" did not come back")
		return false, nil
	}
	rep.Actions = append(rep.Actions, "started "+d.Name)

	timeout := r.Timeout
	if timeout <= 0 {
		timeout = defaultRecoverTimeout
	}
	if !r.waitFor(ctx, func() bool { return r.Probe(addr) }, timeout) {
		rep.Actions = append(rep.Actions, "instance still not serving; falling through to a restart")
		return false, nil
	}
	rep.Outcome = OutcomeRecovered
	rep.Detail = "recovered by restoring " + d.Name + "; the instance was not restarted"
	return true, nil
}

// cooldown is the configured minimum gap between restarts, or the default.
func (r Recoverer) cooldown() time.Duration {
	if r.Cooldown > 0 {
		return r.Cooldown
	}
	return defaultCooldown
}

// inCooldown reports how long ago the last restart was and whether that is
// too recent to try another. With no persisted state there is nothing to
// compare against, so nothing is suppressed.
func (r Recoverer) inCooldown() (time.Duration, bool) {
	last, ok := r.loadLastRestart()
	if !ok {
		return 0, false
	}
	since := r.now().Sub(last)
	return since, since < r.cooldown()
}

// recoverStateFileContents is what survives between watchdog processes.
type recoverStateFileContents struct {
	LastRestart time.Time `json:"last_restart"`
}

// RecoverStatePath is where the cooldown state belongs for a run dir.
func RecoverStatePath(runDir string) string {
	return filepath.Join(runDir, recoverStateFile)
}

func (r Recoverer) loadLastRestart() (time.Time, bool) {
	if r.StatePath == "" {
		return time.Time{}, false
	}
	b, err := os.ReadFile(r.StatePath)
	if err != nil {
		return time.Time{}, false
	}
	var st recoverStateFileContents
	if err := json.Unmarshal(b, &st); err != nil || st.LastRestart.IsZero() {
		return time.Time{}, false
	}
	return st.LastRestart, true
}

// recordRestart stamps the restart for the next process to read. A failure to
// persist is reported in the action list rather than aborting: the restart
// already happened, and losing the stamp costs a cooldown, not the recovery.
func (r Recoverer) recordRestart(rep *Report) {
	if r.StatePath == "" {
		return
	}
	b, err := json.Marshal(recoverStateFileContents{LastRestart: r.now()})
	if err == nil {
		err = os.WriteFile(r.StatePath, b, 0o644)
	}
	if err != nil {
		rep.Actions = append(rep.Actions, "could not record the restart time: "+err.Error())
	}
}

func (r Recoverer) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}

// waitFor polls until cond holds, the deadline passes, or ctx is cancelled.
func (r Recoverer) waitFor(ctx context.Context, cond func() bool, timeout time.Duration) bool {
	sleep := r.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}
	interval := r.Interval
	if interval <= 0 {
		interval = defaultRecoverInterval
	}
	deadline := r.now().Add(timeout)
	for {
		if cond() {
			return true
		}
		if ctx.Err() != nil || !r.now().Before(deadline) {
			return false
		}
		sleep(interval)
	}
}

// waitServing polls until the instance serves, the deadline passes, or the
// context is cancelled.
func (r Recoverer) waitServing(ctx context.Context, addr string, timeout, interval time.Duration) bool {
	sleep := r.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}
	deadline := time.Now().Add(timeout)
	for {
		if r.Probe(addr) {
			return true
		}
		if ctx.Err() != nil || !time.Now().Before(deadline) {
			return false
		}
		sleep(interval)
	}
}
