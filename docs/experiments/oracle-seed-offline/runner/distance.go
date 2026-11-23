package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// walkMode names one way experiment 1 follows edges from the seeds.
type walkMode string

const (
	// walkForward follows edges from source to destination only.
	walkForward walkMode = "forward"
	// walkUndirected follows every edge in both directions.
	walkUndirected walkMode = "undirected"
	// walkUndirectedSemantic follows edges in both directions but leaves out
	// the structural edges, through which any two nodes of one package are
	// a few hops apart.
	walkUndirectedSemantic walkMode = "undirected_semantic"
)

var walkModes = []walkMode{walkForward, walkUndirected, walkUndirectedSemantic}

// maxHops bounds every walk; a gold farther away is reported as not reached.
const maxHops = 6

// hopsNotReached is the Hops value of a gold no walk reached within maxHops.
const hopsNotReached = -1

// structuralEdges are file and package plumbing, not symbol semantics.
var structuralEdges = map[types.EdgeType]bool{
	types.EdgeContains: true, types.EdgeDefines: true,
	types.EdgeImports: true, types.EdgeExports: true,
}

// arc is one edge seen from one of its ends.
type arc struct {
	to      string
	edge    types.EdgeType
	forward bool // true when the walk goes from the edge's source to its destination
}

// adjacency holds the graph's edges per node, each list sorted so a walk
// visits neighbours in the same order on every run.
type adjacency struct {
	out    map[string][]arc
	in     map[string][]arc
	degree map[string]int
}

func newAdjacency(edges []types.Edge) *adjacency {
	type key struct {
		src, dst string
		t        types.EdgeType
	}
	seen := make(map[key]bool, len(edges))
	a := &adjacency{out: map[string][]arc{}, in: map[string][]arc{}, degree: map[string]int{}}
	for _, e := range edges {
		k := key{e.Src, e.Dst, e.Type}
		if seen[k] {
			continue
		}
		seen[k] = true
		a.out[e.Src] = append(a.out[e.Src], arc{to: e.Dst, edge: e.Type, forward: true})
		a.in[e.Dst] = append(a.in[e.Dst], arc{to: e.Src, edge: e.Type, forward: false})
		a.degree[e.Src]++
		a.degree[e.Dst]++
	}
	for _, m := range []map[string][]arc{a.out, a.in} {
		for id := range m {
			sortArcs(m[id])
		}
	}
	return a
}

func sortArcs(as []arc) {
	sort.Slice(as, func(i, j int) bool {
		if as[i].edge != as[j].edge {
			return as[i].edge < as[j].edge
		}
		return as[i].to < as[j].to
	})
}

func (a *adjacency) arcs(id string, m walkMode) []arc {
	if m == walkForward {
		return a.out[id]
	}
	all := make([]arc, 0, len(a.out[id])+len(a.in[id]))
	for _, list := range [][]arc{a.out[id], a.in[id]} {
		for _, x := range list {
			if m == walkUndirectedSemantic && structuralEdges[x.edge] {
				continue
			}
			all = append(all, x)
		}
	}
	return all
}

// hop is one step of a path: the edge taken and the node it arrives at.
type hop struct {
	node string
	edge types.EdgeType
	fwd  bool
}

// shortestPaths walks breadth first from all sources at once and returns,
// for each target reached within maxHops, the path from the nearest source.
// Sources are visited in sorted order and arcs in their sorted order, so the
// chosen path among equal-length ones is the same on every run.
func shortestPaths(a *adjacency, sources []string, targets map[string]bool, m walkMode) map[string][]hop {
	srcs := append([]string(nil), sources...)
	sort.Strings(srcs)
	prev := map[string]hop{}
	dist := map[string]int{}
	queue := make([]string, 0, len(srcs))
	for _, s := range srcs {
		if _, ok := dist[s]; !ok {
			dist[s] = 0
			queue = append(queue, s)
		}
	}
	for i := 0; i < len(queue); i++ {
		cur := queue[i]
		if dist[cur] == maxHops {
			continue
		}
		for _, x := range a.arcs(cur, m) {
			if _, ok := dist[x.to]; ok {
				continue
			}
			dist[x.to] = dist[cur] + 1
			prev[x.to] = hop{node: cur, edge: x.edge, fwd: x.forward}
			queue = append(queue, x.to)
		}
	}
	out := map[string][]hop{}
	for t := range targets {
		if _, ok := dist[t]; !ok {
			continue
		}
		var rev []hop
		for n := t; dist[n] > 0; n = prev[n].node {
			p := prev[n]
			rev = append(rev, hop{node: n, edge: p.edge, fwd: p.fwd})
		}
		path := make([]hop, len(rev))
		for i := range rev {
			path[i] = rev[len(rev)-1-i]
		}
		out[t] = path
	}
	return out
}

