package daemon

import (
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"
)

// HTTPHealth probes GET http://addr/healthz and reports whether it answers 2xx.
// It is the default readiness gate for Reload: a 200 means the instance booted
// and its dataset is serviceable (see the fused server's /healthz).
func HTTPHealth(addr string) bool {
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Get("http://" + addr + "/healthz")
	if err != nil {
		return false
	}
	defer func() { _ = resp.Body.Close() }()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// ReloadOptions tunes the blue-green health gate. Zero values pick safe defaults.
type ReloadOptions struct {
	// TempAddr is the bind address for the green probe instance. Empty picks a
	// free port on the same host as the target address.
	TempAddr string
	// Probe reports whether the instance at an address is serviceable. Empty
	// uses HTTPHealth.
	Probe func(addr string) bool
	// Timeout bounds how long Reload waits for green to become healthy (default
	// 30s); Interval is the poll period (default 500ms).
	Timeout  time.Duration
	Interval time.Duration
}

// Reload performs a health-gated blue-green swap of one instance. It starts a
// green probe instance on a temporary port with the (updated) config, waits for
// it to report healthy, and only then restarts the instance on its real
// address. If green never becomes healthy the running (blue) instance is left
// untouched — a broken new dataset version never takes the server down. On a
// healthy swap it returns the restarted instance; the final restart is
// re-probed and a non-fatal warning is surfaced via the returned error if it
// does not come up serviceable.
func (s *Supervisor) Reload(name, config, addr string, o ReloadOptions) (Instance, error) {
	probe := o.Probe
	if probe == nil {
		probe = HTTPHealth
	}
	timeout := o.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	interval := o.Interval
	if interval == 0 {
		interval = 500 * time.Millisecond
	}

	tempAddr, err := s.tempAddr(o.TempAddr, addr)
	if err != nil {
		return s.Status(name), fmt.Errorf("reload %s: %w", name, err)
	}
	greenName := name + ".green"

	// Bring green up on the temporary port and clean it up no matter what.
	_ = s.Stop(greenName) // clear any stale green from a prior aborted reload
	if _, err := s.Start(greenName, config, tempAddr); err != nil {
		return s.Status(name), fmt.Errorf("reload %s: start green: %w", name, err)
	}
	defer func() { _ = s.Stop(greenName) }()

	if !waitHealthy(probe, probeAddr(tempAddr), timeout, interval) {
		// Green is not serviceable — keep blue running untouched.
		return s.Status(name), fmt.Errorf("reload %s: new instance did not become healthy at %s within %s — keeping current instance",
			name, tempAddr, timeout)
	}

	// Green proved the config is serviceable; hand the real address to a fresh
	// instance. The prior blue is replaced here (brief swap window).
	inst, err := s.Restart(name, config, addr)
	if err != nil {
		return inst, fmt.Errorf("reload %s: restart on %s: %w", name, addr, err)
	}
	// Best-effort confirmation on the real address (cannot roll back to the old
	// version — the config already points at the new one — so this is a warning).
	if !waitHealthy(probe, probeAddr(addr), timeout, interval) {
		return inst, fmt.Errorf("reload %s: restarted on %s but it is not reporting healthy yet", name, addr)
	}
	return inst, nil
}

// probeAddr rewrites a wildcard bind host to loopback so the address is
// connectable for a health probe.
func probeAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	switch host {
	case "", "0.0.0.0", "::", "[::]":
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

// tempAddr resolves the green probe address: an explicit override, else a free
// port on the same host as the target address.
func (s *Supervisor) tempAddr(override, target string) (string, error) {
	if override != "" {
		return override, nil
	}
	host, _, err := net.SplitHostPort(target)
	if err != nil || host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	p, err := PickFreePort(defaultPortBase+100, PortFree)
	if err != nil {
		return "", err
	}
	return net.JoinHostPort(host, strconv.Itoa(p)), nil
}

// waitHealthy polls probe(addr) until it returns true or timeout elapses.
func waitHealthy(probe func(string) bool, addr string, timeout, interval time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for {
		if probe(addr) {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(interval)
	}
}
