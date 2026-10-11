package server_test

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/internal/graph/server"
)

// The API binds 127.0.0.1, so a browser can only reach it under a foreign
// Host through DNS rebinding. Such requests must not read the graph.
func TestServerRejectsNonLoopbackHost(t *testing.T) {
	store, err := persist.Open(filepath.Join(t.TempDir(), "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	ts := httptest.NewServer(server.New(store, nil))
	t.Cleanup(ts.Close)
	for host, want := range map[string]int{
		"attacker.example:8080": http.StatusForbidden,
		"attacker.example":      http.StatusForbidden,
		"10.0.0.5:8080":         http.StatusForbidden,
		"127.0.0.1:8080":        http.StatusOK,
		"localhost:8080":        http.StatusOK,
		"[::1]:8080":            http.StatusOK,
	} {
		req, _ := http.NewRequest("GET", ts.URL+"/api/nodes/top?limit=1", nil)
		req.Host = host
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != want {
			t.Errorf("Host %q: status %d, want %d", host, resp.StatusCode, want)
		}
	}
}

// POST bodies are capped so one request cannot make the server buffer an
// unbounded ID list.
func TestPostBodiesAreBounded(t *testing.T) {
	store, err := persist.Open(filepath.Join(t.TempDir(), "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Migrate(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	ts := httptest.NewServer(server.New(store, nil))
	t.Cleanup(ts.Close)
	huge := `{"ids":["` + strings.Repeat("a", 5<<20) + `"]}`
	for _, path := range []string{"/api/edges", "/api/nodes-by-ids"} {
		for body, want := range map[string]int{huge: http.StatusRequestEntityTooLarge, `{"ids":["x"]}`: http.StatusOK} {
			resp, err := http.Post(ts.URL+path, "application/json", strings.NewReader(body))
			if err != nil {
				t.Fatal(err)
			}
			_ = resp.Body.Close()
			if resp.StatusCode != want {
				t.Errorf("%s body %d bytes: status %d, want %d", path, len(body), resp.StatusCode, want)
			}
		}
	}
}
