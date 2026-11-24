package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Arm names of the curve mode (docs/experiments/cross-test/CURVE.md).
const (
	curveGraph     = "graph"
	curveWide      = "wide"
	curveBase      = "base"
	curveBaseGraph = "base+graph"
	curveBaseWide  = "base+wide"
	// curvePackage lists the declarations of the start's directory outside its
	// file. It uses no edge and is the control of CURVE.md section 6.
	curvePackage     = "package"
	curveBasePackage = "base+package"
)

// testFileSuffix marks files no arm lists.
const testFileSuffix = "_test.go"

// curveLink is the link ranking the wide arm uses, as confirmed in
// DESIGN.md section 12.
const curveLink = "all7/log"

// curveCase is one question of the curve mode. LeaveOneOut cases list their
// changed declarations as starts and take the others as gold, one sub-case per
// start.
type curveCase struct {
	ID          string `json:"id"`
	Family      string `json:"family"`
	Fix         string `json:"fix"`
	Base        string `json:"base"`
	GraphDB     string `json:"graph"`
	LeaveOneOut bool   `json:"leave_one_out"`
	Starts      []loc  `json:"starts"`
	Gold        []loc  `json:"gold"`
}

// curveGold is where one gold declaration appears in one reading list.
type curveGold struct {
	// Rank is the 1-based place in the list, or rankNotReached.
	Rank int `json:"rank"`
	// Lines is the number of source lines read up to and including it.
	Lines int `json:"lines"`
}

// curveArm is one arm's reading list reduced to its measures.
type curveArm struct {
	Candidates int         `json:"candidates"`
	Gold       []curveGold `json:"gold"`
}

// curveStart is the result of one start set.
type curveStart struct {
	Starts []loc               `json:"starts"`
	Gold   []loc               `json:"gold"`
	Arms   map[string]curveArm `json:"arms"`
}

// curveResult is the per-case output file.
type curveResult struct {
	ID      string       `json:"id"`
	Family  string       `json:"family"`
	Fix     string       `json:"fix"`
	Results []curveStart `json:"results"`
	// Skipped says why a case has no results.
	Skipped string `json:"skipped,omitempty"`
}

// folder folds a location to its top-level declaration.
type folder struct {
	fn *fileNodes
}

// fold maps l to the widest function or method holding it, else to itself when
// it is a declaration, else to the innermost declaration around it. Test files
// and locations outside any declaration fold to nothing.
func (f folder) fold(l loc) (loc, bool, error) {
	if l.File == "" || strings.HasSuffix(l.File, testFileSuffix) {
		return loc{}, false, nil
	}
	ns, err := f.fn.get(l.File)
	if err != nil {
		return loc{}, false, err
	}
	var outer, self loc
	okOuter, okSelf := false, false
	for _, n := range ns {
		if !declTypes[n.Type] || n.StartLine > l.StartLine || n.EndLine < l.EndLine {
			continue
		}
		nl := nodeLoc(n)
		if isFuncType(n.Type) && (!okOuter || outer.EndLine-outer.StartLine < span(n)) {
			outer, okOuter = nl, true
		}
		if nl == l {
			self, okSelf = nl, true
		}
	}
	if okOuter {
		return outer, true, nil
	}
	if okSelf {
		return self, true, nil
	}
	n, ok := innermostDecl(ns, l.StartLine)
	return nodeLoc(n), ok, nil
}

func isFuncType(t types.NodeType) bool {
	return t == types.NodeFunction || t == types.NodeMethod || t == types.NodeConstructor || t == types.NodeModifier
}

// foldAll folds and deduplicates, keeping the first place of each.
func (f folder) foldAll(ls []loc) ([]loc, error) {
	seen := map[loc]bool{}
	out := make([]loc, 0, len(ls))
	for _, l := range ls {
		top, ok, err := f.fold(l)
		if err != nil {
			return nil, err
		}
		if ok && !seen[top] {
			seen[top] = true
			out = append(out, top)
		}
	}
	return out, nil
}

// alternate merges lists one entry at a time, keeping the first of each
// location.
func alternate(lists ...[]loc) []loc {
	seen := map[loc]bool{}
	var out []loc
	for i := 0; ; i++ {
		more := false
		for _, l := range lists {
			if i >= len(l) {
				continue
			}
			more = true
			if !seen[l[i]] {
				seen[l[i]] = true
				out = append(out, l[i])
			}
		}
		if !more {
			return out
		}
	}
}

// readingList puts the starts before the candidates.
func readingList(starts, cands []loc) []loc {
	seen := map[loc]bool{}
	out := make([]loc, 0, len(starts)+len(cands))
	for _, ls := range [][]loc{starts, cands} {
		for _, l := range ls {
			if !seen[l] {
				seen[l] = true
				out = append(out, l)
			}
		}
	}
	return out
}

