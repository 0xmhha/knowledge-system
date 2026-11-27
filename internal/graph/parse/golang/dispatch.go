package golang

import (
	"go/ast"
	"go/token"
	gotypes "go/types"
	"sort"
	"time"

	"golang.org/x/tools/go/callgraph"
	"golang.org/x/tools/go/callgraph/vta"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"

	"github.com/0xmhha/knowledge-system/internal/graph/concfacts"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// dispatch.go links dynamic calls to the code they can run (design
// decision D3, "runtime links"). The parser records a call through an
// interface as an invokes edge to the interface method, and a call through a
// function value as an invokes edge to nothing; both stop there, although at
// run time the call reaches a concrete function. A reader who follows the
// code as a program runs needs that next step.
//
// The pass builds SSA for the loaded packages and runs VTA (variable type
// analysis, golang.org/x/tools/go/callgraph/vta), which follows the types that
// flow into each interface value and each function value. A class-hierarchy
// link (every implementation of the interface) was rejected in D3 because
// wide interfaces make it noisy; VTA keeps only the implementations whose
// values can reach a call.
//
// Edges (dispatches_to, confidence INFERRED, the contract's static_may):
//
//	dispatch_kind "interface_method"  interface method -> each implementation
//	                                  whose values reach one of its calls; no
//	                                  line (the calls are the invokes edges)
//	dispatch_kind "func_value"        the function that makes the call -> each
//	                                  declared function the value can hold,
//	                                  with the line of the call
//	dispatch_kind "closure"           the same, when the value is a function
//	                                  literal: the target is the declaration
//	                                  that contains the literal
//
// Closures count for the declaration that contains them, as in the value
// access pass, so a call of a closure inside its own function adds nothing.
// Targets without a node (outside modules, the standard library) are
// skipped and counted. Edges carry no file path: like implements, they are
// rebuilt on every build from the whole program.

// DispatchStats summarises one run of the pass for the build log.
type DispatchStats struct {
	Packages       int // loaded packages given to SSA
	SSAPackages    int // of those, built into SSA with function bodies
	OwnErrors      int // of those, created from types only: errors of their own or no syntax
	BuildPanics    int // of those, whose bodies could not be built
	Functions      int // SSA functions given to VTA
	DynamicSites   int // call edges VTA resolved at dynamic call sites
	InterfaceEdges int // dispatches_to edges of kind interface_method
	FuncValueEdges int // dispatches_to edges of kind func_value or closure
	NoNode         int // resolved targets or sources without a graph node
	ChannelOps     int // channel sends and receives in function bodies
	ChannelLinked  int // of those, linked to the channel they operate on
	ChannelAliases int // flows_to edges: a channel stored in a field or variable
	TypeHierarchy  int // dispatches_to edges of kind type_hierarchy
	ChanLocks      ChanLockStats
	chanLockOps    ChanLocks
	GoroutineCode  int     // calls/reads_field/writes_field edges from a Goroutine (GoroutineKind)
	Receives       int     // receive operations with a CallSite (chanflow.go)
	Handles        int     // handles edges: a receive to the code it makes run
	ReceivedFlows  int     // flows_to edges from a receive to the uses of its value
	Triggers       int     // triggers edges: a sender to a receive it can wake
	HandledPairs   int     // sender x handled-code pairs the two hops stand for
	SSASeconds     float64 // building SSA
	VTASeconds     float64 // the VTA call graph
}

// buildSSA builds SSA function bodies for every loaded package without
// errors of its own, and creates the others from their types only.
//
// ssautil.Packages skips a package whose IllTyped flag is set, and go/packages
// sets that flag when any dependency has an error. One package that fails to
// type-check on this machine (runtime/cgo for a linux/amd64 cgo build on
// macOS) then removes every package that imports it, directly or not: on
// go-stablenet 362 of 469 packages. Their own syntax and types are complete,
// so they are built here; a package whose body still cannot be built is
// counted and left without bodies.
//
// Such a package can have no error of its own: the type checker stays silent
// about a use of an invalid operand (a value whose type comes from a module
// that is not there), and the builder panics on it. The panic leaves the
// functions that packages share (generic instances, wrappers) unfinished, and
// whatever needs one of them later waits for it forever. The whole program is
// therefore dropped and built again without that package's body; every round
// removes one body, so this ends.
func buildSSA(pkgs []*packages.Package, st *DispatchStats) (*ssa.Program, []*ssa.Package) {
	st.Packages = len(pkgs)
	unbuildable := map[*packages.Package]bool{}
	for {
		prog, bodies, ownErrors := createSSA(pkgs, unbuildable)
		failed := buildBodies(bodies)
		if failed != nil {
			unbuildable[failed] = true
			continue
		}
		st.OwnErrors = ownErrors
		st.BuildPanics = len(unbuildable)
		st.SSAPackages = len(bodies)
		built := make([]*ssa.Package, 0, len(bodies))
		for _, b := range bodies {
			built = append(built, b.ssa)
		}
		return prog, built
	}
}

// packageBody is a loaded package and the SSA package made from its syntax.
type packageBody struct {
	loaded *packages.Package
	ssa    *ssa.Package
}

// createSSA creates every package of the program, with syntax for the loaded
// packages that can be built and from types only for the rest. It returns the
// former, dependencies first, and the number of loaded packages left without
// bodies because of their own errors or missing syntax.
func createSSA(pkgs []*packages.Package, unbuildable map[*packages.Package]bool) (*ssa.Program, []packageBody, int) {
	var fset *token.FileSet
	if len(pkgs) > 0 {
		fset = pkgs[0].Fset
	}
	prog := ssa.NewProgram(fset, ssa.InstantiateGenerics)
	initial := make(map[*packages.Package]bool, len(pkgs))
	for _, p := range pkgs {
		initial[p] = true
	}
	var bodies []packageBody
	ownErrors := 0
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		if p.Types == nil {
			return
		}
		complete := initial[p] && len(p.Errors) == 0 && p.TypesInfo != nil && len(p.Syntax) > 0
		if complete && !unbuildable[p] {
			bodies = append(bodies, packageBody{p, prog.CreatePackage(p.Types, p.Syntax, p.TypesInfo, true)})
			return
		}
		if initial[p] && !complete {
			ownErrors++
		}
		prog.CreatePackage(p.Types, nil, nil, true)
	})
	return prog, bodies, ownErrors
}

