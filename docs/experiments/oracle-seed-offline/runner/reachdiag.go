package main

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// diagHubLimit is the degree above which a hub-limited walk stops expanding.
const diagHubLimit = 100

// Hop limits of the section 11 walks.
const (
	diagHops      = 3
	diagFieldHops = 2
	diagFarHops   = 6
)

// reachUnreached is the reason of a pair no walk reached.
const reachUnreached = "none"

// arcFilter reports whether a walk may take an arc.
type arcFilter func(a arc) bool

// diagWalk is one walk of section 11: the union of its passes.
type diagWalk struct {
	name   string
	passes []arcFilter
	hops   int
	hub    int // 0: no limit
}

var walkEdges = map[types.EdgeType]bool{
	types.EdgeCalls: true, types.EdgeInvokes: true, types.EdgeImplements: true,
	types.EdgeReferences: true, types.EdgeExtends: true, types.EdgeDefines: true,
}

var fieldEdges = map[types.EdgeType]bool{types.EdgeReadsField: true, types.EdgeWritesField: true}

// plumbing marks the edges W4 leaves out whatever their ends.
var plumbing = map[types.EdgeType]bool{types.EdgeImports: true, types.EdgeExports: true}

func diagWalks() []diagWalk {
	fwd := func(a arc) bool { return a.forward && walkEdges[a.edge] }
	callers := func(a arc) bool { return !a.forward && (a.edge == types.EdgeCalls || a.edge == types.EdgeInvokes) }
	mixed := func(a arc) bool { return walkEdges[a.edge] }
	field := func(a arc) bool { return fieldEdges[a.edge] }
	all := func(a arc) bool { return !plumbing[a.edge] }
	return []diagWalk{
		{name: "W1", passes: []arcFilter{fwd, callers}, hops: diagHops},
		{name: "W2", passes: []arcFilter{mixed}, hops: diagHops},
		{name: "W3", passes: []arcFilter{field}, hops: diagFieldHops},
		{name: "W4", passes: []arcFilter{all}, hops: diagHops},
		{name: "W5", passes: []arcFilter{all}, hops: diagFarHops},
		{name: "W2hub", passes: []arcFilter{mixed}, hops: diagHops, hub: diagHubLimit},
		{name: "W3hub", passes: []arcFilter{field}, hops: diagFieldHops, hub: diagHubLimit},
		{name: "W4hub", passes: []arcFilter{all}, hops: diagHops, hub: diagHubLimit},
	}
}

// reachable returns the ids a walk reaches from src, src excluded.
func reachable(adj *adjacency, src string, w diagWalk) map[string]bool {
	out := map[string]bool{}
	for _, allow := range w.passes {
		dist := map[string]int{src: 0}
		queue := []string{src}
		for i := 0; i < len(queue); i++ {
			cur := queue[i]
			if dist[cur] == w.hops {
				continue
			}
			if w.hub > 0 && cur != src && adj.degree[cur] > w.hub {
				continue
			}
			for _, list := range [][]arc{adj.out[cur], adj.in[cur]} {
				for _, x := range list {
					if _, seen := dist[x.to]; seen || !allow(x) {
						continue
					}
					dist[x.to] = dist[cur] + 1
					queue = append(queue, x.to)
					out[x.to] = true
				}
			}
		}
	}
	return out
}

// diagPair is one start-gold pair with the walks that reach it.
type diagPair struct {
	Start     loc             `json:"start"`
	Gold      loc             `json:"gold"`
	OtherFile bool            `json:"other_file"`
	Shipped   bool            `json:"shipped"`
	Walks     map[string]bool `json:"walks"`
	Reason    string          `json:"reason"`
}

// diagResult is the per-question output of the reachdiag mode.
type diagResult struct {
	ID     string           `json:"id"`
	Stage1 bool             `json:"stage1"`
	Pairs  []diagPair       `json:"pairs"`
	Sizes  map[string][]int `json:"decls_reached_per_start"`
	Starts []loc            `json:"starts"`
}

// diagReasons is the order in which a pair's reason is decided.
var diagReasons = []string{"W1", "W2", "W3", "W4", "W5"}

// runDiag walks every start of one co-change question over the raw edges.
func runDiag(cfg config, q cochangeQuestion, shipped map[[2]loc]bool, walks []diagWalk) (diagResult, error) {
	res := diagResult{ID: q.ID, Stage1: q.Stage1, Pairs: []diagPair{}, Sizes: map[string][]int{}, Starts: []loc{}}
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
	declLoc, container := indexNodes(nodes)
	kept := edges[:0:0]
	for _, e := range edges {
		if e.Type == types.EdgeContains && container[e.Src] {
			continue
		}
		kept = append(kept, e)
	}
	adj := newAdjacency(kept)
	for _, s := range decls {
		sl := nodeLoc(s)
		res.Starts = append(res.Starts, sl)
		hit := map[string]map[loc]bool{}
		for _, w := range walks {
			locs := map[loc]bool{}
			for id := range reachable(adj, s.ID, w) {
				if l, ok := declLoc[id]; ok && l != sl {
					locs[l] = true
				}
			}
			hit[w.name] = locs
			res.Sizes[w.name] = append(res.Sizes[w.name], len(locs))
		}
		for _, g := range decls {
			gl := nodeLoc(g)
			if gl == sl {
				continue
			}
			p := diagPair{Start: sl, Gold: gl, OtherFile: gl.File != sl.File, Shipped: shipped[[2]loc{sl, gl}], Walks: map[string]bool{}}
			for _, w := range walks {
				p.Walks[w.name] = hit[w.name][gl]
			}
			p.Reason = pairReason(p)
			res.Pairs = append(res.Pairs, p)
		}
	}
	return res, nil
}

