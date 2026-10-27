// Package stage3 implements the composer pipeline's Stage 3: expand the
// citations from Stage 2 with their graph neighbors via ckg.Neighbors,
// adding caller/callee/imports/etc. context that Stage 2 (BM25 + Symbol)
// cannot reach on its own.
//
// Strategy:
//
//  1. Take Stage 2 citations and pass them through as Seeds (the Stage 4
//     budget and the composer wire-up see the full Stage 2 work).
//  2. Expand only the top N seeds by score (default 10). Lower-ranked
//     seeds are weak signals — expanding them mostly amplifies noise.
//  3. For each expanded seed, call ckg.Neighbors with an Intent-specific
//     Relation set and Hop depth.
//  4. Aggregate by Target citation: dedup, taking the MAX score across
//     multiple paths (closest-path evidence wins). Skip neighbors that
//     are already in the Seed set — we don't double-count.
//  5. Cap and rank for downstream consumption.
//
// Intent influence at this stage is the strongest in the pipeline:
// "BugFix" wants caller chains; "ArchExplain" wants imports/implements/
// embeds; "TestAdd" wants tested_by. See intent_relations.go for the
// full mapping. Stage 3 deliberately leans on Intent because graph
// traversal is the place where "which structural relationships matter"
// differs most across user intents.
package stage3

