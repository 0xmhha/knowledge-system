package stage2

import (
	"fmt"
	"sort"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// ScoredCitation is one citation accompanied by its accumulated evidence.
//
// Score is the Reciprocal Rank Fusion (RRF) total across every ranked list
// the citation appeared in:
//
//	Score = Σ (weight_i / (RRFK + rank_i))
//
// where each BM25Search result list and each FindSymbol result list (per
// keyword) is a separate ranked input. BM25 lists carry BMWeight (1.0 by
// default); FindSymbol lists carry SymbolWeight (1.5 by default — exact
// symbol matches outweigh keyword-only overlap by 50%). rank is 1-based.
//
// RRF is rank-only fusion (Cormack et al., 2009): backend score
// magnitudes never enter the merge, so BM25's wide score range stops
// dominating the SymbolBonus heuristic that the previous score-sum
// aggregator used.
//
// Sources is a human-readable evidence trail of the form
// "bm25:<keyword>@rank=<rank>(+<contribution>)" or
// "symbol:<keyword>@rank=<rank>(+<contribution>)". Preserved so the
// audit log and Phase E evaluation can answer "why did this citation
// score this high" without rerunning the search.
type ScoredCitation struct {
	Citation contract.Citation
	Score    float64
	Sources  []string
	// ChunkKind is ckv's chunk-strategy label, carried from the hit that
	// first introduced this citation (empty for ckg-only citations).
	// The budget allocator's knowledge quota routes on it.
	ChunkKind string
}

// DefaultRRFK is the RRF tuning constant from Cormack et al. (2009).
// 60 is the value that paper recommends and that most rank-fusion
// implementations adopt; smaller values give more weight to the head
// of each ranked list, larger values flatten the influence of rank.
const DefaultRRFK = 60

// aggregator collects per-citation evidence as Stage 2 walks each
// keyword. Final results() drains the map into a sorted, capped slice.
type aggregator struct {
	byCitation   map[string]*ScoredCitation
	rrfK         int
	bmWeight     float64
	symbolWeight float64
	ckvWeight    float64
}

func newAggregator(rrfK int, bmWeight, symbolWeight, ckvWeight float64) *aggregator {
	return &aggregator{
		byCitation:   make(map[string]*ScoredCitation),
		rrfK:         rrfK,
		bmWeight:     bmWeight,
		symbolWeight: symbolWeight,
		ckvWeight:    ckvWeight,
	}
}

// addCkvList credits every hit in a ranked ckv semantic-search result
// list. Unlike the per-keyword BM25/symbol lists, this is a single list
// (the semantic recall for the whole prompt), so it is added once. rank
// is 1-based by list position. Empty lists are a no-op.
//
// Wiring the ckv hits into the same RRF as ckg restores the semantic
// signal to the final citation set: Stage 1 already computed these hits
// (broad embedding recall) but earlier only mined their symbol names for
// keywords and then discarded the citations. Measured in isolation the
// ckv list out-recalls keyword BM25 on natural-language prompts, so it
// carries CkvWeight >= BMWeight by default.
func (a *aggregator) addCkvList(hits []contract.Hit) {
	for i, h := range hits {
		rank := i + 1
		contribution := a.ckvWeight / float64(a.rrfK+rank)
		sc := a.entry(h.Citation)
		if sc.ChunkKind == "" {
			sc.ChunkKind = h.ChunkKind
		}
		sc.Score += contribution
		sc.Sources = append(sc.Sources,
			fmt.Sprintf("ckv:semantic@rank=%d(+%.5f)", rank, contribution))
	}
}

// addBM25List credits every hit in a ranked BM25 result list. rank is
// derived from list position (1-based). Empty lists are a no-op.
func (a *aggregator) addBM25List(keyword string, hits []contract.Hit) {
	for i, h := range hits {
		rank := i + 1
		contribution := a.bmWeight / float64(a.rrfK+rank)
		sc := a.entry(h.Citation)
		sc.Score += contribution
		sc.Sources = append(sc.Sources,
			fmt.Sprintf("bm25:%s@rank=%d(+%.5f)", keyword, rank, contribution))
	}
}

// addSymbolList credits every citation in a ranked FindSymbol result
// list. rank is 1-based by list position. Empty lists are a no-op.
func (a *aggregator) addSymbolList(keyword string, cits []contract.Citation) {
	a.addSymbolListWeighted(keyword, cits, a.symbolWeight, false)
}

// SourceExactSymbolPrefix marks a citation that came from a
// prompt-verbatim, unambiguously resolved FindSymbol hit — the gate in
// searcher.go. The budget allocator reserves a slot for these, so the
// provenance has to survive as more than a score contribution.
const SourceExactSymbolPrefix = "symbol_exact:"

// addSymbolListWeighted is addSymbolList with an explicit RRF weight —
// used for the prompt-verbatim boost (see DefaultPromptExactBoost).
// exact tags the provenance so downstream stages can tell a boosted hit
// from an ordinary symbol hit; the score alone does not say which gate
// fired.
func (a *aggregator) addSymbolListWeighted(keyword string, cits []contract.Citation, weight float64, exact bool) {
	prefix := "symbol:"
	if exact {
		prefix = SourceExactSymbolPrefix
	}
	for i, c := range cits {
		rank := i + 1
		contribution := weight / float64(a.rrfK+rank)
		sc := a.entry(c)
		sc.Score += contribution
		sc.Sources = append(sc.Sources,
			fmt.Sprintf("%s%s@rank=%d(+%.5f)", prefix, keyword, rank, contribution))
	}
}

// HasExactSymbolSource reports whether any of sources came from the
// prompt-verbatim symbol gate.
//
// Written without strings.HasPrefix on purpose: adding an import to this
// file shifts every line below it, and the rrfk-constant-lookup scenario
// anchors on DefaultRRFK's span near the top. Keeping the import block
// untouched keeps eval coordinates stable.
func HasExactSymbolSource(sources []string) bool {
	return ExactSymbolKeyword(sources) != ""
}

// ExactSymbolKeyword returns the keyword behind the first exact-symbol
// source, or "" when there is none. The budget allocator reserves per
// distinct keyword: two different symbols named verbatim in one prompt
// are two answers, not one competing for a single slot.
func ExactSymbolKeyword(sources []string) string {
	p := SourceExactSymbolPrefix
	for _, s := range sources {
		if len(s) < len(p) || s[:len(p)] != p {
			continue
		}
		kw := s[len(p):]
		for i := 0; i < len(kw); i++ {
			if kw[i] == '@' {
				return kw[:i]
			}
		}
		return kw
	}
	return ""
}

func (a *aggregator) entry(c contract.Citation) *ScoredCitation {
	key := c.Key()
	sc, ok := a.byCitation[key]
	if !ok {
		sc = &ScoredCitation{Citation: c}
		a.byCitation[key] = sc
	}
	return sc
}

// results returns the accumulated citations sorted by descending Score.
// Ties are broken by File path for deterministic output (eval reports
// must reproduce). When cap > 0, the slice is truncated to that length.
//
// When demoteTests is true, every citation whose File is a test path
// (as classified by isTestPath) has its Score multiplied by
// testDemotionFactor before sorting. This ensures production code ranks
// above test files when the active intent is not test-oriented, while
// keeping test files available lower in the evidence pack.
//
// When demoteDocs is true, documentation citations (isDocCitation) are
// likewise multiplied by docDemotionFactor and file_header chunks by
// headerDemotionFactor, so code-seeking intents rank symbol chunks
// above markdown and above package-orientation headers. Independently
// of both flags, citations under an archive/ path are multiplied by
// archiveDemotionFactor — superseded material is never the current
// answer (factors do not stack: archive > doc > header precedence).
//
// Demotion is applied to the returned copy — the aggregator's internal
// map is not mutated, so results() can be called multiple times with
// different flags.
func (a *aggregator) results(cap int, demoteTests, demoteDocs bool) []ScoredCitation {
	if len(a.byCitation) == 0 {
		return nil
	}

	out := make([]ScoredCitation, 0, len(a.byCitation))
	for _, sc := range a.byCitation {
		entry := *sc
		if demoteTests && isTestPath(entry.Citation.File) {
			entry.Score *= testDemotionFactor
		}
		switch {
		case isArchivePath(entry.Citation.File):
			entry.Score *= archiveDemotionFactor
		case demoteDocs && isDocCitation(entry.ChunkKind, entry.Citation.File):
			entry.Score *= docDemotionFactor
		case demoteDocs && isHeaderChunk(entry.ChunkKind):
			entry.Score *= headerDemotionFactor
		}
		out = append(out, entry)
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		// Stable tiebreaker: file path lexical order, then start line.
		if out[i].Citation.File != out[j].Citation.File {
			return out[i].Citation.File < out[j].Citation.File
		}
		return out[i].Citation.StartLine < out[j].Citation.StartLine
	})

	if cap > 0 && len(out) > cap {
		out = out[:cap]
	}
	return out
}
