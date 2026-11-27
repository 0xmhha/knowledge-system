package concurrency

import (
	"fmt"
	"sort"
	"strings"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// ConcurrencyEdgeTypes is the contract's five concurrency edge types.
// releases_lock is intentionally excluded — see doc.go.
var ConcurrencyEdgeTypes = []string{
	string(types.EdgeSpawns),
	string(types.EdgeSendsTo),
	string(types.EdgeRecvsFrom),
	string(types.EdgeAcquiresLock),
	string(types.EdgeAccessedUnderLock),
}

// DepthCap bounds BFS depth, mirroring pkg/impact.
const DepthCap = 5

// defaultDepth is used when Options.Depth is unset (zero).
const defaultDepth = 2

// Options bundles tunable knobs. The zero value resolves to documented
// defaults (Depth=2, MaxTotal=0=unbounded).
type Options struct {
	Depth    int // clamped to [1, DepthCap]; 0 -> defaultDepth
	MaxTotal int // 0 = unbounded; caps the returned Modules slice
}

// Module is one symbol reachable from the seed via a concurrency edge.
type Module struct {
	ID        string         `json:"id"`
	Type      types.NodeType `json:"type"`
	Name      string         `json:"name"`
	Qname     string         `json:"qname"`
	FilePath  string         `json:"file_path"`
	StartLine int            `json:"start_line"`
	Citation  string         `json:"citation,omitempty"` // "file:line" when both present
	Direction string         `json:"direction"`          // affects | affected_by | both
	// qualified is the node's qualified name, the first key of the order of
	// Modules (Qname is the printed name, which for two declarations of one
	// name carries the line as text: ":1016" sorted before ":978").
	qualified string
}

// mutexSuffix ends the qualified name of the Mutex node the Go extractor
// makes for a struct field that is a mutex ("pkg.T.mu#mutex" next to the
// Field node "pkg.T.mu"). The Mutex node of a package-level or local
// variable carries the variable's own name ("pkg.mu", "pkg.F.mu"). Node
// names stay as they are (ids and stored graphs depend on them); a seed
// names any mutex in either form, its declaring name or that name with the
// suffix (see Analyze).
const mutexSuffix = store.MutexSuffix

// Result is the payload cks's concurrency_impact tool consumes.
type Result struct {
	Seed     string `json:"seed"`
	NotFound bool   `json:"not_found"`
	// Ambiguous and Candidates report a bare seed name that matched several
	// qualified names; the caller retries with one of them.
	Ambiguous  bool     `json:"ambiguous,omitempty"`
	Candidates []string `json:"candidates,omitempty"`
	// OtherCase lists names that differ from an unresolved seed only in case.
	OtherCase []string `json:"other_case,omitempty"`
	// AlsoNamed lists the other declarations whose qualified name is the
	// seed, when the seed was the canonical id of one node.
	AlsoNamed []string `json:"also_named,omitempty"`
	// PromotedFrom and SeedNote are set when the seed named the copy of a
	// promoted method: the method the answer is for, and a sentence saying
	// so (the keys every seed tool uses, store.SeedKeyPromotedFrom and
	// store.SeedKeyNote). Seed is then that method.
	PromotedFrom string `json:"promoted_from,omitempty"`
	SeedNote     string `json:"seed_note,omitempty"`
	// Mutex is the qualified name of the Mutex node, when the seed names a
	// mutex in either form (see mutexSuffix). The answer is then the same
	// for both forms: Modules and Edges are the walk from the Mutex node,
	// and the keys below say the rest.
	Mutex string `json:"mutex,omitempty"`
	// DeclaredBy is the qualified name of the Field, Variable or
	// LocalVariable that declares the mutex; it is not among Modules.
	DeclaredBy string `json:"declared_by,omitempty"`
	// LockedBy are the functions that lock this mutex, one per lock site
	// (acquires_lock edges to the Mutex node), sorted.
	LockedBy []Site `json:"locked_by,omitempty"`
	// GuardedBy are the other mutexes held where the declaration itself is
	// accessed (its accessed_under_lock edges): the locks that guard the
	// field or variable that holds this mutex, not the ones this mutex is
	// taken under. Seed one of them for its lockers.
	GuardedBy []Site   `json:"guarded_by,omitempty"`
	Depth     int      `json:"depth"`
	Modules   []Module `json:"modules"`
	Edges     [][]any  `json:"edges"` // [src, dst, type, line] tuples, sorted
	// Goroutines are the sends and receives of goroutines on the channels
	// the walk reached (from the SSA channel linker): their own category,
	// kept out of Modules and Edges, which are the function-level answer.
	Goroutines []GoroutineOp  `json:"goroutines,omitempty"`
	Totals     map[string]any `json:"totals"`
}

// Site is a node at a line: a function with the line of its lock call, or a
// Mutex node with the line of its declaration.
type Site struct {
	ID    string `json:"id"`
	Qname string `json:"qname"`
	File  string `json:"file_path"`
	Line  int    `json:"line"`
}

// Analyze returns the concurrency blast radius of symbol: every module that
// affects or is affected by it via the five ConcurrencyEdgeTypes, BFS to
// depth in both directions, backed by store.Reader.NeighborhoodByIDs.
//
// symbol is resolved by store.ResolveName, as in every tool that takes a
// symbol name: a canonical id, a qualified name or a bare name. A seed that
// names a mutex, by its declaring name or with "#mutex", is walked from its
// Mutex node, where the lock edges end (the field alone reached none of its
// lockers: 0 of 183 on go-stablenet), and is answered with the mutex keys of
// Result (mutexAnswer): the two forms of 150 of 150 field mutexes gave two
// different answers while the walk also started at the field.
//
// Deterministic: modules are sorted by qualified name, then file, line and
// id, edges by (type, src, dst, line), so cks's prompt cache stays stable.
func Analyze(r store.Reader, symbol string, opt Options) (Result, error) {
	d := opt.Depth
	if d == 0 {
		d = defaultDepth
	}
	if d < 1 {
		d = 1
	}
	if d > DepthCap {
		d = DepthCap
	}

	res := Result{Seed: symbol, Depth: d, Modules: []Module{}, Edges: [][]any{}}

	name, err := resolveSeed(r, symbol)
	if err != nil {
		return Result{}, err
	}
	if !name.Found() {
		res.NotFound = !name.Ambiguous()
		res.Ambiguous, res.Candidates, res.OtherCase = name.Ambiguous(), name.Candidates, name.OtherCase
		res.Totals = map[string]any{"modules": 0, "edges": 0}
		return res, nil
	}
	if note := name.SeedNote(); note != "" {
		res.Seed, res.PromotedFrom, res.SeedNote = name.QName, name.QName, note
	}
	res.AlsoNamed = name.AlsoNamed
	roots, hidden, err := mutexAnswer(r, name.Nodes, &res)
	if err != nil {
		return Result{}, err
	}
	rootIDs := make(map[string]bool, len(roots)+len(hidden))
	for id := range hidden {
		rootIDs[id] = true
	}
	ids := make([]string, 0, len(roots))
	for _, n := range roots {
		if !rootIDs[n.ID] {
			rootIDs[n.ID] = true
			ids = append(ids, n.ID)
		}
	}

	fwdN, fwdE, fwdG, err := walk(r, ids, d, false)
	if err != nil {
		return Result{}, err
	}
	revN, revE, revG, err := walk(r, ids, d, true)
	if err != nil {
		return Result{}, err
	}

	dir := map[string]string{}
	nodeByID := map[string]types.Node{}
	for _, n := range fwdN {
		if rootIDs[n.ID] {
			continue
		}
		nodeByID[n.ID] = n
		dir[n.ID] = "affected_by"
	}
	for _, n := range revN {
		if rootIDs[n.ID] {
			continue
		}
		nodeByID[n.ID] = n
		if dir[n.ID] == "affected_by" {
			dir[n.ID] = "both"
		} else {
			dir[n.ID] = "affects"
		}
	}

	modules := make([]Module, 0, len(nodeByID))
	for id, n := range nodeByID {
		m := Module{
			ID:        id,
			Type:      n.Type,
			Name:      n.Name,
			Qname:     store.PrintedName(r, n),
			qualified: n.QualifiedName,
			FilePath:  n.FilePath,
			StartLine: n.StartLine,
			Direction: dir[id],
		}
		if n.FilePath != "" && n.StartLine > 0 {
			m.Citation = fmt.Sprintf("%s:%d", n.FilePath, n.StartLine)
		}
		modules = append(modules, m)
	}
	// Ordered by qualified name, then file, line and id.
	sort.Slice(modules, func(i, j int) bool {
		a, b := modules[i], modules[j]
		switch {
		case a.qualified != b.qualified:
			return a.qualified < b.qualified
		case a.FilePath != b.FilePath:
			return a.FilePath < b.FilePath
		case a.StartLine != b.StartLine:
			return a.StartLine < b.StartLine
		}
		return a.ID < b.ID
	})
	if opt.MaxTotal > 0 && len(modules) > opt.MaxTotal {
		modules = modules[:opt.MaxTotal]
	}

	type ekey struct {
		t, s, dst string
		line      int
	}
	eseen := map[ekey]bool{}
	edges := make([][]any, 0, len(fwdE)+len(revE))
	for _, es := range [][]types.Edge{fwdE, revE} {
		for _, e := range es {
			k := ekey{string(e.Type), e.Src, e.Dst, e.Line}
			if eseen[k] {
				continue
			}
			eseen[k] = true
			edges = append(edges, []any{e.Src, e.Dst, string(e.Type), e.Line})
		}
	}
	sort.Slice(edges, func(i, j int) bool {
		ti, tj := edges[i][2].(string), edges[j][2].(string)
		if ti != tj {
			return ti < tj
		}
		si, sj := edges[i][0].(string), edges[j][0].(string)
		if si != sj {
			return si < sj
		}
		di, dj := edges[i][1].(string), edges[j][1].(string)
		if di != dj {
			return di < dj
		}
		return edges[i][3].(int) < edges[j][3].(int)
	})

	res.Goroutines = goroutineRows(r, append(fwdG, revG...))
	res.Modules = modules
	res.Edges = edges
	res.Totals = map[string]any{"modules": len(modules), "edges": len(edges)}
	return res, nil
}

// mutexAnswer fills the mutex keys of res when seeds, the resolved nodes,
// name a mutex in either form, and returns the nodes to walk from and the
// ids to keep out of Modules. For a mutex the walk starts at its Mutex node
// alone, whichever form named it, so both forms give one answer; its
// declaration is DeclaredBy, and the mutexes that guard the declaration are
// GuardedBy. Other seeds are walked from as they are.
func mutexAnswer(r store.Reader, seeds []types.Node, res *Result) ([]types.Node, map[string]bool, error) {
	mutexes, err := mutexNodes(r, seeds)
	if err != nil {
		return nil, nil, err
	}
	for _, n := range seeds {
		if isLock(n) {
			mutexes = append(mutexes, n)
		}
	}
	if len(mutexes) == 0 {
		return seeds, map[string]bool{}, nil
	}
	decls, err := declarationsOf(r, mutexes)
	if err != nil {
		return nil, nil, err
	}
	hidden := map[string]bool{}
	for _, n := range decls {
		hidden[n.ID] = true
	}
	roots := append([]types.Node{}, mutexes...)
	for _, n := range seeds {
		if !isLock(n) && !hidden[n.ID] {
			roots = append(roots, n)
		}
	}
	res.Mutex = store.PrintedName(r, mutexes[0])
	if len(decls) > 0 {
		res.DeclaredBy = store.PrintedName(r, decls[0])
	}
	mutexIDs := make([]string, len(mutexes))
	own := map[string]bool{}
	for i, m := range mutexes {
		mutexIDs[i], own[m.ID] = m.ID, true
	}
	lockers, lockEdges, err := r.NeighborhoodByIDs(mutexIDs, 1, true, string(types.EdgeAcquiresLock))
	if err != nil {
		return nil, nil, fmt.Errorf("lockers of %s: %w", res.Mutex, err)
	}
	res.LockedBy = sites(r, lockers, lockEdges, func(e types.Edge) (string, bool) { return e.Src, own[e.Dst] })
	if len(decls) > 0 {
		ids := make([]string, len(decls))
		for i, n := range decls {
			ids[i] = n.ID
		}
		guards, guardEdges, err := r.NeighborhoodByIDs(ids, 1, false, string(types.EdgeAccessedUnderLock))
		if err != nil {
			return nil, nil, fmt.Errorf("mutexes guarding %s: %w", res.DeclaredBy, err)
		}
		res.GuardedBy = sites(r, guards, guardEdges, func(e types.Edge) (string, bool) {
			return e.Dst, hidden[e.Src] && !own[e.Dst]
		})
	}
	return roots, hidden, nil
}

// sites returns one Site per edge of edges that pick keeps, for the node
// pick names, among nodes: at the edge's line, or at the node's line when the
// edge has none. Sorted by qualified name, then line.
func sites(r store.Reader, nodes []types.Node, edges []types.Edge, pick func(types.Edge) (string, bool)) []Site {
	byID := make(map[string]types.Node, len(nodes))
	for _, n := range nodes {
		byID[n.ID] = n
	}
	seen := map[Site]bool{}
	var out []Site
	for _, e := range edges {
		id, ok := pick(e)
		n, known := byID[id]
		if !ok || !known {
			continue
		}
		line := e.Line
		if line == 0 {
			line = n.StartLine
		}
		s := Site{ID: id, Qname: store.PrintedName(r, n), File: n.FilePath, Line: line}
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Qname != out[j].Qname {
			return out[i].Qname < out[j].Qname
		}
		if out[i].Line != out[j].Line {
			return out[i].Line < out[j].Line
		}
		return out[i].ID < out[j].ID
	})
	return out
}

