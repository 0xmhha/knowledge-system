package stage1

import (
	"context"
	"sort"
	"strings"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
)

// scoredKeyword captures one BM25 rerank result.
type scoredKeyword struct {
	Keyword string
	Score   float64
	Hits    int
}

// rerank scores each candidate by ckg BM25 hit aggregation, drops zero-
// score candidates, sorts by descending score, and returns the kept
// keywords plus a confidence metric.
//
// Confidence reports how concentrated the BM25 scores are at the top:
//
//	confidence = top1.Score / sum(all kept scores)
//
// 1.0 means the leading keyword dominates. 1/N means uniform distribution
// (ambiguity). The composer uses this to decide whether another retrieval
// round is worth running.
//
// Per-keyword ckg errors are tolerated — that keyword is skipped, the
// rest proceed. Total-failure (all keywords error) returns an empty
// slice and zero confidence.
func (e *Extractor) rerank(ctx context.Context, candidates []string) ([]string, float64) {
	if len(candidates) == 0 {
		return nil, 0
	}

	scored := make([]scoredKeyword, 0, len(candidates))
	for _, kw := range candidates {
		hits, err := e.ckg.BM25Search(ctx, kw, ckgclient.SearchOpts{K: e.config.RerankPerKW})
		if err != nil {
			continue
		}
		// Score policy: MAX of per-hit scores, not the sum.
		//
		// History: an earlier draft summed hit.Score. With ckgclient.Real's
		// synthesized score (1 - i/(N+1)), any keyword that filled all K
		// slots ended up with the same constant sum (3.333 for K=5),
		// regardless of whether the keyword was a unique identifier or a
		// common word. The cap-at-MaxKeywords step then dropped rare
		// identifiers ("ErrFailClosed", 1 hit, sum=1.0) in favor of
		// common ones ("level", 5 hits, sum=3.333).
		//
		// MAX preserves "how strong was the strongest match for this
		// keyword" — exactly the signal a reranker should pick on. A
		// unique identifier with one strong hit beats a common word with
		// five mediocre hits, which matches reader intuition.
		best := 0.0
		for _, h := range hits {
			if h.Score > best {
				best = h.Score
			}
		}
		if best <= 0 {
			continue // zero-score keyword = not present in indexed code
		}
		scored = append(scored, scoredKeyword{Keyword: kw, Score: best, Hits: len(hits)})
	}

	sort.SliceStable(scored, func(i, j int) bool { return scored[i].Score > scored[j].Score })

	// Case-insensitive dedup AFTER scoring, BEFORE the MaxKeywords cap.
	// compoundCandidates deliberately emits both PascalCase and camelCase
	// of every join ("CksMCP" + "cksMCP") because the prompt doesn't say
	// which symbol exists — but ckg's BM25 tokenization case-folds, so
	// BOTH variants score identically and BOTH survive into the cap,
	// burning two of the eight slots per concept. Observed in production:
	// the mcp-tool-handlers prompt's cap filled with
	// CksMCP/cksMCP/MCPServer/mCPServer, evicting "registration",
	// "get_for_task", and "health" entirely. Keep the highest-scored
	// spelling per folded form (stable sort ⇒ first wins). Trade-off:
	// if BOTH spellings exist as distinct Go symbols and both matter,
	// the dropped one loses its exact FindSymbol shot — rare, and BM25
	// still surfaces that code by keyword; cap pollution hurt far more.
	seenFold := make(map[string]struct{}, len(scored))
	deduped := scored[:0]
	for _, s := range scored {
		fold := strings.ToLower(s.Keyword)
		if _, ok := seenFold[fold]; ok {
			continue
		}
		seenFold[fold] = struct{}{}
		deduped = append(deduped, s)
	}
	scored = deduped

	out := make([]string, 0, len(scored))
	for _, s := range scored {
		out = append(out, s.Keyword)
	}
	return out, computeConfidence(scored)
}

// computeConfidence returns top1.Score / sum(scores), or 0 when the slice
// is empty or all scores sum to zero. The result is in [0, 1]; higher
// means more concentrated (more confident).
func computeConfidence(scored []scoredKeyword) float64 {
	if len(scored) == 0 {
		return 0
	}
	var total float64
	for _, s := range scored {
		total += s.Score
	}
	if total <= 0 {
		return 0
	}
	return scored[0].Score / total
}
