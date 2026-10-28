package ollama

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fakeOllama(t *testing.T, model string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/tags":
			_ = json.NewEncoder(w).Encode(map[string]any{"models": []map[string]string{{
				"name": model, "digest": strings.Repeat("b", 64),
			}}})
		case "/api/embed":
			_ = json.NewEncoder(w).Encode(embedResponse{Model: model, Embeddings: [][]float32{{1, 2, 3}}})
		default:
			http.NotFound(w, r)
		}
	}))
}

func TestChunkBudget_DefaultIsBGEOnly(t *testing.T) {
	t.Setenv("CKV_CHUNK_BUDGET_BYTES", "")
	server := fakeOllama(t, "qwen3-embedding:4b")
	defer server.Close()
	a, err := Open(Options{Endpoint: server.URL, ModelName: "qwen3-embedding:4b"})
	if err != nil {
		t.Fatal(err)
	}
	if got := a.Identity().ChunkBudgetBytes; got != 0 {
		t.Fatalf("qwen3 default chunk budget = %d, want 0", got)
	}
}

func TestChunkBudget_OverrideChangesIdentity(t *testing.T) {
	server := fakeOllama(t, "qwen3-embedding:4b")
	defer server.Close()
	t.Setenv("CKV_CHUNK_BUDGET_BYTES", "")
	plain, err := Open(Options{Endpoint: server.URL, ModelName: "qwen3-embedding:4b"})
	if err != nil {
		t.Fatal(err)
	}
	opt, err := Open(Options{Endpoint: server.URL, ModelName: "qwen3-embedding:4b", ChunkBudgetBytes: 6144})
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("CKV_CHUNK_BUDGET_BYTES", "6144")
	env, err := Open(Options{Endpoint: server.URL, ModelName: "qwen3-embedding:4b"})
	if err != nil {
		t.Fatal(err)
	}
	if opt.Identity().ChunkBudgetBytes != 6144 || env.Identity().ChunkBudgetBytes != 6144 {
		t.Fatalf("override not applied: option %d, env %d",
			opt.Identity().ChunkBudgetBytes, env.Identity().ChunkBudgetBytes)
	}
	if plain.Identity().Checksum() == env.Identity().Checksum() {
		t.Fatal("a different chunk budget must change the identity checksum")
	}
	if opt.Identity().Checksum() != env.Identity().Checksum() {
		t.Fatal("option and env override must give the same identity")
	}
}

func TestChunkBudget_RejectsInvalidEnv(t *testing.T) {
	server := fakeOllama(t, "qwen3-embedding:4b")
	defer server.Close()
	for _, v := range []string{"abc", "-1"} {
		t.Setenv("CKV_CHUNK_BUDGET_BYTES", v)
		if _, err := Open(Options{Endpoint: server.URL, ModelName: "qwen3-embedding:4b"}); err == nil {
			t.Fatalf("CKV_CHUNK_BUDGET_BYTES=%q accepted", v)
		}
	}
}
