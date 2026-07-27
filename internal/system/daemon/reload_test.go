package daemon

import (
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestReloadHealthySwap(t *testing.T) {
	s := newTestSupervisor(t)
	t.Cleanup(func() { _ = s.Stop("svc"); _ = s.Stop("svc.green") })

	blue, err := s.Start("svc", "cfg", "127.0.0.1:9400")
	if err != nil {
		t.Fatalf("start blue: %v", err)
	}
	inst, err := s.Reload("svc", "cfg", "127.0.0.1:9400", ReloadOptions{
		Probe:    func(string) bool { return true }, // new instance is healthy
		Timeout:  time.Second,
		Interval: 20 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if !inst.Running || inst.PID == blue.PID {
		t.Errorf("expected a fresh running instance; blue=%d got=%+v", blue.PID, inst)
	}
	if g := s.Status("svc.green"); g.Running {
		t.Errorf("green probe instance should be cleaned up, got %+v", g)
	}
}

func TestReloadKeepsBlueWhenGreenUnhealthy(t *testing.T) {
	s := newTestSupervisor(t)
	t.Cleanup(func() { _ = s.Stop("svc2"); _ = s.Stop("svc2.green") })

	blue, err := s.Start("svc2", "cfg", "127.0.0.1:9401")
	if err != nil {
		t.Fatalf("start blue: %v", err)
	}
	_, err = s.Reload("svc2", "cfg", "127.0.0.1:9401", ReloadOptions{
		Probe:    func(string) bool { return false }, // new instance never becomes healthy
		Timeout:  200 * time.Millisecond,
		Interval: 40 * time.Millisecond,
	})
	if err == nil {
		t.Fatalf("expected reload to fail when green stays unhealthy")
	}
	// Blue must be left untouched (same pid, still running).
	st := s.Status("svc2")
	if !st.Running || st.PID != blue.PID {
		t.Errorf("blue should be untouched; blue=%d status=%+v", blue.PID, st)
	}
	if g := s.Status("svc2.green"); g.Running {
		t.Errorf("green should be stopped after a failed reload, got %+v", g)
	}
}

func TestHTTPHealth(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/healthz" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ok.Close()
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer bad.Close()

	if !HTTPHealth(strings.TrimPrefix(ok.URL, "http://")) {
		t.Error("HTTPHealth on a 200 /healthz should be true")
	}
	if HTTPHealth(strings.TrimPrefix(bad.URL, "http://")) {
		t.Error("HTTPHealth on a 503 should be false")
	}
	if HTTPHealth("127.0.0.1:1") {
		t.Error("HTTPHealth on an unreachable addr should be false")
	}
}

func TestWaitReady(t *testing.T) {
	var ready atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ready.Load() {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}))
	defer srv.Close()
	addr := strings.TrimPrefix(srv.URL, "http://")

	// Not serviceable yet → WaitReady times out and reports false.
	if WaitReady(addr, 150*time.Millisecond, 30*time.Millisecond) {
		t.Error("WaitReady should be false while /healthz returns 503")
	}
	// Flip to serviceable → WaitReady observes it and reports true.
	ready.Store(true)
	if !WaitReady(addr, time.Second, 30*time.Millisecond) {
		t.Error("WaitReady should become true once /healthz returns 200")
	}
}

func TestServiceable(t *testing.T) {
	ok := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ok.Close()
	addr := strings.TrimPrefix(ok.URL, "http://") // 127.0.0.1:PORT
	if !Serviceable(addr) {
		t.Error("Serviceable on a 200 /healthz should be true")
	}
	// A wildcard bind host is normalized to loopback, hitting the same server.
	_, port, _ := net.SplitHostPort(addr)
	if !Serviceable(net.JoinHostPort("0.0.0.0", port)) {
		t.Error("Serviceable should normalize 0.0.0.0 to loopback and reach the server")
	}
	if Serviceable("127.0.0.1:1") {
		t.Error("Serviceable on an unreachable addr should be false")
	}
}

func TestPortFreeOn(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	_, portStr, _ := net.SplitHostPort(l.Addr().String())
	port, _ := strconv.Atoi(portStr)
	if portFreeOn("127.0.0.1", port) {
		t.Errorf("port %d is bound, portFreeOn should be false", port)
	}
	_ = l.Close()
	if !portFreeOn("127.0.0.1", port) {
		t.Errorf("port %d is free after close, portFreeOn should be true", port)
	}
}

func TestProbeAddr(t *testing.T) {
	cases := map[string]string{
		"0.0.0.0:8801":   "127.0.0.1:8801",
		"[::]:8801":      "127.0.0.1:8801",
		":8801":          "127.0.0.1:8801",
		"192.168.1.5:80": "192.168.1.5:80",
		"garbage":        "garbage",
	}
	for in, want := range cases {
		if got := probeAddr(in); got != want {
			t.Errorf("probeAddr(%q) = %q, want %q", in, got, want)
		}
	}
}
