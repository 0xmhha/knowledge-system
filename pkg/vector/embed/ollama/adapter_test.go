package ollama

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestResolveMaxInput_RegistryAware(t *testing.T) {
	cases := map[string]int{
		"bge-m3":              8192, // registry value (also the default)
		"bge-large-en-v1.5":   512,  // registry value, differs from default
		"embeddinggemma-300m": 2048, // registry value, differs from default
		"some-ollama-only":    DefaultMaxInputTokens,
		"":                    DefaultMaxInputTokens,
	}
	for model, want := range cases {
		if got := resolveMaxInput(model); got != want {
			t.Errorf("resolveMaxInput(%q) = %d; want %d", model, got, want)
		}
	}
}

func TestResolveRuntimeOptions_BGEOnly(t *testing.T) {
	for _, model := range []string{"bge-m3", "bge-m3:latest", "bge-m3:567m"} {
		got := resolveRuntimeOptions(model)
		if got == nil || got.NumCtx != 8192 || got.NumBatch != 8192 {
			t.Errorf("%s: unexpected runtime options %+v", model, got)
		}
	}
	for _, model := range []string{"qwen3-embedding:0.6b", "bge-large-en-v1.5", "other"} {
		if got := resolveRuntimeOptions(model); got != nil {
			t.Errorf("%s: unexpected runtime options %+v", model, got)
		}
	}
}

func TestOpen_BGERuntimeOptionsAndIdentity(t *testing.T) {
	var embedCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []map[string]string{{
				"name": "bge-m3:latest", "digest": strings.Repeat("a", 64),
			}}})
		case "/api/embed":
			var req embedRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("decode: %v", err)
			}
			if req.Truncate || req.Options == nil || req.Options.NumCtx != 8192 || req.Options.NumBatch != 8192 {
				t.Errorf("unsafe bge-m3 embedding request: %+v", req)
			}
			embedCalls++
			_ = json.NewEncoder(w).Encode(embedResponse{Model: "bge-m3:latest", Embeddings: [][]float32{{1, 2, 3}}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	a, err := Open(Options{Endpoint: server.URL, ModelName: "bge-m3"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := a.Embed(context.Background(), []string{"full passage"}); err != nil {
		t.Fatal(err)
	}
	if embedCalls != 2 { // startup probe and ordinary passage
		t.Fatalf("embed calls = %d, want 2", embedCalls)
	}
	id := a.Identity()
	if id.RuntimeContextTokens != 8192 || id.RuntimeBatchTokens != 8192 {
		t.Fatalf("runtime identity not pinned: %+v", id)
	}
	legacy := id
	legacy.RuntimeContextTokens, legacy.RuntimeBatchTokens = 0, 0
	if id.Checksum() == legacy.Checksum() {
		t.Fatal("new runtime settings must require reindex from previous Ollama bge-m3")
	}
}

func TestMaxInputTokens_FallsBackWhenUnset(t *testing.T) {
	// A directly-constructed adapter (e.g. in tests) has no maxInput; the
	// accessor must still return a bounded default rather than 0.
	if got := (&Adapter{}).MaxInputTokens(); got != DefaultMaxInputTokens {
		t.Errorf("zero-value MaxInputTokens() = %d; want %d", got, DefaultMaxInputTokens)
	}
	if got := (&Adapter{maxInput: 512}).MaxInputTokens(); got != 512 {
		t.Errorf("MaxInputTokens() = %d; want 512", got)
	}
}

func TestOpen_RequiresModelName(t *testing.T) {
	_, err := Open(Options{})
	if err == nil {
		t.Fatal("expected error when ModelName is empty")
	}
}

func TestEmbed_SendsCorrectRequest(t *testing.T) {
	var receivedModel string
	var receivedInput []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req embedRequest
		json.NewDecoder(r.Body).Decode(&req)
		receivedModel = req.Model
		receivedInput = req.Input

		resp := embedResponse{
			Embeddings: [][]float32{{0.1, 0.2, 0.3}},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	a := &Adapter{
		endpoint:  server.URL,
		modelName: "test-model",
		dim:       3,
	}

	vecs, err := a.Embed(context.Background(), []string{"hello world"})
	if err != nil {
		t.Fatalf("Embed: %v", err)
	}
	if receivedModel != "test-model" {
		t.Errorf("model = %q, want test-model", receivedModel)
	}
	if len(receivedInput) != 1 || receivedInput[0] != "hello world" {
		t.Errorf("input = %v, want [hello world]", receivedInput)
	}
	if len(vecs) != 1 || len(vecs[0]) != 3 {
		t.Errorf("vecs shape = [%d][%d], want [1][3]", len(vecs), len(vecs[0]))
	}
}

func TestEmbed_EmptyBatch(t *testing.T) {
	a := &Adapter{endpoint: "http://unused", modelName: "m", dim: 3}
	vecs, err := a.Embed(context.Background(), nil)
	if err != nil {
		t.Fatalf("Embed(nil): %v", err)
	}
	if vecs != nil {
		t.Errorf("expected nil for empty batch, got %v", vecs)
	}
}

func TestEmbed_ServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("model not found"))
	}))
	defer server.Close()

	a := &Adapter{endpoint: server.URL, modelName: "bad", dim: 3}
	_, err := a.Embed(context.Background(), []string{"test"})
	if err == nil {
		t.Fatal("expected error on server 500")
	}
}

