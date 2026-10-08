// Package ollama implements the Embedder interface via Ollama's HTTP
// API. Useful when ONNX model files are unavailable (e.g. HuggingFace
// blocked) or when the user prefers Ollama's model management.
//
// Prerequisites: Ollama running locally (ollama serve) with the
// desired model pulled (ollama pull bge-m3).
//
// Usage:
//
//	ckv build --embedder=ollama --model-name=bge-m3 --src ./project
package ollama

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/0xmhha/knowledge-system/internal/vector/embed/registry"
	"github.com/0xmhha/knowledge-system/pkg/vector/types"
)

// DefaultEndpoint is the default Ollama API base URL.
// Override with CKV_OLLAMA_ENDPOINT environment variable.
const DefaultEndpoint = "http://localhost:11434"

// DefaultTimeout bounds every request to the Ollama daemon. Without it a
// wedged daemon (model loading, stuck GPU) that accepts the connection but
// never responds would block embed calls — and the startup probe — forever,
// hanging the build, the query path, and any consumer that opens the adapter
// at startup. A single embed of a chunk batch should be well under this.
const DefaultTimeout = 60 * time.Second

// DefaultMaxInputTokens is the fallback context limit for an Ollama model not
// present in the embedding registry. bge-m3's value; safe for BERT-family
// embedders that don't advertise a larger window.
const DefaultMaxInputTokens = 8192

// bgeM3ChunkBudgetBytes leaves room for the contextual prefix and tokenizer
// overhead within the 8192-token Ollama window. The byte bound is deliberately
// conservative: a token cannot consume less than one UTF-8 input byte.
const bgeM3ChunkBudgetBytes = 6144

// Adapter implements types.Embedder via Ollama's /api/embed endpoint.
type Adapter struct {
	endpoint       string
	modelName      string
	modelDigest    string
	dim            int
	nativeDim      int
	targetDim      int    // >0 → truncate each embedding to this many dims (MRL)
	queryInstruct  string // non-empty → wrap queries in this Qwen3 instruct prompt
	maxInput       int
	runtimeOptions *embedRuntimeOptions
	client         *http.Client
}

// Options configures the Ollama adapter.
type Options struct {
	// ObserveHTTP is optional telemetry, excluded from embedding identity.
	ObserveHTTP HTTPObserver
	Endpoint    string        // Ollama API URL (default: http://localhost:11434)
	ModelName   string        // model name as known to Ollama (e.g. "bge-m3")
	Timeout     time.Duration // per-request timeout (default: DefaultTimeout); <=0 uses the default
	// TargetDim, when >0 and smaller than the model's native dimension,
	// truncates every embedding to its first TargetDim components and
	// re-normalizes to unit length (Matryoshka Representation Learning). Used
	// by Qwen3-Embedding, which is MRL-trained, to trade a little precision for
	// a smaller vector (storage + search cost). 0 keeps the native dimension.
	TargetDim int
	// QueryPrefixPolicy is "registry" (default) or "none". The legacy
	// CKV_DISABLE_QUERY_PREFIX variable is read once only when unset.
	QueryPrefixPolicy string
}

