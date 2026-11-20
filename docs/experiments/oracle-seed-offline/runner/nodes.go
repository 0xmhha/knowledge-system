package main

import (
	"fmt"
	"sort"
	"strings"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// declTypes are the node kinds a grep hit or an oracle location can resolve
// to. Fields, parameters, locals and imports are left out: a hit inside them
// belongs to the declaration that holds them.
var declTypes = map[types.NodeType]bool{
	types.NodeFunction: true, types.NodeMethod: true, types.NodeConstructor: true,
	types.NodeModifier: true, types.NodeStruct: true, types.NodeInterface: true,
	types.NodeClass: true, types.NodeTypeAlias: true, types.NodeEnum: true,
	types.NodeContract: true, types.NodeEvent: true, types.NodeConstant: true,
	types.NodeVariable: true, types.NodeMapping: true,
}

// loc identifies a candidate the way both arms see it: the graph node's file
// and line span. Stage 3 returns citations, not node ids, so the span is the
// shared key.
type loc struct {
	File      string `json:"file"`
	StartLine int    `json:"start_line"`
	EndLine   int    `json:"end_line"`
}

func nodeLoc(n types.Node) loc {
	return loc{File: n.FilePath, StartLine: n.StartLine, EndLine: n.EndLine}
}

// fileNodes caches NodesByFilePath per file for one graph.
type fileNodes struct {
	r     store.Reader
	cache map[string][]types.Node
}

func newFileNodes(r store.Reader) *fileNodes {
	return &fileNodes{r: r, cache: map[string][]types.Node{}}
}

func (f *fileNodes) get(file string) ([]types.Node, error) {
	if ns, ok := f.cache[file]; ok {
		return ns, nil
	}
	ns, err := f.r.NodesByFilePath(file)
	if err != nil {
		return nil, fmt.Errorf("nodes of %s: %w", file, err)
	}
	f.cache[file] = ns
	return ns, nil
}

// innermostDecl returns the smallest declaration node of ns whose span holds
// line. Equal spans fall back to the node id so the choice is stable.
func innermostDecl(ns []types.Node, line int) (types.Node, bool) {
	var best types.Node
	found := false
	for _, n := range ns {
		if !declTypes[n.Type] || line < n.StartLine || line > n.EndLine {
			continue
		}
		if !found || span(n) < span(best) || (span(n) == span(best) && n.ID < best.ID) {
			best, found = n, true
		}
	}
	return best, found
}

func span(n types.Node) int { return n.EndLine - n.StartLine }

// resolvedSeed is a seed with the graph nodes it names.
type resolvedSeed struct {
	seedRef
	Nodes []loc `json:"nodes"`
}

// resolveSeeds finds, for every seed, the declaration nodes in the seed's file
// whose name is the seed symbol. A symbol can name several nodes (methods of
// different receivers); all of them are kept and listed.
func resolveSeeds(fn *fileNodes, seeds []seedRef) ([]resolvedSeed, []types.Node, error) {
	var out []resolvedSeed
	var nodes []types.Node
	for _, s := range seeds {
		ns, err := fn.get(s.File)
		if err != nil {
			return nil, nil, err
		}
		rs := resolvedSeed{seedRef: s}
		for _, n := range sortedByLine(ns) {
			if declTypes[n.Type] && n.Name == s.Symbol {
				rs.Nodes = append(rs.Nodes, nodeLoc(n))
				nodes = append(nodes, n)
			}
		}
		out = append(out, rs)
	}
	return out, nodes, nil
}

// resolvedGold is a core oracle location with the graph node it maps to.
type resolvedGold struct {
	oracleRef
	Node     *loc `json:"node"`
	IsSeed   bool `json:"gold_is_seed"`
	Resolved bool `json:"resolved"`
}

// resolveGold maps each core location to the declaration node in its file
// whose name is the last dotted part of the oracle symbol and whose span
// overlaps the oracle span; among several, the nearest start line wins. This
// is the replay scripts' rule (same file, qname without package equals the
// symbol) applied to graph nodes.
func resolveGold(fn *fileNodes, core []oracleRef, seeds []types.Node) ([]resolvedGold, error) {
	seedKeys := map[loc]bool{}
	for _, n := range seeds {
		seedKeys[nodeLoc(n)] = true
	}
	out := make([]resolvedGold, 0, len(core))
	for _, c := range core {
		ns, err := fn.get(c.File)
		if err != nil {
			return nil, err
		}
		g := resolvedGold{oracleRef: c}
		if n, ok := matchOracle(ns, c); ok {
			l := nodeLoc(n)
			g.Node, g.Resolved, g.IsSeed = &l, true, seedKeys[l]
		}
		out = append(out, g)
	}
	return out, nil
}

func matchOracle(ns []types.Node, c oracleRef) (types.Node, bool) {
	name := c.Symbol
	if i := strings.LastIndex(name, "."); i >= 0 {
		name = name[i+1:]
	}
	var best types.Node
	found := false
	for _, n := range sortedByLine(ns) {
		if !declTypes[n.Type] || n.Name != name {
			continue
		}
		if n.EndLine < c.StartLine || n.StartLine > c.EndLine {
			continue
		}
		if !found || absInt(n.StartLine-c.StartLine) < absInt(best.StartLine-c.StartLine) {
			best, found = n, true
		}
	}
	return best, found
}

func sortedByLine(ns []types.Node) []types.Node {
	out := append([]types.Node(nil), ns...)
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].StartLine != out[j].StartLine {
			return out[i].StartLine < out[j].StartLine
		}
		return out[i].ID < out[j].ID
	})
	return out
}

func absInt(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
