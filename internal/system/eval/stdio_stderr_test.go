package eval

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
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
					fmt.Fprintf(os.Stdout, "{\"jsonrpc\":\"2.0\",\"id\":%s,\"error\":{\"code\":-32603,\"message\":\"fixture initialize rejected\"}}\n", request.ID)
					continue
				}
				fmt.Fprintf(os.Stdout, "{\"jsonrpc\":\"2.0\",\"id\":%s,\"result\":{\"protocolVersion\":%q,\"capabilities\":{},\"serverInfo\":{\"name\":\"noisy-fixture\",\"version\":\"1\"}}}\n", request.ID, request.Params.ProtocolVersion)
			}
		}
	}
	os.Exit(m.Run())
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