// buildBodies builds the bodies in order and returns the first package whose
// build panics, or nil. After a panic the program must not be used.
func buildBodies(bodies []packageBody) *packages.Package {
	for _, b := range bodies {
		if !buildBody(b.ssa) {
			return b.loaded
		}
	}
	return nil
}

// buildBody reports whether the package's bodies could be built.
func buildBody(p *ssa.Package) (ok bool) {
	defer func() {
		if recover() != nil {
			ok = false
		}
	}()
	p.Build()
	return true
}

// allBodies is every function the analysis should see: what
// ssautil.AllFunctions reaches from package members and runtime types, plus
// every method of every named type of the built packages and the function
// literals inside them. AllFunctions alone leaves out a method whose type is
// never converted to an interface and whose caller is not reached (a method
// only tests or outside code call), and a graph must cover those too.
func allBodies(prog *ssa.Program, built []*ssa.Package) map[*ssa.Function]bool {
	funcs := ssautil.AllFunctions(prog)
	var addFn func(f *ssa.Function)
	addFn = func(f *ssa.Function) {
		if f == nil {
			return
		}
		funcs[f] = true
		for _, a := range f.AnonFuncs {
			if !funcs[a] {
				addFn(a)
			}
		}
	}
	for f := range funcs {
		for _, a := range f.AnonFuncs {
			addFn(a)
		}
	}
	for _, p := range built {
		for _, m := range p.Members {
			switch x := m.(type) {
			case *ssa.Function:
				addFn(x)
			case *ssa.Type:
				named, ok := x.Type().(*gotypes.Named)
				if !ok || named.TypeParams().Len() > 0 {
					continue // generic types: their instantiations are reached by use
				}
				for _, t := range []gotypes.Type{named, gotypes.NewPointer(named)} {
					ms := prog.MethodSets.MethodSet(t)
					for i := 0; i < ms.Len(); i++ {
						addFn(prog.MethodValue(ms.At(i)))
					}
				}
			}
		}
	}
	return funcs
}

