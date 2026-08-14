package service

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

// linkFixture drives a watcher whose host addresses and clock the test moves,
// and whose restart is counted rather than performed.
type linkFixture struct {
	addrs      []string
	clock      time.Time
	restarts   int
	restartErr error
	w          LinkWatcher
}

func newLinkFixture(t *testing.T, grace time.Duration) *linkFixture {
	t.Helper()
	f := &linkFixture{clock: time.Date(2026, 8, 12, 9, 0, 0, 0, time.UTC)}
	f.w = LinkWatcher{
		Instance:        "example-project",
		Port:            "8930",
		Addrs:           func() ([]string, error) { return f.addrs, nil },
		StatePath:       filepath.Join(t.TempDir(), linkStateFile),
		DisconnectGrace: grace,
		Now:             func() time.Time { return f.clock },
		Recoverer: Recoverer{
			Probe:   func(string) bool { return true },
			Restart: func(context.Context) error { f.restarts++; return f.restartErr },
			Sleep:   func(time.Duration) {},
		},
	}
	return f
}

func (f *linkFixture) check(t *testing.T) *LinkChange {
	t.Helper()
	c, err := f.w.Check(context.Background())
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	return c
}

func (f *linkFixture) advance(d time.Duration) { f.clock = f.clock.Add(d) }

// TestLinkWatcher_BlipOnTheSameAddressCostsNothing is the case the watcher
// exists to get right. Wireless drops and returns on its own constantly, and
// almost always at the same address — nothing was ever stale, so a restart
// would be pure downtime. Tracking the address last PUBLISHED rather than the
// address last SEEN is what makes the outage invisible.
func TestLinkWatcher_BlipOnTheSameAddressCostsNothing(t *testing.T) {
	t.Parallel()
	f := newLinkFixture(t, 2*time.Minute)

	f.addrs = []string{"127.0.0.1/8", "192.168.10.5/24"}
	if c := f.check(t); c == nil || c.Current != "192.168.10.5" {
		t.Fatalf("first observation = %+v, want the current address published", c)
	}
	if f.restarts != 0 {
		t.Errorf("first observation restarted %d times; nothing was pinned yet", f.restarts)
	}
	before := f.restarts

	// The link drops and comes back at the same address, well inside grace.
	f.addrs = []string{"127.0.0.1/8"}
	if c := f.check(t); c != nil {
		t.Errorf("a fresh drop should stay quiet, got %+v", c)
	}
	f.advance(20 * time.Second)
	if c := f.check(t); c != nil {
		t.Errorf("a short outage should stay quiet, got %+v", c)
	}
	f.addrs = []string{"127.0.0.1/8", "192.168.10.5/24"}
	if c := f.check(t); c != nil {
		t.Errorf("returning to the same address should stay quiet, got %+v", c)
	}
	if f.restarts != before {
		t.Errorf("a blip restarted the instance (%d -> %d)", before, f.restarts)
	}
}

// A long outage that still ends on the same address must also cost no
// restart — duration is not what makes an address stale, a different address
// is. The outage is reported, and so is its end, because someone was told.
func TestLinkWatcher_LongOutageEndingOnTheSameAddress(t *testing.T) {
	t.Parallel()
	f := newLinkFixture(t, 2*time.Minute)
	f.addrs = []string{"192.168.10.5/24"}
	f.check(t)
	before := f.restarts

	f.addrs = nil
	f.check(t) // start the clock on the outage
	f.advance(10 * time.Minute)

	c := f.check(t)
	if c == nil || c.State != LinkDisconnected {
		t.Fatalf("a sustained outage must be reported, got %+v", c)
	}
	if c.Published != "192.168.10.5" {
		t.Errorf("the published address must survive the outage, got %q", c.Published)
	}
	if c.DownFor != "10m0s" {
		t.Errorf("DownFor = %q", c.DownFor)
	}
	// Reported once, not on every tick.
	f.advance(time.Minute)
	if again := f.check(t); again != nil {
		t.Errorf("the outage was reported twice: %+v", again)
	}

	f.addrs = []string{"192.168.10.5/24"}
	back := f.check(t)
	if back == nil || back.State != LinkConnected {
		t.Fatalf("the end of a reported outage should be reported, got %+v", back)
	}
	if f.restarts != before {
		t.Errorf("a 10-minute outage on an unchanged address restarted the instance")
	}
}