import (
	"context"
	"errors"
	"fmt"

	"go.uber.org/zap"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/internal/system/composer/stage2"
	"github.com/0xmhha/knowledge-system/internal/system/debugtrace"
	"github.com/0xmhha/knowledge-system/internal/system/footprint"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// Default tuning; to be revisited with real-prompt data.
const (
	DefaultMaxSeedsToExpand  = 10
	DefaultNeighborsPerSeed  = 50
	DefaultMaxFinalNeighbors = 50
)

// Config tunes the expander's call budget and ranking weights.
type Config struct {
	// MaxSeedsToExpand caps how many of Stage 2's citations get expanded.
	// Lower-ranked seeds become passthrough only.
	MaxSeedsToExpand int

	// NeighborsPerSeed is the per-ckg-call MaxTotal passed to
	// ckg.NeighborsOpts. Bounds the work ckg does per seed.
	NeighborsPerSeed int

	// MaxFinalNeighbors caps the deduped Neighbors slice that Stage 3
	// emits. Stage 4 (budget) may trim further.
	MaxFinalNeighbors int
}

// DefaultConfig returns the baseline tuning.
func DefaultConfig() Config {
	return Config{
		MaxSeedsToExpand:  DefaultMaxSeedsToExpand,
		NeighborsPerSeed:  DefaultNeighborsPerSeed,
		MaxFinalNeighbors: DefaultMaxFinalNeighbors,
	}
}

// ScoredNeighbor is one graph-expanded edge with derived score and provenance.
type ScoredNeighbor struct {
	// Edge carries the canonical neighbor data (source seed citation,
	// target citation, relation, distance).
	Edge contract.Neighbor

	// Score is derived from the originating seed's score, discounted by
	// distance: seed.Score / (1 + distance). When multiple seeds reach
	// the same target, the maximum is kept (closest-path evidence wins).
	//
	// The decay formula was chosen for simplicity; alternatives
	// (exponential, logarithmic, threshold) and their trade-offs are
	// analyzed in docs/composer/stage3-scoring.md §1, with a measurement
	// plan to revise based on PR #70 data.
	Score float64

	// Sources records every (seed, relation, distance) that produced this
	// target. Format: "seed:<file>:<relation>:dist=<n>". Multi-path
	// neighbors have multiple entries. The colon-delimited format is
	// chosen for grep-and-parse uniformity with Stage 2's Sources;
	// see docs/composer/stage3-scoring.md §4.
	Sources []string
}

// Stage3Output captures the result of one Expand call.
type Stage3Output struct {
	// Seeds is the full Stage 2 passthrough. Preserves the work that
	// Stage 2 did so the Stage 4 budget and the composer wire-up have access to BM25 +
	// Symbol evidence even when Stage 3 declined to expand a seed.
	Seeds []stage2.ScoredCitation

	// Neighbors is the deduped, score-sorted graph expansion. Each entry
	// is a citation NOT already present in Seeds — Stage 3 skips
	// self-loops via the seed-key set.
	Neighbors []ScoredNeighbor

	// SeedsExpanded is how many seeds were actually expanded (the rest
	// are passthrough-only). Bounded by Config.MaxSeedsToExpand.
	SeedsExpanded int

	// RelationCoverage counts how many neighbors arrived via each
	// relation type. Footprint and evaluation use this to spot
	// "Intent X always produces 0 implements" type signals.
	RelationCoverage map[contract.Relation]int

	// FailedSeeds lists seeds whose ckg.Neighbors call errored. The
	// rest of expansion proceeds; the operator can investigate why ckg
	// rejected specific citations.
	FailedSeeds []contract.Citation
}

// Expander runs Stage 3 of the composer pipeline.
type Expander struct {
	ckg    ckgclient.Client
	fp     *footprint.Logger
	config Config
}

// Option configures an Expander.
type Option func(*Expander)

// WithFootprint attaches a footprint.Logger; Expand emits
// composer.stage3_expanded on completion. Nil-safe.
func WithFootprint(fp *footprint.Logger) Option {
	return func(e *Expander) { e.fp = fp }
}

// WithConfig overrides the default tuning.
func WithConfig(cfg Config) Option {
	return func(e *Expander) { e.config = cfg }
}

// New constructs an Expander. Returns an error if ckg is nil.
func New(ckg ckgclient.Client, opts ...Option) (*Expander, error) {
	if ckg == nil {
		return nil, errors.New("stage3: nil ckg client")
	}
	e := &Expander{
		ckg:    ckg,
		config: DefaultConfig(),
	}
	for _, opt := range opts {
		opt(e)
	}
	return e, nil
}

// Expand takes Stage 2's citations and produces a Stage3Output that pairs
// them with their graph neighbors. Intent shapes both the Relation set
// passed to ckg and the hop depth.
//
// Empty seeds returns an empty (non-error) Stage3Output — the composer
// can decide whether a Stage-2 miss should abort the pipeline.
func (e *Expander) Expand(ctx context.Context, seeds []stage2.ScoredCitation, intent contract.Intent) (Stage3Output, error) {
	out := Stage3Output{
		Seeds:            seeds,
		RelationCoverage: make(map[contract.Relation]int),
	}
	if len(seeds) == 0 {
		e.emitFootprint(ctx, intent, out, 0)
		return out, nil
	}

	// Cap how many seeds we expand. Lower-ranked seeds pass through but
	// don't burn ckg calls.
	toExpand := seeds
	if len(toExpand) > e.config.MaxSeedsToExpand {
		toExpand = toExpand[:e.config.MaxSeedsToExpand]
	}
	out.SeedsExpanded = len(toExpand)

	relations := intentToRelations(intent)
	hops := intentToHops(intent)

	// ckgclient.Neighbors traverses one direction per call (mixed
	// forward+reverse relation sets are rejected), but several Intents
	// legitimately want both — BugFix wants calls AND called_by. Split
	// the relation set into per-direction groups and traverse each;
	// the aggregator dedups targets across groups. nil relations (the
	// Unknown broad fan-out) walks every forward edge type, then callers.
	relationGroups := splitByDirection(relations)

	seedKeys := buildSeedKeys(seeds)
	agg := newNeighborAggregator()
	failedCount := 0
	rec := debugtrace.From(ctx)
	if rec != nil {
		rec.Before("stage3", "expand", "", map[string]any{
			"seeds": len(seeds), "expanded": len(toExpand), "relations": relations, "hops": hops,
			"neighbors_per_seed": e.config.NeighborsPerSeed,
		})
	}

	for _, seed := range toExpand {
		var neighbors []contract.Neighbor
		var seedErr error
		edgeSeen := make(map[string]struct{})
		for _, group := range relationGroups {
			nopts := ckgclient.NeighborsOpts{
				Relations: group,
				Hops:      hops,
				MaxTotal:  e.config.NeighborsPerSeed,
			}
			ns, err := e.ckg.Neighbors(ctx, seed.Citation, nopts)
			if err != nil {
				seedErr = err
				continue
			}
			// Dedup identical edges across direction groups — a backend
			// that ignores the Relations filter (or overlapping groups)
			// must not double-count an edge's score/coverage.
			for _, n := range ns {
				n = orientFromSeed(seed.Citation, n)
				k := n.Source.Key() + "|" + n.Target.Key() + "|" + string(n.Relation)
				if _, ok := edgeSeen[k]; ok {
					continue
				}
				edgeSeen[k] = struct{}{}
				neighbors = append(neighbors, n)
			}
		}
		if rec != nil {
			data := map[string]any{"seed": debugtrace.Citations([]contract.Citation{seed.Citation})[0], "neighbors": traceNeighbors(neighbors)}
			if seedErr != nil {
				data["error"] = seedErr.Error()
			}
			rec.After("stage3", "seed_neighbors", seed.Citation.Key(), data)
		}
		if seedErr != nil && len(neighbors) == 0 {
			out.FailedSeeds = append(out.FailedSeeds, seed.Citation)
			failedCount++
			continue
		}

		for _, n := range neighbors {
			// Skip self-loops: target is already a seed, so it carries
			// stronger evidence from Stage 2 — no need to demote it to
			// graph-derived score.
			if _, isSeed := seedKeys[n.Target.Key()]; isSeed {
				continue
			}
			// Closer paths get higher scores. Distance is 1-based per
			// ckgclient doc; the +1 guard avoids divide-by-zero if a
			// backend ever returns distance 0.
			score := seed.Score * relationWeight(intent, n.Relation) / float64(1+n.Distance)
			source := fmt.Sprintf("seed:%s:%s:dist=%d",
				seed.Citation.File, n.Relation, n.Distance)
			agg.add(n, score, source)
			out.RelationCoverage[n.Relation]++
		}
	}

	out.Neighbors = agg.results(e.config.MaxFinalNeighbors)
	if rec != nil {
		ranked := make([]debugtrace.Cite, 0, len(out.Neighbors))
		for _, n := range out.Neighbors {
			ranked = append(ranked, debugtrace.Cite{File: n.Edge.Target.File, Start: n.Edge.Target.StartLine,
				End: n.Edge.Target.EndLine, Kind: string(n.Edge.Relation), Score: n.Score, Sources: n.Sources})
		}
		rec.After("stage3", "expand", "", map[string]any{
			"neighbors": ranked, "max_final_neighbors": e.config.MaxFinalNeighbors,
			"failed_seeds": debugtrace.Citations(out.FailedSeeds),
		})
	}
	e.emitFootprint(ctx, intent, out, failedCount)
	return out, nil
}

// traceNeighbors is the debug-trace form of one seed's raw neighbor list.
func traceNeighbors(ns []contract.Neighbor) []map[string]any {
	out := make([]map[string]any, 0, len(ns))
	for _, n := range ns {
		out = append(out, map[string]any{"file": n.Target.File, "start": n.Target.StartLine, "end": n.Target.EndLine,
			"relation": n.Relation, "distance": n.Distance})
	}
	return out
}

func (e *Expander) emitFootprint(ctx context.Context, intent contract.Intent, out Stage3Output, failedCount int) {
	if e.fp == nil {
		return
	}
	// Convert RelationCoverage to sorted-key string list for stable
	// JSON output (zap.Any on map produces unstable ordering).
	relTypes := make([]string, 0, len(out.RelationCoverage))
	for r := range out.RelationCoverage {
		relTypes = append(relTypes, string(r))
	}
	sortStrings(relTypes)

	fields := []zap.Field{
		zap.String("intent", string(intent)),
		zap.Int("seeds_total", len(out.Seeds)),
		zap.Int("seeds_expanded", out.SeedsExpanded),
		zap.Int("neighbor_count", len(out.Neighbors)),
		zap.Int("failed_seeds", failedCount),
		zap.Strings("relation_types", relTypes),
	}
	if len(out.Neighbors) > 0 {
		fields = append(fields,
			zap.String("top_neighbor", out.Neighbors[0].Edge.Target.String()),
			zap.Float64("top_neighbor_score", out.Neighbors[0].Score),
		)
	}
	e.fp.Event(ctx, "composer.stage3_expanded", fields...)
}

// orientFromSeed returns n with the seed side as Source and the neighbor as
// Target, the shape ScoredNeighbor.Edge promises. ckgclient keeps the graph's
// orientation: on a callers walk a calls edge still runs caller -> callee, so
// the seed comes back as Target. The seed side is the endpoint overlapping the
// seed citation; a seed chunk and its graph node can start on different lines
// (a doc comment is part of the chunk, not the node).
func orientFromSeed(seed contract.Citation, n contract.Neighbor) contract.Neighbor {
	if overlaps(n.Target, seed) && !overlaps(n.Source, seed) {
		n.Source, n.Target = n.Target, n.Source
	}
	return n
}

func overlaps(a, b contract.Citation) bool {
	return a.File == b.File && a.StartLine <= b.EndLine && b.StartLine <= a.EndLine
}

// splitByDirection partitions a relation set into direction-homogeneous
// groups for ckgclient.Neighbors, which traverses one direction per call.
// called_by is the only reverse relation in the contract. nil input (the
// broad Unknown fan-out) becomes a nil forward group plus a called_by group:
// ckgclient reads nil as every forward edge type, so on its own it never
// returns a seed's callers.
func splitByDirection(rels []contract.Relation) [][]contract.Relation {
	if len(rels) == 0 {
		return [][]contract.Relation{nil, {contract.RelationCalledBy}}
	}
	var forward, reverse []contract.Relation
	for _, r := range rels {
		if r == contract.RelationCalledBy {
			reverse = append(reverse, r)
		} else {
			forward = append(forward, r)
		}
	}
	groups := make([][]contract.Relation, 0, 2)
	if len(forward) > 0 {
		groups = append(groups, forward)
	}
	if len(reverse) > 0 {
		groups = append(groups, reverse)
	}
	return groups
}

// buildSeedKeys returns the dedup set of seed citation keys for fast
// self-loop detection.
func buildSeedKeys(seeds []stage2.ScoredCitation) map[string]struct{} {
	out := make(map[string]struct{}, len(seeds))
	for _, sc := range seeds {
		out[sc.Citation.Key()] = struct{}{}
	}
	return out
}
