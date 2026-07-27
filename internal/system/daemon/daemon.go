// Package daemon supervises long-running fused-server (system-mcp) HTTP
// instances on one host: start/stop/restart/status/list, keyed by name via a
// pidfile per instance, plus registry-driven multi-instance up/down (one port
// per dataset). It ports the process-supervision core of serve-cks-http.sh /
// cks-mcpd.sh. LAN-IP autodetect for the advertised address lives in the sibling
// netutil package; the caller pins the bind address it passes here.
package daemon

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Supervisor manages named instances under RunDir. Binary is the server
// executable each instance runs; Args builds its argv from (name, config,
// addr). Env is extra environment for children (e.g. the Ollama endpoint).
type Supervisor struct {
	RunDir string
	Binary string
	Env    []string
	// Args maps an instance to its argv (without Binary). Defaults to the
	// system-mcp flags. Injectable so tests can supervise a dummy process.
	Args func(name, config, addr string) []string
	// StartGrace is how long Start waits before confirming the child stayed up.
	StartGrace time.Duration
}

// Instance is one managed server's status.
type Instance struct {
	Name    string `json:"name"`
	PID     int    `json:"pid"`
	Running bool   `json:"running"`
}

func (s *Supervisor) pidfile(name string) string { return filepath.Join(s.RunDir, name+".pid") }
func (s *Supervisor) logfile(name string) string { return filepath.Join(s.RunDir, name+".log") }

func (s *Supervisor) argv(name, config, addr string) []string {
	if s.Args != nil {
		return s.Args(name, config, addr)
	}
	a := []string{"--config", config, "--name", name}
	if addr != "" {
		a = append(a, "--http-addr", addr)
	}
	return a
}

// readPID returns the recorded pid for name, or 0 when there is no pidfile.
func (s *Supervisor) readPID(name string) int {
	b, err := os.ReadFile(s.pidfile(name))
	if err != nil {
		return 0
	}
	pid, _ := strconv.Atoi(strings.TrimSpace(string(b)))
	return pid
}

// alive reports whether pid is a live process (signal 0 probe).
func alive(pid int) bool {
	if pid <= 0 {
		return false
	}
	return syscall.Kill(pid, 0) == nil
}

func (s *Supervisor) running(name string) (int, bool) {
	pid := s.readPID(name)
	return pid, alive(pid)
}

// Start launches a detached instance if one is not already running under name.
// The child is put in its own session so it outlives this process; stdout and
// stderr go to the per-instance log. Returns the instance status.
func (s *Supervisor) Start(name, config, addr string) (Instance, error) {
	if pid, up := s.running(name); up {
		return Instance{Name: name, PID: pid, Running: true}, nil
	}
	if err := os.MkdirAll(s.RunDir, 0o755); err != nil {
		return Instance{}, fmt.Errorf("daemon: prepare run dir: %w", err)
	}
	logf, err := os.OpenFile(s.logfile(name), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return Instance{}, fmt.Errorf("daemon: open log: %w", err)
	}
	defer func() { _ = logf.Close() }()

	cmd := exec.Command(s.Binary, s.argv(name, config, addr)...)
	cmd.Stdout, cmd.Stderr = logf, logf
	if len(s.Env) > 0 {
		cmd.Env = append(os.Environ(), s.Env...)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true} // detach: new session, survives parent
	if err := cmd.Start(); err != nil {
		return Instance{}, fmt.Errorf("daemon: start %s: %w", name, err)
	}
	pid := cmd.Process.Pid
	// Release the child so it is not reaped as a zombie when it outlives us.
	_ = cmd.Process.Release()
	if err := os.WriteFile(s.pidfile(name), []byte(strconv.Itoa(pid)), 0o644); err != nil {
		return Instance{}, fmt.Errorf("daemon: write pidfile: %w", err)
	}

	grace := s.StartGrace
	if grace == 0 {
		grace = time.Second
	}
	time.Sleep(grace)
	if !alive(pid) {
		_ = os.Remove(s.pidfile(name))
		return Instance{Name: name}, fmt.Errorf("daemon: %s exited during startup — see %s", name, s.logfile(name))
	}
	return Instance{Name: name, PID: pid, Running: true}, nil
}

// Stop signals the instance (SIGTERM, then SIGKILL after a grace period) and
// removes its pidfile. Stopping a not-running instance is a no-op success.
func (s *Supervisor) Stop(name string) error {
	pid, up := s.running(name)
	if !up {
		_ = os.Remove(s.pidfile(name))
		return nil
	}
	_ = syscall.Kill(pid, syscall.SIGTERM)
	for i := 0; i < 50; i++ {
		if !alive(pid) {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if alive(pid) {
		_ = syscall.Kill(pid, syscall.SIGKILL)
	}
	_ = os.Remove(s.pidfile(name))
	return nil
}

// Restart stops then starts an instance.
func (s *Supervisor) Restart(name, config, addr string) (Instance, error) {
	if err := s.Stop(name); err != nil {
		return Instance{}, err
	}
	return s.Start(name, config, addr)
}

// Status returns one named instance's state.
func (s *Supervisor) Status(name string) Instance {
	pid, up := s.running(name)
	return Instance{Name: name, PID: pid, Running: up}
}

// List returns every instance with a pidfile under RunDir, sorted by name.
func (s *Supervisor) List() ([]Instance, error) {
	entries, err := filepath.Glob(filepath.Join(s.RunDir, "*.pid"))
	if err != nil {
		return nil, err
	}
	out := make([]Instance, 0, len(entries))
	for _, pf := range entries {
		name := strings.TrimSuffix(filepath.Base(pf), ".pid")
		out = append(out, s.Status(name))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}
