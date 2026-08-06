package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/server"
)

// TestAPIOnlySurface verifies the server's contract since the dashboard
// moved to the composition engine: /api/* answers, and /<anything> 404s —
// there is no static mount here. `cks viewer` serves the UI and proxies
// /api/* to this server.
func TestAPIOnlySurface(t *testing.T) {
	store := buildFixture(t)
	srv := server.New(store, nil)
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)

	resp, err := http.Get(ts.URL + "/api/manifest")
	if err != nil {
		t.Fatalf("GET /api/manifest: %v", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("api answer required; got %d", resp.StatusCode)
	}

	resp, err = http.Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	_ = resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("GET / = %d, want 404 (no static mount in the API server)", resp.StatusCode)
	}
}
