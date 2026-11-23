package main

import (
	"fmt"
	"math"
	"path/filepath"
	"sort"
	"strings"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// directScore is what one edge between the start and a candidate adds,
// however many edges join them.
const directScore = 1.0

// linkWeight is what a shared middle node of the given degree adds.
type linkWeight struct {
	name string
	of   func(degree int) float64
}

// linkGroup names the edge types one link ranking walks.
type linkGroup struct {
	name  string
	edges map[types.EdgeType]bool
}

func linkGroups() []linkGroup {
	field := []types.EdgeType{types.EdgeReadsField, types.EdgeWritesField}
	calls := append([]types.EdgeType{types.EdgeCalls, types.EdgeInvokes}, field...)
	all := append([]types.EdgeType{types.EdgeReferences, types.EdgeExtends, types.EdgeDefines}, calls...)
	set := func(ts []types.EdgeType) map[types.EdgeType]bool {
		m := map[types.EdgeType]bool{}
		for _, t := range ts {
			m[t] = true
		}
		return m
	}
	return []linkGroup{{"field", set(field)}, {"calls+field", set(calls)}, {"all7", set(all)}}
}

func linkWeights() []linkWeight {
	return []linkWeight{
		{"count", func(int) float64 { return 1 }},
		{"log", func(d int) float64 { return 1 / math.Log(1+float64(d)) }},
		{"inverse", func(d int) float64 { return 1 / float64(d) }},
	}
}

func linkArmName(g linkGroup, w linkWeight) string { return g.name + "/" + w.name }

// linkGraph is the undirected neighbour sets of one edge group. Two nodes
// joined by several edges are neighbours once.
type linkGraph struct{ next map[string][]string }

func newLinkGraph(edges []types.Edge, g linkGroup) *linkGraph {
	seen := map[[2]string]bool{}
	lg := &linkGraph{next: map[string][]string{}}
	for _, e := range edges {
		if !g.edges[e.Type] || e.Src == e.Dst {
			continue
		}
		a, b := e.Src, e.Dst
		if a > b {
			a, b = b, a
		}
		if seen[[2]string{a, b}] {
			continue
		}
		seen[[2]string{a, b}] = true
		lg.next[a] = append(lg.next[a], b)
		lg.next[b] = append(lg.next[b], a)
	}
	// Sorted lists keep the floating-point sums in one order on every run.
	for id := range lg.next {
		sort.Strings(lg.next[id])
	}
	return lg
}

// linkScores scores every node within two hops of src: directScore for a
// neighbour, plus the weight of each shared middle node. The degree is the
// node's neighbour count in this edge group.
func (lg *linkGraph) linkScores(src string, w linkWeight) map[string]float64 {
	out := map[string]float64{}
	for _, m := range lg.next[src] {
		out[m] += directScore
	}
	for _, m := range lg.next[src] {
		add := w.of(len(lg.next[m]))
		for _, v := range lg.next[m] {
			if v != src {
				out[v] += add
			}
		}
	}
	return out
}

// linkArm ranks the declarations a score map reaches. A location shared by
// several nodes keeps its best score.
func linkArm(scores map[string]float64, declLoc map[string]loc, start loc) []candidate {
	best := map[loc]float64{}
	for id, s := range scores {
		l, ok := declLoc[id]
		if !ok || l == start {
			continue
		}
		if cur, seen := best[l]; !seen || s > cur {
			best[l] = s
		}
	}
	cands := make([]candidate, 0, len(best))
	for l, s := range best {
		cands = append(cands, candidate{loc: l, Score: s})
	}
	sort.Slice(cands, func(i, j int) bool {
		a, b := cands[i], cands[j]
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		if a.File != b.File {
			return a.File < b.File
		}
		if a.StartLine != b.StartLine {
			return a.StartLine < b.StartLine
		}
		return a.EndLine < b.EndLine
	})
	for i := range cands {
		cands[i].Rank = i + 1
	}
	return cands
}

// goDecls maps the declaration nodes of Go files, tests included, to their
// location.
func goDecls(nodes []types.Node) map[string]loc {
	out := map[string]loc{}
	for _, n := range nodes {
		if declTypes[n.Type] && strings.HasSuffix(n.FilePath, ".go") {
			out[n.ID] = nodeLoc(n)
		}
	}
	return out
}

// linkResult is the per-question output of the link ranking.
type linkResult struct {
	ID     string            `json:"id"`
	Stage1 bool              `json:"stage1"`
	Starts []linkStartResult `json:"starts"`
}

type linkStartResult struct {
	Start     loc                   `json:"start"`
	Measures  map[string]armMeasure `json:"measures"`
	OtherFile map[string]armMeasure `json:"measures_other_file"`
}

// linkRow is the per-question mean over starts.
type linkRow struct {
	ID              string             `json:"id"`
	Stage1          bool               `json:"stage1"`
	StartsWithOther int                `json:"starts_with_other_file_gold"`
	MeanRecall      map[string]float64 `json:"mean_recall_at_10"`
	MeanRecallOther map[string]float64 `json:"mean_recall_at_10_other_file"`
	MeanReachOther  map[string]float64 `json:"mean_reach_other_file"`
	MeanCandidates  map[string]float64 `json:"mean_candidates"`
}

// runLink ranks the changed declarations of one question from each other with
// every link ranking, or only the one named by cfg.link.
func runLink(cfg config, q cochangeQuestion) (linkResult, error) {
	res := linkResult{ID: q.ID, Stage1: q.Stage1, Starts: []linkStartResult{}}
	dbPath := filepath.Join(cfg.root, q.GraphDB)
	r, err := store.OpenReadOnly(dbPath)
	if err != nil {
		return res, fmt.Errorf("open %s: %w", dbPath, err)
	}
	// Read-only (immutable=1): a close error loses nothing.
	defer func() { _ = r.Close() }()
	decls, err := changedDecls(newFileNodes(r), q.Hunks)
	if err != nil {
		return res, err
	}
	if len(decls) < minChangedDecls {
		return res, nil
	}
	nodes, err := r.AllNodes()
	if err != nil {
		return res, fmt.Errorf("all nodes: %w", err)
	}
	edges, err := r.AllEdges()
	if err != nil {
		return res, fmt.Errorf("all edges: %w", err)
	}
	declLoc := goDecls(nodes)
	for i := range decls {
		res.Starts = append(res.Starts, linkStartResult{
			Start: nodeLoc(decls[i]), Measures: map[string]armMeasure{}, OtherFile: map[string]armMeasure{},
		})
	}
	for _, g := range linkGroups() {
		lg := newLinkGraph(edges, g)
		for _, w := range linkWeights() {
			name := linkArmName(g, w)
			if cfg.link != "" && cfg.link != name {
				continue
			}
			for i := range decls {
				sl := nodeLoc(decls[i])
				cands := linkArm(lg.linkScores(decls[i].ID, w), declLoc, sl)
				gold, other := goldWithout(decls, i)
				res.Starts[i].Measures[name] = measure(cands, gold)
				if len(other) > 0 {
					res.Starts[i].OtherFile[name] = measure(cands, other)
				}
			}
		}
	}
	return res, nil
}

// goldWithout returns the other declarations as gold, and those of them in
// another file than declaration i.
func goldWithout(decls []types.Node, i int) (gold, other []resolvedGold) {
	start := nodeLoc(decls[i])
	for j, d := range decls {
		if j == i {
			continue
		}
		l := nodeLoc(d)
		g := resolvedGold{Node: &l, Resolved: true}
		gold = append(gold, g)
		if l.File != start.File {
			other = append(other, g)
		}
	}
	return gold, other
}

func (r linkResult) row() linkRow {
	out := linkRow{
		ID: r.ID, Stage1: r.Stage1, MeanRecall: map[string]float64{}, MeanRecallOther: map[string]float64{},
		MeanReachOther: map[string]float64{}, MeanCandidates: map[string]float64{},
	}
	n := float64(len(r.Starts))
	for _, s := range r.Starts {
		if len(s.OtherFile) > 0 {
			out.StartsWithOther++
		}
	}
	for _, s := range r.Starts {
		for arm, m := range s.Measures {
			out.MeanRecall[arm] += m.RecallAt10 / n
			out.MeanCandidates[arm] += float64(m.Candidates) / n
		}
		for arm, m := range s.OtherFile {
			out.MeanRecallOther[arm] += m.RecallAt10 / float64(out.StartsWithOther)
			out.MeanReachOther[arm] += m.Reach / float64(out.StartsWithOther)
		}
	}
	return out
}
