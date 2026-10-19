package viewercli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"
)

type backendReady struct {
	URL string
	PID int
}

// The helper runs only in a separately started test executable. No shell,
// operational graph, external listener or independently managed API is used.
func TestViewerBackendHelper(t *testing.T) {
	args := flag.Args()
	if len(args) < 1 || args[0] != "VIEWER-BACKEND" {
		return
	}
	if len(args) != 5 {
		t.Fatal("invalid owned helper arguments")
	}
	mode, ready, stopped, port := args[1], args[2], args[3], args[4]
	if mode == "exit" {
		os.Exit(7)
	}
	if mode == "ignore" {
		signal.Ignore(syscall.SIGTERM)
		if err := os.WriteFile(ready, []byte(`{"URL":"","PID":`+strconv.Itoa(os.Getpid())+`}`), 0o600); err != nil {
			t.Fatal(err)
		}
		<-time.After(time.Hour)
		return
	}
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM)
	defer cancel()
	if mode == "serve-ignore" {
		signal.Ignore(syscall.SIGTERM)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:"+port)
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"owned_fixture":true}`)
	}), ReadHeaderTimeout: time.Second}
	served := make(chan error, 1)
	go func() { served <- server.Serve(listener) }()
	data, err := json.Marshal(backendReady{URL: "http://" + listener.Addr().String(), PID: os.Getpid()})
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(ready, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if mode == "exit-after-gate" {
		ticker := time.NewTicker(10 * time.Millisecond)
		defer ticker.Stop()
		for {
			if _, err := os.Stat(stopped + ".release"); err == nil {
				os.Exit(9)
			}
			select {
			case <-ctx.Done():
				goto shutdown
			case <-ticker.C:
			}
		}
	}
	<-ctx.Done()
shutdown:
	shutdown, stop := context.WithTimeout(context.Background(), time.Second)
	defer stop()
	if err = server.Shutdown(shutdown); err != nil {
		t.Fatal(err)
	}
	if err = <-served; !errors.Is(err, http.ErrServerClosed) {
		t.Fatal(err)
	}
	if err = os.WriteFile(stopped, []byte("SIGTERM handled before exit"), 0o600); err != nil {
		t.Fatal(err)
	}
}

func helperBackend(t *testing.T, ctx context.Context, mode, port string, grace time.Duration) (*graphBackend, string, string) {
	t.Helper()
	dir := t.TempDir()
	ready, stopped := filepath.Join(dir, "ready.json"), filepath.Join(dir, "stopped")
	backend, err := startGraphBackend(ctx, os.Args[0], []string{"-test.run=^TestViewerBackendHelper$", "--", "VIEWER-BACKEND", mode, ready, stopped, port}, io.Discard, grace)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(backend.stop)
	return backend, ready, stopped
}
func awaitBackendReady(t *testing.T, path string) backendReady {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		data, err := os.ReadFile(path)
		if err == nil {
			var result backendReady
			if json.Unmarshal(data, &result) == nil {
				return result
			}
		}
		select {
		case <-ctx.Done():
			t.Fatal("owned helper did not enter ready state")
		case <-ticker.C:
		}
	}
}
func TestBackendStartFailureDoesNotInventProcess(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, tc := range []struct {
		name   string
		ctx    context.Context
		binary string
	}{{"missing", context.Background(), filepath.Join(t.TempDir(), "absent")}, {"canceled", ctx, os.Args[0]}} {
		t.Run(tc.name, func(t *testing.T) {
			b, err := startGraphBackend(tc.ctx, tc.binary, nil, io.Discard, time.Second)
			if err == nil || b != nil {
				t.Fatal("failed start retained a backend")
			}
		})
	}
}
func TestBackendCancellationGracefullyStopsAndReaps(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGTERM observation is POSIX-specific")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	backend, ready, stopped := helperBackend(t, ctx, "serve", "0", 3*time.Second)
	status := awaitBackendReady(t, ready)
	if err := waitReady(ctx, status.URL, 5*time.Second, backend); err != nil {
		t.Fatal(err)
	}
	cancel()
	backend.stop()
	backend.stop()
	if _, err := os.Stat(stopped); err != nil {
		t.Fatal("backend did not handle graceful stop", err)
	}
	if backend.cmd.ProcessState == nil || backend.cmd.ProcessState.ExitCode() != 0 {
		t.Fatal("owned backend not reaped with exit0", backend.err)
	}
	if err := backend.cmd.Process.Signal(syscall.Signal(0)); !errors.Is(err, os.ErrProcessDone) {
		t.Fatal("process handle not marked reaped", err)
	}
}
func TestBackendEarlyExitEndsReadiness(t *testing.T) {
	backend, _, _ := helperBackend(t, context.Background(), "exit", "0", time.Second)
	err := waitReady(context.Background(), "http://127.0.0.1:1/api/manifest", 10*time.Second, backend)
	if err == nil || !strings.Contains(err.Error(), "graph API exited before readiness") {
		t.Fatal("early backend exit hidden", err)
	}
	backend.stop()
	if backend.cmd.ProcessState.ExitCode() != 7 {
		t.Fatal("lost actual exit code")
	}
}
func TestBackendIgnoringTermIsStillReaped(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGTERM observation is POSIX-specific")
	}
	backend, ready, _ := helperBackend(t, context.Background(), "ignore", "0", 100*time.Millisecond)
	awaitBackendReady(t, ready)
	backend.stop()
	if backend.cmd.ProcessState == nil || backend.cmd.ProcessState.Success() {
		t.Fatal("unresponsive owned backend was not killed/reaped")
	}
	status, ok := backend.cmd.ProcessState.Sys().(syscall.WaitStatus)
	if !ok || !status.Signaled() || status.Signal() != syscall.SIGKILL {
		t.Fatal("bounded fallback did not kill the helper", backend.err)
	}
}
func TestReadinessCancellationInterruptsInflightHTTP(t *testing.T) {
	entered := make(chan struct{})
	released := make(chan struct{})
	var enteredOnce, releasedOnce sync.Once
	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		enteredOnce.Do(func() { close(entered) })
		<-r.Context().Done()
		releasedOnce.Do(func() { close(released) })
	}))
	defer server.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- waitReady(ctx, server.URL, time.Minute, nil) }()
	select {
	case <-entered:
	case <-time.After(5 * time.Second):
		t.Fatal("request did not enter handler")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("readiness ignored cancellation")
	}
	select {
	case <-released:
	case <-time.After(5 * time.Second):
		t.Fatal("request context did not close")
	}
}
func TestReadinessTimeoutIsAnAbsoluteRequestBoundary(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusServiceUnavailable) }))
	defer server.Close()
	err := waitReady(context.Background(), server.URL, 100*time.Millisecond, nil)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatal("readiness timeout was hidden", err)
	}
}
func TestViewerBindFailureStopsItsOwnedBackend(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGTERM observation is POSIX-specific")
	}
	occupied, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = occupied.Close() }()
	var backend *graphBackend
	var stopped string
	launch := func(ctx context.Context, _ string, port int, _ interface{ Write([]byte) (int, error) }) (*graphBackend, error) {
		var ready string
		backend, ready, stopped = helperBackend(t, ctx, "serve", fmt.Sprint(port), 3*time.Second)
		awaitBackendReady(t, ready)
		return backend, nil
	}
	err = runViewerWithBackend(context.Background(), "owned-graph", "", occupied.Addr().(*net.TCPAddr).Port, false, io.Discard, launch)
	if err == nil {
		t.Fatal("occupied dashboard port was accepted")
	}
	if backend == nil || backend.cmd.ProcessState == nil || backend.cmd.ProcessState.ExitCode() != 0 {
		t.Fatal("bind failure left owned backend unconfirmed")
	}
	if _, err = os.Stat(stopped); err != nil {
		t.Fatal("bind failure did not request graceful backend stop", err)
	}
}
func TestViewerEarlyBackendFailureIsReaped(t *testing.T) {
	var backend *graphBackend
	launch := func(ctx context.Context, _ string, _ int, _ interface{ Write([]byte) (int, error) }) (*graphBackend, error) {
		backend, _, _ = helperBackend(t, ctx, "exit", "0", time.Second)
		return backend, nil
	}
	err := runViewerWithBackend(context.Background(), "owned-graph", "", 0, false, io.Discard, launch)
	if err == nil || !strings.Contains(err.Error(), "graph API exited before readiness") {
		t.Fatal("early child failure hidden", err)
	}
	if backend.cmd.ProcessState == nil || backend.cmd.ProcessState.ExitCode() != 7 {
		t.Fatal("readiness failure did not reap child")
	}
}

func TestViewerUnexpectedBackendExitClosesDashboard(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	port, err := freePort()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	ready, stopped := filepath.Join(dir, "ready"), filepath.Join(dir, "stopped")
	launched := make(chan *graphBackend, 1)
	launch := func(ctx context.Context, _ string, apiPort int, _ interface{ Write([]byte) (int, error) }) (*graphBackend, error) {
		b, e := startGraphBackend(ctx, os.Args[0], []string{"-test.run=^TestViewerBackendHelper$", "--", "VIEWER-BACKEND", "exit-after-gate", ready, stopped, fmt.Sprint(apiPort)}, io.Discard, 3*time.Second)
		if e == nil {
			launched <- b
		}
		return b, e
	}
	done := make(chan error, 1)
	go func() { done <- runViewerWithBackend(ctx, "owned-graph", "", port, false, io.Discard, launch) }()
	var b *graphBackend
	select {
	case b = <-launched:
	case e := <-done:
		t.Fatal(e)
	case <-time.After(10 * time.Second):
		t.Fatal("owned backend not launched")
	}
	t.Cleanup(b.stop)
	if err = waitReady(ctx, fmt.Sprintf("http://127.0.0.1:%d/api/manifest", port), 10*time.Second, nil); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(stopped+".release", []byte("after-dashboard-ready"), 0o600); err != nil {
		t.Fatal(err)
	}
	select {
	case err = <-done:
		if err == nil || !strings.Contains(err.Error(), "graph API exited while serving") {
			t.Fatal("backend crash hidden as successful dashboard", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("dashboard stayed alive after owned backend failure")
	}
	if b.cmd.ProcessState == nil || b.cmd.ProcessState.ExitCode() != 9 {
		t.Fatal("failed backend not reaped")
	}
}
func TestExternalAPIIsNotOwnedByViewer(t *testing.T) {
	external := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = io.WriteString(w, `{"external_fixture":true}`) }))
	defer external.Close()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	port, err := freePort()
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- runViewer(ctx, "", external.URL, port, false, io.Discard) }()
	if err = waitReady(ctx, fmt.Sprintf("http://127.0.0.1:%d/api/manifest", port), 10*time.Second, nil); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case err = <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("dashboard did not stop")
	}
	response, err := external.Client().Get(external.URL)
	if err != nil {
		t.Fatal("viewer stopped an external API", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		t.Fatal("external API became unavailable")
	}
}

func TestViewerCancellationReportsForcedBackendStop(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGTERM observation is POSIX-specific")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	port, err := freePort()
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	ready, stopped := filepath.Join(dir, "ready"), filepath.Join(dir, "stopped")
	launched := make(chan *graphBackend, 1)
	launch := func(ctx context.Context, _ string, apiPort int, _ interface{ Write([]byte) (int, error) }) (*graphBackend, error) {
		b, e := startGraphBackend(ctx, os.Args[0], []string{"-test.run=^TestViewerBackendHelper$", "--", "VIEWER-BACKEND", "serve-ignore", ready, stopped, fmt.Sprint(apiPort)}, io.Discard, 100*time.Millisecond)
		if e == nil {
			launched <- b
		}
		return b, e
	}
	done := make(chan error, 1)
	go func() { done <- runViewerWithBackend(ctx, "owned-graph", "", port, false, io.Discard, launch) }()
	var backend *graphBackend
	select {
	case backend = <-launched:
	case err = <-done:
		t.Fatal(err)
	case <-time.After(10 * time.Second):
		t.Fatal("owned backend not launched")
	}
	t.Cleanup(backend.stop)
	if err = waitReady(ctx, fmt.Sprintf("http://127.0.0.1:%d/api/manifest", port), 10*time.Second, nil); err != nil {
		t.Fatal(err)
	}
	cancel()
	select {
	case err = <-done:
		if err == nil || !strings.Contains(err.Error(), "graph API shutdown failed") {
			t.Fatal("forced child stop hidden as graceful success", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("unresponsive backend stop unbounded")
	}
	if backend.cmd.ProcessState == nil || backend.cmd.ProcessState.Success() {
		t.Fatal("forced backend not reaped")
	}
}
