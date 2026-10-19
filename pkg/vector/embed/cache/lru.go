// Package cache wraps a types.Embedder with a hot-path LRU cache so
// repeated embed calls on the same text return without paying the
// model cost.
//
// Use case: the agent issues semantic_search with the same intent
// multiple times during a multi-hop flow, and explain_match re-embeds
// the intent to compare each candidate. The cache keeps both hot.
//
// The cache is intentionally in-process and not persisted — the
// embeddings depend on model identity, which changes when the user
// swaps backends. Restart clears the cache, which is the correct
// behavior.
package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"sync"

	"github.com/0xmhha/knowledge-system/pkg/vector/types"
)

// DefaultSize is the cache capacity when the caller does not specify.
// 1024 entries × 1024-dim × 4 bytes ≈ 4 MB per cache instance for the
// stored vectors plus a small per-entry book-keeping overhead.
const DefaultSize = 1024

// MaxSize bounds explicitly configured process-local caches. Zero disables
// factory caching; New retains its historical default for nonpositive sizes.
const MaxSize = 65536

func ValidateSize(size int) error {
	if size < 0 || size > MaxSize {
		return fmt.Errorf("embedding cache entries must be 0..%d", MaxSize)
	}
	return nil
}

// Cached wraps an inner Embedder. Cache hits return immediately from
// memory; misses pass through to inner.Embed and store the result.
type Cached struct {
	inner types.Embedder
	size  int

	mu   sync.Mutex
	keys []string             // LRU recency order; index 0 = oldest
	data map[string][]float32 // key → vector
	hits int
	miss int
}

var _ types.QueryEmbedder = (*Cached)(nil)
var _ types.IdentityVerifier = (*Cached)(nil)
var _ types.EmbeddingProber = (*Cached)(nil)

// New wraps emb with an LRU cache. size <= 0 uses DefaultSize.
func New(emb types.Embedder, size int) *Cached {
	if size <= 0 {
		size = DefaultSize
	}
	return &Cached{
		inner: emb,
		size:  size,
		data:  make(map[string][]float32, size),
	}
}

// Name / Dimension / MaxInputTokens forward to the inner embedder so
// the wrapper is a drop-in for types.Embedder.
func (c *Cached) Name() string        { return c.inner.Name() }
func (c *Cached) Dimension() int      { return c.inner.Dimension() }
func (c *Cached) MaxInputTokens() int { return c.inner.MaxInputTokens() }

// Capacity reports the immutable entry bound; Stats reports current occupancy.
func (c *Cached) Capacity() int { return c.size }

// Identity forwards to the inner embedder: caching does not change the
// embedding space, so cached vectors carry the inner's identity.
func (c *Cached) Identity() types.EmbeddingIdentity { return c.inner.Identity() }

// VerifyIdentity preserves publication checks and verifies cached vectors still
// belong to the configured external model. It never caches a verification.
func (c *Cached) VerifyIdentity(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("embedding cache: context required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if verifier, ok := c.inner.(types.IdentityVerifier); ok {
		return verifier.VerifyIdentity(ctx)
	}
	return nil
}

