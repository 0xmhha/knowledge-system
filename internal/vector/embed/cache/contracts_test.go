package cache

import (
	"context"
	"errors"
	"math"
	"reflect"
	"sync"
	"testing"
)

type scriptedEmbedder struct {
	fakeEmbedder
	embed func(context.Context, []string) ([][]float32, error)
}

type verifiedEmbedder struct {
	fakeEmbedder
	verifyError   error
	verifications int
}

func (v *verifiedEmbedder) VerifyIdentity(context.Context) error {
	v.verifications++
	return v.verifyError
}

func TestCacheForwardsIdentityVerificationOnCachedReads(t *testing.T) {
	inner := &verifiedEmbedder{fakeEmbedder: *mkEmb()}
	c := New(inner, 2)
	if _, err := c.Embed(context.Background(), []string{"cached"}); err != nil {
		t.Fatal(err)
	}
	inner.verifyError = errors.New("model replaced")
	if _, err := c.Embed(context.Background(), []string{"cached"}); err == nil {
		t.Fatal("cached vector concealed model replacement")
	}
	if err := c.VerifyIdentity(context.Background()); err == nil {
		t.Fatal("publication identity check not forwarded")
	}
	if err := c.ProbeEmbedding(context.Background()); err == nil {
		t.Fatal("liveness probe concealed model identity replacement")
	}
	inner.verifyError = nil
	if _, err := c.Embed(context.Background(), []string{"cached"}); err != nil || inner.calls != 1 {
		t.Fatal("verified old model did not recover with cached vector", err)
	}
}

func TestCacheLivenessProbeBypassesStoredProbeVector(t *testing.T) {
	inner := &scriptedEmbedder{fakeEmbedder: *mkEmb()}
	down := false
	calls := 0
	inner.embed = func(_ context.Context, texts []string) ([][]float32, error) {
		calls++
		if down {
			return nil, errors.New("inference unavailable")
		}
		return [][]float32{{1, 2, 3, 4}}, nil
	}
	c := New(inner, 2)
	if _, err := c.Embed(context.Background(), []string{"liveness probe"}); err != nil {
		t.Fatal(err)
	}
	down = true
	if err := c.ProbeEmbedding(context.Background()); err == nil {
		t.Fatal("cached probe concealed inference failure")
	}
	down = false
	if err := c.ProbeEmbedding(context.Background()); err != nil || calls != 3 {
		t.Fatal("fresh inference recovery not observed", err, calls)
	}
}

type asymmetricEmbedder struct {
	fakeEmbedder
	queryCalls int
}

func (a *asymmetricEmbedder) EmbedQuery(_ context.Context, texts []string) ([][]float32, error) {
	a.queryCalls += len(texts)
	out := make([][]float32, len(texts))
	for i := range out {
		out[i] = []float32{9, 0, 0, 0}
	}
	return out, nil
}

func TestCachePreservesAsymmetricQueryAndPassageSpaces(t *testing.T) {
	inner := &asymmetricEmbedder{fakeEmbedder: *mkEmb()}
	c := New(inner, 4)
	for range 2 {
		doc, err := c.Embed(context.Background(), []string{"x"})
		if err != nil {
			t.Fatal(err)
		}
		query, err := c.EmbedQuery(context.Background(), []string{"x"})
		if err != nil {
			t.Fatal(err)
		}
		if doc[0][0] != 1 || query[0][0] != 9 {
			t.Fatal("query preprocessing lost or passage/query cache collided", doc, query)
		}
	}
	if inner.calls != 1 || inner.queryCalls != 1 {
		t.Fatal("separate embedding paths not cached", inner.calls, inner.queryCalls)
	}
}

func (s *scriptedEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	return s.embed(ctx, texts)
}