// declarationKinds are the kinds of node that declare a mutex.
var declarationKinds = map[types.NodeType]bool{
	types.NodeField: true, types.NodeVariable: true, types.NodeLocalVariable: true,
}

// declarationsOf returns the Field, Variable or LocalVariable nodes that
// declare mutexes: of the name without "#mutex", at the Mutex node's file
// and line, where the extractor puts both (a Mutex node has the span of its
// declaration).
func declarationsOf(r store.Reader, mutexes []types.Node) ([]types.Node, error) {
	var out []types.Node
	seen := map[string]bool{}
	for _, m := range mutexes {
		if m.Type == types.NodeChannel {
			// A channel used as a lock is declared by the fields and
			// variables it is stored in (flows_to, kind chan).
			held, _, err := r.NeighborhoodByIDs([]string{m.ID}, 1, false, string(types.EdgeFlowsTo))
			if err != nil {
				return nil, fmt.Errorf("names of %s: %w", m.QualifiedName, err)
			}
			for _, n := range held {
				if n.ID != m.ID && declarationKinds[n.Type] && !seen[n.ID] {
					seen[n.ID] = true
					out = append(out, n)
				}
			}
			continue
		}
		q := strings.TrimSuffix(m.QualifiedName, mutexSuffix)
		nodes, err := r.FindSymbol(q, true, store.FindSymbolOptions{})
		if err != nil {
			return nil, fmt.Errorf("declaration of %s: %w", m.QualifiedName, err)
		}
		for _, n := range nodes {
			if declarationKinds[n.Type] && n.FilePath == m.FilePath && n.StartLine == m.StartLine && !seen[n.ID] {
				seen[n.ID] = true
				out = append(out, n)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// resolveSeed resolves symbol as store.ResolveName does; the resolver
// itself answers "<variable>#mutex" with the variable's Mutex node
// (persist.MutexSuffix).
func resolveSeed(r store.Reader, symbol string) (store.NameResolution, error) {
	return store.ResolveName(r, symbol, "")
}

// mutexNodes returns the Mutex nodes of the fields and variables among
// seeds that are mutexes: for the field "pkg.T.mu" the node "pkg.T.mu#mutex",
// for a variable the Mutex node of its name at its place (the resolver
// answers the variable's name with the variable alone).
func mutexNodes(r store.Reader, seeds []types.Node) ([]types.Node, error) {
	var out []types.Node
	for _, n := range seeds {
		var name string
		switch n.Type {
		case types.NodeField:
			name = n.QualifiedName + mutexSuffix
		case types.NodeVariable, types.NodeLocalVariable:
			name = n.QualifiedName
		default:
			continue
		}
		nodes, err := r.FindSymbol(name, true, store.FindSymbolOptions{})
		if err != nil {
			return nil, fmt.Errorf("mutex node of %s: %w", n.QualifiedName, err)
		}
		// A field or variable that holds a channel used as a lock names
		// that lock (the make site the channel aliases lead from).
		from, _, err := r.NeighborhoodByIDs([]string{n.ID}, 1, true, string(types.EdgeFlowsTo))
		if err != nil {
			return nil, fmt.Errorf("lock channel of %s: %w", n.QualifiedName, err)
		}
		for _, c := range from {
			if isLockChannel(c) {
				out = append(out, c)
			}
		}
		for _, m := range nodes {
			if m.Type == types.NodeMutex && (n.Type == types.NodeField || m.FilePath == n.FilePath && m.StartLine == n.StartLine) {
				out = append(out, m)
			}
		}
	}
	return out, nil
}

// isLockChannel reports whether n is a channel used as a lock.
func isLockChannel(n types.Node) bool {
	return n.Type == types.NodeChannel && n.SubKind == types.ChannelLockSubKind
}

// isLock reports whether n is a lock: a Mutex node or a channel used as one.
func isLock(n types.Node) bool { return n.Type == types.NodeMutex || isLockChannel(n) }

// GoroutineOp is a goroutine's send or receive on a channel.
type GoroutineOp struct {
	Goroutine string `json:"goroutine"`
	Edge      string `json:"edge"` // sends_to or recvs_from
	Channel   string `json:"channel"`
	File      string `json:"file_path"`
	Line      int    `json:"line"`
}

// isGoroutineChannel reports whether e, from a node of type src, is a
// goroutine's channel operation.
func isGoroutineChannel(e types.Edge, src types.NodeType) bool {
	return src == types.NodeGoroutine && (e.Type == types.EdgeSendsTo || e.Type == types.EdgeRecvsFrom)
}

// walk is the BFS of Analyze over ConcurrencyEdgeTypes, depth hops from ids
// in one direction, without the goroutines' channel operations: those are
// returned apart (gor), with the nodes they lead to not walked from.
func walk(r store.Reader, ids []string, depth int, reverse bool) (nodes []types.Node, edges, gor []types.Edge, err error) {
	seen := map[string]bool{}
	for _, id := range ids {
		seen[id] = true
	}
	frontier := ids
	typeOf := map[string]types.NodeType{}
	for d := 0; d < depth && len(frontier) > 0; d++ {
		ns, es, err := r.NeighborhoodByIDs(frontier, 1, reverse, ConcurrencyEdgeTypes...)
		if err != nil {
			return nil, nil, nil, err
		}
		for _, n := range ns {
			typeOf[n.ID] = n.Type
		}
		byID := make(map[string]types.Node, len(ns))
		for _, n := range ns {
			byID[n.ID] = n
		}
		var next []string
		for _, e := range es {
			if isGoroutineChannel(e, typeOf[e.Src]) {
				gor = append(gor, e)
				continue
			}
			edges = append(edges, e)
			far := e.Dst
			if reverse {
				far = e.Src
			}
			if !seen[far] {
				seen[far] = true
				next = append(next, far)
				if n, ok := byID[far]; ok {
					nodes = append(nodes, n)
				}
			}
		}
		frontier = next
	}
	return nodes, edges, gor, nil
}

// goroutineRows names the goroutine operations gor, once each, in order of
// goroutine, edge, channel and line.
func goroutineRows(r store.Reader, gor []types.Edge) []GoroutineOp {
	if len(gor) == 0 {
		return nil
	}
	ids := make([]string, 0, 2*len(gor))
	for _, e := range gor {
		ids = append(ids, e.Src, e.Dst)
	}
	ns, err := r.NodesByIDs(ids)
	if err != nil {
		return nil
	}
	byID := make(map[string]types.Node, len(ns))
	for _, n := range ns {
		byID[n.ID] = n
	}
	seen := map[GoroutineOp]bool{}
	var out []GoroutineOp
	for _, e := range gor {
		op := GoroutineOp{Goroutine: store.PrintedName(r, byID[e.Src]), Edge: string(e.Type),
			Channel: store.PrintedName(r, byID[e.Dst]), File: e.FilePath, Line: e.Line}
		if !seen[op] {
			seen[op] = true
			out = append(out, op)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		switch {
		case a.Goroutine != b.Goroutine:
			return a.Goroutine < b.Goroutine
		case a.Edge != b.Edge:
			return a.Edge < b.Edge
		case a.Channel != b.Channel:
			return a.Channel < b.Channel
		}
		return a.Line < b.Line
	})
	return out
}