func pairReason(p diagPair) string {
	if p.Shipped {
		return "S"
	}
	for _, w := range diagReasons {
		if p.Walks[w] {
			return w
		}
	}
	return reachUnreached
}

// indexNodes maps declaration nodes of non-test Go files to their location
// and marks the package and file nodes whose contains edges W4 leaves out.
func indexNodes(nodes []types.Node) (map[string]loc, map[string]bool) {
	declLoc := map[string]loc{}
	container := map[string]bool{}
	for _, n := range nodes {
		if n.Type == types.NodePackage || n.Type == types.NodeFile {
			container[n.ID] = true
		}
		if declTypes[n.Type] && strings.HasSuffix(n.FilePath, ".go") && !strings.HasSuffix(n.FilePath, "_test.go") {
			declLoc[n.ID] = nodeLoc(n)
		}
	}
	return declLoc, container
}

// shippedPairs returns the start-gold pairs a co-change run's graph_uncapped
// arm reached. Gold ranks follow the changed order with the start left out.
func shippedPairs(c cochangeResult) map[[2]loc]bool {
	out := map[[2]loc]bool{}
	for i, s := range c.Starts {
		ranks := s.Measures[string(variantUncapped)].GoldRanks
		k := 0
		for j, g := range c.Changed {
			if j == i {
				continue
			}
			if k < len(ranks) && ranks[k] > 0 {
				out[[2]loc{s.Start, g}] = true
			}
			k++
		}
	}
	return out
}

// diagSummary pools pairs: how many each reason takes and each walk reaches,
// and the median number of declarations a walk reaches from one start.
type diagSummary struct {
	Set     string         `json:"set"`
	Pairs   int            `json:"pairs"`
	Reasons map[string]int `json:"reasons"`
	Reached map[string]int `json:"reached_by_walk"`
	Median  map[string]int `json:"median_decls_reached"`
}

func summariseDiag(name string, rs []diagResult, keep func(diagResult, diagPair) bool, starts func(diagResult) bool) diagSummary {
	s := diagSummary{Set: name, Reasons: map[string]int{}, Reached: map[string]int{}, Median: map[string]int{}}
	sizes := map[string][]int{}
	for _, r := range rs {
		for _, p := range r.Pairs {
			if !keep(r, p) {
				continue
			}
			s.Pairs++
			s.Reasons[p.Reason]++
			if p.Shipped {
				s.Reached["S"]++
			}
			for w, ok := range p.Walks {
				if ok {
					s.Reached[w]++
				}
			}
		}
		if starts(r) {
			for w, xs := range r.Sizes {
				sizes[w] = append(sizes[w], xs...)
			}
		}
	}
	for w, xs := range sizes {
		sort.Ints(xs)
		s.Median[w] = xs[len(xs)/2]
	}
	return s
}

// curveHops are the depths of the hop curve.
var curveHops = []int{1, 2, 3}

// curveWalks varies depth, edge group, direction and hub limit. "both" takes
// either direction at every hop; "apart" unions forward and caller walks that
// never mix. They are exploratory: the pairs were already seen (section 11).
func curveWalks() []diagWalk {
	calls := func(t types.EdgeType) bool { return t == types.EdgeCalls || t == types.EdgeInvokes }
	groups := []struct {
		name string
		in   func(types.EdgeType) bool
	}{
		{"calls", calls},
		{"base", func(t types.EdgeType) bool { return walkEdges[t] }},
		{"field", func(t types.EdgeType) bool { return fieldEdges[t] }},
		{"calls+field", func(t types.EdgeType) bool { return calls(t) || fieldEdges[t] }},
		{"base+field", func(t types.EdgeType) bool { return walkEdges[t] || fieldEdges[t] }},
	}
	var out []diagWalk
	for _, g := range groups {
		for _, h := range curveHops {
			both := []arcFilter{func(a arc) bool { return g.in(a.edge) }}
			out = append(out,
				diagWalk{name: fmt.Sprintf("%s both h%d", g.name, h), passes: both, hops: h},
				diagWalk{name: fmt.Sprintf("%s both h%d hub", g.name, h), passes: both, hops: h, hub: diagHubLimit})
		}
	}
	for _, h := range curveHops {
		out = append(out, diagWalk{name: fmt.Sprintf("base apart h%d", h), hops: h, passes: []arcFilter{
			func(a arc) bool { return a.forward && walkEdges[a.edge] },
			func(a arc) bool { return !a.forward && calls(a.edge) },
		}})
	}
	return out
}