// EmitDispatchEdges returns the dispatches_to edges of the loaded packages
// and their concurrency facts (channel operations, goroutine code, message
// links) for the graph writer (concfacts). Neither is in storage order: the
// writer merges them and orders them with OrderDispatchEdges.
func EmitDispatchEdges(pkgs []*packages.Package, nodes []types.Node) ([]types.Edge, concfacts.Facts, DispatchStats) {
	var st DispatchStats
	if len(pkgs) == 0 {
		return nil, nil, st
	}
	byCanonical := make(map[string]*types.Node, len(nodes))
	values := map[string]*types.Node{}  // fields and package-level variables by canonical id
	chans := map[string][]*types.Node{} // Channel nodes (make sites) by file
	gors := map[int][]*types.Node{}     // Goroutine nodes by start byte
	for i := range nodes {
		n := &nodes[i]
		switch {
		case n.Type == types.NodeGoroutine:
			gors[n.StartByte] = append(gors[n.StartByte], n)
			continue
		case n.Type == types.NodeChannel:
			chans[n.FilePath] = append(chans[n.FilePath], n)
			continue
		case (n.Type == types.NodeField || n.Type == types.NodeVariable) && n.CanonicalID != "":
			if _, ok := values[n.CanonicalID]; !ok {
				values[n.CanonicalID] = n
			}
			continue
		}
		if n.Type != types.NodeFunction && n.Type != types.NodeMethod || n.CanonicalID == "" {
			continue
		}
		if _, ok := byCanonical[n.CanonicalID]; !ok {
			byCanonical[n.CanonicalID] = n
		}
	}

	decls := newDeclIndex(nodes)
	t0 := time.Now()
	prog, built := buildSSA(pkgs, &st)
	funcs := allBodies(prog, built)
	st.Functions = len(funcs)
	st.SSASeconds = time.Since(t0).Seconds()

	t1 := time.Now()
	cg := vta.CallGraph(funcs, nil)
	st.VTASeconds = time.Since(t1).Seconds()

	// nodeOf maps an SSA function to its declaration's node: closures to the
	// outermost enclosing function, instantiations to their generic origin,
	// wrappers to the method they wrap (go/types object identity).
	nodeOf := func(fn *ssa.Function) *types.Node {
		for fn.Parent() != nil {
			fn = fn.Parent()
		}
		if o := fn.Origin(); o != nil {
			fn = o
		}
		obj := fn.Object()
		if obj == nil {
			return nil
		}
		return decls.at(goCanonicalID(obj), "", prog.Fset.Position(obj.Pos()))
	}

	// methodNode maps an interface method to its node: by canonical id, or
	// by place for a method that has none (an interface declared in a
	// function or written inline), so both get their dispatches.
	methodNode := func(m *gotypes.Func) *types.Node {
		if cid := goCanonicalID(m); cid != "" {
			return byCanonical[cid]
		}
		return decls.localMethod(m.Name(), prog.Fset.Position(m.Pos()))
	}

	type key struct {
		src, dst, kind string
		line           int
	}
	at := map[key]int{}
	var out []types.Edge
	add := func(src, dst *types.Node, kind string, line int) {
		if src == nil || dst == nil {
			st.NoNode++
			return
		}
		if src.ID == dst.ID {
			return
		}
		k := key{src.ID, dst.ID, kind, line}
		if i, ok := at[k]; ok {
			out[i].Count++
			return
		}
		at[k] = len(out)
		out = append(out, types.Edge{Src: src.ID, Dst: dst.ID, Type: types.EdgeDispatchesTo, Line: line,
			Count: 1, Confidence: types.ConfInferred, DispatchKind: kind})
	}
	_ = callgraph.GraphVisitEdges(cg, func(e *callgraph.Edge) error {
		if e.Site == nil || e.Callee == nil || e.Callee.Func == nil || e.Caller == nil || e.Caller.Func == nil {
			return nil
		}
		common := e.Site.Common()
		if common.StaticCallee() != nil {
			return nil // a static call: already a calls edge
		}
		st.DynamicSites++
		if common.IsInvoke() {
			// A wrapper of a method promoted from an embedded interface
			// stands for that interface method, which has no body: the
			// dispatch goes on to the implementations, not to it.
			if m, ok := e.Callee.Func.Object().(*gotypes.Func); ok && abstractMethod(m) {
				return nil
			}
			add(methodNode(common.Method), nodeOf(e.Callee.Func), "interface_method", 0)
			return nil
		}
		line := 0
		if pos := e.Site.Pos(); pos.IsValid() {
			line = prog.Fset.Position(pos).Line
		}
		kind := "func_value"
		if e.Callee.Func.Parent() != nil {
			kind = "closure"
		}
		add(nodeOf(e.Caller.Func), nodeOf(e.Callee.Func), kind, line)
		return nil
	})
	// Type-hierarchy candidates: the implementations go/types allows that the
	// value flow did not link, for every interface method some call invokes.
	// VTA misses values that reach an interface through reflection or
	// through code without SSA bodies (the standard library, outside
	// modules); these edges cover them, marked type_hierarchy so a reader
	// can tell them from the value-flow ones (user decision 2026-11-18).
	for _, th := range typeHierarchy(prog, built, funcs) {
		src := methodNode(th.method)
		dst := decls.at(goCanonicalID(th.impl), "", prog.Fset.Position(th.impl.Pos()))
		if src == nil || dst == nil || src.ID == dst.ID {
			continue
		}
		if _, linked := at[key{src.ID, dst.ID, "interface_method", 0}]; linked {
			continue
		}
		add(src, dst, "type_hierarchy", 0)
	}
	ch := &chanLinker{prog: prog, cg: cg, nodeOf: nodeOf, values: values, fields: newFieldIndex(prog.Fset, nodes), chans: chans,
		memo: map[ssa.Value][]chanOrigin{}, started: goroutineStarts(prog, funcs, gors), methodNode: methodNode,
		inPlace: calledInPlace(funcs), aliasNodes: map[string]*types.Node{}}
	for fn := range funcs {
		ch.function(fn, &st)
		ch.stores(fn)
		ch.goroutineCode(fn)
	}
	ch.dynamicGoEntries(funcs, gors)
	st.ChannelAliases = len(ch.aliases)
	ch.locks, st.ChanLocks = ch.channelLocks(funcs)
	facts := make(concfacts.Facts, 0, len(ch.ops))
	for _, op := range append(ch.ops, ch.messageLinks(funcs, nodes, &st)...) {
		facts = append(facts, op.fact())
	}
	for _, e := range append(out, facts.Counted()...) {
		switch {
		case types.IsGoroutineCode(e):
			st.GoroutineCode++
		case e.Type != types.EdgeDispatchesTo:
		case e.DispatchKind == "interface_method":
			st.InterfaceEdges++
		case e.DispatchKind == "type_hierarchy":
			st.TypeHierarchy++
		default:
			st.FuncValueEdges++
		}
	}
	st.chanLockOps = ch.locks
	return out, facts, st
}

// ChanLockOps returns the channel lock operations the pass found
// (chanlock.go), for LockOptions.ChanLocks.
func (st DispatchStats) ChanLockOps() ChanLocks { return st.chanLockOps }

// OrderDispatchEdges sorts the edges of the dispatch pass (dispatches_to)
// and of its concurrency facts (concfacts.Facts.Counted) into the order
// the graph stores them: by type, source, destination, dispatch kind and
// line.
func OrderDispatchEdges(out []types.Edge) []types.Edge {
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Type != b.Type {
			return a.Type < b.Type
		}
		if a.Src != b.Src {
			return a.Src < b.Src
		}
		if a.Dst != b.Dst {
			return a.Dst < b.Dst
		}
		if a.DispatchKind != b.DispatchKind {
			return a.DispatchKind < b.DispatchKind
		}
		return a.Line < b.Line
	})
	return out
}