// Open pins an exact model tag and digest, then embeds a probe to determine
// native dimension. Model identity is checked again around every batch.
func Open(opts Options) (*Adapter, error) {
	endpoint := opts.Endpoint
	if endpoint == "" {
		endpoint = os.Getenv("CKV_OLLAMA_ENDPOINT")
	}
	if endpoint == "" {
		endpoint = DefaultEndpoint
	}
	if opts.ModelName == "" {
		return nil, fmt.Errorf("ollama: model name is required (--model-name)")
	}

	timeout := opts.Timeout
	if timeout <= 0 {
		timeout = DefaultTimeout
	}

	policy := opts.QueryPrefixPolicy
	if policy == "" {
		policy = "registry"
		if os.Getenv("CKV_DISABLE_QUERY_PREFIX") == "1" {
			policy = "none"
		}
	}
	if policy != "registry" && policy != "none" {
		return nil, fmt.Errorf("ollama: invalid query prefix policy %q", policy)
	}
	queryInstruct := registry.QueryInstruct(opts.ModelName)
	if policy == "none" {
		queryInstruct = "" // opt out of the asymmetric query prompt (A/B, debugging)
	}
	a := &Adapter{
		endpoint:       strings.TrimRight(endpoint, "/"),
		modelName:      opts.ModelName,
		queryInstruct:  queryInstruct,
		maxInput:       resolveMaxInput(opts.ModelName),
		runtimeOptions: resolveRuntimeOptions(opts.ModelName),
		client:         &http.Client{Timeout: timeout},
	}
	if opts.ObserveHTTP != nil {
		a.client.Transport = &observedTransport{base: http.DefaultTransport, observe: opts.ObserveHTTP}
	}

	// Probe: embed a short string to discover the dimension. Bound it with a
	// context deadline too, so a wedged daemon fails fast at startup instead
	// of stalling the consumer that opened the adapter.
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	canonicalName, digest, err := a.resolveModel(ctx)
	if err != nil {
		return nil, fmt.Errorf("ollama: model identity: %w", err)
	}
	a.modelName, a.modelDigest = canonicalName, digest
	vecs, err := a.Embed(ctx, []string{"dimension probe"})
	if err != nil {
		return nil, fmt.Errorf("ollama: connectivity check failed: %w", err)
	}
	if len(vecs) == 0 || len(vecs[0]) == 0 {
		return nil, fmt.Errorf("ollama: model %q returned empty embedding", opts.ModelName)
	}
	nativeDim := len(vecs[0])
	a.dim = nativeDim
	a.nativeDim = nativeDim

	// MRL truncation: the probe above ran with targetDim unset (native), so
	// nativeDim is authoritative for validation. Enabling it here makes every
	// subsequent Embed truncate + renormalize, and reports the reduced
	// dimension via Dimension()/Identity().
	if err := validateTargetDim(opts.ModelName, opts.TargetDim, nativeDim); err != nil {
		return nil, err
	}
	if opts.TargetDim > 0 {
		a.targetDim = opts.TargetDim
		a.dim = opts.TargetDim
	}
	if err := a.VerifyIdentity(ctx); err != nil {
		return nil, fmt.Errorf("ollama: model changed during probe: %w", err)
	}

	return a, nil
}

// httpClient returns the adapter's timeout-bound client, falling back to a
// default-timeout client when the Adapter was constructed directly (e.g. in
// tests) rather than via Open — so no request is ever unbounded.
func (a *Adapter) httpClient() *http.Client {
	if a.client != nil {
		return a.client
	}
	return &http.Client{Timeout: DefaultTimeout}
}

func (a *Adapter) Name() string   { return a.modelName }
func (a *Adapter) Dimension() int { return a.dim }
func (a *Adapter) MaxInputTokens() int {
	if a.maxInput > 0 {
		return a.maxInput
	}
	return DefaultMaxInputTokens
}

// resolveMaxInput returns the model's context limit. It honors the registry's
// per-model MaxInput (matching the bgeonnx backend) so swapping the embedding
// model carries the right truncation budget; models Ollama serves but the
// registry does not know fall back to DefaultMaxInputTokens.
func resolveMaxInput(modelName string) int {
	if cfg, err := registry.Lookup(modelName); err == nil && cfg.MaxInput > 0 {
		return cfg.MaxInput
	}
	return DefaultMaxInputTokens
}

// BGE-M3 advertises an 8K context, but Ollama can load it with a smaller
// physical batch even when num_ctx is 8192. In that case truncate:false can
// reject valid inputs around 2048 tokens. Pin both settings for this model;
// keep other models on their existing runtime defaults.
func resolveRuntimeOptions(modelName string) *embedRuntimeOptions {
	if strings.SplitN(modelName, ":", 2)[0] == "bge-m3" {
		return &embedRuntimeOptions{NumCtx: 8192, NumBatch: 8192}
	}
	return nil
}