// curveMeasure finds each gold in a reading list and the lines read up to it.
func curveMeasure(list, gold []loc) curveArm {
	at := map[loc]curveGold{}
	lines := 0
	for i, l := range list {
		lines += l.EndLine - l.StartLine + 1
		at[l] = curveGold{Rank: i + 1, Lines: lines}
	}
	m := curveArm{Candidates: len(list), Gold: make([]curveGold, 0, len(gold))}
	for _, g := range gold {
		c, ok := at[g]
		if !ok {
			c = curveGold{Rank: rankNotReached}
		}
		m.Gold = append(m.Gold, c)
	}
	return m
}

func candLocs(cs []candidate) []loc {
	out := make([]loc, 0, len(cs))
	for _, c := range cs {
		out = append(out, c.loc)
	}
	return out
}

// curveGraphData is what the arms read from one graph.
type curveGraphData struct {
	ckg     ckgclient.Client
	fold    folder
	files   []string
	srcRoot string
	link    *linkGraph
	declLoc map[string]loc
	// byDir holds every foldable declaration of a directory, by file and line.
	byDir map[string][]loc
}

// packageLocs lists the declarations of the start's directory that lie in other
// files, in file and line order.
func packageLocs(byDir map[string][]loc, start loc) []loc {
	out := []loc{}
	for _, l := range byDir[filepath.Dir(start.File)] {
		if l.File != start.File {
			out = append(out, l)
		}
	}
	return out
}

// declsByDir groups declaration locations by directory, sorted by file and line.
func declsByDir(declLoc map[string]loc) map[string][]loc {
	out := map[string][]loc{}
	for _, l := range declLoc {
		dir := filepath.Dir(l.File)
		out[dir] = append(out[dir], l)
	}
	for _, ls := range out {
		sort.Slice(ls, func(i, j int) bool {
			if ls[i].File != ls[j].File {
				return ls[i].File < ls[j].File
			}
			if ls[i].StartLine != ls[j].StartLine {
				return ls[i].StartLine < ls[j].StartLine
			}
			return ls[i].EndLine > ls[j].EndLine
		})
	}
	return out
}

// startLists ranks candidates from one start declaration with every single arm.
func (d curveGraphData) startLists(ctx context.Context, start types.Node) (map[string][]loc, error) {
	sl := nodeLoc(start)
	seed := resolvedSeed{seedRef: seedRef{Symbol: start.Name, File: sl.File}, Nodes: []loc{sl}}
	gc, _, err := grepArm(d.srcRoot, d.files, d.fold.fn, []resolvedSeed{seed})
	if err != nil {
		return nil, err
	}
	gr, _, err := graphArm(ctx, d.ckg, d.fold.fn, []types.Node{start}, variantUncapped)
	if err != nil {
		return nil, err
	}
	ns, err := d.fold.fn.get(sl.File)
	if err != nil {
		return nil, err
	}
	w, err := curveWeight()
	if err != nil {
		return nil, err
	}
	raw := map[string][]candidate{
		armGrep: gc, curveGraph: gr, armSameFile: sameFileArm(ns, sl),
		curveWide: linkArm(d.link.linkScores(start.ID, w), d.declLoc, sl),
	}
	out := map[string][]loc{}
	for arm, cs := range raw {
		if out[arm], err = d.fold.foldAll(candLocs(cs)); err != nil {
			return nil, err
		}
	}
	if out[curvePackage], err = d.fold.foldAll(packageLocs(d.byDir, sl)); err != nil {
		return nil, err
	}
	return out, nil
}

// curveWeight and curveGroup resolve curveLink to the ranking it names.
func curveWeight() (linkWeight, error) {
	for _, w := range linkWeights() {
		if strings.HasSuffix(curveLink, "/"+w.name) {
			return w, nil
		}
	}
	return linkWeight{}, errors.New("no link weight for " + curveLink)
}

func curveGroup() (linkGroup, error) {
	for _, g := range linkGroups() {
		if strings.HasPrefix(curveLink, g.name+"/") {
			return g, nil
		}
	}
	return linkGroup{}, errors.New("no link group for " + curveLink)
}

// runStartSet measures every arm from one set of starts.
func (d curveGraphData) runStartSet(ctx context.Context, starts, gold []loc) (curveStart, error) {
	res := curveStart{Starts: starts, Gold: gold, Arms: map[string]curveArm{}}
	perArm := map[string][][]loc{}
	for _, s := range starts {
		ns, err := d.fold.fn.get(s.File)
		if err != nil {
			return res, err
		}
		n := canonicalAt(ns, s, types.Node{})
		if n.ID == "" {
			return res, fmt.Errorf("start %s:%d: no declaration node", s.File, s.StartLine)
		}
		lists, err := d.startLists(ctx, n)
		if err != nil {
			return res, err
		}
		for arm, l := range lists {
			perArm[arm] = append(perArm[arm], l)
		}
	}
	single := map[string][]loc{}
	for arm, ls := range perArm {
		single[arm] = alternate(ls...)
	}
	single[curveBase] = alternate(single[armSameFile], single[armGrep])
	single[curveBaseGraph] = alternate(single[armSameFile], single[armGrep], single[curveGraph])
	single[curveBaseWide] = alternate(single[armSameFile], single[armGrep], single[curveWide])
	single[curveBasePackage] = alternate(single[armSameFile], single[armGrep], single[curvePackage])
	for arm, l := range single {
		res.Arms[arm] = curveMeasure(readingList(starts, l), gold)
	}
	return res, nil
}