func TestEmbed_CountMismatch(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// One embedding for a two-input batch — must be rejected at the boundary.
		json.NewEncoder(w).Encode(embedResponse{Embeddings: [][]float32{{0.1, 0.2}}})
	}))
	defer server.Close()

	a := &Adapter{endpoint: server.URL, modelName: "m", dim: 2}
	if _, err := a.Embed(context.Background(), []string{"a", "b"}); err == nil {
		t.Fatal("expected error when response count != input count")
	}
}

func TestEmbed_Timeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		json.NewEncoder(w).Encode(embedResponse{Embeddings: [][]float32{{0.1}}})
	}))
	defer server.Close()

	a := &Adapter{endpoint: server.URL, modelName: "m", dim: 1, client: &http.Client{Timeout: 30 * time.Millisecond}}
	if _, err := a.Embed(context.Background(), []string{"x"}); err == nil {
		t.Fatal("expected timeout error from the bounded client")
	}
}

func TestOpen_TimeoutBounded(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		json.NewEncoder(w).Encode(embedResponse{Embeddings: [][]float32{{0.1}}})
	}))
	defer server.Close()

	start := time.Now()
	_, err := Open(Options{Endpoint: server.URL, ModelName: "m", Timeout: 30 * time.Millisecond})
	if err == nil {
		t.Fatal("expected Open to fail when the probe exceeds the timeout")
	}
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("Open did not honor the timeout (took %v)", elapsed)
	}
}

func TestAdapter_Interface(t *testing.T) {
	a := &Adapter{modelName: "test", dim: 768}
	if a.Name() != "test" {
		t.Errorf("Name = %q", a.Name())
	}
	if a.Dimension() != 768 {
		t.Errorf("Dimension = %d", a.Dimension())
	}
	if a.MaxInputTokens() <= 0 {
		t.Errorf("MaxInputTokens = %d", a.MaxInputTokens())
	}
	if err := a.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
}

func TestOpen_PinsDigestAndQueryPolicy(t *testing.T) {
	var changed atomic.Bool
	var changeDuringEmbed atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			digest := strings.Repeat("a", 64)
			if changed.Load() {
				digest = strings.Repeat("b", 64)
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []map[string]string{{"name": "qwen3-embedding:0.6b", "digest": digest}}})
		case "/api/embed":
			var req embedRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				t.Errorf("decode: %v", err)
			}
			if req.Truncate {
				t.Error("truncate must be false")
			}
			if req.Options != nil {
				t.Errorf("qwen3 runtime options unexpectedly changed: %+v", req.Options)
			}
			_ = json.NewEncoder(w).Encode(embedResponse{Model: "qwen3-embedding:0.6b", Embeddings: [][]float32{{1, 2, 3}}})
			if changeDuringEmbed.Load() {
				changed.Store(true)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	model := "qwen3-embedding:0.6b"
	withPrefix, err := Open(Options{Endpoint: server.URL, ModelName: model, QueryPrefixPolicy: "registry"})
	if err != nil {
		t.Fatal(err)
	}
	withoutPrefix, err := Open(Options{Endpoint: server.URL, ModelName: model, QueryPrefixPolicy: "none"})
	if err != nil {
		t.Fatal(err)
	}
	if withPrefix.Identity().ModelDigest != strings.Repeat("a", 64) {
		t.Fatalf("digest not pinned: %+v", withPrefix.Identity())
	}
	if withPrefix.Identity().Checksum() == withoutPrefix.Identity().Checksum() {
		t.Fatal("query policy must change identity")
	}
	changed.Store(true)
	if _, err := withPrefix.Embed(context.Background(), []string{"hello"}); err == nil || !strings.Contains(err.Error(), "model_changed") {
		t.Fatalf("digest replacement should fail, got %v", err)
	}
	changed.Store(false)
	changeDuringEmbed.Store(true)
	if _, err := withPrefix.Embed(context.Background(), []string{"hello"}); err == nil || !strings.Contains(err.Error(), "model_changed") {
		t.Fatalf("replacement during an embedding batch should fail, got %v", err)
	}
}

func TestOpen_RejectsMissingDigestAndAmbiguousAlias(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"models": []map[string]string{
			{"name": "bge-m3", "digest": strings.Repeat("a", 64)},
			{"name": "bge-m3:latest", "digest": strings.Repeat("b", 64)},
		}})
	}))
	defer server.Close()
	if _, err := Open(Options{Endpoint: server.URL, ModelName: "bge-m3"}); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("ambiguous alias should fail, got %v", err)
	}
	if _, err := Open(Options{Endpoint: server.URL, ModelName: "bge-m3:latest"}); err == nil || !strings.Contains(err.Error(), "connectivity") {
		t.Fatalf("explicit tag should reach embed probe, got %v", err)
	}
	missing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{"models": []map[string]string{{"name": "bge-m3:latest"}}})
	}))
	defer missing.Close()
	if _, err := Open(Options{Endpoint: missing.URL, ModelName: "bge-m3"}); err == nil || !strings.Contains(err.Error(), "valid SHA-256 digest") {
		t.Fatalf("missing digest should fail, got %v", err)
	}
}