// fact turns a linker result into its concurrency fact (concfacts): the
// adapter from the Go analyses to the contract. Every one is approximated:
// channels and fields are told apart by declaration (aliasing), a
// goroutine's code and a message's effects run at a time the analysis does
// not know (interleaving), a received value's uses are read from the value
// flow over all paths (branches).
func (op chanOp) fact() concfacts.Fact {
	file := op.src.FilePath
	if op.file != "" {
		file = op.file
	}
	src, dst := op.src.ID, op.dst.ID
	switch {
	case op.kind == GoroutineKind || op.kind == types.DispatchGoroutineWrite:
		return concfacts.ExecContext{Goroutine: src, File: file, Certainty: concfacts.Approx(concfacts.ReasonInterleaving),
			Runs: []concfacts.CodeUse{{Edge: op.typ, Target: dst, Line: op.line, Write: op.kind == types.DispatchGoroutineWrite}}}
	case op.typ == types.EdgeSendsTo || op.typ == types.EdgeRecvsFrom:
		dir := concfacts.Send
		if op.typ == types.EdgeRecvsFrom {
			dir = concfacts.Recv
		}
		return concfacts.ChanOp{Actor: src, Origin: dst, OriginKind: op.kind, Dir: dir, Line: op.line, File: file,
			Certainty: concfacts.Approx(concfacts.ReasonAliasing)}
	case op.typ == types.EdgeFlowsTo && op.kind == concfacts.ReceivedKind:
		return concfacts.Received{Receive: src, Use: dst, Line: op.line, File: file, Certainty: concfacts.Approx(concfacts.ReasonBranches)}
	case op.typ == types.EdgeFlowsTo:
		return concfacts.ChanAlias{Origin: src, Holder: dst, Line: op.line, File: file, Certainty: concfacts.Approx(concfacts.ReasonAliasing)}
	case op.typ == types.EdgeHandles:
		return concfacts.Handles{Receive: src, Target: dst, Kind: op.kind, Line: op.line, File: file,
			Certainty: concfacts.Approx(concfacts.ReasonInterleaving)}
	default: // triggers
		return concfacts.Trigger{Sender: src, Receive: dst, Channel: op.kind, Line: op.line, File: file,
			Certainty: concfacts.Approx(concfacts.ReasonInterleaving)}
	}
}

// --- channels ---

// A channel origin is what a channel value comes from: the make site (a
// Channel node), a struct field or a package-level variable. Sends and
// receives on values of the same origin are on the same channel.
type chanOrigin struct {
	node *types.Node
	kind string // make, field, global
}

type chanOp struct {
	src, dst *types.Node
	typ      types.EdgeType
	kind     string
	line     int
	file     string // the edge's file when it is not src's (a store's function)
}

type chanLinker struct {
	prog    *ssa.Program
	cg      *callgraph.Graph
	nodeOf  func(*ssa.Function) *types.Node
	values  map[string]*types.Node
	fields  fieldIndex
	chans   map[string][]*types.Node
	memo    map[ssa.Value][]chanOrigin
	ops     []chanOp
	aliases map[[2]string]bool
	// events are the channel operations with their origins, for the
	// message links (chanflow.go).
	events []chanEvent
	// started maps a function a go statement starts (its literal, or a
	// declared function: go f(x)) to the Goroutine nodes of those go
	// statements (goroutineStarts).
	started map[*ssa.Function][]*types.Node
	// methodNode maps an interface method to its node.
	methodNode func(*gotypes.Func) *types.Node
	// inPlace are the function literals called where they are written.
	inPlace map[*ssa.Function]bool
	// aliasNodes are the nodes of the aliases, by id.
	aliasNodes map[string]*types.Node
	// locks are the channel lock operations (chanlock.go).
	locks ChanLocks
	// selectors caches selectorNames.
	selectors map[*ssa.Function]map[token.Pos]string
}

// calledInPlace returns the function literals of funcs that their parent
// calls where they are written: func() { ... }(), the call's parenthesis
// right after the literal. A literal stored in a variable and called later
// (shutdown := func() {...}; ...; shutdown()) is not, though SSA calls the
// same closure value; nor is one run by a go or defer statement.
func calledInPlace(funcs map[*ssa.Function]bool) map[*ssa.Function]bool {
	out := map[*ssa.Function]bool{}
	for fn := range funcs {
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				call, ok := in.(*ssa.Call)
				if !ok {
					continue
				}
				var f *ssa.Function
				switch v := call.Call.Value.(type) {
				case *ssa.Function:
					f = v
				case *ssa.MakeClosure:
					f, _ = v.Fn.(*ssa.Function)
				}
				if f == nil || f.Parent() == nil {
					continue
				}
				if lit, ok := f.Syntax().(*ast.FuncLit); ok && call.Pos() == lit.End() {
					out[f] = true
				}
			}
		}
	}
	return out
}

// literalGoroutines returns the Goroutine nodes whose go statement starts
// the function literal fn, or a literal that calls fn in place (and so on
// up): the goroutines whose own code fn is. A declared function, and a
// literal reached otherwise, has none.
func (c *chanLinker) literalGoroutines(fn *ssa.Function) []*types.Node {
	for f := fn; f != nil && f.Parent() != nil; f = f.Parent() {
		if g := c.started[f]; len(g) > 0 {
			return g
		}
		if !c.inPlace[f] {
			return nil
		}
	}
	return nil
}

// selectorNames maps the position of each selector's name in fn's syntax to
// that name.
func (c *chanLinker) selectorNames(fn *ssa.Function) map[token.Pos]string {
	if m, ok := c.selectors[fn]; ok {
		return m
	}
	m := map[token.Pos]string{}
	if syn := fn.Syntax(); syn != nil {
		ast.Inspect(syn, func(n ast.Node) bool {
			if s, ok := n.(*ast.SelectorExpr); ok {
				m[s.Sel.Pos()] = s.Sel.Name
			}
			return true
		})
	}
	if c.selectors == nil {
		c.selectors = map[*ssa.Function]map[token.Pos]string{}
	}
	c.selectors[fn] = m
	return m
}

