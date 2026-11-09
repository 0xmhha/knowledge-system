package llmprefix

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestOllamaGenerator_TurnsThinkingOff(t *testing.T) {
	var got map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/generate" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&got); err != nil {
			t.Errorf("decode request: %v", err)
		}
		_, _ = w.Write([]byte(`{"response":"ok"}`))
	}))
	defer srv.Close()
	t.Setenv("CKV_OLLAMA_ENDPOINT", srv.URL)

	out, err := NewOllamaGenerator("gemma4:12b").Generate(context.Background(), "hi")
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if out != "ok" {
		t.Fatalf("response = %q", out)
	}
	if got["think"] != false || got["stream"] != false || got["model"] != "gemma4:12b" || got["prompt"] != "hi" {
		t.Fatalf("request = %v", got)
	}
}

func TestOllamaGenerator_HTTPErrorIsReturned(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "model not found", http.StatusNotFound)
	}))
	defer srv.Close()
	t.Setenv("CKV_OLLAMA_ENDPOINT", srv.URL)

	if _, err := NewOllamaGenerator("missing").Generate(context.Background(), "hi"); err == nil {
		t.Fatal("Generate: want error for HTTP 404, got nil")
	}
}
