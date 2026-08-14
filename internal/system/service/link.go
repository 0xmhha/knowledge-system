package service

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"time"
)

const (
	// defaultLinkInterval is how often connectivity is re-read. A network
	// change is a human-timescale event and each poll is a walk of the
	// interface table, so seconds is the right order.
	defaultLinkInterval = 20 * time.Second

	// defaultDisconnectGrace is how long the host must stay off-network
	// before the outage is worth a log line. Wireless drops and returns
	// constantly; reporting every blip buries the outage that matters.
	defaultDisconnectGrace = 2 * time.Minute

	// linkStateFile records what was published, under the deployment's run dir.
	linkStateFile = "network-state.json"
)

// LinkState is what one observation concluded about this host's connectivity.
type LinkState string

const (
	// LinkConnected means the host is reachable at the address clients were
	// already given — including after an outage that ended where it started.
	LinkConnected LinkState = "connected"
	// LinkDisconnected means the host has no externally reachable address and
	// has stayed that way past the grace period.
	LinkDisconnected LinkState = "disconnected"
	// LinkMoved means the host is reachable at a different address than the
	// one clients hold, so that address has to be republished.
	LinkMoved LinkState = "moved"
)

// LinkChange is one observation worth telling someone about. Steady state
// produces none: this runs on a timer forever, so silence has to be the
// normal output.
type LinkChange struct {
	State LinkState `json:"state"`
	// Published is the address clients were last given.
	Published string `json:"published,omitempty"`
	// Current is what the host has now ("" while disconnected).
	Current string  `json:"current,omitempty"`
	URL     string  `json:"url,omitempty"`
	Outcome Outcome `json:"outcome,omitempty"`
	Detail  string  `json:"detail,omitempty"`
	// DownFor is how long the host had been off-network, on the observations
	// that report an outage or its end.
	DownFor string `json:"down_for,omitempty"`
}

// LinkWatcher keeps the address clients were given true, across the moves a
// laptop makes: a different AP, a dock in or out, a VPN up or down.
//
// The listener needs no help — it is bound to the wildcard address, so the
// kernel starts accepting on a new IP unasked. What breaks is everything
// pinned to the old one: the URL in a client config, and connections still
// held against an address that no longer routes. So a move restarts the
// instance to drop that state and republishes the address.
//
// What it deliberately does NOT do is react to losing the network. Wireless
// disconnects and reconnects on its own constantly, and the overwhelmingly
// common case is that it comes back on the same address — nothing was ever
// stale, and a restart would be pure downtime. So the watcher tracks the
// address it last PUBLISHED rather than the address it last SAW: an outage
// leaves that untouched, and the comparison on return is against what clients
// actually hold. A blip is silent no matter how long it lasts.
//
// Every dependency is injected — Addrs reads the host, Recoverer restarts and
// re-probes, Now and Sleep are the clock — so the decision is exercisable
// without a network, a launchd domain, or a real move between networks.
type LinkWatcher struct {
	// Instance names the deployment in reports.
	Instance string
	// Port is the port clients connect to. The host half is what is observed.
	Port string
	// Addrs returns the host's candidate addresses in interface order.
	Addrs func() ([]string, error)
	// Recoverer performs the restart and waits for the instance to serve.
	Recoverer Recoverer
	// StatePath persists what was published, so a watcher that is itself
	// restarted — launchd, a wake from sleep — still measures against what
	// clients hold rather than adopting whatever it finds. Empty disables
	// persistence and confines the comparison to one process lifetime.
	StatePath string
	// Interval is the poll period (default 20s).
	Interval time.Duration
	// DisconnectGrace is how long an outage must last before it is reported
	// (default 2m). It affects reporting only: no outage, of any length,
	// causes a restart.
	DisconnectGrace time.Duration
	// Now and Sleep default to time.Now and time.Sleep.
	Now   func() time.Time
	Sleep func(time.Duration)
}

// linkStateFileContents is what survives between observations.
type linkStateFileContents struct {
	// Published is the address clients were last given. An outage never
	// clears it — that is the whole point.
	Published string `json:"published"`
	// DownSince is when the host was first seen with no address, zero when
	// connected. DownReported keeps a sustained outage to one log line.
	DownSince    time.Time `json:"down_since,omitempty"`
	DownReported bool      `json:"down_reported,omitempty"`
}

func (w LinkWatcher) now() time.Time {
	if w.Now == nil {
		return time.Now()
	}
	return w.Now()
}

// Check performs one observation and reports only when something changed that
// a person or a client would care about.
func (w LinkWatcher) Check(ctx context.Context) (*LinkChange, error) {
	if w.Addrs == nil {
		return nil, fmt.Errorf("service: link watcher needs Addrs")
	}
	addrs, err := w.Addrs()
	if err != nil {
		return nil, fmt.Errorf("service: read host addresses: %w", err)
	}
	current := PickExternal(addrs)

	st, err := w.loadState()
	if err != nil {
		return nil, err
	}

	switch {
	case current == "":
		return w.observeDisconnected(st)
	case current == st.Published:
		return w.observeSameAddress(st)
	default:
		return w.observeMoved(ctx, st, current)
	}
}