// pathStep is one hop of a reported path.
type pathStep struct {
	Edge    string `json:"edge"`
	Forward bool   `json:"forward"`
	QName   string `json:"qname"`
	Type    string `json:"type"`
	Degree  int    `json:"degree"`
}

// walkResult is the nearest seed-to-gold path of one walk mode.
type walkResult struct {
	Hops int        `json:"hops"`
	Path []pathStep `json:"path,omitempty"`
}

// goldDistance is experiment 1's record for one core location.
type goldDistance struct {
	Symbol string                `json:"symbol"`
	File   string                `json:"file"`
	Node   *loc                  `json:"node"`
	IsSeed bool                  `json:"gold_is_seed"`
	Walks  map[string]walkResult `json:"walks"`
}

// distanceResult is the per-question output of the distance mode.
type distanceResult struct {
	ID        string         `json:"id"`
	Base      string         `json:"base"`
	SeedNodes []string       `json:"seed_nodes"`
	Gold      []goldDistance `json:"gold"`
}

// goldTargets returns the declaration nodes at the gold span whose name is
// the oracle symbol's last part. A promoted-method copy shares the span and
// name, so every such node counts as the gold.
func goldTargets(ns []types.Node, g resolvedGold) []string {
	name := g.Symbol
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	var ids []string
	for _, n := range ns {
		if declTypes[n.Type] && n.Name == name && nodeLoc(n) == *g.Node {
			ids = append(ids, n.ID)
		}
	}
	sort.Strings(ids)
	return ids
}

func distanceQuestion(r store.Reader, fn *fileNodes, q question, seeds []types.Node, gold []resolvedGold) (distanceResult, error) {
	res := distanceResult{ID: q.ID, Base: q.Base, SeedNodes: []string{}}
	for _, n := range seeds {
		res.SeedNodes = append(res.SeedNodes, n.QualifiedName)
	}
	var adj *adjacency
	if len(seeds) > 0 {
		edges, err := r.AllEdges()
		if err != nil {
			return distanceResult{}, fmt.Errorf("all edges: %w", err)
		}
		adj = newAdjacency(edges)
	}
	srcIDs := make([]string, 0, len(seeds))
	for _, n := range seeds {
		srcIDs = append(srcIDs, n.ID)
	}
	for _, g := range gold {
		gd := goldDistance{Symbol: g.Symbol, File: g.File, Node: g.Node, IsSeed: g.IsSeed, Walks: map[string]walkResult{}}
		for _, m := range walkModes {
			gd.Walks[string(m)] = walkResult{Hops: hopsNotReached}
		}
		if g.Resolved && adj != nil {
			ns, err := fn.get(g.File)
			if err != nil {
				return distanceResult{}, err
			}
			targets := map[string]bool{}
			for _, id := range goldTargets(ns, g) {
				targets[id] = true
			}
			for _, m := range walkModes {
				wr, err := nearestWalk(r, adj, srcIDs, targets, m)
				if err != nil {
					return distanceResult{}, err
				}
				gd.Walks[string(m)] = wr
			}
		}
		res.Gold = append(res.Gold, gd)
	}
	return res, nil
}

// nearestWalk picks the shortest path to any of the gold's target nodes;
// equal lengths fall back to the target id.
func nearestWalk(r store.Reader, adj *adjacency, srcIDs []string, targets map[string]bool, m walkMode) (walkResult, error) {
	paths := shortestPaths(adj, srcIDs, targets, m)
	if len(paths) == 0 {
		return walkResult{Hops: hopsNotReached}, nil
	}
	ids := make([]string, 0, len(paths))
	for id := range paths {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if len(paths[ids[i]]) != len(paths[ids[j]]) {
			return len(paths[ids[i]]) < len(paths[ids[j]])
		}
		return ids[i] < ids[j]
	})
	path := paths[ids[0]]
	wr := walkResult{Hops: len(path), Path: make([]pathStep, 0, len(path))}
	nodeIDs := make([]string, 0, len(path))
	for _, h := range path {
		nodeIDs = append(nodeIDs, h.node)
	}
	nodes, err := r.NodesByIDs(nodeIDs)
	if err != nil {
		return walkResult{}, fmt.Errorf("nodes by ids: %w", err)
	}
	byID := make(map[string]types.Node, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
	}
	for _, h := range path {
		n := byID[h.node]
		wr.Path = append(wr.Path, pathStep{
			Edge: string(h.edge), Forward: h.fwd,
			QName: n.QualifiedName, Type: string(n.Type), Degree: adj.degree[h.node],
		})
	}
	return wr, nil
}