// ProbeEmbedding bypasses cached vectors, including a cached liveness probe.
func (c *Cached) ProbeEmbedding(ctx context.Context) error {
	if ctx == nil {
		return fmt.Errorf("embedding cache: context required")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := c.VerifyIdentity(ctx); err != nil {
		return err
	}
	if probe, ok := c.inner.(types.EmbeddingProber); ok {
		if err := probe.ProbeEmbedding(ctx); err != nil {
			return err
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		return c.VerifyIdentity(ctx)
	}
	vectors, err := c.inner.Embed(ctx, []string{"liveness probe"})
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateVectors(vectors, 1, c.Dimension()); err != nil {
		return err
	}
	return c.VerifyIdentity(ctx)
}

func validateVectors(vectors [][]float32, count, dim int) error {
	if len(vectors) != count {
		return fmt.Errorf("embedding cache: provider returned %d vectors for %d texts", len(vectors), count)
	}
	for _, vector := range vectors {
		if len(vector) != dim || len(vector) == 0 {
			return fmt.Errorf("embedding cache: provider dimension %d, want %d", len(vector), dim)
		}
		for _, value := range vector {
			if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
				return fmt.Errorf("embedding cache: provider returned nonfinite value")
			}
		}
	}
	return nil
}

// Embed checks the cache for each batch entry, calls the inner
// embedder only for misses, and stores all results before returning.
func (c *Cached) Embed(ctx context.Context, batch []string) ([][]float32, error) {
	return c.embed(ctx, batch, "document", c.inner.Embed)
}

// EmbedQuery preserves asymmetric query preprocessing and keeps query vectors
// separate from passage vectors, even when their original text is identical.
func (c *Cached) EmbedQuery(ctx context.Context, batch []string) ([][]float32, error) {
	if query, ok := c.inner.(types.QueryEmbedder); ok {
		return c.embed(ctx, batch, "query", query.EmbedQuery)
	}
	return c.Embed(ctx, batch)
}

func (c *Cached) embed(ctx context.Context, batch []string, namespace string, embed func(context.Context, []string) ([][]float32, error)) ([][]float32, error) {
	if ctx == nil {
		return nil, fmt.Errorf("embedding cache: context required")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := c.VerifyIdentity(ctx); err != nil {
		return nil, err
	}
	out := make([][]float32, len(batch))
	var (
		missTexts []string
		missKeys  []string
	)
	missPositions := map[string][]int{}

	c.mu.Lock()
	for i, t := range batch {
		k := key(namespace + "\x00" + t)
		if v, ok := c.data[k]; ok {
			out[i] = append([]float32(nil), v...)
			c.touch(k)
			c.hits++
		} else {
			if len(missPositions[k]) == 0 {
				missTexts = append(missTexts, t)
				missKeys = append(missKeys, k)
			}
			missPositions[k] = append(missPositions[k], i)
			c.miss++
		}
	}
	c.mu.Unlock()

	if len(missTexts) == 0 {
		if err := c.VerifyIdentity(ctx); err != nil {
			return nil, err
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		return out, nil
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	vecs, err := embed(ctx, missTexts)
	if err != nil {
		return nil, err
	}
	if err := validateVectors(vecs, len(missTexts), c.Dimension()); err != nil {
		return nil, err
	}
	if err := c.VerifyIdentity(ctx); err != nil {
		return nil, err
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	for j, v := range vecs {
		k := missKeys[j]
		c.put(k, append([]float32(nil), v...))
		for _, i := range missPositions[k] {
			out[i] = append([]float32(nil), v...)
		}
	}
	return out, nil
}

// Stats returns hit / miss counters. Useful for /health diagnostics
// and tests asserting the cache is being exercised.
func (c *Cached) Stats() (hits, miss, size int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.hits, c.miss, len(c.data)
}

// key derives the cache key from the input text. SHA-256 keeps the
// key length bounded regardless of text size, and the avalanche
// behaviour avoids accidental collisions.
func key(text string) string {
	h := sha256.Sum256([]byte(text))
	return hex.EncodeToString(h[:])
}

// touch moves k to the most-recent position. Caller must hold c.mu.
func (c *Cached) touch(k string) {
	for i, x := range c.keys {
		if x == k {
			c.keys = append(c.keys[:i], c.keys[i+1:]...)
			break
		}
	}
	c.keys = append(c.keys, k)
}

// put inserts (or refreshes) k and evicts the oldest entry when the
// cache is full. Caller must hold c.mu.
func (c *Cached) put(k string, v []float32) {
	if _, ok := c.data[k]; ok {
		c.data[k] = v
		c.touch(k)
		return
	}
	if len(c.data) >= c.size {
		oldest := c.keys[0]
		c.keys = c.keys[1:]
		delete(c.data, oldest)
	}
	c.keys = append(c.keys, k)
	c.data[k] = v
}
