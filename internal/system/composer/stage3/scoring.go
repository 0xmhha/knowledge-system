package stage3

import (
	"sort"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// neighborAggregator collects ScoredNeighbors keyed by Target Citation.
// When multiple seeds reach the same target the running Score is updated
// to the MAX of competing values — the closest path wins.
//
// Why max (not sum): graph hubs (e.g., a logging utility called from
// hundreds of sites) would dominate a sum-based score regardless of
// semantic relevance. Max preserves "the best evidence we have for this
// target" without over-weighting popularity.
//
// Trade-off documented in docs/composer/stage3-scoring.md §2. The known
// limitation — genuinely central nodes get the same score as one-path
// nodes — will be re-evaluated against PR #70 data in Phase E.
type neighborAggregator struct {
	byTarget map[string]*ScoredNeighbor
}

func newNeighborAggregator() *neighborAggregator {
	return &neighborAggregator{
		byTarget: make(map[string]*ScoredNeighbor),
	}
}

// add registers an edge with the given derived score. On duplicate
// Target (multi-path), keeps the higher score and updates Edge to the
// closer-path edge; the provenance string is always appended.
func (a *neighborAggregator) add(n contract.Neighbor, score float64, source string) {
	key := n.Target.Key()
	existing, ok := a.byTarget[key]
	if !ok {
		a.byTarget[key] = &ScoredNeighbor{
			Edge:    n,
			Score:   score,
			Sources: []string{source},
		}
		return
	}
	existing.Sources = append(existing.Sources, source)
	if score > existing.Score {
		existing.Score = score
		// Update the canonical Edge to reflect the higher-scoring path
		// (closer hop, larger seed score, or both).
		existing.Edge = n
	}
}

// results returns the accumulated neighbors sorted by descending Score,
// then by descending target span, with file/start-line last for
// deterministic output. cap > 0 truncates to that length.
//
// Why span is a tiebreaker and not just decoration: every edge of one
// relation leaving one seed scores identically — seed.Score times the
// relation weight over 1+distance carries nothing about the target. A
// seed with nine callees therefore hands the whole ordering to whatever
// came next in the comparator, which was the file path. Measured on
// composer-pipeline-flow (2026-07-31): Compose's calls edges all tied,
// so allocator.go sorted ahead of composer.go and assemblePack — the
// 127-line function that is half the expected answer — landed behind
// four allocator spans and a 6-line token helper, at citation 15.
//
// Span size is the one signal available here that says anything about
// the target. A larger body is more likely to be the substance the
// prompt asked for than a 6-line helper reached by the same edge; it is
// a weak signal, but it is a signal, and it replaces alphabetical order,
// which is none. Ties beyond that keep the old file/line ordering so
// output stays deterministic.
func (a *neighborAggregator) results(cap int) []ScoredNeighbor {
	if len(a.byTarget) == 0 {
		return nil
	}
	out := make([]ScoredNeighbor, 0, len(a.byTarget))
	for _, sn := range a.byTarget {
		out = append(out, *sn)
	}
	span := func(sn ScoredNeighbor) int {
		return sn.Edge.Target.EndLine - sn.Edge.Target.StartLine
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		if si, sj := span(out[i]), span(out[j]); si != sj {
			return si > sj
		}
		if out[i].Edge.Target.File != out[j].Edge.Target.File {
			return out[i].Edge.Target.File < out[j].Edge.Target.File
		}
		return out[i].Edge.Target.StartLine < out[j].Edge.Target.StartLine
	})
	if cap > 0 && len(out) > cap {
		out = out[:cap]
	}
	return out
}

// sortStrings is a tiny wrapper so the searcher can sort relation-type
// strings without importing "sort" twice. Keeping it here colocates the
// sorting helpers.
func sortStrings(s []string) { sort.Strings(s) }
