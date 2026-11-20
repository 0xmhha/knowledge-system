package main

import "sort"

// recallCutoff is the k of recall@k in DESIGN.md section 4.
const recallCutoff = 10

// Gold rank sentinels.
const (
	rankSeed       = 0  // the core location is itself a seed
	rankNotReached = -1 // the arm's list does not hold it
)

// armMeasure is one arm's result on one question.
type armMeasure struct {
	Candidates int `json:"candidates"`
	// GoldRanks lists, in oracle order, the rank of each resolved core
	// location: 0 when it is a seed, -1 when the list misses it.
	GoldRanks  []int    `json:"gold_ranks"`
	Reached    int      `json:"reached"`
	InTop10    int      `json:"in_top10"`
	GoldTotal  int      `json:"gold_total"`
	Reach      float64  `json:"reach"`
	RecallAt10 float64  `json:"recall_at_10"`
	MedianRank *float64 `json:"median_rank"`
}

// measure ranks the resolved gold locations in one arm's list. Unresolved
// gold is left out of the denominator; summary.json reports it per question.
func measure(cands []candidate, gold []resolvedGold) armMeasure {
	rankOf := make(map[loc]int, len(cands))
	for _, c := range cands {
		if _, ok := rankOf[c.loc]; !ok {
			rankOf[c.loc] = c.Rank
		}
	}
	m := armMeasure{Candidates: len(cands)}
	var reachedRanks []int
	for _, g := range gold {
		if !g.Resolved {
			continue
		}
		m.GoldTotal++
		r := rankNotReached
		if g.IsSeed {
			r = rankSeed
		} else if rk, ok := rankOf[*g.Node]; ok {
			r = rk
		}
		m.GoldRanks = append(m.GoldRanks, r)
		if r == rankNotReached {
			continue
		}
		m.Reached++
		reachedRanks = append(reachedRanks, r)
		if r <= recallCutoff {
			m.InTop10++
		}
	}
	if m.GoldTotal > 0 {
		m.Reach = float64(m.Reached) / float64(m.GoldTotal)
		m.RecallAt10 = float64(m.InTop10) / float64(m.GoldTotal)
	}
	m.MedianRank = median(reachedRanks)
	return m
}

func median(xs []int) *float64 {
	if len(xs) == 0 {
		return nil
	}
	s := append([]int(nil), xs...)
	sort.Ints(s)
	var v float64
	if n := len(s); n%2 == 1 {
		v = float64(s[n/2])
	} else {
		v = float64(s[n/2-1]+s[n/2]) / 2
	}
	return &v
}
