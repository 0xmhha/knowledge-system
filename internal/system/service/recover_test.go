package service

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// probeScript answers a fixed sequence of health probes, then repeats its last
// answer — enough to model "down, then up after the restart".
type probeScript struct {
	answers []bool
	calls   int
}

func (p *probeScript) probe(string) bool {
	i := p.calls
	p.calls++
	if i >= len(p.answers) {
		i = len(p.answers) - 1
	}
	return p.answers[i]
}

func TestRecover(t *testing.T) {
	cases := []struct {
		name        string
		answers     []bool
		force       bool
		restartErr  error
		wantOutcome Outcome
		wantRestart bool
		wantErr     bool
	}{
		{
			name:        "a serving instance is left alone",
			answers:     []bool{true},
			wantOutcome: OutcomeHealthy,
		},
		{
			name:        "force restarts a serving instance",
			answers:     []bool{true, true},
			force:       true,
			wantOutcome: OutcomeRecovered,
			wantRestart: true,
		},
		{
			name:        "a wedged instance is restarted and comes back",
			answers:     []bool{false, false, true},
			wantOutcome: OutcomeRecovered,
			wantRestart: true,
		},
		{
			name:        "an instance that never comes back fails loudly",
			answers:     []bool{false},
			wantOutcome: OutcomeFailed,
			wantRestart: true,
			wantErr:     true,
		},
		{
			name:        "a restart that cannot be issued fails loudly",
			answers:     []bool{false},
			restartErr:  errors.New("launchctl: no such service"),
			wantOutcome: OutcomeFailed,
			wantRestart: true,
			wantErr:     true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			script := &probeScript{answers: tc.answers}
			restarted := false
			r := Recoverer{
				Probe: script.probe,
				Restart: func(context.Context) error {
					restarted = true
					return tc.restartErr
				},
				Timeout:  30 * time.Millisecond,
				Interval: 10 * time.Millisecond,
				Sleep:    func(time.Duration) {}, // no real waiting in tests
			}

			rep, err := r.Recover(context.Background(), "inst", "127.0.0.1:8930", tc.force)

			if rep.Outcome != tc.wantOutcome {
				t.Errorf("outcome = %q, want %q (detail: %s)", rep.Outcome, tc.wantOutcome, rep.Detail)
			}
			if restarted != tc.wantRestart {
				t.Errorf("restarted = %v, want %v", restarted, tc.wantRestart)
			}
			if (err != nil) != tc.wantErr {
				t.Errorf("err = %v, want error: %v", err, tc.wantErr)
			}
			if rep.Instance != "inst" || rep.Addr != "127.0.0.1:8930" {
				t.Errorf("report lost its identity: %+v", rep)
			}
		})
	}
}

func TestRecoverStopsWaitingWhenCancelled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	r := Recoverer{
		Probe:    func(string) bool { return false },
		Restart:  func(context.Context) error { cancel(); return nil },
		Timeout:  time.Hour, // would hang if cancellation were ignored
		Interval: time.Millisecond,
		Sleep:    func(time.Duration) {},
	}
	rep, err := r.Recover(ctx, "inst", "127.0.0.1:8930", false)
	if err == nil {
		t.Fatal("want an error after cancellation")
	}
	if rep.Outcome != OutcomeFailed {
		t.Errorf("outcome = %q, want %q", rep.Outcome, OutcomeFailed)
	}
}

func TestRecoverNeedsItsSeams(t *testing.T) {
	if _, err := (Recoverer{}).Recover(context.Background(), "inst", "addr", false); err == nil {
		t.Fatal("a Recoverer with no Probe/Restart must refuse rather than nil-panic")
	}
}

