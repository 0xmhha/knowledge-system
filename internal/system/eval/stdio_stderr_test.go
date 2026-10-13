package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/0xmhha/knowledge-system/internal/system/config"
)

const noisyChildBytes = 2 << 20

func TestRunnerRejectsHTTPConfigBeforeSpawningStdioChild(t *testing.T) {
	cfg := config.Default()
	cfg.Listen.Transport = "http"
	file := filepath.Join(t.TempDir(), "http.yaml")
	if err := config.Save(file, cfg); err != nil {
		t.Fatal(err)
	}
	_, err := NewRunner(context.Background(), RunnerOpts{CKSMCPBinary: filepath.Join(t.TempDir(), "must-not-start"), CKSMCPConfig: file})
	if err == nil || !strings.Contains(err.Error(), "requires stdio") {
		t.Fatalf("HTTP config reached subprocess launch: %v", err)
	}
}

func TestMain(m *testing.M) {
	if marker := os.Getenv("CKS_EVAL_SHUTDOWN_MARKER"); marker != "" {
		if os.Getenv("CKS_EVAL_IGNORE_TERM") == "1" {
			signal.Ignore(syscall.SIGTERM)
			_, _ = os.Stdout.WriteString("ready\n")
			time.Sleep(time.Minute)
			os.Exit(5)
		}
		signals := make(chan os.Signal, 1)
		signal.Notify(signals, syscall.SIGTERM)
		_, _ = os.Stdout.WriteString("ready\n")
		<-signals
		if err := os.WriteFile(marker, []byte("cleanup completed"), 0600); err != nil {
			os.Exit(4)
		}
		os.Exit(0)
	}
	if os.Getenv("CKS_EVAL_STDERR_HELPER") == "1" {
		// Write more than an OS pipe can hold BEFORE accepting initialize.
		if _, err := os.Stderr.Write(bytes.Repeat([]byte("x"), noisyChildBytes)); err != nil {
			os.Exit(2)
		}
		decoder := json.NewDecoder(os.Stdin)
		for {
			var request struct {
				ID     json.RawMessage `json:"id"`
				Method string          `json:"method"`
				Params struct {
					ProtocolVersion string `json:"protocolVersion"`
				} `json:"params"`
			}
			if err := decoder.Decode(&request); err != nil {
				if err == io.EOF {
					os.Exit(0)
				}
				os.Exit(3)
			}
			if request.Method == "initialize" {
				if os.Getenv("CKS_EVAL_STDERR_BAD_INIT") == "1" {
					_, _ = fmt.Fprintf(os.Stdout, "{\"jsonrpc\":\"2.0\",\"id\":%s,\"error\":{\"code\":-32603,\"message\":\"fixture initialize rejected\"}}\n", request.ID)
					continue
				}
				_, _ = fmt.Fprintf(os.Stdout, "{\"jsonrpc\":\"2.0\",\"id\":%s,\"result\":{\"protocolVersion\":%q,\"capabilities\":{},\"serverInfo\":{\"name\":\"noisy-fixture\",\"version\":\"1\"}}}\n", request.ID, request.Params.ProtocolVersion)
			}
		}
	}
	os.Exit(m.Run())
}

func TestEvaluationCancellationBoundsUnresponsiveServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	marker := filepath.Join(t.TempDir(), "must-not-exist")
	cmd, err := gracefulCommand(ctx, os.Args[0], []string{"CKS_EVAL_SHUTDOWN_MARKER=" + marker, "CKS_EVAL_IGNORE_TERM=1"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	ready := make(chan error, 1)
	go func() { buffer := make([]byte, 6); _, err := io.ReadFull(stdout, buffer); ready <- err }()
	select {
	case err := <-ready:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		cancel()
		_ = cmd.Wait()
		t.Fatal("child did not become ready")
	}
	started := time.Now()
	cancel()
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		if err == nil || !strings.Contains(err.Error(), "killed") {
			t.Fatalf("forced termination hidden: %v", err)
		}
		if time.Since(started) < 5*time.Second {
			t.Fatal("killed before graceful deadline")
		}
	case <-time.After(8 * time.Second):
		_ = cmd.Process.Kill()
		<-done
		t.Fatal("unresponsive child cleanup was unbounded")
	}
}

func TestEvaluationCancellationLetsServerCleanUp(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "cleanup")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cmd, err := gracefulCommand(ctx, os.Args[0], []string{"CKS_EVAL_SHUTDOWN_MARKER=" + marker}, nil)
	if err != nil {
		t.Fatal(err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	ready := make(chan error, 1)
	go func() { buffer := make([]byte, 6); _, err := io.ReadFull(stdout, buffer); ready <- err }()
	select {
	case err := <-ready:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		cancel()
		_ = cmd.Wait()
		t.Fatal("child did not become ready")
	}
	cancel()
	// The parent still reports cancellation even though the child exits zero.
	if err := cmd.Wait(); err == nil || !strings.Contains(err.Error(), "canceled") {
		t.Fatalf("cancellation result lost: %v", err)
	}
	raw, err := os.ReadFile(marker)
	if err != nil || string(raw) != "cleanup completed" {
		t.Fatalf("child was killed before cleanup: %q, %v", raw, err)
	}
}

func TestRunnerNoisyInitializeFailureClosesDiagnosticReader(t *testing.T) {
	var diagnostics bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	runner, err := NewRunner(ctx, RunnerOpts{CKSMCPBinary: os.Args[0], Env: []string{"CKS_EVAL_STDERR_HELPER=1", "CKS_EVAL_STDERR_BAD_INIT=1"}, InitializeTimeout: 2 * time.Second, Stderr: &diagnostics})
	if err == nil || runner != nil {
		t.Fatalf("failed initialize returned runner=%v err=%v", runner, err)
	}
	// NewRunner waits for the reader even on failure, so reading this buffer
	// is safe and all pre-handshake diagnostics remain available.
	if got := diagnostics.Len(); got != noisyChildBytes {
		t.Fatalf("failure lost diagnostics: %d", got)
	}
}

func TestRunnerDrainsAndPreservesNoisyChildStderrBeforeInitialize(t *testing.T) {
	var diagnostics bytes.Buffer
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	runner, err := NewRunner(ctx, RunnerOpts{CKSMCPBinary: os.Args[0], Env: []string{"CKS_EVAL_STDERR_HELPER=1"}, InitializeTimeout: 2 * time.Second, Stderr: &diagnostics})
	if err != nil {
		t.Fatalf("noisy child blocked MCP initialize: %v", err)
	}
	if err := runner.Close(); err != nil {
		t.Fatal(err)
	}
	if err := runner.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
	if got := diagnostics.Len(); got != noisyChildBytes {
		t.Fatalf("lost child diagnostics: %d, want %d", got, noisyChildBytes)
	}
}