// explicitField returns the Field node of field i of the struct t denotes
// when the source names it at pos, as reads_field and writes_field record
// a field: an embedded field a selection only passes through (p.Log()
// through an embedded *Logger) is not a use of it. A position that is not a
// selector's name (a composite literal's key) names its field, unless the
// field is embedded: that is an implicit hop.
func (c *chanLinker) explicitField(fn *ssa.Function, t gotypes.Type, i int, pos token.Pos, written bool) *types.Node {
	_, st := literalStruct(t)
	if st == nil || i >= st.NumFields() || !pos.IsValid() {
		return nil
	}
	f := st.Field(i)
	name, isSel := c.selectorNames(fn)[pos]
	switch {
	case isSel && name != f.Name():
		return nil
	case !isSel && f.Embedded() && !written:
		// An implicit hop has the position of the selection, not of a
		// name of its own; a composite literal that sets the embedded
		// field (T{base}) writes it.
		return nil
	}
	return c.fields.node(f)
}

// goroutineStarts maps each function that a go statement of funcs starts to
// the Goroutine nodes of its go statements, found by the go statement's file
// and start byte (the Goroutine node's span).
func goroutineStarts(prog *ssa.Program, funcs map[*ssa.Function]bool, gors map[int][]*types.Node) map[*ssa.Function][]*types.Node {
	out := map[*ssa.Function][]*types.Node{}
	for fn := range funcs {
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				g, ok := in.(*ssa.Go)
				if !ok || !g.Pos().IsValid() {
					continue
				}
				var callee *ssa.Function
				switch v := g.Call.Value.(type) {
				case *ssa.Function:
					callee = v
				case *ssa.MakeClosure:
					callee, _ = v.Fn.(*ssa.Function)
				}
				if callee == nil {
					continue
				}
				// The Goroutine of the longest node path that ends the file
				// name: eth/handler_test.go also ends .../eth/protocols/eth/...
				p := prog.Fset.Position(g.Pos())
				var best *types.Node
				for _, n := range gors[p.Offset] {
					if sameFile(n.FilePath, p.Filename) && (best == nil || len(n.FilePath) > len(best.FilePath)) {
						best = n
					}
				}
				if best != nil {
					out[callee] = append(out[callee], best)
				}
			}
		}
	}
	return out
}

// GoroutineKind is the dispatch kind of an edge from a Goroutine node to the
// code it runs (types.DispatchGoroutine): calls to the function a go
// statement starts (go f(x), go x.m()) or to the declared functions its
// literal calls, and reads_field / writes_field for the fields its literal
// uses. With spawns (function -> Goroutine) and the Goroutine's sends_to /
// recvs_from, the goroutine's own work is reachable from its node.
const GoroutineKind = concfacts.GoroutineKind

// goroutineCode records the edges from a Goroutine node to the code it
// runs (GoroutineKind). The rule: a goroutine started on a declared function
// (go f(x), go x.m()) links to that entry function only, with calls; the
// entry's body is reached through the function's own edges. A goroutine
// started on a function literal links to what the literal does, and what
// the literals it calls in place do: calls to declared functions (calls),
// interface calls (invokes, to the interface method), calls through a
// function value or closure (dispatches_to, to the code the value flow says
// it runs) and the fields it names (reads_field, writes_field). The edge's
// file is the file of the code (the line's file).
func (c *chanLinker) goroutineCode(fn *ssa.Function) {
	if fn.Parent() == nil {
		if gors := c.started[fn]; len(gors) > 0 {
			for _, g := range gors {
				if n := c.nodeOf(fn); n != nil {
					c.ops = append(c.ops, chanOp{src: g, dst: n, typ: types.EdgeCalls, kind: GoroutineKind, file: g.FilePath})
				}
			}
		}
		return
	}
	gors := c.literalGoroutines(fn)
	where := c.nodeOf(fn)
	if len(gors) == 0 || where == nil {
		return
	}
	link := func(dst *types.Node, typ types.EdgeType, pos token.Pos) {
		if dst == nil {
			return
		}
		line := 0
		if pos.IsValid() {
			line = c.prog.Fset.Position(pos).Line
		}
		for _, g := range gors {
			c.ops = append(c.ops, chanOp{src: g, dst: dst, typ: typ, kind: GoroutineKind, line: line, file: where.FilePath})
		}
	}
	cgNode := c.cg.Nodes[fn]
	for _, b := range fn.Blocks {
		for _, in := range b.Instrs {
			switch x := in.(type) {
			case *ssa.Go:
				// A go statement starts another goroutine.
			case ssa.CallInstruction: // a call, or a deferred call (it runs at the end)
				common := x.Common()
				switch {
				case common.StaticCallee() != nil:
					if callee := common.StaticCallee(); callee.Parent() == nil {
						link(c.nodeOf(callee), types.EdgeCalls, x.Pos())
					}
				case common.IsInvoke():
					link(c.methodNode(common.Method), types.EdgeInvokes, x.Pos())
				case cgNode != nil:
					for _, e := range cgNode.Out {
						if e.Site == x && e.Callee != nil && e.Callee.Func != nil {
							link(c.nodeOf(e.Callee.Func), types.EdgeDispatchesTo, x.Pos())
						}
					}
				}
			case *ssa.FieldAddr:
				typ := types.EdgeReadsField
				if refs := x.Referrers(); refs != nil {
					for _, r := range *refs {
						if st, ok := r.(*ssa.Store); ok && st.Addr == x {
							typ = types.EdgeWritesField
						}
					}
				}
				link(c.explicitField(fn, x.X.Type(), x.Field, x.Pos(), typ == types.EdgeWritesField), typ, x.Pos())
			case *ssa.Field:
				link(c.explicitField(fn, x.X.Type(), x.Field, x.Pos(), false), types.EdgeReadsField, x.Pos())
			}
			// A package-level variable the code reads or assigns, as the
			// function-level references edges record it (constants are
			// folded into the code and have no use here).
			for _, op := range in.Operands(nil) {
				g, ok := (*op).(*ssa.Global)
				if !ok {
					continue
				}
				v := c.values[goCanonicalID(g.Object())]
				if v == nil || v.Type != types.NodeVariable {
					continue
				}
				kind := GoroutineKind
				if st, ok := in.(*ssa.Store); ok && st.Addr == g {
					kind = types.DispatchGoroutineWrite
				}
				pos := in.Pos()
				line := 0
				if pos.IsValid() {
					line = c.prog.Fset.Position(pos).Line
				}
				for _, gr := range gors {
					c.ops = append(c.ops, chanOp{src: gr, dst: v, typ: types.EdgeReferences, kind: kind, line: line, file: where.FilePath})
				}
			}
		}
	}
}

