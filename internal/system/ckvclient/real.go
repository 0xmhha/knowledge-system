package ckvclient

import (
	"context"
	"errors"
	"fmt"
	"github.com/0xmhha/knowledge-system/pkg/sourcepath"
	"sync"
	"time"

	"github.com/0xmhha/knowledge-system/pkg/vector/ckv"
	ckvtypes "github.com/0xmhha/knowledge-system/pkg/vector/types"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// DefaultK is the SemanticSearch limit when SearchOpts.K is zero.
const DefaultK = 10

// modelProbeTTL bounds how often Health issues a fresh embedding-model
// liveness probe. Health is a fast, frequently-polled signal, so we cache
// the probe result for this window instead of embedding on every call; a
// failed embedding request records a negative result immediately. Both
// positive and negative results expire so Health can discover recovery.
const modelProbeTTL = 5 * time.Second

// RealOpts configures the in-process ckv adapter. The old subprocess
// fields (BinaryPath / Env / CallTimeout) are gone — cks imports pkg/ckv
// directly, so there is no child process to spawn or restart.
type RealOpts struct {
	// DataPath is the ckv-data directory (vector.db + manifest.json)
	// produced by `ckv build`. Passed to ckv.Open.
	DataPath string
	// SourceRoot overrides the manifest's logical path only for citation
	// enforcement. A mounted retained tree keeps queries independent of the
	// mutable checkout; empty preserves the legacy manifest-root behavior.
	SourceRoot     string
	RetainedSource bool
	SourceReader   sourcepath.Reader

	// Embedder must match the embedder the index was built with (same
	// name + dimension) or ckv.Open returns ErrIndexUnavailable. The
	// caller constructs it once (typically the Ollama bge-m3 adapter)
	// and shares the same instance with the intent classifier so anchor,
	// query, and chunk vectors live in one model space.
	Embedder ckvtypes.Embedder
}

// Real is the in-process ckv adapter. It holds an open *ckv.Engine and
// translates ckv.Hit / Manifest / FreshnessReport into cks contract types.
//
// No subprocess, no MCP transport: the 543-LOC proxy this replaced spawned
// the ckv binary and proxied every query over stdio, which hung 2/9
// self-evaluation runs. ckv.Engine is concurrent-safe for reads, so Real needs no locking.
type Real struct {
	eng            *ckv.Engine
	emb            ckvtypes.Embedder
	sourceRoot     string
	retainedSource bool
	sourceReader   sourcepath.Reader
	closed         bool

	// probe* cache the most recent embedding-model liveness probe under a
	// modelProbeTTL window (see probeModel).
	probeMu         sync.Mutex
	lastProbe       time.Time
	lastProbeOK     bool
	lastProbeReason string
}

// Compile-time guarantee Real satisfies Client.
var _ Client = (*Real)(nil)

// NewReal opens the ckv index at opts.DataPath with opts.Embedder, in
// process. It fails fast when DataPath is empty, the Embedder is nil,
// or the embedder can't serve the index (identity mismatch) — the caller
// (cmd/cks-mcp) catches the error and falls back to the Smart Dummy plus a
// degraded cks.ops.health status rather than crashing.
func NewReal(ctx context.Context, opts RealOpts) (*Real, error) {
	if opts.DataPath == "" {
		return nil, fmt.Errorf("ckvclient: empty DataPath")
	}
	if opts.Embedder == nil {
		return nil, fmt.Errorf("ckvclient: nil Embedder")
	}
	eng, err := ckv.Open(opts.DataPath, ckv.OpenOptions{Embedder: opts.Embedder})
	if err != nil {
		return nil, fmt.Errorf("ckvclient: ckv.Open %q: %w", opts.DataPath, err)
	}
	// The caller (cmd/cks-mcp) only constructs Real after buildEmbedder
	// probed the model successfully, so the model starts known-up.
	return &Real{eng: eng, emb: opts.Embedder, sourceRoot: opts.SourceRoot,
		retainedSource: opts.RetainedSource, sourceReader: opts.SourceReader, lastProbeOK: true}, nil
}

// SemanticSearch runs an in-process vector query and translates each
// ckv.Hit into a contract.Hit stamped HitSourceCKV. The score is ckv's
// normalized distance (Score.Normalized, in [0,1]).
//
// Filter push-down (Language/PathGlob/SymbolKinds) is not yet mapped onto
// ckv.SearchOptions — filter fields (language/path/symbol-kind/commit/
// chunk-kind) thread straight through to ckv's pre-filter.
func (r *Real) SemanticSearch(ctx context.Context, query string, opts SearchOpts) ([]contract.Hit, error) {
	if query == "" {
		return nil, fmt.Errorf("ckvclient: empty query")
	}
	k := opts.K
	if k <= 0 {
		k = DefaultK
	}
	filter := ckvtypes.Filter{
		Language:     opts.Filter.Language,
		PathGlob:     opts.Filter.PathGlob,
		CommitHash:   opts.Filter.CommitHash,
		ExcludeTests: opts.Filter.ExcludeTests,
	}
	for _, sk := range opts.Filter.SymbolKinds {
		filter.SymbolKinds = append(filter.SymbolKinds, ckvtypes.SymbolKind(sk))
	}
	for _, ck := range opts.Filter.ChunkKinds {
		filter.ChunkKinds = append(filter.ChunkKinds, ckvtypes.ChunkKind(ck))
	}
	resp, err := r.eng.SemanticSearch(ctx, query, ckv.SearchOptions{K: k, Filter: filter,
		SrcRoot: r.sourceRoot, RetainedSource: r.retainedSource, SourceReader: r.sourceReader, EnableBM25Rerank: opts.BM25Rerank})
	r.recordQueryResult(ctx, err)
	if err != nil {
		return nil, fmt.Errorf("ckvclient: SemanticSearch: %w", err)
	}
	if resp == nil {
		return nil, nil
	}
	out := make([]contract.Hit, 0, len(resp.Hits))
	for i, h := range resp.Hits {
		var parent *contract.Citation
		if h.ParentCitation != nil {
			parent = &contract.Citation{File: h.ParentCitation.File, StartLine: h.ParentCitation.StartLine, EndLine: h.ParentCitation.EndLine, CommitHash: h.ParentCitation.CommitHash}
		}
		// Symbol / CanonicalID are the composer's bridge to ckg: Stage 1
		// extracts hit.Symbol as a candidate keyword (instead of the file
		// basename — the basename fallback survives in extractKeywords for
		// hits with empty Symbol); CanonicalID is the build-stable ckv↔ckg join
		// key (ADR-0001) the chunk inherited from the aligned ckg node,
		// resolvable via ckg's canonical-first FindSymbol. Empty values are
		// fine (omitempty on the wire); they only mean the ckv chunk lacked
		// the metadata (e.g. doc/header chunks).
		out = append(out, contract.Hit{
			NonCitableMetadata: h.ChunkKind == ckvtypes.ChunkConvention || h.Citation.StartLine < 1 || h.Citation.EndLine < h.Citation.StartLine,
			Text:               h.Snippet,
			ParentCitation:     parent,
			HeadingPath:        h.HeadingPath,
			Citation: contract.Citation{
				File:       h.Citation.File,
				StartLine:  h.Citation.StartLine,
				EndLine:    h.Citation.EndLine,
				CommitHash: h.Citation.CommitHash,
			},
			Rank:        i + 1,
			Score:       h.Score.Normalized,
			Source:      contract.HitSourceCKV,
			Symbol:      h.Symbol,
			ChunkKind:   string(h.ChunkKind),
			CanonicalID: h.CanonicalID,
		})
	}
	return out, nil
}

// Freshness reports index-vs-HEAD staleness via ckv's structured
// Engine.Freshness. Git unavailability is reported by ckv as
// Fresh=false with warnings, not as an error here.
func (r *Real) Freshness(ctx context.Context) (FreshnessReport, error) {
	rep, err := r.eng.Freshness()
	if err != nil {
		return FreshnessReport{}, fmt.Errorf("ckvclient: freshness: %w", err)
	}
	return FreshnessReport{
		Fresh:        rep.Fresh,
		IndexedHead:  rep.IndexedHead,
		CurrentHead:  rep.CurrentHead,
		ChangedFiles: rep.ChangedFiles,
	}, nil
}

// Health reports two distinct signals: index identity (the engine is open,
// Reachable=true, with StatsHash/IndexedHead = the indexed git head and
// LastIndexAt from the manifest) and embedding-model liveness
// (ModelReachable, via probeModel). Separating them lets cks.ops.health tell
// "index loaded but model down" apart from "fully healthy" — the runtime
// disconnect the old manifest-only Health silently reported as reachable.
// DocsRoots returns the `ckv build --docs` corpus roots recorded in the
// index manifest. The composer's body fetcher resolves doc/markdown
// citations against these — they live outside the code source_root, so
// without them domain-corpus chunks have no body and get dropped.
func (r *Real) DocsRoots() []string {
	return r.eng.Manifest().DocsRoots
}

func (r *Real) Health(ctx context.Context) (Health, error) {
	man := r.eng.Manifest()
	modelUp, reason := r.probeModel(ctx)
	h := Health{
		ProjectID: man.ProjectID, DatasetID: man.DatasetID, SnapshotID: man.SnapshotID, SourceMode: man.SourceMode,
		Reachable:      true,
		ModelReachable: modelUp,
		StatsHash:      man.IndexedHead,
		IndexedHead:    man.IndexedHead,
		Reason:         reason,
	}
	if man.BuiltAt != "" {
		if t, perr := time.Parse(time.RFC3339, man.BuiltAt); perr == nil {
			h.LastIndexAt = t
		}
	}
	return h, nil
}

// recordQueryResult changes model health only on a successful search or a
// typed embedding failure. Source/store/budget failures supply no model-health
// evidence; cancellation belongs to the caller, not to shared backend state.
func (r *Real) recordQueryResult(ctx context.Context, err error) {
	if ctx.Err() != nil || (err != nil && !errors.Is(err, ckv.ErrEmbeddingFailed)) {
		return
	}
	r.probeMu.Lock()
	defer r.probeMu.Unlock()
	r.lastProbe = time.Now()
	r.lastProbeOK = err == nil
	r.lastProbeReason = ""
	if err != nil {
		r.lastProbeReason = "query embedding failed"
	}
}

// probeModel caches either outcome for modelProbeTTL. A negative result must
// expire too: otherwise serviceability-gated v2 callers could never recover
// without an out-of-band successful raw search or a restart.
func (r *Real) probeModel(ctx context.Context) (bool, string) {
	r.probeMu.Lock()
	defer r.probeMu.Unlock()
	if !r.lastProbe.IsZero() && time.Since(r.lastProbe) < modelProbeTTL {
		return r.lastProbeOK, r.lastProbeReason
	}
	vectors, err := r.emb.Embed(ctx, []string{"liveness probe"})
	if ctx.Err() != nil {
		return false, "embedding model probe canceled by caller"
	}
	if err == nil && (len(vectors) == 0 || len(vectors[0]) == 0) {
		err = ckv.ErrEmbeddingFailed
	}
	r.lastProbe = time.Now()
	if err != nil {
		r.lastProbeOK = false
		r.lastProbeReason = "embedding model probe failed"
	} else {
		r.lastProbeOK = true
		r.lastProbeReason = ""
	}
	return r.lastProbeOK, r.lastProbeReason
}

// Close releases the underlying ckv engine. Idempotent.
func (r *Real) Close() error {
	if r.closed || r.eng == nil {
		return nil
	}
	r.closed = true
	return r.eng.Close()
}
