package execflow

import (
	"fmt"
	"sort"
	"strings"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// graph.go reads the graph for one Trace call: each node, edge list and
// source text (only quoted) once.

// outEdgeTypes are the edges a body's steps and a receive's handled code
// are read from.
var outEdgeTypes = []string{
	string(types.EdgeCalls), string(types.EdgeInvokes), string(types.EdgeDispatchesTo),
	string(types.EdgeSpawns), string(types.EdgeSendsTo), string(types.EdgeRecvsFrom),
	string(types.EdgeTriggers), string(types.EdgeHandles), string(types.EdgeSiteCalls),
	string(types.EdgeAcquiresLock), string(types.EdgeReleasesLock),
	string(types.EdgeReadsField), string(types.EdgeWritesField), string(types.EdgeReferences),
	string(types.EdgeAccessedUnderLock), string(types.EdgeContains), string(types.EdgeCloses), string(types.EdgeOperatesOn),
}

// reader is what one Trace call reads, cached.
type reader struct {
	r      store.Reader
	isTest func(string) bool
	nodes  map[string]types.Node
	out    map[string][]types.Edge
	in     map[string][]types.Edge
	lines  map[string][]string
	names  map[string]string
	// has caches recorded.
	has map[types.EdgeType]bool
}

func newReader(r store.Reader) (*reader, error) {
	isTest, err := store.TestFileRule(r)
	if err != nil {
		return nil, fmt.Errorf("test files: %w", err)
	}
	return &reader{r: r, isTest: isTest, nodes: map[string]types.Node{}, out: map[string][]types.Edge{},
		in: map[string][]types.Edge{}, lines: map[string][]string{}, names: map[string]string{}, has: map[types.EdgeType]bool{}}, nil
}

// node returns the node of id; found is false when the graph has none.
func (g *reader) node(id string) (types.Node, bool, error) {
	if n, ok := g.nodes[id]; ok {
		return n, true, nil
	}
	ns, err := g.r.NodesByIDs([]string{id})
	if err != nil {
		return types.Node{}, false, fmt.Errorf("node %s: %w", id, err)
	}
	for _, n := range ns {
		g.nodes[n.ID] = n
	}
	n, ok := g.nodes[id]
	return n, ok, nil
}

// remember keeps nodes read along with edges.
func (g *reader) remember(ns []types.Node) {
	for _, n := range ns {
		g.nodes[n.ID] = n
	}
}

// outOf returns the edges that start at id (outEdgeTypes, and a
// goroutine's code edges), in a total order.
func (g *reader) outOf(id string) ([]types.Edge, error) {
	if es, ok := g.out[id]; ok {
		return es, nil
	}
	ns, es, err := g.r.NeighborhoodByIDs([]string{id}, 1, false, outEdgeTypes...)
	if err != nil {
		return nil, fmt.Errorf("edges from %s: %w", id, err)
	}
	g.remember(ns)
	code, err := store.GoroutineCode(g.r, []string{id}, false)
	if err != nil {
		return nil, fmt.Errorf("goroutine code of %s: %w", id, err)
	}
	if err := g.fetchEnds(code, false); err != nil {
		return nil, err
	}
	out := ownEdges(append(es, code...), func(e types.Edge) bool { return e.Src == id })
	sortEdges(out)
	g.out[id] = out
	return out, nil
}

// inOf returns the edges of the given types that end at id, goroutine code
// edges included, in a total order.
func (g *reader) inOf(id string, edgeTypes ...types.EdgeType) ([]types.Edge, error) {
	all, ok := g.in[id]
	if !ok {
		names := make([]string, 0, len(inEdgeTypes))
		for _, t := range inEdgeTypes {
			names = append(names, string(t))
		}
		ns, es, err := g.r.NeighborhoodByIDs([]string{id}, 1, true, names...)
		if err != nil {
			return nil, fmt.Errorf("edges to %s: %w", id, err)
		}
		g.remember(ns)
		code, err := store.GoroutineCode(g.r, []string{id}, true)
		if err != nil {
			return nil, fmt.Errorf("goroutine code at %s: %w", id, err)
		}
		if err := g.fetchEnds(code, true); err != nil {
			return nil, err
		}
		all = ownEdges(append(es, code...), func(e types.Edge) bool { return e.Dst == id })
		sortEdges(all)
		g.in[id] = all
	}
	var out []types.Edge
	for _, e := range all {
		for _, t := range edgeTypes {
			if e.Type == t {
				out = append(out, e)
				break
			}
		}
	}
	return out, nil
}

// ownEdges returns the edges of es that keep holds, each stored edge once.
func ownEdges(es []types.Edge, keep func(types.Edge) bool) []types.Edge {
	var out []types.Edge
	seen := map[int64]bool{}
	for _, e := range es {
		if !keep(e) || e.ID != 0 && seen[e.ID] {
			continue
		}
		seen[e.ID] = true
		out = append(out, e)
	}
	return out
}

// inEdgeTypes are the edges read backward.
var inEdgeTypes = []types.EdgeType{types.EdgeCalls, types.EdgeInvokes, types.EdgeDispatchesTo,
	types.EdgeSpawns, types.EdgeSiteCalls, types.EdgeHandles, types.EdgeTriggers, types.EdgeRecvsFrom, types.EdgeFlowsTo, types.EdgeContains,
	types.EdgeAcquiresLock, types.EdgeReleasesLock, types.EdgeOperatesOn}

// fetchEnds reads the nodes at the far ends of es (their sources when
// reverse) that are not read yet.
func (g *reader) fetchEnds(es []types.Edge, reverse bool) error {
	var ids []string
	for _, e := range es {
		id := e.Dst
		if reverse {
			id = e.Src
		}
		if _, ok := g.nodes[id]; !ok {
			ids = append(ids, id)
		}
	}
	if len(ids) == 0 {
		return nil
	}
	ns, err := g.r.NodesByIDs(ids)
	if err != nil {
		return fmt.Errorf("nodes of %d edges: %w", len(ids), err)
	}
	g.remember(ns)
	return nil
}

// sortEdges orders edges by line, type, kind, ends and id: a total order.
func sortEdges(es []types.Edge) {
	sort.SliceStable(es, func(i, j int) bool {
		a, b := es[i], es[j]
		switch {
		case a.Line != b.Line:
			return a.Line < b.Line
		case a.Type != b.Type:
			return a.Type < b.Type
		case a.DispatchKind != b.DispatchKind:
			return a.DispatchKind < b.DispatchKind
		case a.Src != b.Src:
			return a.Src < b.Src
		case a.Dst != b.Dst:
			return a.Dst < b.Dst
		}
		return a.ID < b.ID
	})
}

// batchOut returns the edges of type t that start at ids, in a total
// order; the nodes at their ends are read with them.
func (g *reader) batchOut(ids []string, t types.EdgeType) ([]types.Edge, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	ns, es, err := g.r.NeighborhoodByIDs(ids, 1, false, string(t))
	if err != nil {
		return nil, fmt.Errorf("%s edges of %d nodes: %w", t, len(ids), err)
	}
	g.remember(ns)
	from := map[string]bool{}
	for _, id := range ids {
		from[id] = true
	}
	out := ownEdges(es, func(e types.Edge) bool { return from[e.Src] && e.Type == t })
	sortEdges(out)
	return out, nil
}

// closesRecorded reports whether the graph records closes of channels
// (schema 1.44): only then is "nothing sends or closes" a conclusion.
func (g *reader) closesRecorded() bool { return g.recorded(types.EdgeCloses) }

// recorded reports whether the graph holds any edge of type t: a fact a
// newer schema adds is used only when the graph has it.
func (g *reader) recorded(t types.EdgeType) bool {
	if has, ok := g.has[t]; ok {
		return has
	}
	es, err := g.r.QueryEdgesByType(string(t))
	has := err == nil && len(es) > 0
	g.has[t] = has
	return has
}

// closesAt reports whether n closes a channel on line (a closes edge).
func (g *reader) closesAt(n types.Node, line int) bool {
	es, err := g.outOf(n.ID)
	if err != nil {
		return false
	}
	for _, e := range es {
		if e.Type == types.EdgeCloses && e.Line == line {
			return true
		}
	}
	return false
}

// name is the printed name of n: the name that opens exactly n.
func (g *reader) name(n types.Node) string {
	if s, ok := g.names[n.ID]; ok {
		return s
	}
	s := store.PrintedName(g.r, n)
	g.names[n.ID] = s
	return s
}

// textAt returns the source line at line of body's text, trimmed, or "".
func (g *reader) textAt(body types.Node, line int) string {
	ls, ok := g.lines[body.ID]
	if !ok {
		if b, err := g.r.GetBlob(body.ID); err == nil {
			ls = strings.Split(string(b), "\n")
		}
		g.lines[body.ID] = ls
	}
	i := line - body.StartLine
	if i < 0 || i >= len(ls) {
		return ""
	}
	return strings.TrimSpace(ls[i])
}

// outside reports whether n is outside the build: an External stub, or a
// node without a file.
func outside(n types.Node) bool {
	return n.Type == types.NodeExternal || n.FilePath == types.ExternalFilePath || n.FilePath == ""
}

// isBody reports whether n has steps of its own: a function or method, or
// a goroutine.
func isBody(n types.Node) bool {
	return n.Type == types.NodeGoroutine || store.IsCallable(n)
}

// test reports whether n is test code (the graph's is_test record).
func (g *reader) test(n types.Node) bool { return !outside(n) && g.isTest(n.FilePath) }