// observeDisconnected records the outage without touching the published
// address and without restarting anything. Only a sustained outage is worth
// a line, and only one.
func (w LinkWatcher) observeDisconnected(st linkStateFileContents) (*LinkChange, error) {
	grace := w.DisconnectGrace
	if grace <= 0 {
		grace = defaultDisconnectGrace
	}
	now := w.now()
	if st.DownSince.IsZero() {
		st.DownSince = now
		return nil, w.saveState(st) // just dropped; say nothing yet
	}
	down := now.Sub(st.DownSince)
	if st.DownReported || down < grace {
		return nil, nil
	}
	st.DownReported = true
	if err := w.saveState(st); err != nil {
		return nil, err
	}
	return &LinkChange{
		State:     LinkDisconnected,
		Published: st.Published,
		DownFor:   down.Round(time.Second).String(),
		Detail: "host has no externally reachable address; the published address is kept " +
			"so a return to it costs no restart",
	}, nil
}

// observeSameAddress covers both steady state and the end of an outage that
// ended where it began — the common wireless case. Nothing is republished
// because nothing changed; the only output is closing a reported outage.
func (w LinkWatcher) observeSameAddress(st linkStateFileContents) (*LinkChange, error) {
	if st.DownSince.IsZero() {
		return nil, nil // steady state: silent
	}
	down := w.now().Sub(st.DownSince)
	reported := st.DownReported
	st.DownSince = time.Time{}
	st.DownReported = false
	if err := w.saveState(st); err != nil {
		return nil, err
	}
	if !reported {
		return nil, nil // a blip nobody was told about needs no all-clear
	}
	return &LinkChange{
		State:     LinkConnected,
		Published: st.Published,
		Current:   st.Published,
		URL:       URLFor(st.Published, w.Port),
		DownFor:   down.Round(time.Second).String(),
		Detail:    "back on the same address; clients need no change",
	}, nil
}

// observeMoved republishes. The restart is forced because the instance is
// still serving on its wildcard bind: a health probe would report "fine" and
// skip the restart the move is precisely what needs.
//
// The first observation of all has nothing to invalidate — no address was ever
// published — so it announces the URL without restarting.
func (w LinkWatcher) observeMoved(ctx context.Context, st linkStateFileContents, current string) (*LinkChange, error) {
	change := &LinkChange{
		State:     LinkMoved,
		Published: st.Published,
		Current:   current,
		URL:       URLFor(current, w.Port),
	}
	first := st.Published == ""
	if first {
		change.State = LinkConnected
		change.Outcome = OutcomeHealthy
		change.Detail = "publishing the current address; nothing was pinned to a previous one"
	} else {
		rep, rerr := w.Recoverer.Recover(ctx, w.Instance, net.JoinHostPort(current, w.Port), true)
		change.Outcome = rep.Outcome
		change.Detail = rep.Detail
		if rerr != nil {
			// Record the address anyway: repeating a failed restart on every
			// tick buries the log without changing the outcome.
			st.Published, st.DownSince, st.DownReported = current, time.Time{}, false
			if serr := w.saveState(st); serr != nil {
				return change, fmt.Errorf("%w (and: %w)", rerr, serr)
			}
			return change, rerr
		}
	}
	st.Published, st.DownSince, st.DownReported = current, time.Time{}, false
	if err := w.saveState(st); err != nil {
		return change, err
	}
	return change, nil
}

// Run polls until ctx is cancelled, handing each reportable observation to
// report. A failed restart is reported and the loop continues: the next move
// is still worth acting on, and a watcher that exits leaves the host unwatched.
func (w LinkWatcher) Run(ctx context.Context, report func(LinkChange, error)) error {
	interval := w.Interval
	if interval <= 0 {
		interval = defaultLinkInterval
	}
	sleep := w.Sleep
	if sleep == nil {
		sleep = time.Sleep
	}
	for {
		change, err := w.Check(ctx)
		if report != nil && (change != nil || err != nil) {
			var c LinkChange
			if change != nil {
				c = *change
			}
			report(c, err)
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		sleep(interval)
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

// LinkStatePath is where a deployment's watcher records what it published.
func LinkStatePath(runDir string) string {
	return filepath.Join(runDir, linkStateFile)
}

// loadState reads the persisted view. A missing or unreadable file is the
// first run, not an error: the worst it costs is one announcement.
func (w LinkWatcher) loadState() (linkStateFileContents, error) {
	var st linkStateFileContents
	if w.StatePath == "" {
		return st, nil
	}
	b, err := os.ReadFile(w.StatePath)
	if os.IsNotExist(err) {
		return st, nil
	}
	if err != nil {
		return st, fmt.Errorf("service: read %s: %w", w.StatePath, err)
	}
	if err := json.Unmarshal(b, &st); err != nil {
		return linkStateFileContents{}, nil
	}
	return st, nil
}

func (w LinkWatcher) saveState(st linkStateFileContents) error {
	if w.StatePath == "" {
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(w.StatePath), 0o755); err != nil {
		return fmt.Errorf("service: create state dir: %w", err)
	}
	b, err := json.Marshal(st)
	if err != nil {
		return fmt.Errorf("service: encode link state: %w", err)
	}
	if err := os.WriteFile(w.StatePath, append(b, '\n'), 0o644); err != nil {
		return fmt.Errorf("service: write %s: %w", w.StatePath, err)
	}
	return nil
}
