package viewer

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

// TestHandler_ProxiesAPIAndServesStatic asserts the two halves of the
// viewer surface: /api/* reaches the backend verbatim, and / serves the
// embedded dashboard index.
func TestHandler_ProxiesAPIAndServesStatic(t *testing.T) {
	backend := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/manifest" {
			t.Errorf("backend got path %q, want /api/manifest", r.URL.Path)
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer backend.Close()

	base, err := url.Parse(backend.URL)
	if err != nil {
		t.Fatal(err)
	}
	h, err := Handler(base, "")
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	front := httptest.NewServer(h)
	defer front.Close()

	res, err := http.Get(front.URL + "/api/manifest")
	if err != nil {
		t.Fatalf("GET /api/manifest: %v", err)
	}
	body, _ := io.ReadAll(res.Body)
	_ = res.Body.Close()
	if string(body) != `{"ok":true}` {
		t.Errorf("proxied body = %q", body)
	}

	res, err = http.Get(front.URL + "/")
	if err != nil {
		t.Fatalf("GET /: %v", err)
	}
	_ = res.Body.Close()
	if res.StatusCode != http.StatusOK {
		t.Errorf("GET / status = %d, want 200 (embedded index.html)", res.StatusCode)
	}
}
