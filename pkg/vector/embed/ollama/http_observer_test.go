package ollama

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

type observerScopeKey struct{}

func TestHTTPObserverCountsRedirectsPreservesSpaceAndSeesCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			json.NewEncoder(w).Encode(map[string]any{"models": []map[string]string{{"name": "fixture:latest", "digest": strings.Repeat("a", 64)}}})
		case "/api/embed":
			http.Redirect(w, r, "/redirect/embed", http.StatusTemporaryRedirect)
		case "/redirect/embed":
			var request embedRequest
			if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
				t.Error(err)
				return
			}
			if request.Input[0] == "error" {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			if request.Input[0] == "delay" {
				select {
				case <-time.After(time.Second):
				case <-r.Context().Done():
					return
				}
			}
			json.NewEncoder(w).Encode(embedResponse{Model: "fixture:latest", Embeddings: [][]float32{{1}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	type observation struct {
		Method, Path                           string
		RequestBytes, ResponseBytes, ElapsedNS int64
		Status                                 int
		Err                                    error
	}
	var records []observation
	observer := func(ctx context.Context, method, path string, bytes int64) func(int, int64, int64, error) {
		if ctx.Value(observerScopeKey{}) != true {
			return nil
		}
		return func(status int, responseBytes, ns int64, err error) {
			records = append(records, observation{Method: method, Path: path, RequestBytes: bytes, ResponseBytes: responseBytes, ElapsedNS: ns, Status: status, Err: err})
		}
	}
	opts := Options{Endpoint: server.URL, ModelName: "fixture", ObserveHTTP: observer}
	a, err := Open(opts)
	if err != nil {
		t.Fatal(err)
	}
	opts.ObserveHTTP = nil
	b, err := Open(opts)
	if err != nil {
		t.Fatal(err)
	}
	if a.Identity().Checksum() != b.Identity().Checksum() {
		t.Fatal("observer altered model space")
	}
	ctx := context.WithValue(context.Background(), observerScopeKey{}, true)
	vecs, err := a.Embed(ctx, []string{"PRIVATE_QUERY"})
	if err != nil || len(vecs) != 1 || vecs[0][0] != 1 {
		t.Fatalf("observer changed embedding: %v, %v", vecs, err)
	}
	if len(records) != 4 {
		t.Fatalf("lost before/after tags or redirect: %+v", records)
	}
	redirects := 0
	for _, record := range records {
		if record.Status == http.StatusTemporaryRedirect {
			redirects++
		}
		if record.ElapsedNS <= 0 || record.Err != nil {
			t.Fatalf("invalid timing: %+v", record)
		}
		if record.Method == http.MethodPost && record.RequestBytes <= 0 {
			t.Fatal("missing request size")
		}
		if record.Status == http.StatusOK && record.ResponseBytes <= 0 {
			t.Fatal("missing decoded response size")
		}
	}
	if redirects != 1 {
		t.Fatal("redirect attempt collapsed into final request")
	}
	raw, _ := json.Marshal(records)
	if strings.Contains(string(raw), "PRIVATE_QUERY") || strings.Contains(string(raw), server.URL) {
		t.Fatal("observer received payload or URL host")
	}
	records = nil
	if _, err := a.Embed(ctx, []string{"error"}); err == nil {
		t.Fatal("HTTP 503 swallowed")
	}
	if records[len(records)-1].Status != http.StatusServiceUnavailable {
		t.Fatal("failed transport response not recorded")
	}
	records = nil
	cancelCtx, cancel := context.WithTimeout(ctx, 30*time.Millisecond)
	defer cancel()
	if _, err := a.Embed(cancelCtx, []string{"delay"}); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout changed: %v", err)
	}
	if len(records) == 0 || !errors.Is(records[len(records)-1].Err, context.DeadlineExceeded) {
		t.Fatal("failed transport attempt disappeared")
	}
}
