package daemon

import (
	"os"
	"os/exec"
	"testing"
	"time"
)

// TestStartDetectsStartupCrash covers the reap-based crash check: a child that
// exits during the grace window (bad config, port already bound) must surface as
// a startup error, not a false "running" — a signal-0 probe can't tell a live
// process from an exited-but-unreaped zombie.
func TestStartDetectsStartupCrash(t *testing.T) {
	falsebin, err := exec.LookPath("false")
	if err != nil {
		t.Skip("false not available")
	}
	s := &Supervisor{
		RunDir:     t.TempDir(),
		Binary:     falsebin, // exits 1 immediately
		Args:       func(name, config, addr string) []string { return nil },
		StartGrace: 150 * time.Millisecond,
	}
	inst, err := s.Start("crash", "cfg", "127.0.0.1:9999")
	if err == nil || inst.Running {
		t.Fatalf("expected a startup-crash error, got inst=%+v err=%v", inst, err)
	}
	if _, statErr := os.Stat(s.pidfile("crash")); !os.IsNotExist(statErr) {
		t.Errorf("pidfile should be removed after a startup crash")
	}
}

// newTestSupervisor supervises a real but throwaway process (`sleep`) so the
// pidfile/start/stop/status logic is exercised without a system-mcp instance.
func newTestSupervisor(t *testing.T) *Supervisor {
	t.Helper()
	sleep, err := exec.LookPath("sleep")
	if err != nil {
		t.Skip("sleep not available")
	}
	return &Supervisor{
		RunDir:     t.TempDir(),
		Binary:     sleep,
		Args:       func(name, config, addr string) []string { return []string{"30"} },
		StartGrace: 200 * time.Millisecond,
	}
}

func TestSupervisorLifecycle(t *testing.T) {
	s := newTestSupervisor(t)
	t.Cleanup(func() { _ = s.Stop("a") })

	inst, err := s.Start("a", "cfg", "127.0.0.1:9999")
	if err != nil || !inst.Running || inst.PID <= 0 {
		t.Fatalf("start: %+v err=%v", inst, err)
	}
	if st := s.Status("a"); !st.Running || st.PID != inst.PID {
		t.Errorf("status after start: %+v, want running pid %d", st, inst.PID)
	}
	// idempotent: a second start returns the same running instance.
	again, err := s.Start("a", "cfg", "")
	if err != nil || again.PID != inst.PID {
		t.Errorf("second start should be a no-op returning pid %d, got %+v err=%v", inst.PID, again, err)
	}
	// list sees it.
	list, err := s.List()
	if err != nil || len(list) != 1 || list[0].Name != "a" || !list[0].Running {
		t.Errorf("list: %+v err=%v", list, err)
	}

	if err := s.Stop("a"); err != nil {
		t.Fatalf("stop: %v", err)
	}
	if st := s.Status("a"); st.Running {
		t.Errorf("status after stop: still running (%+v)", st)
	}
	// pidfile removed → list is empty.
	if list, _ := s.List(); len(list) != 0 {
		t.Errorf("list after stop: %+v, want empty", list)
	}
	// stopping a stopped instance is a no-op success.
	if err := s.Stop("a"); err != nil {
		t.Errorf("stop of stopped instance: %v", err)
	}
}

func TestSupervisorRestart(t *testing.T) {
	s := newTestSupervisor(t)
	t.Cleanup(func() { _ = s.Stop("b") })
	first, err := s.Start("b", "cfg", "")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	second, err := s.Restart("b", "cfg", "")
	if err != nil || !second.Running {
		t.Fatalf("restart: %+v err=%v", second, err)
	}
	if second.PID == first.PID {
		t.Errorf("restart should spawn a new pid; got %d twice", first.PID)
	}
}