// dynamicGoEntries records the entry links of go statements whose target
// is not a declared function or literal: go p.SendWBFTConsensus() on an
// interface method links to the method with invokes, go f() on a function
// value to the code the value flow says it runs, with dispatches_to (both
// kind goroutine), as goroutineCode links a static entry with calls.
func (c *chanLinker) dynamicGoEntries(funcs map[*ssa.Function]bool, gors map[int][]*types.Node) {
	for fn := range funcs {
		cgNode := c.cg.Nodes[fn]
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				g, ok := in.(*ssa.Go)
				if !ok || !g.Pos().IsValid() || g.Call.StaticCallee() != nil {
					continue
				}
				if _, closure := g.Call.Value.(*ssa.MakeClosure); closure {
					continue
				}
				p := c.prog.Fset.Position(g.Pos())
				var gor *types.Node
				for _, n := range gors[p.Offset] {
					if sameFile(n.FilePath, p.Filename) && (gor == nil || len(n.FilePath) > len(gor.FilePath)) {
						gor = n
					}
				}
				if gor == nil {
					continue
				}
				if g.Call.IsInvoke() {
					if m := c.methodNode(g.Call.Method); m != nil {
						c.ops = append(c.ops, chanOp{src: gor, dst: m, typ: types.EdgeInvokes, kind: GoroutineKind, file: gor.FilePath})
					}
					continue
				}
				if cgNode == nil {
					continue
				}
				for _, e := range cgNode.Out {
					if e.Site == g && e.Callee != nil && e.Callee.Func != nil {
						if n := c.nodeOf(e.Callee.Func); n != nil {
							c.ops = append(c.ops, chanOp{src: gor, dst: n, typ: types.EdgeDispatchesTo, kind: GoroutineKind, file: gor.FilePath})
						}
					}
				}
			}
		}
	}
}

// goroutinesOf returns the Goroutine nodes that run fn's code: those of the
// go statements that start fn, or else those of the nearest enclosing
// function literal a go statement starts (a closure inside a goroutine's
// literal runs in that goroutine).
func (c *chanLinker) goroutinesOf(fn *ssa.Function) []*types.Node {
	for f := fn; f != nil; f = f.Parent() {
		if g := c.started[f]; len(g) > 0 {
			return g
		}
	}
	return nil
}

// stores records, for each store of a channel into a field or a
// package-level variable, flows_to from the channel's origins to that
// holder: a channel made in a constructor and kept in a field is then one
// channel under both names.
func (c *chanLinker) stores(fn *ssa.Function) {
	where := c.nodeOf(fn)
	if where == nil {
		return
	}
	if c.aliases == nil {
		c.aliases = map[[2]string]bool{}
	}
	for _, b := range fn.Blocks {
		for _, in := range b.Instrs {
			s, ok := in.(*ssa.Store)
			if !ok {
				continue
			}
			if _, isChan := s.Val.Type().Underlying().(*gotypes.Chan); !isChan {
				continue
			}
			var holder *types.Node
			switch a := s.Addr.(type) {
			case *ssa.FieldAddr:
				holder = c.field(a.X.Type(), a.Field)
			case *ssa.Global:
				holder = c.values[goCanonicalID(a.Object())]
			}
			if holder == nil {
				continue
			}
			line := 0
			if s.Pos().IsValid() {
				line = c.prog.Fset.Position(s.Pos()).Line
			}
			for _, o := range c.origins(s.Val, 0, map[ssa.Value]bool{}) {
				if o.node.ID == holder.ID {
					continue
				}
				c.aliases[[2]string{o.node.ID, holder.ID}] = true
				c.aliasNodes[o.node.ID], c.aliasNodes[holder.ID] = o.node, holder
				c.ops = append(c.ops, chanOp{src: o.node, dst: holder, typ: types.EdgeFlowsTo, kind: "chan", line: line, file: where.FilePath})
			}
		}
	}
}

// chanDepth bounds how far an origin is followed through parameters,
// closures, stores and calls.
const chanDepth = 8

// function records the channel operations of one SSA function.
func (c *chanLinker) function(fn *ssa.Function, st *DispatchStats) {
	src := c.nodeOf(fn)
	if src == nil {
		return
	}
	op := func(v ssa.Value, typ types.EdgeType, pos token.Pos) {
		st.ChannelOps++
		line := 0
		if pos.IsValid() {
			line = c.prog.Fset.Position(pos).Line
		}
		os := c.origins(v, 0, map[ssa.Value]bool{})
		if len(os) > 0 {
			st.ChannelLinked++
		}
		c.events = append(c.events, chanEvent{fn: fn, typ: typ, pos: pos, origins: os})
		// The function the operation is written in gets the edge, and so
		// does the goroutine that runs it, from the same origins.
		gors := c.goroutinesOf(fn)
		for _, o := range os {
			c.ops = append(c.ops, chanOp{src: src, dst: o.node, typ: typ, kind: o.kind, line: line})
			for _, g := range gors {
				// The edge names the operation's place: its line, in the
				// file of the code it is written in.
				c.ops = append(c.ops, chanOp{src: g, dst: o.node, typ: typ, kind: o.kind, line: line, file: src.FilePath})
			}
		}
	}
	for _, b := range fn.Blocks {
		for _, in := range b.Instrs {
			switch x := in.(type) {
			case *ssa.Send:
				op(x.Chan, types.EdgeSendsTo, x.Pos())
			case *ssa.UnOp:
				if x.Op == token.ARROW {
					op(x.X, types.EdgeRecvsFrom, x.Pos())
				}
			case *ssa.Select:
				for _, s := range x.States {
					typ := types.EdgeRecvsFrom
					if s.Dir == gotypes.SendOnly {
						typ = types.EdgeSendsTo
					}
					op(s.Chan, typ, s.Pos)
				}
			}
		}
	}
}