// The move is the only thing that invalidates what clients hold, so it is the
// only thing that restarts and republishes.
func TestLinkWatcher_MovedAddressRestartsAndRepublishes(t *testing.T) {
	t.Parallel()
	f := newLinkFixture(t, 2*time.Minute)
	f.addrs = []string{"192.168.10.5/24"}
	f.check(t)
	before := f.restarts

	f.addrs = []string{"192.168.1.42/24"}
	c := f.check(t)
	if c == nil || c.State != LinkMoved {
		t.Fatalf("move = %+v, want LinkMoved", c)
	}
	if c.Published != "192.168.10.5" || c.Current != "192.168.1.42" {
		t.Errorf("move endpoints = %q -> %q", c.Published, c.Current)
	}
	if c.URL != "http://192.168.1.42:8930/mcp" {
		t.Errorf("republished URL = %q", c.URL)
	}
	if f.restarts != before+1 {
		t.Errorf("restarts = %d, want %d", f.restarts, before+1)
	}
	// Settled: no further noise, no further restarts.
	if again := f.check(t); again != nil {
		t.Errorf("steady state after a move should be quiet, got %+v", again)
	}
	if f.restarts != before+1 {
		t.Errorf("steady state restarted again")
	}
}

// Moving while disconnected — drop on one network, come back on another — is
// still a move, and the comparison is against what clients hold, not against
// the outage.
func TestLinkWatcher_OutageEndingOnADifferentAddressIsAMove(t *testing.T) {
	t.Parallel()
	f := newLinkFixture(t, 2*time.Minute)
	f.addrs = []string{"192.168.10.5/24"}
	f.check(t)
	before := f.restarts

	f.addrs = nil
	f.check(t)
	f.advance(5 * time.Minute)
	f.check(t) // reports the outage

	f.addrs = []string{"10.0.0.9/8"}
	c := f.check(t)
	if c == nil || c.State != LinkMoved || c.Current != "10.0.0.9" {
		t.Fatalf("return on a new network = %+v, want a move", c)
	}
	if f.restarts != before+1 {
		t.Errorf("restarts = %d, want %d", f.restarts, before+1)
	}
}

// A watcher restarted by launchd, or resumed after sleep, must still measure
// against what clients hold. Without the persisted address it would adopt
// whatever it finds and never republish — precisely the case a move produces.
func TestLinkWatcher_PublishedAddressSurvivesWatcherRestart(t *testing.T) {
	t.Parallel()
	state := filepath.Join(t.TempDir(), linkStateFile)
	build := func(addrs []string, restarts *int) LinkWatcher {
		return LinkWatcher{
			Instance: "example-project", Port: "8930",
			Addrs:     func() ([]string, error) { return addrs, nil },
			StatePath: state,
			Now:       time.Now,
			Recoverer: Recoverer{
				Probe:   func(string) bool { return true },
				Restart: func(context.Context) error { *restarts++; return nil },
				Sleep:   func(time.Duration) {},
			},
		}
	}
	ctx := context.Background()
	var first, second int
	if _, err := build([]string{"192.168.10.5/24"}, &first).Check(ctx); err != nil {
		t.Fatal(err)
	}

	// Fresh process; the host moved while nothing was watching.
	c, err := build([]string{"192.168.1.42/24"}, &second).Check(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if c == nil || c.Published != "192.168.10.5" || c.State != LinkMoved {
		t.Fatalf("a restarted watcher missed the move: %+v", c)
	}
	if second != 1 {
		t.Errorf("restarts after watcher restart = %d, want 1", second)
	}
}

func TestLinkWatcher_RecordsAddressEvenWhenRestartFails(t *testing.T) {
	t.Parallel()
	f := newLinkFixture(t, 2*time.Minute)
	f.addrs = []string{"192.168.10.5/24"}
	f.check(t)

	f.restartErr = errors.New("kickstart refused")
	f.addrs = []string{"192.168.1.42/24"}
	if _, err := f.w.Check(context.Background()); err == nil {
		t.Fatal("expected the restart failure to surface")
	}
	// Retrying the same doomed restart every tick would bury the log without
	// changing anything.
	if c, err := f.w.Check(context.Background()); err != nil || c != nil {
		t.Errorf("second observation should be quiet, got %+v (err %v)", c, err)
	}
	if f.restarts != 1 {
		t.Errorf("restarts = %d, want 1 (no retry storm)", f.restarts)
	}
}
