package main

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Weights of RULES.md, fixed before the first run.
const (
	weightInner    = 1.0
	weightOuter    = 0.5
	weightGuard    = 0.8
	weightOmitted  = 0.8
	weightWriter   = 0.6
	weightReturned = 0.6
	weightCaller   = 0.5
)

// Limits of RULES.md.
const (
	maxGuardSteps = 2
	maxDefSteps   = 2
	maxReturned   = 3
	maxCallerHops = 4
	maxFunctions  = 200
	minScore      = 1e-9
)

// flowLines is what the value flow tells about one function and focus lines.
type flowLines struct {
	guards  []int // lines of conditions that control the focus lines
	defs    []int // lines that define values used there
	returns []int // lines of returns that do not end in nil
	failed  bool
}

// flowFunc answers flowLines for a function node.
type flowFunc func(ctx context.Context, node string, focus []int) flowLines

// result is one ranked declaration.
type result struct {
	Rank  int      `json:"rank"`
	ID    string   `json:"-"`
	QName string   `json:"qname"`
	File  string   `json:"file"`
	Line  int      `json:"line"`
	Score float64  `json:"score"`
	Why   []string `json:"why"`

	best float64
}

// tracer runs the rules over one graph.
type tracer struct {
	ix       *index
	flow     flowFunc
	found    map[string]*result
	analysed int
}

func newTracer(ix *index, flow flowFunc) *tracer {
	return &tracer{ix: ix, flow: flow, found: map[string]*result{}}
}

// site is one emitting function with its focus lines.
type site struct {
	fn    string
	lines []int
}

// sites maps the lines of one message part to the functions that emit it.
func (t *tracer) sites(ps []position) []site {
	focus := map[string][]int{}
	for _, p := range ps {
		if fn, ok := t.ix.enclosing(p, funcTypes); ok {
			focus[fn.ID] = append(focus[fn.ID], p.line)
			continue
		}
		v, ok := t.ix.enclosing(p, valueTypes)
		if !ok {
			continue
		}
		for fn, lines := range t.ix.refs[v.ID] {
			focus[fn] = append(focus[fn], lines...)
		}
	}
	out := make([]site, 0, len(focus))
	for fn, lines := range focus {
		slices.Sort(lines)
		out = append(out, site{fn, slices.Compact(lines)})
	}
	slices.SortFunc(out, func(a, b site) int { return cmp.Compare(a.fn, b.fn) })
	return out
}

// run traces every part of the message and returns the ranked candidates.
func (t *tracer) run(ctx context.Context, parts [][]position) []result {
	for i, ps := range parts {
		w := weightOuter
		if i == len(parts)-1 {
			w = weightInner
		}
		ss := t.sites(ps)
		for _, s := range ss {
			score := w / float64(len(ss))
			why := []string{fmt.Sprintf("emits message part %d", i+1)}
			t.add(s.fn, score, why)
			t.expand(ctx, s, score, why, 0)
			t.callers(s.fn, score, why)
		}
	}
	return t.ranked()
}

// add gives a declaration score by one way of reaching it.
func (t *tracer) add(id string, score float64, why []string) {
	n, ok := t.ix.nodes[id]
	if !ok || score < minScore {
		return
	}
	r := t.found[id]
	if r == nil {
		r = &result{ID: id, QName: n.QualifiedName, File: n.FilePath, Line: n.StartLine}
		t.found[id] = r
	}
	r.Score += score
	if score > r.best {
		r.best, r.Why = score, why
	}
}

// expand applies rules G, O, W and R to one function and its focus lines.
func (t *tracer) expand(ctx context.Context, s site, score float64, why []string, depth int) {
	if t.analysed >= maxFunctions {
		return
	}
	t.analysed++
	f := t.flow(ctx, s.fn, s.lines)
	guards := f.guards
	if len(guards) == 0 {
		guards = s.lines
	}
	near := union(guards, f.defs)
	t.values(s, near, score, why)
	t.fields(s, guards, near, score, why)
	t.returned(ctx, s, near, score, why, depth)
}

