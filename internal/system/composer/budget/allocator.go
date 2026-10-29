package budget

import (
	"context"
	"errors"
	"sort"
	"strings"

	"go.uber.org/zap"

	"github.com/0xmhha/knowledge-system/internal/system/composer/stage2"
	"github.com/0xmhha/knowledge-system/internal/system/composer/stage3"
	"github.com/0xmhha/knowledge-system/internal/system/debugtrace"
	"github.com/0xmhha/knowledge-system/internal/system/footprint"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// Defaults; to be revisited with real-prompt data.
const (
	// DefaultMaxTokens approximates "typical Claude/GPT context budget
	// allocated to evidence" — composer is one of several inputs the
	// downstream caller will combine with prompts and instructions.
	DefaultMaxTokens = 8000

	// DefaultOverheadReserve is the fraction of MaxTokens held back for
	// pack metadata (sanitize_report, citations index, integrity_hash).
	// Bodies share the remaining 90%.
	DefaultOverheadReserve = 0.10

	// DefaultMaxCitations caps the number of bodies the allocator selects,
	// independent of the token budget. Dogfooding returned 35-53 citations
	// per request (precision 2-24%), flooding the agent; capping the default
	// keeps packs focused. The agent overrides via the budget/depth knobs.
	// 0 means "no cap" (token budget is the only gate).
	DefaultMaxCitations = 12

	// DefaultNeighborReserve is how many of the MaxCitations body slots
	// are held back for neighbor-origin candidates. Seeds outrank
	// distance-discounted neighbors by construction, so without a
	// reserve the citation cap starves graph-expansion bodies entirely
	// (the same failure mode the edge-only pack fix addresses at the
	// edge level). 0 disables the reserve.
	DefaultNeighborReserve = 4

	// DefaultSnippetLines is the head-snippet length used when a full
	// body does not fit the remaining budget: instead of dropping the
	// candidate, the allocator degrades it to its first N lines
	// (signature + doc comment territory) and marks it Degraded.
	DefaultSnippetLines = 8

	// OriginSeed / OriginNeighbor label a candidate's provenance.
	OriginSeed     = "seed"
	OriginNeighbor = "neighbor"

	// DefaultKnowledgeReserve is how many selections are guaranteed to
	// knowledge-kind candidates (invariant/convention chunks) when any
	// are present: they carry the decision facts code cannot (measured:
	// "empty block = same state root" existed only as an invariant
	// chunk), yet never outscore code seeds. 0 disables the reserve.
	DefaultKnowledgeReserve = 2

	// DefaultExactSymbolReserve is how many selections are guaranteed to
	// citations that came from the prompt-verbatim, unambiguous symbol
	// gate (stage2.SourceExactSymbolPrefix). Same starvation shape as
	// the knowledge reserve: the declaration of a symbol the user named
	// by hand is a single small span, so it loses the greedy pass to
	// larger name-similar bodies. Measured 2026-07-31 on
	// bm25-rerank-option — EnableBM25Rerank resolved to exactly one node
	// and the boost fired, but the citation still fell outside the body
	// cap and the scenario scored R 0.00. 0 disables the reserve.
	DefaultExactSymbolReserve = 2
)

// Config tunes the allocator's budget and reserve.
type Config struct {
	// MaxTokens is the total token budget for the produced bodies plus
	// pack metadata overhead.
	MaxTokens int

	// OverheadReserve is the fraction of MaxTokens NOT available for
	// bodies (range 0..1). Defaults to 0.10 — bodies get 90% of MaxTokens.
	OverheadReserve float64

	// MaxCitations caps the number of selected bodies regardless of the
	// token budget (0 = no cap). Defaults to DefaultMaxCitations.
	MaxCitations int

	// NeighborReserve holds back this many MaxCitations slots for
	// neighbor-origin candidates. See DefaultNeighborReserve.
	NeighborReserve int

	// SnippetLines is the degraded-body head length. See
	// DefaultSnippetLines. 0 disables degradation (fit-or-drop).
	SnippetLines int

	// KnowledgeReserve guarantees this many selections to knowledge-kind
	// candidates. See DefaultKnowledgeReserve.
	KnowledgeReserve int

	// ExactSymbolReserve guarantees this many selections to
	// prompt-verbatim symbol citations. See DefaultExactSymbolReserve.
	ExactSymbolReserve int
}

// DefaultConfig returns the baseline tuning.
func DefaultConfig() Config {
	return Config{
		MaxTokens:          DefaultMaxTokens,
		OverheadReserve:    DefaultOverheadReserve,
		MaxCitations:       DefaultMaxCitations,
		NeighborReserve:    DefaultNeighborReserve,
		SnippetLines:       DefaultSnippetLines,
		KnowledgeReserve:   DefaultKnowledgeReserve,
		ExactSymbolReserve: DefaultExactSymbolReserve,
	}
}

// SelectedItem is one body that made it into the budget.
//
// Body intentionally holds a copy of the fetched text (not a reference
// into the fetcher's storage). This costs memory for large allocations
// but keeps every stage's state fully inspectable from a single
// Stage4Output value — critical for debugging the composer
// while the pipeline is still being tuned. A streaming variant that
// pipes bodies directly into EvidencePack assembly is a candidate
// optimization once the pipeline has stabilized and evaluation has
// finished measuring the quality of the current design.
type SelectedItem struct {
	Citation      contract.Citation
	Body          string
	TokenEstimate int
	Score         float64
	// Degraded marks a body truncated to a head snippet to fit budget.
	Degraded bool

	// Origin is OriginSeed (from Stage 2) or OriginNeighbor (from Stage 3).
	Origin string

	// ChunkKind is ckv's chunk label (invariant/convention/…); empty for
	// code chunks and neighbors. Drives the knowledge reserve.
	ChunkKind string
	// Sources is copied from the originating ScoredCitation/ScoredNeighbor
	// for full evidence-trail preservation.
	Sources []string
}

// Stage4Output captures the result of one Allocate call.
type Stage4Output struct {
	// Selected is the score-ordered list of bodies that fit in the
	// budget. EvidencePack assembly consumes this directly.
	Selected []SelectedItem

	// Skipped lists citations that were ranked but didn't fit (over
	// budget) or had no body (fetcher returned "" or errored). Useful
	// for footprint debugging.
	Skipped []contract.Citation

	// BudgetTokens is the body-share of MaxTokens (MaxTokens * (1-Overhead)).
	BudgetTokens int

	// UsedTokens is the sum of TokenEstimate across Selected.
	UsedTokens int

	// Utilization is UsedTokens / BudgetTokens (0..1). Tight allocations
	// approach 1.0; sparse ones stay low.
	Utilization float64

	// FetchErrors counts how many BodyFetcher.Fetch calls returned a
	// non-nil error. Empty-body responses are counted separately below.
	FetchErrors int

	// EmptyBodies counts how many Fetch calls returned ("", nil) — the
	// "available but vanished" signal (deleted file, missing chunk, etc.).
	EmptyBodies int

	// CapTruncated counts candidates never processed because the
	// MaxCitations cap ended the selection loop early. Distinct from
	// Skipped: these were not evaluated at all. Exposed so operators can
	// see when the citation cap — not the token budget — is the binding
	// constraint (the neighbor-starvation failure mode).
	CapTruncated int
}

// Allocator runs Stage 4 of the composer pipeline.
type Allocator struct {
	fetcher BodyFetcher
	fp      *footprint.Logger
	config  Config
}

// Option configures an Allocator.
type Option func(*Allocator)

// WithFootprint attaches a footprint.Logger. Nil-safe.
func WithFootprint(fp *footprint.Logger) Option {
	return func(a *Allocator) { a.fp = fp }
}

// WithConfig overrides the default tuning.
func WithConfig(cfg Config) Option {
	return func(a *Allocator) { a.config = cfg }
}

// New constructs an Allocator. Returns an error if fetcher is nil or
// the config has out-of-range values.
func New(fetcher BodyFetcher, opts ...Option) (*Allocator, error) {
	if fetcher == nil {
		return nil, errors.New("budget: nil fetcher")
	}
	a := &Allocator{
		fetcher: fetcher,
		config:  DefaultConfig(),
	}
	for _, opt := range opts {
		opt(a)
	}
	if a.config.MaxTokens < 0 {
		return nil, errors.New("budget: MaxTokens must be >= 0")
	}
	if a.config.OverheadReserve < 0 || a.config.OverheadReserve > 1 {
		return nil, errors.New("budget: OverheadReserve must be in [0, 1]")
	}
	return a, nil
}

// Allocate merges seeds and neighbors into a single ranked list, fetches
// each candidate's body in order, and greedily fits within the budget.
//
// Candidates whose Fetcher returns an error are counted in FetchErrors;
// empty-body responses are counted in EmptyBodies. Both appear in Skipped.
// A candidate that doesn't fit (body too large for remaining budget) is
// skipped — the loop continues, since a smaller later candidate might fit.
//
// Empty inputs are not an error; the output is empty (with a non-zero
// BudgetTokens so callers can still inspect what budget was set).
func (a *Allocator) Allocate(ctx context.Context, seeds []stage2.ScoredCitation, neighbors []stage3.ScoredNeighbor) (Stage4Output, error) {
	candidates := mergeCandidates(seeds, neighbors)

	bodyBudget := int(float64(a.config.MaxTokens) * (1.0 - a.config.OverheadReserve))
	out := Stage4Output{
		BudgetTokens: bodyBudget,
	}

	// Slot accounting: seeds may not exhaust the citation cap when a
	// neighbor reserve is configured — distance-discounted neighbors
	// never outscore their seeds, so without a reserve the cap starves
	// them (fit-or-drop starvation, 2026-07 postmortem).
	seedCap := a.config.MaxCitations
	if a.config.MaxCitations > 0 && a.config.NeighborReserve > 0 {
		seedCap = a.config.MaxCitations - a.config.NeighborReserve
		if seedCap < 1 {
			seedCap = 1
		}
	}
	seedSelected := 0
	neighborSelected := 0
	knowledgeSelected := 0
	exactSymbolRescued := map[string]bool{}
	selectedKeys := map[string]bool{}

	// decide records why each candidate was kept or left out, for the debug
	// trace: only "over_budget", "fetch_error" and "empty_body" reach
	// Skipped, so the quota and reserve exits are otherwise invisible.
	rec := debugtrace.From(ctx)
	var decisions []allocDecision
	decide := func(c candidate, reason string) {
		if rec != nil {
			decisions = append(decisions, allocDecision{File: c.Citation.File, Start: c.Citation.StartLine,
				End: c.Citation.EndLine, Origin: c.Origin, Score: c.Score, Decision: reason})
		}
	}
	if rec != nil {
		rec.Before("stage4", "allocate", "", map[string]any{
			"candidates": len(candidates), "budget_tokens": bodyBudget, "max_citations": a.config.MaxCitations,
			"seed_cap": seedCap, "neighbor_reserve": a.config.NeighborReserve,
			"knowledge_reserve": a.config.KnowledgeReserve, "exact_symbol_reserve": a.config.ExactSymbolReserve,
		})
	}

	// A reserve holds a slot only while a candidate of its kind is still
	// ahead in the list. A slot held for a kind that never arrives only cuts
	// the next seed: on the 2026-10-27 grpc-go evaluation every answer Stage
	// 4 cut came from a pack of 8 to 11 bodies, its held slots unused.
	ahead := newReserveLookahead(candidates)

	used := 0
	processed := 0
	for i, c := range candidates {
		processed++
		if c.ParentOf != "" && !selectedKeys[c.ParentOf] {
			decide(c, "parent_not_selected")
			continue
		}
		// Domain-knowledge chunks get a reserved slot because they arrive
		// from a kind-scoped retrieval, rank below code seeds, and would
		// never survive a plain greedy pass.
		//
		// "convention" is deliberately NOT listed. A convention chunk is
		// built at line 0-0 by construction (internal/vector/build/
		// pipeline.go: StartLine/EndLine are literal zeros), and since
		// #72 stage 2 splits every zero-line hit out into the pack's
		// knowledge section before the citation aggregator runs. One can
		// therefore never reach this loop as a candidate; keeping the arm
		// would only suggest a path that does not exist.
		//
		// Out-of-tree markdown arrives as chunk_kind "doc" with an empty
		// commit hash — in-tree READMEs carry a real one and are not
		// knowledge. That arm stays even though this corpus produces zero
		// of them (measured 2026-08-03: 0 out-of-tree doc chunks against
		// 3,679 in-tree), because a project pack indexing external
		// documentation would produce them and nothing else would catch
		// it.
		//
		// What actually fills the reserve here is the 44 invariant
		// chunks: sampled across all fifteen scenarios on 2026-08-03,
		// they took 16 body slots in 8 of them, twice each in six.
		isKnowledge := isKnowledgeCandidate(c)
		// A prompt-verbatim, unambiguously resolved symbol gets the same
		// protection as knowledge for the same reason: it is one small
		// declaration span competing against whole bodies that merely
		// share its name, so the greedy pass drops it.
		exactKeyword := stage2.ExactSymbolKeyword(c.Sources)
		isExactSymbol := exactKeyword != ""
		// A slot is held per distinct verbatim symbol, not one overall:
		// measured 2026-07-31, a prompt naming both EnableBM25Rerank and
		// BM25 had the second symbol's declaration evicted because the
		// first had already spent the single reserve.
		exactUnrescued := isExactSymbol && !exactSymbolRescued[exactKeyword] &&
			len(exactSymbolRescued) < a.config.ExactSymbolReserve
		if c.Origin == OriginSeed && seedCap > 0 && seedSelected >= ahead.seedCap(i, a.config, seedCap, neighborSelected) {
			// Seed slots exhausted; leave room for neighbor-origin
			// candidates. Not a Skip: the candidate lost to the quota,
			// not to budget or fetch. Knowledge-kind candidates are
			// exempt while their reserve is unfilled — domain rules must
			// not lose their slot to yet another code body.
			if (!isKnowledge || knowledgeSelected >= a.config.KnowledgeReserve) && !exactUnrescued {
				decide(c, "seed_quota")
				continue
			}
		}
		// Knowledge holdback: while the knowledge reserve is unfilled,
		// that many of the remaining cap slots are held for knowledge-kind
		// candidates. Without this the total-cap break below fires before
		// the list reaches them — knowledge hits come from a separate
		// kind-scoped retrieval and rank below the code seeds, so a plain
		// greedy pass never selects one (the same starvation shape as the
		// neighbor reserve, one level up).
		if !isKnowledge && !isExactSymbol && a.config.MaxCitations > 0 {
			reserveLeft := 0
			if a.config.KnowledgeReserve > 0 {
				if left := a.config.KnowledgeReserve - knowledgeSelected; left > 0 {
					reserveLeft += min(left, ahead.knowledge(i))
				}
			}
			if a.config.ExactSymbolReserve > 0 {
				if left := a.config.ExactSymbolReserve - len(exactSymbolRescued); left > 0 {
					reserveLeft += min(left, ahead.exactSymbols(i, exactSymbolRescued))
				}
			}
			if reserveLeft > 0 && len(out.Selected) >= a.config.MaxCitations-reserveLeft {
				decide(c, "reserve_holdback")
				continue
			}
		}
		body, err := a.fetcher.Fetch(ctx, c.Citation)
		if err != nil {
			out.FetchErrors++
			out.Skipped = append(out.Skipped, c.Citation)
			decide(c, "fetch_error")
			continue
		}
		if body == "" {
			out.EmptyBodies++
			out.Skipped = append(out.Skipped, c.Citation)
			decide(c, "empty_body")
			continue
		}
		tokens := EstimateTokens(body)
		degraded := false
		if used+tokens > bodyBudget {
			// Fit-or-degrade: try the head snippet before dropping.
			if a.config.SnippetLines > 0 {
				if snip := headSnippet(body, a.config.SnippetLines); snip != "" {
					st := EstimateTokens(snip)
					if used+st <= bodyBudget {
						body, tokens, degraded = snip, st, true
					}
				}
			}
			if !degraded {
				out.Skipped = append(out.Skipped, c.Citation)
				decide(c, "over_budget")
				continue
			}
		}
		if degraded {
			decide(c, "selected_degraded")
		} else {
			decide(c, "selected")
		}
		out.Selected = append(out.Selected, SelectedItem{
			Citation:      c.Citation,
			Body:          body,
			TokenEstimate: tokens,
			Score:         c.Score,
			Degraded:      degraded,
			Origin:        c.Origin,
			ChunkKind:     c.ChunkKind,
			Sources:       c.Sources,
		})
		selectedKeys[c.Citation.Key()] = true
		used += tokens
		switch c.Origin {
		case OriginSeed:
			seedSelected++
		case OriginNeighbor:
			neighborSelected++
		}
		if isKnowledge {
			knowledgeSelected++
		}
		if isExactSymbol {
			exactSymbolRescued[exactKeyword] = true
		}

		// Citation cap: stop once we've selected MaxCitations bodies, even
		// if the token budget has room. Candidates are merged in descending
		// score order, so the cap keeps the strongest hits. Remaining
		// candidates are simply not processed (a hard ceiling on selection,
		// distinct from the budget/fetch skips tracked in out.Skipped).
		if a.config.MaxCitations > 0 && len(out.Selected) >= a.config.MaxCitations {
			break
		}
	}

	out.CapTruncated = len(candidates) - processed
	out.UsedTokens = used
	if bodyBudget > 0 {
		out.Utilization = float64(used) / float64(bodyBudget)
	}
	if rec != nil {
		for _, c := range candidates[processed:] {
			decide(c, "citation_cap")
		}
		rec.After("stage4", "allocate", "", map[string]any{
			"decisions": decisions, "used_tokens": used, "selected": len(out.Selected),
		})
	}

	a.emitFootprint(ctx, out, len(candidates))
	return out, nil
}

// headSnippet returns the first n lines of body plus a truncation marker,
// or "" when body already fits in n lines (degrading would gain nothing).
func headSnippet(body string, n int) string {
	lines := strings.Split(body, "\n")
	if len(lines) <= n {
		return ""
	}
	return strings.Join(lines[:n], "\n") + "\n// … cks: truncated to head snippet (degraded); read the file for full text"
}

func (a *Allocator) emitFootprint(ctx context.Context, out Stage4Output, candidateCount int) {
	if a.fp == nil {
		return
	}
	fields := []zap.Field{
		zap.Int("budget_tokens", out.BudgetTokens),
		zap.Int("candidate_count", candidateCount),
		zap.Int("selected_count", len(out.Selected)),
		zap.Int("skipped_count", len(out.Skipped)),
		zap.Int("used_tokens", out.UsedTokens),
		zap.Float64("utilization", out.Utilization),
		zap.Int("fetch_errors", out.FetchErrors),
		zap.Int("empty_bodies", out.EmptyBodies),
		zap.Int("cap_truncated", out.CapTruncated),
		zap.Int("degraded_count", countDegraded(out)),
	}
	if len(out.Selected) > 0 {
		fields = append(fields,
			zap.String("top_selected", out.Selected[0].Citation.String()),
			zap.Float64("top_score", out.Selected[0].Score),
		)
	}
	a.fp.Event(ctx, "composer.stage4_allocated", fields...)
}

func countDegraded(out Stage4Output) int {
	n := 0
	for _, it := range out.Selected {
		if it.Degraded {
			n++
		}
	}
	return n
}

// candidate is the unified shape used by greedy allocation. Either
// origin (seed or neighbor) contributes the same fields, so the
// allocator doesn't need parallel branches.
// allocDecision is one candidate's outcome in the debug trace, in the
// order the greedy pass visited them.
type allocDecision struct {
	File     string  `json:"file"`
	Start    int     `json:"start"`
	End      int     `json:"end"`
	Origin   string  `json:"origin"`
	Score    float64 `json:"score"`
	Decision string  `json:"decision"`
}

type candidate struct {
	Citation contract.Citation
	Score    float64
	Origin   string
	// ParentOf is set for a nearby document context span. It may enter the
	// pack only when its matched child was selected first.
	ParentOf string
	// ChunkKind is ckv's chunk label (invariant/convention/…); empty for
	// code chunks and neighbors. Drives the knowledge reserve.
	ChunkKind string
	Sources   []string
}

// mergeCandidates combines seeds and neighbors into one score-sorted
// list. Stage 3's self-loop guard already prevents a citation from
// appearing in both, but this function dedupes by Citation.Key() as a
// defensive backstop — if a future Stage 3 change breaks the guard,
// the allocator still produces sensible output.
// isKnowledgeCandidate reports a domain-knowledge chunk: an invariant, or
// out-of-tree markdown (chunk_kind "doc" with an empty commit hash).
func isKnowledgeCandidate(c candidate) bool {
	return c.ChunkKind == "invariant" || (c.ChunkKind == "doc" && c.Citation.CommitHash == "")
}

// reserveLookahead answers, for a position in the merged candidate list,
// how many candidates each reserve could still be spent on.
type reserveLookahead struct {
	cands []candidate
	// neighborsAfter[i] and knowledgeAfter[i] count candidates after index i.
	neighborsAfter []int
	knowledgeAfter []int
}

func newReserveLookahead(cands []candidate) reserveLookahead {
	r := reserveLookahead{cands: cands, neighborsAfter: make([]int, len(cands)), knowledgeAfter: make([]int, len(cands))}
	n, k := 0, 0
	for i := len(cands) - 1; i >= 0; i-- {
		r.neighborsAfter[i], r.knowledgeAfter[i] = n, k
		if cands[i].Origin == OriginNeighbor {
			n++
		}
		if isKnowledgeCandidate(cands[i]) {
			k++
		}
	}
	return r
}

// seedCap is the seed slot limit at index i: the configured cap, widened by
// every neighbor-reserve slot that no selected or upcoming neighbor can use.
func (r reserveLookahead) seedCap(i int, cfg Config, seedCap, neighborSelected int) int {
	if cfg.MaxCitations <= 0 || cfg.NeighborReserve <= 0 {
		return seedCap
	}
	usable := min(cfg.NeighborReserve, neighborSelected+r.neighborsAfter[i])
	return max(seedCap, cfg.MaxCitations-usable)
}

func (r reserveLookahead) knowledge(i int) int { return r.knowledgeAfter[i] }

// exactSymbols counts the distinct prompt-verbatim symbols after index i that
// have not been rescued yet.
func (r reserveLookahead) exactSymbols(i int, rescued map[string]bool) int {
	seen := map[string]bool{}
	for _, c := range r.cands[i+1:] {
		if kw := stage2.ExactSymbolKeyword(c.Sources); kw != "" && !rescued[kw] {
			seen[kw] = true
		}
	}
	return len(seen)
}

func mergeCandidates(seeds []stage2.ScoredCitation, neighbors []stage3.ScoredNeighbor) []candidate {
	out := make([]candidate, 0, len(seeds)*3+len(neighbors))
	for _, s := range seeds {
		out = append(out, candidate{
			Citation:  s.Citation,
			Score:     s.Score,
			Origin:    OriginSeed,
			ChunkKind: s.ChunkKind,
			Sources:   s.Sources,
		})
		// A split Markdown child is the cited match. Fetch only a bounded
		// window on either side, each with its own source-line citation.
		// This avoids relabeling parent text as if the child citation covered it.
		if s.ChunkKind == "doc" && s.ParentCitation != nil &&
			s.ParentCitation.File == s.Citation.File &&
			s.ParentCitation.CommitHash == s.Citation.CommitHash &&
			s.ParentCitation.StartLine <= s.Citation.StartLine &&
			s.ParentCitation.EndLine >= s.Citation.EndLine {
			parent := s.ParentCitation
			const contextLines = 6
			if s.Citation.StartLine > parent.StartLine {
				start := max(parent.StartLine, s.Citation.StartLine-contextLines)
				out = append(out, candidate{Citation: contract.Citation{File: s.Citation.File, StartLine: start, EndLine: s.Citation.StartLine - 1, CommitHash: s.Citation.CommitHash}, Score: s.Score * 0.999, Origin: OriginSeed, ParentOf: s.Citation.Key(), ChunkKind: "doc_context", Sources: []string{"ckv:parent-before"}})
			}
			if s.Citation.EndLine < parent.EndLine {
				end := min(parent.EndLine, s.Citation.EndLine+contextLines)
				out = append(out, candidate{Citation: contract.Citation{File: s.Citation.File, StartLine: s.Citation.EndLine + 1, EndLine: end, CommitHash: s.Citation.CommitHash}, Score: s.Score * 0.998, Origin: OriginSeed, ParentOf: s.Citation.Key(), ChunkKind: "doc_context", Sources: []string{"ckv:parent-after"}})
			}
		}
	}
	for _, n := range neighbors {
		out = append(out, candidate{
			Citation: n.Edge.Target,
			Score:    n.Score,
			Origin:   OriginNeighbor,
			Sources:  n.Sources,
		})
	}

	// Ties keep input order: seeds in Stage 2 order, then neighbors in Stage
	// 3 order. Every neighbor of one seed and relation ties on score, and
	// Stage 3 already broke those ties by target span (stage3/scoring.go);
	// re-sorting by file and line here threw that away. Measured 2026-10-27
	// on grpc-go pr-8875 (bge-m3): the answer, Stage 3 neighbor 2, lost the
	// neighbor slots to shorter spans of its file that start on lower lines.
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Score > out[j].Score
	})

	seen := make(map[string]struct{}, len(out))
	dedup := make([]candidate, 0, len(out))
	for _, c := range out {
		key := c.Citation.Key()
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		dedup = append(dedup, c)
	}
	return dedup
}
