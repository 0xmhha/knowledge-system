package main

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// armSameFile lists the declarations of the start's own file by line
// distance. It is a reference arm, not judged.
const armSameFile = "same_file"

// minChangedDecls is the smallest question the leave-one-out can use.
const minChangedDecls = 2

// hunk is one changed line range of the fix at the base commit. An insert
// has no base lines; Start is the line it follows.
type hunk struct {
	File   string `json:"file"`
	Start  int    `json:"start"`
	End    int    `json:"end"`
	Insert bool   `json:"insert"`
}

// cochangeQuestion is one fix commit of the co-change experiment.
type cochangeQuestion struct {
	ID      string `json:"id"`
	Stage1  bool   `json:"stage1"`
	Fix     string `json:"fix"`
	Base    string `json:"base"`
	GraphDB string `json:"graph"`
	Hunks   []hunk `json:"hunks"`
}

// startResult is the leave-one-out result of one start declaration.
type startResult struct {
	Start      loc                   `json:"start"`
	QName      string                `json:"qname"`
	GoldTotal  int                   `json:"gold_total"`
	GoldOther  int                   `json:"gold_other_file"`
	GraphError bool                  `json:"graph_start_unresolved"`
	Measures   map[string]armMeasure `json:"measures"`
	OtherFile  map[string]armMeasure `json:"measures_other_file"`
}

// cochangeResult is the per-question output.
type cochangeResult struct {
	ID      string        `json:"id"`
	Stage1  bool          `json:"stage1"`
	Base    string        `json:"base"`
	Changed []loc         `json:"changed"`
	Starts  []startResult `json:"starts"`
}

// cochangeSummary is one line of the distilled output.
type cochangeSummary struct {
	ID              string             `json:"id"`
	Stage1          bool               `json:"stage1"`
	Changed         int                `json:"changed"`
	MeanRecall      map[string]float64 `json:"mean_recall_at_10"`
	MeanReach       map[string]float64 `json:"mean_reach"`
	StartsWithOther int                `json:"starts_with_other_file_gold"`
	MeanRecallOther map[string]float64 `json:"mean_recall_at_10_other_file"`
	StartUnresolved int                `json:"starts_graph_unresolved"`
}

func loadCochange(dataDir, name string) ([]cochangeQuestion, error) {
	var doc struct {
		Questions []cochangeQuestion `json:"questions"`
	}
	if err := readJSON(filepath.Join(dataDir, name), &doc); err != nil {
		return nil, err
	}
	return doc.Questions, nil
}

// changedDecls maps hunks to the innermost declarations they touch, one node
// per location, sorted by file and line. An insert counts only when the lines
// on both sides of it lie in the same declaration.
func changedDecls(fn *fileNodes, hs []hunk) ([]types.Node, error) {
	byLoc := map[loc]types.Node{}
	for _, h := range hs {
		ns, err := fn.get(h.File)
		if err != nil {
			return nil, err
		}
		if h.Insert {
			a, okA := innermostDecl(ns, h.Start)
			b, okB := innermostDecl(ns, h.Start+1)
			if okA && okB && nodeLoc(a) == nodeLoc(b) {
				byLoc[nodeLoc(a)] = a
			}
			continue
		}
		for line := h.Start; line <= h.End; line++ {
			if n, ok := innermostDecl(ns, line); ok {
				byLoc[nodeLoc(n)] = n
			}
		}
	}
	out := make([]types.Node, 0, len(byLoc))
	for l := range byLoc {
		// Two nodes can share a location (promoted methods); the canonical
		// one is the declaration itself.
		ns, err := fn.get(l.File)
		if err != nil {
			return nil, err
		}
		out = append(out, canonicalAt(ns, l, byLoc[l]))
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].FilePath != out[j].FilePath {
			return out[i].FilePath < out[j].FilePath
		}
		if out[i].StartLine != out[j].StartLine {
			return out[i].StartLine < out[j].StartLine
		}
		return out[i].EndLine < out[j].EndLine
	})
	return out, nil
}

func canonicalAt(ns []types.Node, l loc, fallback types.Node) types.Node {
	for _, n := range seedOrder(ns) {
		if declTypes[n.Type] && nodeLoc(n) == l {
			return n
		}
	}
	return fallback
}

// sameFileArm ranks the other declarations of the start's file by line
// distance from the start.
func sameFileArm(ns []types.Node, start loc) []candidate {
	seen := map[loc]bool{start: true}
	var cands []candidate
	for _, n := range ns {
		l := nodeLoc(n)
		if !declTypes[n.Type] || seen[l] {
			continue
		}
		seen[l] = true
		cands = append(cands, candidate{loc: l, QName: n.QualifiedName})
	}
	sort.Slice(cands, func(i, j int) bool {
		di, dj := absInt(cands[i].StartLine-start.StartLine), absInt(cands[j].StartLine-start.StartLine)
		if di != dj {
			return di < dj
		}
		if cands[i].StartLine != cands[j].StartLine {
			return cands[i].StartLine < cands[j].StartLine
		}
		return cands[i].EndLine < cands[j].EndLine
	})
	for i := range cands {
		cands[i].Rank = i + 1
	}
	return cands
}