// runCurveCase runs one case against an opened graph.
func (d curveGraphData) runCurveCase(ctx context.Context, c curveCase) (curveResult, error) {
	res := curveResult{ID: c.ID, Family: c.Family, Fix: c.Fix, Results: []curveStart{}}
	starts, err := d.fold.foldAll(c.Starts)
	if err != nil {
		return res, err
	}
	if c.LeaveOneOut {
		if len(starts) < minChangedDecls {
			res.Skipped = "fewer than two changed declarations"
			return res, nil
		}
		for i, s := range starts {
			gold := append(append([]loc{}, starts[:i]...), starts[i+1:]...)
			r, err := d.runStartSet(ctx, []loc{s}, gold)
			if err != nil {
				return res, err
			}
			res.Results = append(res.Results, r)
		}
		return res, nil
	}
	gold, err := d.fold.foldAll(c.Gold)
	if err != nil {
		return res, err
	}
	if len(starts) == 0 || len(gold) == 0 {
		res.Skipped = fmt.Sprintf("starts in graph %d, gold in graph %d", len(starts), len(gold))
		return res, nil
	}
	r, err := d.runStartSet(ctx, starts, gold)
	if err != nil {
		return res, err
	}
	res.Results = append(res.Results, r)
	return res, nil
}

// runCurveGraph opens one graph and runs its cases.
func runCurveGraph(ctx context.Context, cfg config, cases []curveCase) ([]curveResult, error) {
	dbPath := filepath.Join(cfg.root, cases[0].GraphDB)
	r, err := store.OpenReadOnly(dbPath)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", dbPath, err)
	}
	// Both handles are read-only (immutable=1): a close error loses nothing.
	defer func() { _ = r.Close() }()
	ckg, err := ckgclient.NewReal(dbPath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = ckg.Close() }()
	files, err := scopeFiles(filepath.Dir(dbPath))
	if err != nil {
		return nil, err
	}
	nodes, err := r.AllNodes()
	if err != nil {
		return nil, fmt.Errorf("all nodes: %w", err)
	}
	edges, err := r.AllEdges()
	if err != nil {
		return nil, fmt.Errorf("all edges: %w", err)
	}
	g, err := curveGroup()
	if err != nil {
		return nil, err
	}
	d := curveGraphData{
		ckg: ckg, fold: folder{newFileNodes(r)}, files: files,
		srcRoot: filepath.Join(cfg.root, "knowledge-data", "agent-eval", "sources", cases[0].Base),
		link:    newLinkGraph(edges, g), declLoc: goDecls(nodes),
	}
	d.byDir = declsByDir(d.declLoc)
	out := make([]curveResult, 0, len(cases))
	for _, c := range cases {
		res, err := d.runCurveCase(ctx, c)
		if err != nil {
			return nil, fmt.Errorf("case %s/%s: %w", c.Family, c.ID, err)
		}
		slog.Info("case done", "family", c.Family, "id", c.ID, "starts", len(res.Results), "skipped", res.Skipped)
		out = append(out, res)
	}
	return out, nil
}

// runCurveAll runs every case of the file named by -from and writes one
// summary file.
func runCurveAll(ctx context.Context, cfg config) error {
	var cases []curveCase
	if err := readJSON(cfg.from, &cases); err != nil {
		return err
	}
	if err := os.MkdirAll(cfg.outDir, 0o755); err != nil {
		return fmt.Errorf("create %s: %w", cfg.outDir, err)
	}
	byGraph := map[string][]curveCase{}
	for _, c := range cases {
		if len(cfg.only) > 0 && !cfg.only[c.ID] {
			continue
		}
		byGraph[c.GraphDB] = append(byGraph[c.GraphDB], c)
	}
	graphs := make([]string, 0, len(byGraph))
	for g := range byGraph {
		graphs = append(graphs, g)
	}
	sort.Strings(graphs)
	all := []curveResult{}
	for _, g := range graphs {
		rs, err := runCurveGraph(ctx, cfg, byGraph[g])
		if err != nil {
			return err
		}
		all = append(all, rs...)
	}
	sort.Slice(all, func(i, j int) bool {
		if all[i].Family != all[j].Family {
			return all[i].Family < all[j].Family
		}
		return all[i].ID < all[j].ID
	})
	return writeJSON(filepath.Join(cfg.outDir, summaryFile), all)
}
