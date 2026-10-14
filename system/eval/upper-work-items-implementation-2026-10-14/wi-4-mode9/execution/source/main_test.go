package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestMain(m *testing.M) {
	if os.Getenv("KS_OWNED_FIXTURE_CHILD") == "1" {
		main()
		return
	}
	os.Exit(m.Run())
}
func TestReadBeforeAndAfterActualCommit(t *testing.T) {
	listener, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	_ = listener.Close()
	dir := t.TempDir()
	config := filepath.Join(dir, "config")
	if e = os.WriteFile(config, []byte(fmt.Sprintf("HTTPPort = %d\n", port)), 0600); e != nil {
		t.Fatal(e)
	}
	cmd := exec.Command(os.Args[0], "--config", config)
	cmd.Env = append(os.Environ(), "KS_OWNED_FIXTURE_CHILD=1")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	defer func() {
		_ = cmd.Process.Signal(syscall.SIGTERM)
		if e := cmd.Wait(); e != nil {
			t.Errorf("owned child exit: %v %s", e, stderr.String())
		}
	}()
	url := fmt.Sprintf("http://127.0.0.1:%d", port)
	client := http.Client{Timeout: time.Second}
	query := func(address string) (string, error) {
		body, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "eth_getBalance", "params": []any{address, "latest"}})
		req, _ := http.NewRequestWithContext(context.Background(), "POST", url, bytes.NewReader(body))
		resp, e := client.Do(req)
		if e != nil {
			return "", e
		}
		defer func() { _ = resp.Body.Close() }()
		var out struct{ Result string }
		e = json.NewDecoder(resp.Body).Decode(&out)
		return out.Result, e
	}
	deadline := time.Now().Add(5 * time.Second)
	var value string
	for time.Now().Before(deadline) {
		value, e = query("0x0000000000000000000000000000000000000002")
		if e == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if e != nil || value != "0x0" {
		t.Fatalf("pre-commit read=%s %v", value, e)
	}
	if _, e = query(activation); e != nil {
		t.Fatal(e)
	}
	time.Sleep(450 * time.Millisecond)
	value, e = query("0x0000000000000000000000000000000000000002")
	if e != nil || value != "0x1" {
		t.Fatalf("post-commit read=%s %v", value, e)
	}
}
