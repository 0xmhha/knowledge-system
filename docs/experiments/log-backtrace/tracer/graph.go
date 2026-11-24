package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

const testSuffix = "_test.go"

var (
	funcTypes  = map[types.NodeType]bool{types.NodeFunction: true, types.NodeMethod: true}
	valueTypes = map[types.NodeType]bool{types.NodeVariable: true, types.NodeConstant: true}
	callEdges  = map[types.EdgeType]bool{types.EdgeCalls: true, types.EdgeInvokes: true}
)

// lineEdge is an edge from a function with the line it is on.
type lineEdge struct {
	kind types.EdgeType
	dst  string
	line int
}

// index holds the part of a graph the rules use. Test files and external
// nodes are left out, so no rule can list or pass through them.
type index struct {
	nodes    map[string]types.Node
	byFile   map[string][]types.Node
	from     map[string][]lineEdge       // function -> its edges with lines
	refs     map[string]map[string][]int // value -> function that references it -> lines
	callers  map[string][]string         // callee -> callers
	impls    map[string][]string         // interface method -> implementations
	ifaces   map[string][]string         // implementation -> interface methods
	writers  map[string][]string         // field -> declarations that write it
	siblings map[string]map[string]bool  // owner type name -> declarations that create it or write a field of it
	writes   map[string]map[string]bool  // field qualified name -> declarations that write it
}

func buildIndex(r store.Reader) (*index, error) {
	nodes, err := r.AllNodes()
	if err != nil {
		return nil, fmt.Errorf("all nodes: %w", err)
	}
	edges, err := r.AllEdges()
	if err != nil {
		return nil, fmt.Errorf("all edges: %w", err)
	}
	return newIndex(nodes, edges), nil
}

func newIndex(nodes []types.Node, edges []types.Edge) *index {
	ix := &index{
		nodes: map[string]types.Node{}, byFile: map[string][]types.Node{}, from: map[string][]lineEdge{},
		refs: map[string]map[string][]int{}, callers: map[string][]string{}, impls: map[string][]string{},
		ifaces: map[string][]string{}, writers: map[string][]string{},
		siblings: map[string]map[string]bool{}, writes: map[string]map[string]bool{},
	}
	for _, n := range nodes {
		if n.Type == types.NodeExternal || !strings.HasSuffix(n.FilePath, ".go") || strings.HasSuffix(n.FilePath, testSuffix) {
			continue
		}
		ix.nodes[n.ID] = n
		ix.byFile[n.FilePath] = append(ix.byFile[n.FilePath], n)
	}
	for _, e := range edges {
		src, okS := ix.nodes[e.Src]
		dst, okD := ix.nodes[e.Dst]
		if okS && okD {
			ix.add(e, src, dst)
		}
	}
	for _, m := range []map[string][]string{ix.callers, ix.impls, ix.ifaces, ix.writers} {
		for k := range m {
			slices.Sort(m[k])
			m[k] = slices.Compact(m[k])
		}
	}
	return ix
}

func (ix *index) add(e types.Edge, src, dst types.Node) {
	switch e.Type {
	case types.EdgeCalls, types.EdgeInvokes:
		ix.callers[dst.ID] = append(ix.callers[dst.ID], src.ID)
	case types.EdgeDispatchesTo:
		// A function value has no interface method: its source is the caller.
		if e.Line == 0 {
			ix.impls[src.ID] = append(ix.impls[src.ID], dst.ID)
			ix.ifaces[dst.ID] = append(ix.ifaces[dst.ID], src.ID)
		}
		return
	case types.EdgeReferences:
		if valueTypes[dst.Type] && funcTypes[src.Type] {
			if ix.refs[dst.ID] == nil {
				ix.refs[dst.ID] = map[string][]int{}
			}
			ix.refs[dst.ID][src.ID] = append(ix.refs[dst.ID][src.ID], e.Line)
		}
	case types.EdgeWritesField:
		ix.writers[dst.ID] = append(ix.writers[dst.ID], src.ID)
		mark(ix.writes, dst.QualifiedName, src.ID)
		mark(ix.siblings, owner(dst.QualifiedName), src.ID)
	case types.EdgeInstantiates:
		mark(ix.siblings, dst.QualifiedName, src.ID)
		return
	case types.EdgeReadsField:
	default:
		return
	}
	if funcTypes[src.Type] && e.Line > 0 {
		ix.from[src.ID] = append(ix.from[src.ID], lineEdge{e.Type, dst.ID, e.Line})
	}
}

func mark(m map[string]map[string]bool, key, id string) {
	if m[key] == nil {
		m[key] = map[string]bool{}
	}
	m[key][id] = true
}

// owner returns the qualified name without its last part.
func owner(qname string) string {
	if i := strings.LastIndexByte(qname, '.'); i >= 0 {
		return qname[:i]
	}
	return qname
}

// enclosing returns the smallest node of the kinds around a line; among equal
// spans one with a canonical id, then the lowest id.
func (ix *index) enclosing(p position, kinds map[types.NodeType]bool) (types.Node, bool) {
	var best types.Node
	found := false
	for _, n := range ix.byFile[p.file] {
		if !kinds[n.Type] || p.line < n.StartLine || p.line > n.EndLine {
			continue
		}
		if !found || tighter(n, best) {
			best, found = n, true
		}
	}
	return best, found
}

func tighter(a, b types.Node) bool {
	if sa, sb := a.EndLine-a.StartLine, b.EndLine-b.StartLine; sa != sb {
		return sa < sb
	}
	if ca, cb := a.CanonicalID != "", b.CanonicalID != ""; ca != cb {
		return ca
	}
	return a.ID < b.ID
}

// packageLevel reports whether a value is declared outside every function.
func (ix *index) packageLevel(n types.Node) bool {
	_, inside := ix.enclosing(position{n.FilePath, n.StartLine}, funcTypes)
	return !inside
}

// omitters returns the declarations that create the field's type or write a
// sibling field and never write the field, sorted.
func (ix *index) omitters(field types.Node) []string {
	var out []string
	for id := range ix.siblings[owner(field.QualifiedName)] {
		if !ix.writes[field.QualifiedName][id] {
			out = append(out, id)
		}
	}
	slices.Sort(out)
	return out
}

// callersOf returns the callers of a function, through the interface methods
// it implements too, sorted.
func (ix *index) callersOf(id string) []string {
	out := slices.Clone(ix.callers[id])
	for _, m := range ix.ifaces[id] {
		out = append(out, ix.callers[m]...)
	}
	slices.Sort(out)
	return slices.Compact(out)
}
