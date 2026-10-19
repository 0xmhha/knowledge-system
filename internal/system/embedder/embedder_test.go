package embedder

import (
	"context"
	"github.com/0xmhha/knowledge-system/pkg/vector/embed/cache"
	"testing"
)

func TestOpenWithCacheSharesProviderIdentityAndDisabledDefault(t *testing.T) {
	e, cap, err := OpenWithCache("mock", "", "", 0, "", 2)
	if err != nil {
		t.Fatal(err)
	}
	cached, ok := e.(*cache.Cached)
	if !ok || cap.CacheEntries != 2 {
		t.Fatal("cache not selected", cap)
	}
	for range 2 {
		if _, err := e.Embed(context.Background(), []string{"repeat"}); err != nil {
			t.Fatal(err)
		}
	}
	hits, miss, _ := cached.Stats()
	if hits != 1 || miss != 1 {
		t.Fatal("cache unused", hits, miss)
	}
	plain, plainCap, err := OpenWithCache("mock", "", "", 0, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := plain.(*cache.Cached); ok || plainCap.CacheEntries != 0 || plain.Identity() != e.Identity() {
		t.Fatal("disabled/identity contract changed")
	}
	for _, size := range []int{-1, cache.MaxSize + 1} {
		if _, _, err := OpenWithCache("ollama", "", "http://127.0.0.1:1", 0, "", size); err == nil {
			t.Fatal("invalid cache size opened provider")
		}
	}
}

func TestOpenMockForStructuralEvaluation(t *testing.T) {
	e, cap, err := Open("mock", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if cap.Provider != "mock" || cap.Model != e.Name() || cap.Dim != e.Dimension() || cap.Endpoint != "" {
		t.Fatalf("capability does not describe mock embedder: %+v", cap)
	}
	vecs, err := e.Embed(context.Background(), []string{"fixture"})
	if err != nil || len(vecs) != 1 || len(vecs[0]) != e.Dimension() {
		t.Fatalf("mock embedding failed: vectors=%d err=%v", len(vecs), err)
	}
	if _, _, err := Open("mock", "bge-m3", ""); err == nil {
		t.Fatal("mismatched model name accepted")
	}
}