// Identity includes the Ollama digest, dimension method and query transform.
// Ollama does not expose pooling details, so Pooling remains empty.
func (a *Adapter) Identity() types.EmbeddingIdentity {
	queryTransform := "raw:v1"
	if a.queryInstruct != "" {
		sum := sha256.Sum256([]byte(a.queryInstruct))
		queryTransform = "qwen3-instruct:v1:sha256:" + hex.EncodeToString(sum[:])
	}
	method := "native:v1"
	if a.targetDim > 0 {
		method = "mrl-prefix-l2:v1"
	}
	return types.EmbeddingIdentity{
		Provider:             "ollama",
		Model:                a.modelName,
		Dim:                  a.dim,
		Version:              2,
		ModelDigest:          a.modelDigest,
		NativeDim:            a.nativeDim,
		DimensionMethod:      method,
		PassageTransform:     "raw:v1",
		QueryTransform:       queryTransform,
		TruncatePolicy:       "reject:v1",
		RuntimeContextTokens: a.runtimeContextTokens(),
		RuntimeBatchTokens:   a.runtimeBatchTokens(),
		ChunkBudgetBytes:     a.chunkBudgetBytes(),
	}
}

func (a *Adapter) chunkBudgetBytes() int {
	if strings.SplitN(a.modelName, ":", 2)[0] == "bge-m3" {
		return bgeM3ChunkBudgetBytes
	}
	return 0
}

func (a *Adapter) runtimeContextTokens() int {
	if a.runtimeOptions != nil {
		return a.runtimeOptions.NumCtx
	}
	return 0
}

func (a *Adapter) runtimeBatchTokens() int {
	if a.runtimeOptions != nil {
		return a.runtimeOptions.NumBatch
	}
	return 0
}
func (a *Adapter) Close() error { return nil }

// Embed calls Ollama's /api/embed endpoint with batch input.
func (a *Adapter) Embed(ctx context.Context, batch []string) ([][]float32, error) {
	if len(batch) == 0 {
		return nil, nil
	}
	if a.modelDigest != "" {
		if err := a.VerifyIdentity(ctx); err != nil {
			return nil, err
		}
	}

	reqBody := embedRequest{
		Model:    a.modelName,
		Input:    batch,
		Truncate: false,
		Options:  a.runtimeOptions,
	}
	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("ollama: marshal request: %w", err)
	}

	url := a.endpoint + "/api/embed"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ollama: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.httpClient().Do(req)
	if err != nil {
		return nil, fmt.Errorf("ollama: HTTP request to %s failed: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("ollama: HTTP %d from embedding endpoint", resp.StatusCode)
	}

	var result embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("ollama: decode response: %w", err)
	}

	// Validate the response shape at the boundary: a short response would
	// otherwise be paired positionally with the wrong chunks downstream,
	// surfacing as a confusing error far from the cause.
	if len(result.Embeddings) != len(batch) {
		return nil, fmt.Errorf("ollama: embedding count mismatch: got %d for %d inputs from %s",
			len(result.Embeddings), len(batch), url)
	}
	if a.modelDigest != "" && result.Model != a.modelName {
		return nil, fmt.Errorf("ollama: embedding response model mismatch: expected %q, got %q", a.modelName, result.Model)
	}
	for i, vec := range result.Embeddings {
		if len(vec) == 0 || (a.nativeDim > 0 && len(vec) != a.nativeDim) {
			return nil, fmt.Errorf("ollama: embedding %d dimension mismatch: got %d, expected %d", i, len(vec), a.nativeDim)
		}
		for _, v := range vec {
			if math.IsNaN(float64(v)) || math.IsInf(float64(v), 0) {
				return nil, fmt.Errorf("ollama: embedding %d contains a non-finite value", i)
			}
		}
	}

	if a.targetDim > 0 {
		for i := range result.Embeddings {
			result.Embeddings[i] = truncateNormalize(result.Embeddings[i], a.targetDim)
		}
	}
	if a.modelDigest != "" {
		if err := a.VerifyIdentity(ctx); err != nil {
			return nil, err
		}
	}

	return result.Embeddings, nil
}

// EmbedQuery embeds retrieval queries. For an asymmetric model (Qwen3, which
// carries a QueryInstruct in the registry) it wraps each query in the model's
// instruct prompt before embedding; symmetric models (bge-*) fall through to
// Embed unchanged. Passages always go through Embed. Implements
// types.QueryEmbedder.
func (a *Adapter) EmbedQuery(ctx context.Context, queries []string) ([][]float32, error) {
	if a.queryInstruct == "" || len(queries) == 0 {
		return a.Embed(ctx, queries)
	}
	wrapped := make([]string, len(queries))
	for i, q := range queries {
		wrapped[i] = qwen3QueryText(a.queryInstruct, q)
	}
	return a.Embed(ctx, wrapped)
}

