package semantic

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ConceptCandidate is an interpretation of a query term, never a source
// filter or a verified implementation claim by itself.
type ConceptCandidate struct {
	ConceptID string  `json:"concept_id"`
	Term      string  `json:"term"`
	Status    Status  `json:"status"`
	Score     float64 `json:"score"`
}

// MatchConcepts finds preferred/alternative terms in the raw query. It keeps
// all concepts tied at the cutoff, so an ambiguous term never silently turns
// into one asserted meaning. Rejected concepts are ignored. No query rewrite
// or candidate filtering occurs here.
func (p Projection) MatchConcepts(query string, limit int) ([]ConceptCandidate, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	if limit <= 0 || strings.TrimSpace(query) == "" {
		return nil, nil
	}
	lowerQuery := strings.ToLower(query)
	var candidates []ConceptCandidate
	for _, concept := range p.Concepts {
		if concept.Status == StatusRejected {
			continue
		}
		var best ConceptCandidate
		for _, term := range concept.Terms {
			if !containsTerm(lowerQuery, strings.ToLower(strings.TrimSpace(term.Value))) {
				continue
			}
			score := 0.8
			if term.Preferred {
				score = 1
			}
			if concept.Status == StatusProposed {
				score *= 0.5
			}
			if score > best.Score {
				best = ConceptCandidate{ConceptID: concept.ID, Term: term.Value,
					Status: concept.Status, Score: score}
			}
		}
		if best.Score > 0 {
			candidates = append(candidates, best)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].Score == candidates[j].Score {
			return candidates[i].ConceptID < candidates[j].ConceptID
		}
		return candidates[i].Score > candidates[j].Score
	})
	if len(candidates) > limit {
		cutoff := candidates[limit-1].Score
		end := limit
		for end < len(candidates) && candidates[end].Score == cutoff {
			end++
		}
		candidates = candidates[:end]
	}
	return candidates, nil
}

func containsTerm(query, term string) bool {
	if term == "" {
		return false
	}
	// Latin identifiers need letter/digit boundaries: "api" should not match
	// inside "capillary". Hangul terms may be followed by particles without
	// spaces, so substring matching is retained for those.
	latin := false
	for _, r := range term {
		if unicode.In(r, unicode.Latin) {
			latin = true
			break
		}
	}
	for offset := 0; offset < len(query); {
		at := strings.Index(query[offset:], term)
		if at < 0 {
			return false
		}
		at += offset
		end := at + len(term)
		if !latin || (termBoundary(query[:at], true) && termBoundary(query[end:], false)) {
			return true
		}
		offset = end
	}
	return false
}

func termBoundary(context string, before bool) bool {
	if context == "" {
		return true
	}
	var r rune
	if before {
		r, _ = utf8.DecodeLastRuneInString(context)
	} else {
		r, _ = utf8.DecodeRuneInString(context)
	}
	return !unicode.IsLetter(r) && !unicode.IsDigit(r)
}
