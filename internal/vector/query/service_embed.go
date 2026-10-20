package query

import (
	"context"
	"fmt"

	"github.com/0xmhha/knowledge-system/pkg/vector/types"
)

// EmbedService converts text into a vector using the configured embedder.
// Can be called independently via Engine.Embed() or as part of the
// search pipeline.
type EmbedService struct {
	emb types.Embedder
}

// Run embeds the given text and returns the vector.
func (s *EmbedService) Run(ctx context.Context, text string) ([]float32, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if s.emb == nil {
		return nil, fmt.Errorf("%w: no embedder configured", ErrEmbeddingFailed)
	}
	vecs, err := embedQueryBatch(ctx, s.emb, []string{text})
	if cancelErr := ctx.Err(); cancelErr != nil {
		return nil, cancelErr
	}
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrEmbeddingFailed, err)
	}
	if len(vecs) == 0 || len(vecs[0]) == 0 {
		return nil, fmt.Errorf("%w: embedder returned no vector", ErrEmbeddingFailed)
	}
	return vecs[0], nil
}

// embedQueryBatch embeds query texts, using the embedder's query-specific path
// (types.QueryEmbedder.EmbedQuery — e.g. Qwen3's "Instruct:" prefix) when the
// embedder implements it, and falling back to Embed for symmetric models with
// no query/passage asymmetry.
func embedQueryBatch(ctx context.Context, emb types.Embedder, texts []string) ([][]float32, error) {
	if qe, ok := emb.(types.QueryEmbedder); ok {
		return qe.EmbedQuery(ctx, texts)
	}
	return emb.Embed(ctx, texts)
}

// RunContext populates sc.QueryVec from sc.EmbedIntent.
func (s *EmbedService) RunContext(ctx context.Context, sc *SearchContext) error {
	vec, err := s.Run(ctx, sc.EmbedIntent)
	if err != nil {
		return err
	}
	sc.QueryVec = vec
	return nil
}
