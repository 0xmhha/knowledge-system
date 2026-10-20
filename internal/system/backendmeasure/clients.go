package backendmeasure

import (
	"context"
	"encoding/json"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/internal/system/ckvclient"
	"github.com/0xmhha/knowledge-system/internal/system/composer/intent"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// WrapCKV preserves the optional flow interface. Direct flow methods are
// delegated; only get_for_task composition methods are measured here.
func WrapCKV(base ckvclient.Client) ckvclient.Client {
	v := &vector{Client: base}
	if flow, ok := base.(ckvclient.FlowClient); ok {
		return &vectorWithFlow{vector: v, FlowClient: flow}
	}
	return v
}

type vector struct{ ckvclient.Client }
type vectorWithFlow struct {
	*vector
	ckvclient.FlowClient
}

func (v *vector) SemanticSearch(ctx context.Context, query string, opts ckvclient.SearchOpts) ([]contract.Hit, error) {
	finish := begin(ctx, "ckv", "semantic_search", query, opts)
	result, err := v.Client.SemanticSearch(ctx, query, opts)
	finish(len(result), err)
	return result, err
}
func (v *vector) Health(ctx context.Context) (ckvclient.Health, error) {
	finish := begin(ctx, "ckv", "health", "", nil)
	result, err := v.Client.Health(ctx)
	finish(0, err)
	return result, err
}
func (v *vector) Freshness(ctx context.Context) (ckvclient.FreshnessReport, error) {
	finish := begin(ctx, "ckv", "freshness", "", nil)
	result, err := v.Client.Freshness(ctx)
	finish(len(result.ChangedFiles), err)
	return result, err
}

// WrapCKG preserves optional citation identity resolution. Providers without
// that capability must remain distinguishable from supported, failing ones.
func WrapCKG(base ckgclient.Client) ckgclient.Client {
	return preserveGraphCapabilities(&graph{Client: base}, base)
}

type graph struct{ ckgclient.Client }
type graphWithResolver struct {
	*graph
	resolver ckgclient.CitationResolver
}

func (g *graphWithResolver) ResolveCitation(ctx context.Context, citation contract.Citation) (ckgclient.SymbolIdentity, error) {
	input := ""
	if scope, _ := ctx.Value(scopeKey{}).(*Scope); scope != nil {
		// As with neighbors, retain a hash/length only. Identity results and
		// backend error messages never enter the measurement record.
		encoded, _ := json.Marshal(citation)
		input = string(encoded)
	}
	finish := begin(ctx, "ckg", "resolve_citation", input, nil)
	result, err := g.resolver.ResolveCitation(ctx, citation)
	count := 0
	if err == nil {
		count = 1
	}
	finish(count, err)
	return result, err
}

func (g *graph) BM25Search(ctx context.Context, query string, opts ckgclient.SearchOpts) ([]contract.Hit, error) {
	finish := begin(ctx, "ckg", "bm25_search", query, opts)
	result, err := g.Client.BM25Search(ctx, query, opts)
	finish(len(result), err)
	return result, err
}
func (g *graph) FindSymbol(ctx context.Context, name string, opts ckgclient.SymbolOpts) ([]contract.Citation, error) {
	finish := begin(ctx, "ckg", "find_symbol", name, opts)
	result, err := g.Client.FindSymbol(ctx, name, opts)
	finish(len(result), err)
	return result, err
}
func (g *graph) Neighbors(ctx context.Context, src contract.Citation, opts ckgclient.NeighborsOpts) ([]contract.Neighbor, error) {
	input := ""
	if scope, _ := ctx.Value(scopeKey{}).(*Scope); scope != nil {
		// Citation contains only strings/integers, so JSON encoding cannot
		// fail. The recorder retains its SHA/length, never the source path.
		// Avoid serialization entirely when measurement is disabled.
		encoded, _ := json.Marshal(src)
		input = string(encoded)
	}
	finish := begin(ctx, "ckg", "neighbors", input, opts)
	result, err := g.Client.Neighbors(ctx, src, opts)
	finish(len(result), err)
	return result, err
}

func (g *graph) GetNodePRs(ctx context.Context, symbol string, opts ckgclient.PRRefOpts) ([]contract.PRRef, error) {
	finish := begin(ctx, "ckg", "get_node_prs", symbol, opts)
	result, err := g.Client.GetNodePRs(ctx, symbol, opts)
	finish(len(result), err)
	return result, err
}

func (g *graph) ImpactOfChange(ctx context.Context, symbol string, opts ckgclient.ImpactOpts) (contract.ImpactResult, error) {
	finish := begin(ctx, "ckg", "impact_analysis", symbol, opts)
	result, err := g.Client.ImpactOfChange(ctx, symbol, opts)
	count := 0
	for _, group := range result.Groups {
		count += len(group.Hits)
	}
	finish(count, err)
	return result, err
}

func (g *graph) ConcurrencyImpact(ctx context.Context, symbol string, opts ckgclient.ConcurrencyOpts) (contract.ConcurrencyResult, error) {
	finish := begin(ctx, "ckg", "concurrency_impact", symbol, opts)
	result, err := g.Client.ConcurrencyImpact(ctx, symbol, opts)
	finish(len(result.Modules), err)
	return result, err
}

func (g *graph) Health(ctx context.Context) (ckgclient.Health, error) {
	finish := begin(ctx, "ckg", "health", "", nil)
	result, err := g.Client.Health(ctx)
	finish(0, err)
	return result, err
}

func WrapIntent(base intent.Embedder) intent.Embedder {
	scalar := &embedding{Embedder: base}
	if batch, ok := base.(intent.BatchEmbedder); ok {
		return &batchEmbedding{embedding: scalar, batch: batch}
	}
	return scalar
}

type embedding struct{ intent.Embedder }

type batchEmbedding struct {
	*embedding
	batch intent.BatchEmbedder
}

func (e *batchEmbedding) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	input, _ := json.Marshal(texts)
	finish := begin(ctx, "intent", "embed_batch", string(input), nil)
	result, err := e.batch.EmbedBatch(ctx, texts)
	// This count describes returned vector rows, not model inference items.
	finish(len(result), err)
	return result, err
}

func (e *embedding) Embed(ctx context.Context, text string) ([]float32, error) {
	finish := begin(ctx, "intent", "embed", text, nil)
	result, err := e.Embedder.Embed(ctx, text)
	finish(len(result), err)
	return result, err
}