// origins follows a channel value back to where it comes from.
func (c *chanLinker) origins(v ssa.Value, depth int, busy map[ssa.Value]bool) []chanOrigin {
	if v == nil || depth > chanDepth || busy[v] {
		return nil
	}
	if os, ok := c.memo[v]; ok {
		return os
	}
	busy[v] = true
	defer delete(busy, v)
	var out []chanOrigin
	add := func(os []chanOrigin) { out = append(out, os...) }
	switch x := v.(type) {
	case *ssa.MakeChan:
		if n := c.makeSite(x); n != nil {
			out = append(out, chanOrigin{n, "make"})
		}
	case *ssa.UnOp:
		if x.Op == token.MUL { // a load: of a field, a global or a local cell
			switch a := x.X.(type) {
			case *ssa.FieldAddr:
				if n := c.field(a.X.Type(), a.Field); n != nil {
					out = append(out, chanOrigin{n, "field"})
				}
			case *ssa.Global:
				if n := c.values[goCanonicalID(a.Object())]; n != nil {
					out = append(out, chanOrigin{n, "global"})
				}
			case *ssa.Alloc:
				add(c.stored(a, depth, busy))
			case *ssa.FreeVar:
				// A variable a closure captures by reference: the closure
				// holds a pointer to the enclosing function's cell.
				for _, cell := range c.cells(a, depth) {
					add(c.stored(cell, depth+1, busy))
				}
			}
		}
	case *ssa.Field:
		if n := c.field(x.X.Type(), x.Field); n != nil {
			out = append(out, chanOrigin{n, "field"})
		}
	case *ssa.ChangeType:
		add(c.origins(x.X, depth, busy))
	case *ssa.Convert:
		add(c.origins(x.X, depth, busy))
	case *ssa.MakeInterface:
		add(c.origins(x.X, depth, busy))
	case *ssa.TypeAssert:
		add(c.origins(x.X, depth, busy))
	case *ssa.Phi:
		for _, e := range x.Edges {
			add(c.origins(e, depth, busy))
		}
	case *ssa.Parameter:
		add(c.parameter(x, depth, busy))
	case *ssa.FreeVar:
		add(c.freeVar(x, depth, busy))
	case *ssa.Call:
		add(c.returned(&x.Call, 0, x.Parent(), depth, busy))
	case *ssa.Extract:
		if call, ok := x.Tuple.(*ssa.Call); ok {
			add(c.returned(&call.Call, x.Index, x.Parent(), depth, busy))
		}
	}
	out = dedupOrigins(out)
	if depth == 0 {
		c.memo[v] = out
	}
	return out
}

// makeSite is the Channel node whose span holds the make call.
func (c *chanLinker) makeSite(m *ssa.MakeChan) *types.Node {
	fnNode := c.nodeOf(m.Parent())
	if fnNode == nil || !m.Pos().IsValid() {
		return nil
	}
	off := c.prog.Fset.Position(m.Pos()).Offset
	for _, n := range c.chans[fnNode.FilePath] {
		if off >= n.StartByte && off < n.EndByte {
			return n
		}
	}
	return nil
}

// field is the Field node of field index i of the struct t (or *t) denotes.
func (c *chanLinker) field(t gotypes.Type, i int) *types.Node {
	_, st := literalStruct(t)
	if st == nil || i >= st.NumFields() {
		return nil
	}
	return c.fields.node(st.Field(i))
}

// stored follows a local cell (an escaping local) to the values stored in it.
func (c *chanLinker) stored(a *ssa.Alloc, depth int, busy map[ssa.Value]bool) []chanOrigin {
	var out []chanOrigin
	refs := a.Referrers()
	if refs == nil {
		return nil
	}
	for _, r := range *refs {
		if s, ok := r.(*ssa.Store); ok && s.Addr == a {
			out = append(out, c.origins(s.Val, depth+1, busy)...)
		}
	}
	return out
}

// parameter follows a parameter to the arguments of every call of its
// function in the VTA call graph.
func (c *chanLinker) parameter(p *ssa.Parameter, depth int, busy map[ssa.Value]bool) []chanOrigin {
	fn := p.Parent()
	idx := -1
	for i, q := range fn.Params {
		if q == p {
			idx = i
		}
	}
	node := c.cg.Nodes[fn]
	if idx < 0 || node == nil {
		return nil
	}
	var out []chanOrigin
	for _, e := range node.In {
		if e.Site == nil {
			continue
		}
		common := e.Site.Common()
		args := common.Args
		i := idx
		if common.IsInvoke() { // the receiver is common.Value, not an argument
			if i == 0 {
				out = append(out, c.origins(common.Value, depth+1, busy)...)
				continue
			}
			i--
		}
		if i < len(args) {
			out = append(out, c.origins(args[i], depth+1, busy)...)
		}
	}
	return out
}