// qwen3QueryText builds Qwen3-Embedding's query prompt:
// "Instruct: {task}\nQuery: {query}". Applied to queries only.
func qwen3QueryText(instruct, query string) string {
	return "Instruct: " + instruct + "\nQuery: " + query
}

// validateTargetDim rejects an --embed-dim that exceeds the model's native
// dimension, or that isn't one of the model's standard MRL dims when it
// advertises a ladder (registry.KnownDims) — keeping indexes on a consistent,
// comparable set of dimensions. 0 (no truncation, native dim) is always valid.
func validateTargetDim(modelName string, targetDim, nativeDim int) error {
	if targetDim <= 0 {
		return nil
	}
	if targetDim > nativeDim {
		return fmt.Errorf("ollama: target dim %d exceeds model %q native dim %d",
			targetDim, modelName, nativeDim)
	}
	if known := registry.KnownDims(modelName); len(known) > 0 && !slices.Contains(known, targetDim) {
		return fmt.Errorf("ollama: --embed-dim %d is not a supported truncation dim for %q; use one of %v",
			targetDim, modelName, known)
	}
	return nil
}

// truncateNormalize returns the first dim components of v, re-normalized to
// unit L2 length. Qwen3-Embedding is trained with Matryoshka Representation
// Learning, so a prefix of the full vector is itself a valid lower-dimensional
// embedding once renormalized. dim <= 0 or dim >= len(v) returns v unchanged.
func truncateNormalize(v []float32, dim int) []float32 {
	if dim <= 0 || dim >= len(v) {
		return v
	}
	out := make([]float32, dim)
	var sum float64
	for i := 0; i < dim; i++ {
		out[i] = v[i]
		sum += float64(v[i]) * float64(v[i])
	}
	if sum > 0 {
		inv := float32(1.0 / math.Sqrt(sum))
		for i := range out {
			out[i] *= inv
		}
	}
	return out
}

type embedRequest struct {
	Model    string               `json:"model"`
	Input    []string             `json:"input"`
	Truncate bool                 `json:"truncate"`
	Options  *embedRuntimeOptions `json:"options,omitempty"`
}

type embedRuntimeOptions struct {
	NumCtx   int `json:"num_ctx"`
	NumBatch int `json:"num_batch"`
}

type embedResponse struct {
	Model      string      `json:"model"`
	Embeddings [][]float32 `json:"embeddings"`
}

var digestPattern = regexp.MustCompile(`^[0-9a-fA-F]{64}$`)

func (a *Adapter) resolveModel(ctx context.Context) (string, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.endpoint+"/api/tags", nil)
	if err != nil {
		return "", "", err
	}
	resp, err := a.httpClient().Do(req)
	if err != nil {
		return "", "", fmt.Errorf("model list unavailable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("model list HTTP %d", resp.StatusCode)
	}
	var tags struct {
		Models []struct {
			Name   string `json:"name"`
			Digest string `json:"digest"`
		} `json:"models"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&tags); err != nil {
		return "", "", fmt.Errorf("invalid model list: %w", err)
	}
	var name, digest string
	for _, model := range tags.Models {
		if model.Name != a.modelName && !(model.Name == a.modelName+":latest" && !strings.Contains(a.modelName, ":")) {
			continue
		}
		if name != "" {
			return "", "", fmt.Errorf("ambiguous model name %q", a.modelName)
		}
		name, digest = model.Name, model.Digest
	}
	if name == "" {
		return "", "", fmt.Errorf("model %q not found", a.modelName)
	}
	if !digestPattern.MatchString(digest) {
		return "", "", fmt.Errorf("model %q has no valid SHA-256 digest", name)
	}
	return name, strings.ToLower(digest), nil
}

// VerifyIdentity rejects model removal or tag replacement before and after
// embedding. Ollama does not offer an atomic model pin across these calls.
func (a *Adapter) VerifyIdentity(ctx context.Context) error {
	if a.modelDigest == "" {
		return fmt.Errorf("ollama: model digest is unavailable")
	}
	name, digest, err := a.resolveModel(ctx)
	if err != nil {
		return fmt.Errorf("ollama: verify model identity: %w", err)
	}
	if name != a.modelName || digest != a.modelDigest {
		return fmt.Errorf("ollama: model_changed: model digest changed; reindex required")
	}
	return nil
}
