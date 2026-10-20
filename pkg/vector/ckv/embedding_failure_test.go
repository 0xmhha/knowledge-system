package ckv_test

import (
	"context"
	"errors"
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/vector/ckv"
	"github.com/0xmhha/knowledge-system/pkg/vector/types"
)

type switchEmbedder struct {
	types.Embedder
	fail error
}

func (e *switchEmbedder) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	if e.fail != nil {
		return nil, e.fail
	}
	return e.Embedder.Embed(ctx, texts)
}

func TestSearchSeparatesEmbeddingFromRetainedSourceFailure(t *testing.T) {
	path := buildSampleIndex(t)
	emb := &switchEmbedder{Embedder: ckv.MockEmbedder()}
	eng, err := ckv.Open(path, ckv.OpenOptions{Embedder: emb})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = eng.Close() }()
	missing := errors.New("source_missing: retained test source")
	opts := ckv.SearchOptions{K: 5, Threshold: -1, RetainedSource: true, SourceReader: func(context.Context, string) ([]byte, error) { return nil, missing }}
	_, err = eng.SemanticSearch(context.Background(), "server listen", opts)
	if !errors.Is(err, missing) || errors.Is(err, ckv.ErrEmbeddingFailed) {
		t.Fatalf("source classified as embed failure: %v", err)
	}
	emb.fail = context.DeadlineExceeded
	_, err = eng.SemanticSearch(context.Background(), "server listen", opts)
	if !errors.Is(err, ckv.ErrEmbeddingFailed) || !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("lost provider timeout cause: %v", err)
	}
	emb.fail = nil
	if _, err = eng.SemanticSearch(context.Background(), "server listen", ckv.SearchOptions{K: 5, Threshold: -1}); err != nil {
		t.Fatalf("search did not recover: %v", err)
	}
}
