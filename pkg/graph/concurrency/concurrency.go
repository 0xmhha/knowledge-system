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
}

// mutexSuffix ends the qualified name of the Mutex node the Go extractor
// makes for a struct field that is a mutex ("pkg.T.mu#mutex" next to the
// Field node "pkg.T.mu"). The Mutex node of a package-level or local
// variable carries the variable's own name ("pkg.mu", "pkg.F.mu"). Node
// names stay as they are (ids and stored graphs depend on them); a seed
// names any mutex in either form, its declaring name or that name with the
// suffix (see Analyze).
const mutexSuffix = "#mutex"

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
	GuardedBy []Site         `json:"guarded_by,omitempty"`
	Depth     int            `json:"depth"`
	Modules   []Module       `json:"modules"`
	Edges     [][]any        `json:"edges"` // [src, dst, type, line] tuples, sorted
	Totals    map[string]any `json:"totals"`
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
// Deterministic: modules are sorted by qname (tiebreak id), edges by
// (type, src, dst, line), so cks's prompt cache stays stable.
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

	fwdN, fwdE, err := r.NeighborhoodByIDs(ids, d, false, ConcurrencyEdgeTypes...)
	if err != nil {
		return Result{}, err
	}
	revN, revE, err := r.NeighborhoodByIDs(ids, d, true, ConcurrencyEdgeTypes...)
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
			Qname:     n.QualifiedName,
			FilePath:  n.FilePath,
			StartLine: n.StartLine,
			Direction: dir[id],
		}
		if n.FilePath != "" && n.StartLine > 0 {
			m.Citation = fmt.Sprintf("%s:%d", n.FilePath, n.StartLine)
		}
		modules = append(modules, m)
	}
	sort.Slice(modules, func(i, j int) bool {
		if modules[i].Qname != modules[j].Qname {
			return modules[i].Qname < modules[j].Qname
		}
		return modules[i].ID < modules[j].ID
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
		if n.Type == types.NodeMutex {
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
		if n.Type != types.NodeMutex && !hidden[n.ID] {
			roots = append(roots, n)
		}
	}
	res.Mutex = mutexes[0].QualifiedName
	if len(decls) > 0 {
		res.DeclaredBy = decls[0].QualifiedName
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
	res.LockedBy = sites(lockers, lockEdges, func(e types.Edge) (string, bool) { return e.Src, own[e.Dst] })
	if len(decls) > 0 {
		ids := make([]string, len(decls))
		for i, n := range decls {
			ids[i] = n.ID
		}
		guards, guardEdges, err := r.NeighborhoodByIDs(ids, 1, false, string(types.EdgeAccessedUnderLock))
		if err != nil {
			return nil, nil, fmt.Errorf("mutexes guarding %s: %w", res.DeclaredBy, err)
		}
		res.GuardedBy = sites(guards, guardEdges, func(e types.Edge) (string, bool) {
			return e.Dst, hidden[e.Src] && !own[e.Dst]
		})
	}
	return roots, hidden, nil
}

// sites returns one Site per edge of edges that pick keeps, for the node
// pick names, among nodes: at the edge's line, or at the node's line when the
// edge has none. Sorted by qualified name, then line.
func sites(nodes []types.Node, edges []types.Edge, pick func(types.Edge) (string, bool)) []Site {
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
		s := Site{ID: id, Qname: n.QualifiedName, File: n.FilePath, Line: line}
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

// resolveSeed resolves symbol as store.ResolveName does, and accepts the
// "#mutex" form for every mutex: a name with the suffix that resolves to
// nothing is the declaring name of a package-level or local variable whose
// Mutex node carries that name without the suffix, and resolves to that
// Mutex node alone, as "pkg.T.mu#mutex" does for a field.
func resolveSeed(r store.Reader, symbol string) (store.NameResolution, error) {
	name, err := store.ResolveName(r, symbol, "")
	if err != nil || name.Found() || name.Ambiguous() {
		return name, err
	}
	declaring, ok := strings.CutSuffix(symbol, mutexSuffix)
	if !ok || declaring == "" {
		return name, nil
	}
	alt, err := store.ResolveName(r, declaring, "")
	if err != nil || alt.Ambiguous() {
		// The declaring name names several declarations: its candidates
		// are the answer, as they are for the name without the suffix.
		return alt, err
	}
	if !alt.Found() {
		return name, nil
	}
	var mutexes []types.Node
	for _, n := range alt.Nodes {
		if n.Type == types.NodeMutex {
			mutexes = append(mutexes, n)
		}
	}
	if len(mutexes) == 0 {
		return name, nil
	}
	return store.NameResolution{QName: alt.QName, Nodes: mutexes, ByID: true}, nil
}

// mutexNodes returns the Mutex nodes of the fields among seeds that are
// mutexes: for the field "pkg.T.mu" the node "pkg.T.mu#mutex".
func mutexNodes(r store.Reader, seeds []types.Node) ([]types.Node, error) {
	var out []types.Node
	for _, n := range seeds {
		if n.Type != types.NodeField {
			continue
		}
		nodes, err := r.FindSymbol(n.QualifiedName+mutexSuffix, true, store.FindSymbolOptions{})
		if err != nil {
			return nil, fmt.Errorf("mutex node of %s: %w", n.QualifiedName, err)
		}
		for _, m := range nodes {
			if m.Type == types.NodeMutex {
				out = append(out, m)
			}
		}
	}
	return out, nil
}
