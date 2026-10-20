package query

import (
	"context"
	"errors"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/vector/types"
)

type failureEmbedder struct {
	types.Embedder
	err     error
	vectors [][]float32
	cancel  context.CancelFunc
	calls   int
}

func (e *failureEmbedder) Embed(context.Context, []string) ([][]float32, error) {
	e.calls++
	if e.cancel != nil {
		e.cancel()
	}
	return e.vectors, e.err
}

func TestEmbeddingFailurePreservesCauseAndSeparatesCallerContext(t *testing.T) {
	providerErr := errors.New("provider: source_missing is text, not a source error")
	for _, tc := range []struct {
		name    string
		err     error
		vectors [][]float32
	}{
		{"provider", providerErr, nil}, {"provider_timeout", context.DeadlineExceeded, nil},
		{"empty_batch", nil, nil}, {"empty_vector", nil, [][]float32{{}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := &failureEmbedder{err: tc.err, vectors: tc.vectors}
			_, err := (&EmbedService{emb: e}).Run(context.Background(), "query")
			if !errors.Is(err, ErrEmbeddingFailed) || (tc.err != nil && !errors.Is(err, tc.err)) {
				t.Fatalf("lost stage/cause: %v", err)
			}
		})
	}
	for _, during := range []bool{false, true} {
		ctx, cancel := context.WithCancel(context.Background())
		e := &failureEmbedder{err: providerErr}
		if during {
			e.cancel = cancel
		} else {
			cancel()
		}
		_, err := (&EmbedService{emb: e}).Run(ctx, "query")
		cancel()
		if !errors.Is(err, context.Canceled) || errors.Is(err, ErrEmbeddingFailed) {
			t.Fatalf("caller cancellation became provider outage: %v", err)
		}
		if !during && e.calls != 0 {
			t.Fatal("already canceled request reached provider")
		}
	}
}
