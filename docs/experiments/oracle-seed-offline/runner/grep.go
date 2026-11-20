package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
)

// maxLineBytes bounds one source line for the scanner. Generated files in
// go-stablenet carry long lines (embedded bytecode); the default 64 KiB
// would stop the scan there.
const maxLineBytes = 16 << 20

// candidate is one ranked entry of either arm.
type candidate struct {
	Rank int `json:"rank"`
	loc
	QName string  `json:"qname"`
	ID    string  `json:"id,omitempty"`
	Score float64 `json:"score"`
	// Grep arm: matching lines per seed symbol.
	SeedHits map[string]int `json:"seed_hits,omitempty"`
	// Graph arm: shortest hop and the relation of the kept path.
	Distance int    `json:"distance,omitempty"`
	Relation string `json:"relation,omitempty"`
}

// grepStats counts what the grep arm saw but could not rank.
type grepStats struct {
	MatchedLines  int `json:"matched_lines"`
	UnmappedLines int `json:"unmapped_lines"`
	SeedSelfLines int `json:"seed_self_lines"`
}

// grepArm searches every scope file for each seed symbol with word
// boundaries (Go's \b is the ASCII [0-9A-Za-z_] boundary rg -w uses), maps a
// matching line to its innermost declaration node and scores a node by its
// matching lines summed over the seeds. Lines inside a seed's own
// declaration are not counted. Ties go to the node id.
func grepArm(srcRoot string, files []string, fn *fileNodes, seeds []resolvedSeed) ([]candidate, grepStats, error) {
	seedKeys := map[loc]bool{}
	symbols := []string{}
	seenSym := map[string]bool{}
	for _, s := range seeds {
		for _, l := range s.Nodes {
			seedKeys[l] = true
		}
		if !seenSym[s.Symbol] && len(s.Nodes) > 0 {
			seenSym[s.Symbol] = true
			symbols = append(symbols, s.Symbol)
		}
	}
	res := make([]*regexp.Regexp, len(symbols))
	for i, s := range symbols {
		res[i] = regexp.MustCompile(`\b` + regexp.QuoteMeta(s) + `\b`)
	}

	var st grepStats
	byLoc := map[loc]*candidate{}
	for _, f := range files {
		hits, err := grepFile(filepath.Join(srcRoot, f), res)
		if err != nil {
			return nil, st, err
		}
		if len(hits) == 0 {
			continue
		}
		ns, err := fn.get(f)
		if err != nil {
			return nil, st, err
		}
		for _, h := range hits {
			st.MatchedLines++
			n, ok := innermostDecl(ns, h.line)
			if !ok {
				st.UnmappedLines++
				continue
			}
			l := nodeLoc(n)
			if seedKeys[l] {
				st.SeedSelfLines++
				continue
			}
			c := byLoc[l]
			if c == nil {
				c = &candidate{loc: l, QName: n.QualifiedName, ID: n.ID, SeedHits: map[string]int{}}
				byLoc[l] = c
			}
			c.Score++
			c.SeedHits[symbols[h.symbol]]++
		}
	}

	out := make([]candidate, 0, len(byLoc))
	for _, c := range byLoc {
		out = append(out, *c)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Score != out[j].Score {
			return out[i].Score > out[j].Score
		}
		return out[i].ID < out[j].ID
	})
	for i := range out {
		out[i].Rank = i + 1
	}
	return out, st, nil
}

type lineHit struct{ line, symbol int }

// grepFile returns one hit per (line, symbol) pair that matches.
func grepFile(path string, res []*regexp.Regexp) ([]lineHit, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }() // read-only: a close error loses nothing
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64<<10), maxLineBytes)
	var out []lineHit
	for ln := 1; sc.Scan(); ln++ {
		b := sc.Bytes()
		for i, re := range res {
			if re.Match(b) {
				out = append(out, lineHit{line: ln, symbol: i})
			}
		}
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("scan %s: %w", path, err)
	}
	return out, nil
}