// cells returns the local cells a free variable that holds a pointer can
// point to: the Alloc bound where the closure is made, through closures
// nested in closures.
func (c *chanLinker) cells(fv *ssa.FreeVar, depth int) []*ssa.Alloc {
	if depth > chanDepth {
		return nil
	}
	fn := fv.Parent()
	idx := -1
	for i, q := range fn.FreeVars {
		if q == fv {
			idx = i
		}
	}
	refs := fn.Referrers()
	if idx < 0 || refs == nil {
		return nil
	}
	var out []*ssa.Alloc
	for _, r := range *refs {
		mc, ok := r.(*ssa.MakeClosure)
		if !ok || mc.Fn != fn || idx >= len(mc.Bindings) {
			continue
		}
		switch b := mc.Bindings[idx].(type) {
		case *ssa.Alloc:
			out = append(out, b)
		case *ssa.FreeVar:
			out = append(out, c.cells(b, depth+1)...)
		}
	}
	return out
}

// freeVar follows a closure's free variable to the value bound where the
// closure is made.
func (c *chanLinker) freeVar(fv *ssa.FreeVar, depth int, busy map[ssa.Value]bool) []chanOrigin {
	fn := fv.Parent()
	idx := -1
	for i, q := range fn.FreeVars {
		if q == fv {
			idx = i
		}
	}
	refs := fn.Referrers()
	if idx < 0 || refs == nil {
		return nil
	}
	var out []chanOrigin
	for _, r := range *refs {
		if mc, ok := r.(*ssa.MakeClosure); ok && mc.Fn == fn && idx < len(mc.Bindings) {
			out = append(out, c.origins(mc.Bindings[idx], depth+1, busy)...)
		}
	}
	return out
}

// returned follows a call's result to what its callees return at index i.
func (c *chanLinker) returned(call *ssa.CallCommon, i int, caller *ssa.Function, depth int, busy map[ssa.Value]bool) []chanOrigin {
	var callees []*ssa.Function
	if f := call.StaticCallee(); f != nil {
		callees = append(callees, f)
	} else if n := c.cg.Nodes[caller]; n != nil {
		for _, e := range n.Out {
			if e.Site != nil && e.Site.Common() == call && e.Callee != nil {
				callees = append(callees, e.Callee.Func)
			}
		}
	}
	var out []chanOrigin
	for _, f := range callees {
		for _, b := range f.Blocks {
			if r, ok := b.Instrs[len(b.Instrs)-1].(*ssa.Return); ok && i < len(r.Results) {
				out = append(out, c.origins(r.Results[i], depth+1, busy)...)
			}
		}
	}
	return out
}

func dedupOrigins(os []chanOrigin) []chanOrigin {
	if len(os) < 2 {
		return os
	}
	seen := map[string]bool{}
	out := os[:0]
	for _, o := range os {
		k := o.node.ID + o.kind
		if !seen[k] {
			seen[k] = true
			out = append(out, o)
		}
	}
	return out
}

type implPair struct {
	method, impl *gotypes.Func
}

// typeHierarchy returns, for each interface method invoked somewhere in
// funcs, each method of a named type of the built packages that implements
// the method's interface (by T or *T).
// abstractMethod reports whether f is a method of an interface. It has no
// body, so no call dispatches to it; a dispatch target is a concrete method.
func abstractMethod(f *gotypes.Func) bool {
	sig, ok := f.Type().(*gotypes.Signature)
	return ok && sig.Recv() != nil && gotypes.IsInterface(sig.Recv().Type())
}

func typeHierarchy(prog *ssa.Program, built []*ssa.Package, funcs map[*ssa.Function]bool) []implPair {
	invoked := map[*gotypes.Func]bool{}
	for fn := range funcs {
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				if c, ok := in.(ssa.CallInstruction); ok && c.Common().IsInvoke() {
					invoked[c.Common().Method] = true
				}
			}
		}
	}
	var named []*gotypes.Named
	for _, p := range built {
		for _, m := range p.Members {
			t, ok := m.(*ssa.Type)
			if !ok {
				continue
			}
			n, ok := t.Type().(*gotypes.Named)
			if !ok || n.TypeParams().Len() > 0 || gotypes.IsInterface(n) {
				continue
			}
			named = append(named, n)
		}
	}
	byIface := map[*gotypes.Interface][]gotypes.Type{}
	implementers := func(it *gotypes.Interface) []gotypes.Type {
		if l, ok := byIface[it]; ok {
			return l
		}
		var l []gotypes.Type
		for _, n := range named {
			for _, t := range []gotypes.Type{n, gotypes.NewPointer(n)} {
				if prog.MethodSets.MethodSet(t).Len() >= it.NumMethods() && gotypes.Implements(t, it) {
					l = append(l, t)
					break
				}
			}
		}
		byIface[it] = l
		return l
	}
	var out []implPair
	for m := range invoked {
		sig, ok := m.Type().(*gotypes.Signature)
		if !ok || sig.Recv() == nil {
			continue
		}
		it, ok := sig.Recv().Type().Underlying().(*gotypes.Interface)
		if !ok {
			continue
		}
		for _, t := range implementers(it) {
			obj, _, _ := gotypes.LookupFieldOrMethod(t, true, m.Pkg(), m.Name())
			// A method promoted from an embedded interface field is that
			// interface's method: not a dispatch target.
			if f, ok := obj.(*gotypes.Func); ok && !abstractMethod(f) {
				out = append(out, implPair{m, f})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.method.FullName() != b.method.FullName() {
			return a.method.FullName() < b.method.FullName()
		}
		return a.impl.FullName() < b.impl.FullName()
	})
	return out
}