// values is rule G: package-level constants and variables on the lines.
func (t *tracer) values(s site, lines []int, score float64, why []string) {
	var ids []string
	for _, e := range t.edgesOn(s.fn, lines) {
		if n := t.ix.nodes[e.dst]; e.kind == types.EdgeReferences && valueTypes[n.Type] && t.ix.packageLevel(n) {
			ids = append(ids, e.dst)
		}
	}
	t.addAll(ids, score*weightGuard, why, "value in a condition of "+t.name(s.fn))
}

// fields is rules O and W.
func (t *tracer) fields(s site, guards, near []int, score float64, why []string) {
	read := func(lines []int) []string {
		var ids []string
		for _, e := range t.edgesOn(s.fn, lines) {
			if e.kind == types.EdgeReadsField {
				ids = append(ids, e.dst)
			}
		}
		slices.Sort(ids)
		return slices.Compact(ids)
	}
	for _, f := range read(guards) {
		n := t.ix.nodes[f]
		t.addAll(t.ix.omitters(n), score*weightOmitted, why, "handles "+owner(n.QualifiedName)+" and never writes "+n.Name+", checked in "+t.name(s.fn))
	}
	for _, f := range read(near) {
		n := t.ix.nodes[f]
		t.addAll(t.ix.writers[f], score*weightWriter, why, "writes "+n.QualifiedName+", read in "+t.name(s.fn))
	}
}

// returned is rule R: the functions called on the lines, and their own error
// returns.
func (t *tracer) returned(ctx context.Context, s site, lines []int, score float64, why []string, depth int) {
	if depth >= maxReturned {
		return
	}
	var ids []string
	for _, e := range t.edgesOn(s.fn, lines) {
		if !callEdges[e.kind] {
			continue
		}
		if impl := t.ix.impls[e.dst]; len(impl) > 0 {
			ids = append(ids, impl...)
			continue
		}
		ids = append(ids, e.dst)
	}
	slices.Sort(ids)
	ids = slices.Compact(ids)
	for _, id := range ids {
		sc := score * weightReturned / float64(len(ids))
		step := append(slices.Clone(why), "called by "+t.name(s.fn)+" for a value it checks")
		t.add(id, sc, step)
		if !funcTypes[t.ix.nodes[id].Type] {
			continue
		}
		if ret := t.flow(ctx, id, nil).returns; len(ret) > 0 {
			t.expand(ctx, site{id, ret}, sc, step, depth+1)
		}
	}
}

// callers is rule C.
func (t *tracer) callers(fn string, score float64, why []string) {
	seen := map[string]bool{fn: true}
	level := []string{fn}
	for hop := 1; hop <= maxCallerHops && len(level) > 0; hop++ {
		var next []string
		for _, id := range level {
			for _, c := range t.ix.callersOf(id) {
				if !seen[c] {
					seen[c] = true
					next = append(next, c)
				}
			}
		}
		score *= weightCaller
		t.addAll(next, score, why, fmt.Sprintf("calls %s, %d hops", t.name(fn), hop))
		level = next
	}
}

// addAll splits a score over the declarations of one step.
func (t *tracer) addAll(ids []string, score float64, why []string, reason string) {
	slices.Sort(ids)
	ids = slices.Compact(ids)
	for _, id := range ids {
		t.add(id, score/float64(len(ids)), append(slices.Clone(why), reason))
	}
}

func (t *tracer) edgesOn(fn string, lines []int) []lineEdge {
	var out []lineEdge
	for _, e := range t.ix.from[fn] {
		if slices.Contains(lines, e.line) {
			out = append(out, e)
		}
	}
	return out
}

func (t *tracer) name(id string) string { return t.ix.nodes[id].QualifiedName }

func (t *tracer) ranked() []result {
	out := make([]result, 0, len(t.found))
	for _, r := range t.found {
		out = append(out, *r)
	}
	slices.SortFunc(out, func(a, b result) int {
		return cmp.Or(cmp.Compare(b.Score, a.Score), strings.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line), strings.Compare(a.ID, b.ID))
	})
	for i := range out {
		out[i].Rank = i + 1
	}
	return out
}

func union(a, b []int) []int {
	out := append(slices.Clone(a), b...)
	slices.Sort(out)
	return slices.Compact(out)
}
