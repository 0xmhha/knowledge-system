package mcphandlers

import (
	"fmt"
	"sort"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// The dispatch_kind values of a dispatches_to edge that joins an interface
// method to an implementation: the value flow saw a value of the type reach
// the interface, or only the type hierarchy allows it.
const (
	dispatchInterfaceMethod = "interface_method"
	dispatchTypeHierarchy   = "type_hierarchy"
)

// walkCalls returns the nodes within depth calls of the seeds, and the call
// edges between them, following the edges backwards when reverse is set.
//
// A call is one hop. Stepping between an interface method and an
// implementation is none: a call through an interface reaches the
// implementation at the same depth as a static call reaches its callee, at
// every step of the walk. It is a 0-1 breadth-first search: each round first
// closes over the zero-cost edges, then takes one call.
//
// Where the value flow names implementations, the ones only the type
// hierarchy allows cost a hop, so they come after the ones a value was seen to
// reach. Nodes are ordered by hop, then by qualified name.
func walkCalls(reader store.Reader, seedIDs []string, depth int, reverse bool) ([]types.Node, []types.Edge, error) {
	seeds, err := reader.NodesByIDs(seedIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("call walk seeds: %w", err)
	}
	w := callWalk{reader: reader, reverse: reverse, nodes: map[string]types.Node{}, edgeSeen: map[edgeKey]bool{}}
	var frontier []string
	for _, n := range seeds {
		if _, dup := w.nodes[n.ID]; !dup {
			w.nodes[n.ID] = n
			frontier = append(frontier, n.ID)
		}
	}
	var out []types.Node
	for hop := 0; hop <= depth && len(frontier) > 0; hop++ {
		level, next, err := w.level(frontier, hop < depth)
		if err != nil {
			return nil, nil, err
		}
		sort.Slice(level, func(i, j int) bool {
			if level[i].QualifiedName != level[j].QualifiedName {
				return level[i].QualifiedName < level[j].QualifiedName
			}
			return level[i].ID < level[j].ID
		})
		out = append(out, level...)
		frontier = next
	}
	return out, w.edges, nil
}

type edgeKey struct {
	src, dst string
	typ      types.EdgeType
	kind     string
	line     int
}

// callWalk is the state of one walkCalls: the nodes reached and the edges listed.
type callWalk struct {
	reader   store.Reader
	reverse  bool
	nodes    map[string]types.Node
	edges    []types.Edge
	edgeSeen map[edgeKey]bool
}

// level returns every node at the frontier's hop, the frontier closed over
// zero-cost edges, and, when calls is set, the ids one call further.
func (w *callWalk) level(frontier []string, calls bool) (level []types.Node, next []string, err error) {
	edgeTypes := callEdgeTypes
	if !calls {
		edgeTypes = []string{string(types.EdgeDispatchesTo)}
	}
	far := map[string]types.Node{} // one call further, unless zero-cost edges reach it in this level
	var order []string
	var pending []types.Edge
	for len(frontier) > 0 {
		for _, id := range frontier {
			level = append(level, w.nodes[id])
		}
		ns, es, err := w.reader.NeighborhoodByIDs(frontier, 1, w.reverse, edgeTypes...)
		if err != nil {
			return nil, nil, fmt.Errorf("call walk: %w", err)
		}
		byID := make(map[string]types.Node, len(ns))
		for _, n := range ns {
			byID[n.ID] = n
		}
		flow := w.flowNodes(es)
		var closed []string
		for _, e := range es {
			id := w.farEnd(e)
			n, ok := byID[id]
			if !ok {
				continue // filtered out by the reader
			}
			if !w.free(e, flow) {
				if !calls {
					continue
				}
				pending = append(pending, e)
				if _, dup := far[id]; !dup {
					far[id] = n
					order = append(order, id)
				}
				continue
			}
			w.list(e)
			if _, dup := w.nodes[id]; !dup {
				w.nodes[id] = n
				closed = append(closed, id)
			}
		}
		frontier = closed
	}
	for _, e := range pending {
		w.list(e)
	}
	for _, id := range order {
		if _, dup := w.nodes[id]; !dup {
			w.nodes[id] = far[id]
			next = append(next, id)
		}
	}
	return level, next, nil
}

// list adds an edge to the answer once. Backwards, a caller is one edge however
// many lines call it; forwards, each line that makes a call keeps its own edge.
func (w *callWalk) list(e types.Edge) {
	k := edgeKey{src: e.Src, dst: e.Dst, typ: e.Type}
	if !w.reverse {
		k.kind, k.line = e.DispatchKind, e.Line
	}
	if !w.edgeSeen[k] {
		w.edgeSeen[k] = true
		w.edges = append(w.edges, e)
	}
}

func (w *callWalk) farEnd(e types.Edge) string {
	if w.reverse {
		return e.Src
	}
	return e.Dst
}

func (w *callWalk) nearEnd(e types.Edge) string {
	if w.reverse {
		return e.Dst
	}
	return e.Src
}

// flowNodes returns the near ends that the value flow joins to the other side
// of an interface.
func (w *callWalk) flowNodes(es []types.Edge) map[string]bool {
	out := map[string]bool{}
	for _, e := range es {
		if e.Type == types.EdgeDispatchesTo && e.DispatchKind == dispatchInterfaceMethod {
			out[w.nearEnd(e)] = true
		}
	}
	return out
}

// free reports whether following e costs no hop.
func (w *callWalk) free(e types.Edge, flow map[string]bool) bool {
	if e.Type != types.EdgeDispatchesTo {
		return false
	}
	return e.DispatchKind == dispatchInterfaceMethod || e.DispatchKind == dispatchTypeHierarchy && !flow[w.nearEnd(e)]
}
