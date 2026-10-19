package main

import (
	"context"
	"github.com/0xmhha/knowledge-system/pkg/vector/embed/cache"
	"strings"
	"testing"
)

func TestResolveEmbedderOptInCacheAndInvalidCapacity(t *testing.T) {
	saved := globalFlags
	t.Cleanup(func() { globalFlags = saved })
	globalFlags.embeddingCacheEntries = 2
	emb, cleanup, err := resolveEmbedder("mock", "")
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	cached, ok := emb.(*cache.Cached)
	if !ok {
		t.Fatal("cache option did not reach CLI factory")
	}
	for range 2 {
		if _, err := emb.Embed(context.Background(), []string{"repeat"}); err != nil {
			t.Fatal(err)
		}
	}
	hits, miss, size := cached.Stats()
	if hits != 1 || miss != 1 || size != 1 {
		t.Fatal("factory cache unused", hits, miss, size)
	}
	globalFlags.embeddingCacheEntries = 0
	plain, closePlain, err := resolveEmbedder("mock", "")
	if err != nil {
		t.Fatal(err)
	}
	defer closePlain()
	if _, ok := plain.(*cache.Cached); ok {
		t.Fatal("default unexpectedly cached")
	}
	for _, entries := range []int{-1, cache.MaxSize + 1} {
		globalFlags.embeddingCacheEntries = entries
		if _, _, err := resolveEmbedder("ollama", ""); err == nil {
			t.Fatal("invalid cache size accepted before provider open")
		}
	}
}

// TestResolveEmbedder_BgeonnxForwardsModelName proves the bgeonnx case forwards
// globalFlags.modelName into bgeonnx.Options (02 §3). We use an unknown model
// name so bgeonnx.Open fails in the registry lookup (LookupModel → "unknown
// model %q") BEFORE any model-file stat or ONNX session — so the assertion
// needs no model files and no live runtime. If the name were NOT forwarded,
// bgeonnx would fall back to its default model and the error would not mention
// our sentinel name (it would either succeed-or-fail on the default model).
func TestResolveEmbedder_BgeonnxForwardsModelName(t *testing.T) {
	const sentinel = "no-such-model-zzz-forward-probe"

	saved := globalFlags.modelName
	t.Cleanup(func() { globalFlags.modelName = saved })
	globalFlags.modelName = sentinel

	_, cleanup, err := resolveEmbedder("bgeonnx", "")
	if cleanup != nil {
		cleanup()
	}
	if err == nil {
		t.Fatalf("expected error for unknown bgeonnx model %q, got nil", sentinel)
	}
	if !strings.Contains(err.Error(), sentinel) {
		t.Errorf("error %q does not mention forwarded model name %q — "+
			"--model-name is not reaching bgeonnx.Options", err.Error(), sentinel)
	}
}

// TestResolveEmbedder_Mock keeps a no-dependency happy path so the table above
// isn't the only coverage of resolveEmbedder.
func TestResolveEmbedder_Mock(t *testing.T) {
	emb, cleanup, err := resolveEmbedder("mock", "")
	if err != nil {
		t.Fatalf("resolveEmbedder(mock): %v", err)
	}
	if cleanup != nil {
		defer cleanup()
	}
	if emb == nil || emb.Dimension() <= 0 {
		t.Fatalf("mock embedder invalid: %v", emb)
	}
}