func TestCacheOwnsVectorsAndDeduplicatesBatch(t *testing.T) {
	providerVector := []float32{1, 2, 3, 4}
	calls := 0
	inner := &scriptedEmbedder{fakeEmbedder: *mkEmb(), embed: func(_ context.Context, texts []string) ([][]float32, error) {
		calls += len(texts)
		return [][]float32{providerVector}, nil
	}}
	c := New(inner, 2)
	first, err := c.Embed(context.Background(), []string{"same", "same"})
	if err != nil {
		t.Fatal(err)
	}
	first[0][0] = 99
	providerVector[1] = 99
	if first[1][0] != 1 {
		t.Fatal("duplicate output aliases another caller slot")
	}
	second, err := c.Embed(context.Background(), []string{"same"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(second[0], []float32{1, 2, 3, 4}) || calls != 1 {
		t.Fatal("provider/caller mutation corrupted cache or duplicate miss", second, calls)
	}
	second[0][2] = 99
	third, _ := c.Embed(context.Background(), []string{"same"})
	if third[0][2] != 3 {
		t.Fatal("cache hit aliases caller vector")
	}
	if c.Identity() != inner.Identity() {
		t.Fatal("cache changed embedding space")
	}
}

func TestCacheRejectsMalformedProviderBatchWithoutPartialInsertion(t *testing.T) {
	for _, reply := range [][][]float32{nil, {{1, 2, 3, 4}, {1, 2, 3, 4}}, {{1}}, {{1, 2, 3, float32(math.NaN())}}, {{1, 2, 3, float32(math.Inf(1))}}} {
		inner := &scriptedEmbedder{fakeEmbedder: *mkEmb(), embed: func(context.Context, []string) ([][]float32, error) { return reply, nil }}
		c := New(inner, 2)
		if _, err := c.Embed(context.Background(), []string{"bad"}); err == nil {
			t.Fatal("invalid provider output accepted", reply)
		}
		_, _, size := c.Stats()
		if size != 0 {
			t.Fatal("invalid output cached")
		}
	}
}

func TestCacheCancellationAndProviderErrorsDoNotPopulate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	inner := mkEmb()
	c := New(inner, 2)
	if _, err := c.Embed(ctx, []string{"x"}); !errors.Is(err, context.Canceled) || inner.calls != 0 {
		t.Fatal("canceled request reached provider", err)
	}
	ctx, cancel = context.WithCancel(context.Background())
	s := &scriptedEmbedder{fakeEmbedder: *mkEmb(), embed: func(context.Context, []string) ([][]float32, error) { cancel(); return [][]float32{{1, 2, 3, 4}}, nil }}
	c = New(s, 2)
	if _, err := c.Embed(ctx, []string{"x"}); !errors.Is(err, context.Canceled) {
		t.Fatal("provider cancellation not observed", err)
	}
	_, _, size := c.Stats()
	if size != 0 {
		t.Fatal("canceled response cached")
	}
	s.embed = func(context.Context, []string) ([][]float32, error) { return nil, errors.New("provider unavailable") }
	if _, err := c.Embed(context.Background(), []string{"x"}); err == nil {
		t.Fatal("provider error hidden")
	}
	_, _, size = c.Stats()
	if size != 0 {
		t.Fatal("provider failure cached")
	}
}

func TestCacheConcurrentCallersCannotMutateStoredVectors(t *testing.T) {
	s := &scriptedEmbedder{fakeEmbedder: *mkEmb(), embed: func(_ context.Context, texts []string) ([][]float32, error) {
		out := make([][]float32, len(texts))
		for i := range out {
			out[i] = []float32{1, 2, 3, 4}
		}
		return out, nil
	}}
	c := New(s, 2)
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			for range 10 {
				out, err := c.Embed(context.Background(), []string{"shared"})
				if err != nil {
					t.Error(err)
					return
				}
				if out[0][0] != 1 {
					t.Error("shared vector mutated")
				}
				out[0][0] = 99
			}
		})
	}
	wg.Wait()
	_, _, size := c.Stats()
	if size != 1 {
		t.Fatal("cache population", size)
	}
}