// runCochange runs the leave-one-out on one question. A question with fewer
// than minChangedDecls declarations comes back without starts.
func runCochange(ctx context.Context, cfg config, q cochangeQuestion) (cochangeResult, error) {
	res := cochangeResult{ID: q.ID, Stage1: q.Stage1, Base: q.Base, Changed: []loc{}, Starts: []startResult{}}
	dbPath := filepath.Join(cfg.root, q.GraphDB)
	r, err := store.OpenReadOnly(dbPath)
	if err != nil {
		return res, fmt.Errorf("open %s: %w", dbPath, err)
	}
	// Both handles are read-only (immutable=1): a close error loses nothing.
	defer func() { _ = r.Close() }()
	fn := newFileNodes(r)
	decls, err := changedDecls(fn, q.Hunks)
	if err != nil {
		return res, err
	}
	for _, d := range decls {
		res.Changed = append(res.Changed, nodeLoc(d))
	}
	if len(decls) < minChangedDecls {
		return res, nil
	}
	ckg, err := ckgclient.NewReal(dbPath)
	if err != nil {
		return res, err
	}
	defer func() { _ = ckg.Close() }()
	files, err := scopeFiles(filepath.Dir(dbPath))
	if err != nil {
		return res, err
	}
	srcRoot := filepath.Join(cfg.root, "knowledge-data", "agent-eval", "sources", q.Base)
	for i := range decls {
		sr, err := runStart(ctx, ckg, fn, srcRoot, files, decls, i)
		if err != nil {
			return res, fmt.Errorf("start %s: %w", decls[i].QualifiedName, err)
		}
		res.Starts = append(res.Starts, sr)
	}
	return res, nil
}

func runStart(ctx context.Context, ckg ckgclient.Client, fn *fileNodes, srcRoot string, files []string, decls []types.Node, i int) (startResult, error) {
	start := decls[i]
	sl := nodeLoc(start)
	sr := startResult{Start: sl, QName: start.QualifiedName, Measures: map[string]armMeasure{}, OtherFile: map[string]armMeasure{}}
	var gold, other []resolvedGold
	for j, d := range decls {
		if j == i {
			continue
		}
		l := nodeLoc(d)
		g := resolvedGold{Node: &l, Resolved: true}
		gold = append(gold, g)
		if l.File != sl.File {
			other = append(other, g)
		}
	}
	sr.GoldTotal, sr.GoldOther = len(gold), len(other)

	seed := resolvedSeed{seedRef: seedRef{Symbol: start.Name, File: sl.File}, Nodes: []loc{sl}}
	gc, _, err := grepArm(srcRoot, files, fn, []resolvedSeed{seed})
	if err != nil {
		return sr, err
	}
	gr, failed, err := graphArm(ctx, ckg, fn, []types.Node{start}, variantUncapped)
	if err != nil {
		return sr, err
	}
	sr.GraphError = len(failed) > 0
	ns, err := fn.get(sl.File)
	if err != nil {
		return sr, err
	}
	for arm, cands := range map[string][]candidate{
		armGrep: gc, string(variantUncapped): gr, armSameFile: sameFileArm(ns, sl),
	} {
		sr.Measures[arm] = measure(cands, gold)
		if len(other) > 0 {
			sr.OtherFile[arm] = measure(cands, other)
		}
	}
	return sr, nil
}

// summary averages the start results of one question.
func (r cochangeResult) summary() cochangeSummary {
	s := cochangeSummary{
		ID: r.ID, Stage1: r.Stage1, Changed: len(r.Changed),
		MeanRecall: map[string]float64{}, MeanReach: map[string]float64{}, MeanRecallOther: map[string]float64{},
	}
	for _, st := range r.Starts {
		for arm, m := range st.Measures {
			s.MeanRecall[arm] += m.RecallAt10
			s.MeanReach[arm] += m.Reach
		}
		if st.GoldOther > 0 {
			s.StartsWithOther++
		}
		for arm, m := range st.OtherFile {
			s.MeanRecallOther[arm] += m.RecallAt10
		}
		if st.GraphError {
			s.StartUnresolved++
		}
	}
	if n := float64(len(r.Starts)); n > 0 {
		for arm := range s.MeanRecall {
			s.MeanRecall[arm] /= n
			s.MeanReach[arm] /= n
		}
	}
	if n := float64(s.StartsWithOther); n > 0 {
		for arm := range s.MeanRecallOther {
			s.MeanRecallOther[arm] /= n
		}
	}
	return s
}