// TestRecoverTriesTheDependencyBeforeTheInstance covers the rung that exists
// because the failure it addresses cannot be fixed by what recovery normally
// does. A dependency the instance needs but does not own — the embedding
// daemon — takes /healthz to 503 while the server is fine, and a restart there
// is downtime with no recovery at the end of it.
//
// Each case states the causality directly (what makes the instance serve
// again) rather than a sequence of answers, because the distinction under test
// is exactly which action was the cure.
func TestRecoverTriesTheDependencyBeforeTheInstance(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		// depUpFrom is the dependency's state before Start is called.
		depUpFrom bool
		// healthy says what has to be true for the instance to serve.
		healthy      func(depUp, restarted bool) bool
		wantRestart  bool
		wantStarted  bool
		wantInDetail string
	}{
		{
			name:        "dependency was the fault: restoring it recovers, the server is left alone",
			depUpFrom:   false,
			healthy:     func(depUp, _ bool) bool { return depUp },
			wantStarted: true, wantRestart: false,
			wantInDetail: "the embedding daemon",
		},
		{
			name:        "dependency is up, so it is not the fault: restart as before",
			depUpFrom:   true,
			healthy:     func(_, restarted bool) bool { return restarted },
			wantStarted: false, wantRestart: true,
		},
		{
			// Restoring it was necessary but not sufficient — something else is
			// wrong, and that is what the restart is for.
			name:        "dependency comes back but the instance does not: fall through",
			depUpFrom:   false,
			healthy:     func(_, restarted bool) bool { return restarted },
			wantStarted: true, wantRestart: true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			depUp, started, restarted := tc.depUpFrom, false, false
			r := Recoverer{
				Probe:   func(string) bool { return tc.healthy(depUp, restarted) },
				Restart: func(context.Context) error { restarted = true; return nil },
				Dependency: &Dependency{
					Name:  "the embedding daemon",
					Probe: func(context.Context) bool { return depUp },
					Start: func(context.Context) error { started, depUp = true, true; return nil },
				},
				Timeout: time.Second,
				Sleep:   func(time.Duration) {},
			}
			rep, err := r.Recover(context.Background(), "inst", "127.0.0.1:1", false)
			if err != nil {
				t.Fatalf("Recover: %v (%+v)", err, rep)
			}
			if rep.Outcome != OutcomeRecovered {
				t.Errorf("Outcome = %q, want %q (%v)", rep.Outcome, OutcomeRecovered, rep.Actions)
			}
			if restarted != tc.wantRestart {
				t.Errorf("restarted = %v, want %v (%v)", restarted, tc.wantRestart, rep.Actions)
			}
			if started != tc.wantStarted {
				t.Errorf("dependency started = %v, want %v", started, tc.wantStarted)
			}
			if tc.wantInDetail != "" && !strings.Contains(rep.Detail, tc.wantInDetail) {
				t.Errorf("Detail = %q, want it to name %q", rep.Detail, tc.wantInDetail)
			}
		})
	}
}

// TestRecoverCooldownSuppressesARestartStorm pins the rate limit. A watchdog
// runs as a fresh process every tick, so the last restart has to be read from
// disk or there is no cooldown at all.
func TestRecoverCooldownSuppressesARestartStorm(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		name        string
		lastRestart time.Time
		force       bool
		wantOutcome Outcome
		wantRestart bool
	}{
		{name: "restarted a minute ago: suppressed", lastRestart: now.Add(-time.Minute),
			wantOutcome: OutcomeSuppressed, wantRestart: false},
		{name: "cooldown elapsed: restart", lastRestart: now.Add(-10 * time.Minute),
			wantOutcome: OutcomeRecovered, wantRestart: true},
		{name: "force ignores the cooldown", lastRestart: now.Add(-time.Minute), force: true,
			wantOutcome: OutcomeRecovered, wantRestart: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), recoverStateFile)
			if err := os.WriteFile(path,
				[]byte(`{"last_restart":"`+tc.lastRestart.Format(time.RFC3339Nano)+`"}`), 0o644); err != nil {
				t.Fatal(err)
			}
			restarted := false
			hp := &probeScript{answers: []bool{false, true}}
			r := Recoverer{
				Probe:     hp.probe,
				Restart:   func(context.Context) error { restarted = true; return nil },
				StatePath: path,
				Now:       func() time.Time { return now },
				Sleep:     func(time.Duration) {},
			}
			rep, err := r.Recover(context.Background(), "inst", "127.0.0.1:1", tc.force)
			if (err != nil) != (tc.wantOutcome == OutcomeSuppressed) {
				t.Fatalf("err = %v, outcome %q", err, rep.Outcome)
			}
			if rep.Outcome != tc.wantOutcome {
				t.Errorf("Outcome = %q, want %q", rep.Outcome, tc.wantOutcome)
			}
			if restarted != tc.wantRestart {
				t.Errorf("restarted = %v, want %v", restarted, tc.wantRestart)
			}
		})
	}
}

// TestRecoverRecordsTheRestartForTheNextProcess closes the loop the cooldown
// depends on: without the stamp, every tick reads an empty state and restarts.
func TestRecoverRecordsTheRestartForTheNextProcess(t *testing.T) {
	t.Parallel()
	now := time.Date(2026, 8, 14, 12, 0, 0, 0, time.UTC)
	path := filepath.Join(t.TempDir(), recoverStateFile)
	hp := &probeScript{answers: []bool{false, true}}
	r := Recoverer{
		Probe:     hp.probe,
		Restart:   func(context.Context) error { return nil },
		StatePath: path,
		Now:       func() time.Time { return now },
		Sleep:     func(time.Duration) {},
	}
	if _, err := r.Recover(context.Background(), "inst", "127.0.0.1:1", false); err != nil {
		t.Fatal(err)
	}
	got, ok := r.loadLastRestart()
	if !ok || !got.Equal(now) {
		t.Fatalf("persisted last restart = %v (ok=%v), want %v", got, ok, now)
	}
}
