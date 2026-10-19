package ckvclient

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/0xmhha/knowledge-system/pkg/vector/ckv"
	"github.com/0xmhha/knowledge-system/pkg/vector/embed/cache"
	"github.com/0xmhha/knowledge-system/pkg/vector/types"
)

type healthEmbedder struct {
	types.Embedder
	calls  int
	err    error
	cancel context.CancelFunc
	empty  bool
}

func (e *healthEmbedder) Embed(context.Context, []string) ([][]float32, error) {
	e.calls++
	if e.cancel != nil {
		e.cancel()
	}
	if e.empty {
		return nil, e.err
	}
	if e.Embedder != nil {
		vector := make([]float32, e.Dimension())
		vector[0] = 1
		return [][]float32{vector}, e.err
	}
	return [][]float32{{1}}, e.err
}

func TestModelHealthDoesNotReuseCachedLivenessVector(t *testing.T) {
	inner := &healthEmbedder{Embedder: ckv.MockEmbedder()}
	cached := cache.New(inner, 2)
	if _, err := cached.Embed(context.Background(), []string{"liveness probe"}); err != nil {
		t.Fatal(err)
	}
	r := &Real{emb: cached}
	inner.err = errors.New("offline")
	if up, _ := r.probeModel(context.Background()); up || inner.calls != 2 {
		t.Fatal("cached probe masked unavailable model", inner.calls)
	}
	previousProbe := r.lastProbe
	r.recordQueryResult(context.Background(), nil)
	if up, _ := r.probeModel(context.Background()); up || r.lastProbe != previousProbe || inner.calls != 2 {
		t.Fatal("successful cached query cleared or renewed model-health evidence")
	}
	inner.err = nil
	expireProbe(r)
	if up, _ := r.probeModel(context.Background()); !up || inner.calls != 3 {
		t.Fatal("cache-aware health could not recover", inner.calls)
	}
}
func expireProbe(r *Real) {
	r.probeMu.Lock()
	defer r.probeMu.Unlock()
	r.lastProbe = time.Now().Add(-modelProbeTTL - time.Second)
}

func TestModelHealthSourceErrorsDoNotPoisonOrClearModelEvidence(t *testing.T) {
	e := &healthEmbedder{}
	r := &Real{emb: e}
	if up, _ := r.probeModel(context.Background()); !up {
		t.Fatal("initial healthy probe")
	}
	for _, err := range []error{errors.New("source_missing"), errors.New("snapshot_mismatch"), ckv.ErrCitationNotFound, ckv.ErrBudgetExceeded, errors.New("embedding failed: untyped text"), context.Canceled} {
		r.recordQueryResult(context.Background(), err)
		if up, _ := r.probeModel(context.Background()); !up || e.calls != 1 {
			t.Fatalf("non-model error poisoned health: %v calls=%d", err, e.calls)
		}
	}
	r.recordQueryResult(context.Background(), fmt.Errorf("wrapped: %w", ckv.ErrEmbeddingFailed))
	r.recordQueryResult(context.Background(), errors.New("source_missing"))
	if up, _ := r.probeModel(context.Background()); up || e.calls != 1 {
		t.Fatal("source error erased negative evidence")
	}
}

func TestModelHealthRecoversByTTLProbeAndSuccessfulQuery(t *testing.T) {
	e := &healthEmbedder{}
	r := &Real{emb: e}
	r.recordQueryResult(context.Background(), ckv.ErrEmbeddingFailed)
	if up, _ := r.probeModel(context.Background()); up || e.calls != 0 {
		t.Fatal("outage not immediately cached")
	}
	expireProbe(r)
	if up, _ := r.probeModel(context.Background()); !up || e.calls != 1 {
		t.Fatal("health-only recovery remained latched down")
	}
	e.err = errors.New("offline")
	expireProbe(r)
	if up, _ := r.probeModel(context.Background()); up {
		t.Fatal("failed probe reported up")
	}
	e.err = nil
	r.recordQueryResult(context.Background(), nil)
	if up, _ := r.probeModel(context.Background()); !up || e.calls != 2 {
		t.Fatal("successful query did not clear negative cache immediately")
	}
}

func TestModelHealthCancellationAndEmptyProbe(t *testing.T) {
	e := &healthEmbedder{}
	r := &Real{emb: e}
	r.recordQueryResult(context.Background(), nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r.recordQueryResult(ctx, ckv.ErrEmbeddingFailed)
	if up, _ := r.probeModel(context.Background()); !up {
		t.Fatal("canceled query poisoned global health")
	}
	expireProbe(r)
	ctx, cancel = context.WithCancel(context.Background())
	e.cancel = cancel
	if up, _ := r.probeModel(ctx); up {
		t.Fatal("canceled probe reported fresh success")
	}
	cancel()
	e.cancel = nil
	if up, _ := r.probeModel(context.Background()); !up || e.calls != 2 {
		t.Fatal("canceled probe persisted as an outage")
	}
	e.empty = true
	expireProbe(r)
	if up, _ := r.probeModel(context.Background()); up {
		t.Fatal("empty probe response reported usable model")
	}
}

func TestModelHealthConcurrentProbesShareOneCachedObservation(t *testing.T) {
	e := &healthEmbedder{}
	r := &Real{emb: e}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if up, _ := r.probeModel(context.Background()); !up {
				t.Error("healthy model reported down")
			}
		}()
	}
	wg.Wait()
	if e.calls != 1 {
		t.Fatalf("concurrent probes=%d want1", e.calls)
	}
}
