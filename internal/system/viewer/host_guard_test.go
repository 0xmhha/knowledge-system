package viewer

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// The dashboard proxies /api/* to the graph API; a rebinding page must not
// read through it.
func TestHandlerRejectsNonLoopbackHost(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("[]")) }))
	t.Cleanup(backend.Close)
	base, _ := url.Parse(backend.URL)
	h, err := Handler(base, "")
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(h)
	t.Cleanup(ts.Close)
	for host, want := range map[string]int{"attacker.example:8080": http.StatusForbidden, "127.0.0.1:8080": http.StatusOK, "localhost": http.StatusOK} {
		for _, path := range []string{"/api/nodes", "/"} {
			req, _ := http.NewRequest("GET", ts.URL+path, nil)
			req.Host = host
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatal(err)
			}
			_ = resp.Body.Close()
			if resp.StatusCode != want {
				t.Errorf("Host %q %s: status %d, want %d", host, path, resp.StatusCode, want)
			}
		}
	}
}
