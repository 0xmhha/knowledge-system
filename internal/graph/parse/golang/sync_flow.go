package golang

import (
	"go/token"
	gotypes "go/types"
	"slices"
	"sort"

	"golang.org/x/tools/go/ssa"

	"github.com/0xmhha/knowledge-system/internal/graph/concfacts"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// sync_flow.go follows a sync primitive the syntax pass (syncops.go) could
// not name to the values it comes from, as the channel linker follows a
// channel: func worker(wg *sync.WaitGroup) { defer wg.Done() } called as
// go worker(&wg) operates on the caller's wg. Only the operations the
// syntax left unnamed are followed (syncSyntaxFacts.unnamed): one held by a
// parameter, a captured parameter, a call result, a conversion.
//
// The rule: the value is followed back through parameters (to what every
// caller in the call graph passes), the cells of captured variables (to
// what is stored in them), phis and conversions, up to syncFlowDepth calls.
// Each value it ends at is a source:
//
//   - the address or value of a field, a package-level variable or a local
//     variable with a node: that declaration;
//   - the value of a call or a composite literal assigned to a local
//     variable with a node: that variable (syncSyntaxFacts.assigned);
//   - the value of a call of package context or time (context.WithCancel,
//     time.NewTimer, ...) that no variable with a node takes: the site of
//     that call, as for <-time.After(d);
//   - anything else (a value returned by another function, a map element, a
//     parameter of a function no call in the graph reaches, a value past the
//     depth): unresolved.
//
// A parameter of an exported function or method is unresolved too: code the
// graph does not see can call it (another module, a reflective dispatch such
// as an RPC server's), so the calls the graph has are not all of them. On
// go-stablenet the ctx.Done() of requestOp.wait (rpc/client.go) named one
// test's variable, its only caller with a declaration, as the context.
//
// An operates_on edge from the site names the whole set or nothing: when
// every source resolves, the site has an edge to each (approximated,
// aliasing, once the flow crossed a parameter, a cell or a phi: the value
// flow joins every call); when any source is unresolved the site has no
// edge at all. An edge never names part of the set as if it were all of
// it. The operation is decided over every SSA function its site appears in
// (a package and its test variant): one unresolved reading drops it.
//
// An operation the syntax left unnamed that gets no edge is recorded on its
// CallSite instead (Node.SyncUnknown): the operation, and why its primitive
// is not known (types.SyncUnknownReason; for a parameter, how many of its
// direct callers resolve). So a site with neither an edge nor a mark makes
// no operation on a sync primitive. The graph has no node for a parameter
// and an edge needs a target, so the mark is an attribute of the site, not
// an edge to a made-up node. DispatchStats.SyncNoOrigin counts the marks.
//
// A context or timer made into a variable no node stands for (ctx, cancel
// := context.WithCancel(ctx), assigning a parameter) is held by the
// declarations its value is stored into (&it{ctx: ctx} stores it in the
// field it.ctx): ctx_make (cancel_make, timer_make) names them, all or none
// (a store into anything without a node makes the set incomplete); with no
// store, the site is marked no_holder.
//
// A call of sync.Once.Do, time.AfterFunc or sync.WaitGroup.Go whose
// function is a parameter gets a runs_func edge to every declared function
// the callers pass.

// syncFlowDepth bounds how many calls a parameter is followed through.
const syncFlowDepth = 5

// runsLaterArg is the index, in a call's SSA arguments, of the function a
// call runs later, by the operation (a method's receiver is argument 0).
var runsLaterArg = map[types.SyncOp]int{types.SyncOnceDo: 1, types.SyncWaitGroupGo: 1, types.SyncAfterFunc: 1}

// syncFlow is the state of one pass.
type syncFlow struct {
	c      *chanLinker
	ix     *siteIndex
	locals localIndex
	syn    syncSyntaxFacts
	st     *DispatchStats
	seen   map[[3]string]bool
	out    concfacts.Facts
	// ops are the readings of each operation, by unnamedKey, decided when
	// every function is read.
	ops   map[string]*opReading
	order []string
	// cancelFields are the fields (of any function type) a cancel function
	// is stored into, by id; doneFields the fields a context's Done channel
	// is stored into, with the contexts' sources (stored).
	cancelFields map[string]*types.Node
	doneFields   map[string][]*types.Node
}

// opReading is what the readings of one operation found.
type opReading struct {
	site    *types.Node
	line    int
	op      types.SyncOp
	nodes   []*types.Node
	joined  bool
	unknown *types.SyncUnknown // the first unresolved reading's reason
}

// syncParams returns the facts of the operations on primitives the syntax
// could not name, and of the functions run later that a function receives
// as a parameter.
func (c *chanLinker) syncParams(funcs map[*ssa.Function]bool, syn syncSyntaxFacts, nodes []types.Node, st *DispatchStats) concfacts.Facts {
	f := &syncFlow{c: c, ix: newSiteIndex(nodes), locals: newLocalIndex(c.prog.Fset, nodes), syn: syn, st: st, seen: map[[3]string]bool{},
		ops: map[string]*opReading{}, cancelFields: map[string]*types.Node{}, doneFields: map[string][]*types.Node{}}
	fns := make([]*ssa.Function, 0, len(funcs))
	for fn := range funcs {
		fns = append(fns, fn)
	}
	sort.Slice(fns, func(i, j int) bool {
		return fns[i].Pos() < fns[j].Pos() || fns[i].Pos() == fns[j].Pos() && fns[i].String() < fns[j].String()
	})
	// The stores first: a call or a receive through a field reads what
	// every store put there, wherever it is written.
	for _, fn := range fns {
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				if x, ok := in.(*ssa.Store); ok {
					f.stored(x)
				}
			}
		}
	}
	for _, fn := range fns {
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				switch x := in.(type) {
				case *ssa.Call:
					f.call(x.Common())
					f.maker(x)
					f.heldCancel(x)
				case ssa.CallInstruction:
					f.call(x.Common())
				case *ssa.UnOp:
					if x.Op == token.ARROW {
						f.receive(x.X, x.Pos())
					}
				case *ssa.Select:
					for _, s := range x.States {
						if s.Dir == gotypes.RecvOnly {
							f.receive(s.Chan, s.Pos)
						}
					}
				}
			}
		}
	}
	f.decide(nodes)
	return f.out
}

// decide emits the edges of the operations every reading resolved, and
// marks the sites of the others, and of the unnamed operations no reading
// reached, on the nodes.
func (f *syncFlow) decide(nodes []types.Node) {
	marks := map[*types.Node][]types.SyncUnknown{}
	for _, k := range f.order {
		r := f.ops[k]
		if r.unknown != nil || len(r.nodes) == 0 {
			u := types.SyncUnknown{Op: r.op, Reason: types.SyncNoOrigin}
			if r.unknown != nil {
				u = *r.unknown
			}
			marks[r.site] = append(marks[r.site], u)
			continue
		}
		certainty := concfacts.Proof
		if r.joined {
			certainty = concfacts.Approx(concfacts.ReasonAliasing)
		}
		for _, h := range dedupNodes(r.nodes) {
			if sk := [3]string{r.site.ID, h.ID, string(r.op)}; !f.seen[sk] {
				f.seen[sk] = true
				f.st.SyncFollowed++
				f.out = append(f.out, concfacts.SyncUse{Site: r.site.ID, Primitive: h.ID, Op: r.op, Line: r.line, File: r.site.FilePath,
					Certainty: certainty})
			}
		}
	}
	for i := range nodes {
		n := &nodes[i]
		if n.Type != types.NodeCallSite {
			continue
		}
		for _, op := range types.AllSyncOps() {
			if k := unnamedKey(n.ID, op); f.syn.unnamed[k] && f.ops[k] == nil {
				marks[n] = append(marks[n], types.SyncUnknown{Op: op, Reason: types.SyncNotFollowed})
			}
		}
	}
	for n, us := range marks {
		sort.Slice(us, func(i, j int) bool { return us[i].Op < us[j].Op })
		n.SyncUnknown = us
		f.st.SyncNoOrigin += len(us)
	}
}

// stored records a sync primitive (a context, a cancel function, a timer,
// a WaitGroup, a Once, a Cond held by pointer) stored into a field or a
// package-level variable as flowing there from every source of the stored
// value (concfacts.SyncAlias), when every source resolves: an operation on
// the field is one on what the source holds. A store whose sources do not
// all resolve gives none.
func (f *syncFlow) stored(st *ssa.Store) {
	// A cancel function stored in a field of type func() is converted on
	// the way: the value is still the primitive.
	t := st.Val.Type()
	val := st.Val
	if ct, ok := st.Val.(*ssa.ChangeType); ok {
		t, val = ct.X.Type(), ct.X
	}
	var dst *types.Node
	switch a := st.Addr.(type) {
	case *ssa.FieldAddr:
		dst = f.c.field(a.X.Type(), a.Field)
	case *ssa.Global:
		dst = f.c.values[goCanonicalID(a.Object())]
	}
	if dst == nil {
		return
	}
	if call, ok := val.(*ssa.Call); ok {
		if op, recv, isSync := syncCall(&call.Call); isSync && op == types.SyncContextDone {
			f.storedDone(st, call, recv, dst)
			return
		}
	}
	if !isSyncPrimitive(t) {
		return
	}
	if pkg, name := namedOf(t); pkg == contextPkgPath && cancelFuncTypes[name] {
		if fpkg, fname := namedOf(st.Val.Type()); fpkg != contextPkgPath || !cancelFuncTypes[fname] {
			// A field of a plain function type: the syntax cannot tell a
			// call through it is a cancel (heldCancel).
			f.cancelFields[dst.ID] = dst
		}
	}
	src := f.sources(st.Val, 0, map[ssa.Value]bool{})
	if src.unresolved > 0 {
		return
	}
	pos := f.c.prog.Fset.Position(st.Pos())
	for _, h := range dedupNodes(src.nodes) {
		if k := [3]string{h.ID, dst.ID, concfacts.SyncAliasKind}; h.ID != dst.ID && !f.seen[k] {
			f.seen[k] = true
			// The edge's file is the store's, not the field's.
			f.out = append(f.out, concfacts.SyncAlias{Holder: h.ID, Store: dst.ID, Line: pos.Line, File: f.ix.file(pos),
				Certainty: concfacts.Approx(concfacts.ReasonAliasing)})
		}
	}
}

// storedDone records the Done channel of a context, made by the call at
// its site, stored into the field dst: flows_to of kind sync from the
// call's site to the field, and the contexts (the sources of the receiver
// of Done, when they all resolve) a receive on the field waits on.
func (f *syncFlow) storedDone(st *ssa.Store, call *ssa.Call, recv ssa.Value, dst *types.Node) {
	site := f.ix.site(f.c.prog.Fset.Position(call.Pos()), "", false)
	if site == nil {
		return
	}
	pos := f.c.prog.Fset.Position(st.Pos())
	if k := [3]string{site.ID, dst.ID, concfacts.SyncAliasKind}; !f.seen[k] {
		f.seen[k] = true
		f.out = append(f.out, concfacts.SyncAlias{Holder: site.ID, Store: dst.ID, Line: pos.Line, File: f.ix.file(pos),
			Certainty: concfacts.Approx(concfacts.ReasonAliasing)})
	}
	if src := f.sources(recv, 0, map[ssa.Value]bool{}); src.unresolved == 0 {
		f.doneFields[dst.ID] = dedupNodes(append(f.doneFields[dst.ID], src.nodes...))
	}
}

// heldCancel records a call through a field of a plain function type that
// a cancel function is stored into (cancelFields) as a cancel on that
// field (approximated, aliasing: the field may hold other functions too).
func (f *syncFlow) heldCancel(call *ssa.Call) {
	load, ok := call.Call.Value.(*ssa.UnOp)
	if !ok || load.Op != token.MUL {
		return
	}
	fa, ok := load.X.(*ssa.FieldAddr)
	if !ok {
		return
	}
	field := f.c.field(fa.X.Type(), fa.Field)
	if field == nil || f.cancelFields[field.ID] == nil {
		return
	}
	pos := f.c.prog.Fset.Position(call.Pos())
	site := f.ix.site(pos, "", false)
	if site == nil {
		return
	}
	if k := [3]string{site.ID, field.ID, string(types.SyncCancel)}; !f.seen[k] {
		f.seen[k] = true
		f.out = append(f.out, concfacts.SyncUse{Site: site.ID, Primitive: field.ID, Op: types.SyncCancel, Line: pos.Line,
			File: site.FilePath, Certainty: concfacts.Approx(concfacts.ReasonAliasing)})
	}
}

// heldDone records a receive at site from a field a context's Done channel
// is stored into as a done_recv on each of those contexts.
func (f *syncFlow) heldDone(ch ssa.Value, site *types.Node, line int) {
	load, ok := ch.(*ssa.UnOp)
	if !ok || load.Op != token.MUL || site == nil {
		return
	}
	fa, ok := load.X.(*ssa.FieldAddr)
	if !ok {
		return
	}
	field := f.c.field(fa.X.Type(), fa.Field)
	if field == nil {
		return
	}
	for _, ctx := range f.doneFields[field.ID] {
		if k := [3]string{site.ID, ctx.ID, string(types.SyncDoneRecv)}; !f.seen[k] {
			f.seen[k] = true
			f.out = append(f.out, concfacts.SyncUse{Site: site.ID, Primitive: ctx.ID, Op: types.SyncDoneRecv, Line: line,
				File: site.FilePath, Certainty: concfacts.Approx(concfacts.ReasonAliasing)})
		}
	}
}

// syncPrimitiveTypes are the named types whose values are sync primitives
// a field can hold, by "<import path>.<name>"; a pointer to the ones marked
// pointer.
var syncPrimitiveTypes = map[string]bool{
	"context.Context": false, "context.CancelFunc": false, "context.CancelCauseFunc": false,
	"time.Timer": true, "time.Ticker": true, "sync.WaitGroup": true, "sync.Once": true, "sync.Cond": true,
}

// isSyncPrimitive reports whether t is the type of a sync primitive value.
func isSyncPrimitive(t gotypes.Type) bool {
	_, isPtr := gotypes.Unalias(t).(*gotypes.Pointer)
	pkg, name := namedOf(t)
	wantPtr, ok := syncPrimitiveTypes[pkg+"."+name]
	return ok && wantPtr == isPtr
}

// reading returns the record of op at site, made on its first reading.
func (f *syncFlow) reading(site *types.Node, line int, op types.SyncOp) *opReading {
	k := unnamedKey(site.ID, op)
	r := f.ops[k]
	if r == nil {
		r = &opReading{site: site, line: line, op: op}
		f.ops[k] = r
		f.order = append(f.order, k)
	}
	return r
}

// madeResults are the results of a maker of package context or time and
// the operation each makes, by result index.
var madeResults = map[string][]types.SyncOp{
	contextPkgPath: {types.SyncContextMake, types.SyncCancelMake},
	timePkgPath:    {types.SyncTimerMake},
}

// maker handles a call of a maker of contexts or timers the syntax could not
// name a holder for: the declarations each result is stored into.
func (f *syncFlow) maker(call *ssa.Call) {
	callee := call.Call.StaticCallee()
	if callee == nil || callee.Pkg == nil || callee.Parent() != nil {
		return
	}
	path, name := callee.Pkg.Pkg.Path(), callee.Name()
	isMaker := path == contextPkgPath && contextMakers[name] || path == timePkgPath && timerMakers[name]
	if !isMaker {
		return
	}
	pos := f.c.prog.Fset.Position(call.Pos())
	site := f.ix.site(pos, "", false)
	if site == nil {
		return
	}
	for i, op := range madeResults[path] {
		if !f.syn.unnamed[unnamedKey(site.ID, op)] {
			continue
		}
		r := f.reading(site, pos.Line, op)
		holders, complete := f.storedIn(resultOf(call, i))
		if !complete || len(holders) == 0 {
			if r.unknown == nil {
				r.unknown = &types.SyncUnknown{Op: op, Reason: types.SyncNoHolder}
			}
			continue
		}
		r.nodes = append(r.nodes, holders...)
	}
}

// resultOf returns result i of call: the call itself for a single result,
// the Extract of it for a tuple (nil when the result is not used).
func resultOf(call *ssa.Call, i int) ssa.Value {
	if _, tuple := call.Type().(*gotypes.Tuple); !tuple {
		return call
	}
	refs := call.Referrers()
	if refs == nil {
		return nil
	}
	for _, r := range *refs {
		if x, ok := r.(*ssa.Extract); ok && x.Index == i {
			return x
		}
	}
	return nil
}

// storedIn returns the declarations v is stored into (a field, a variable
// with a node), and whether every store of v names one.
func (f *syncFlow) storedIn(v ssa.Value) ([]*types.Node, bool) {
	if v == nil || v.Referrers() == nil {
		return nil, true
	}
	var out []*types.Node
	for _, r := range *v.Referrers() {
		st, ok := r.(*ssa.Store)
		if !ok || st.Val != v {
			continue
		}
		var n *types.Node
		switch a := st.Addr.(type) {
		case *ssa.FieldAddr:
			n = f.c.field(a.X.Type(), a.Field)
		case *ssa.Global:
			n = f.c.values[goCanonicalID(a.Object())]
		case *ssa.Alloc:
			n = f.locals.pos.declaredAt(a.Comment, a.Pos())
		}
		if n == nil {
			return nil, false
		}
		out = append(out, n)
	}
	return out, true
}

// call handles one call on a primitive.
func (f *syncFlow) call(common *ssa.CallCommon) {
	op, recv, ok := syncCall(common)
	if !ok {
		return
	}
	pos := f.c.prog.Fset.Position(common.Pos())
	site := f.ix.site(pos, "", false)
	if recv != nil {
		f.uses(site, pos.Line, recv, op)
	}
	if i, later := runsLaterArg[op]; later && i < len(common.Args) {
		f.runs(site, pos.Line, common.Args[i], op)
	}
}

// receive handles a receive at pos from ch: <-ctx.Done() and <-t.C of a
// time.Timer or time.Ticker.
func (f *syncFlow) receive(ch ssa.Value, at token.Pos) {
	if p := f.c.prog.Fset.Position(at); len(f.doneFields) > 0 {
		f.heldDone(ch, f.ix.recvSite(p), p.Line)
	}
	var op types.SyncOp
	var recv ssa.Value
	switch x := ch.(type) {
	case *ssa.Call:
		got, r, ok := syncCall(&x.Call)
		if !ok || got != types.SyncContextDone {
			return
		}
		op, recv = types.SyncDoneRecv, r
	case *ssa.UnOp:
		fa, ok := x.X.(*ssa.FieldAddr)
		if x.Op != token.MUL || !ok || !isTimerChan(fa) {
			return
		}
		op, recv = types.SyncTimerRecv, fa.X
	default:
		return
	}
	pos := f.c.prog.Fset.Position(at)
	f.uses(f.ix.recvSite(pos), pos.Line, recv, op)
}

// isTimerChan reports whether fa is the address of the field C of a
// time.Timer or time.Ticker.
func isTimerChan(fa *ssa.FieldAddr) bool {
	ptr, ok := fa.X.Type().Underlying().(*gotypes.Pointer)
	if !ok {
		return false
	}
	st, ok := ptr.Elem().Underlying().(*gotypes.Struct)
	if !ok || fa.Field >= st.NumFields() || st.Field(fa.Field).Name() != timerChanField {
		return false
	}
	pkg, typ := namedOf(ptr.Elem())
	return pkg == timePkgPath && (typ == "Timer" || typ == "Ticker")
}

// syncCall returns the operation of a call on a primitive and the value it
// operates on: the receiver of a method, the first argument of a
// sync/atomic function. time.AfterFunc operates on nothing it is passed.
func syncCall(common *ssa.CallCommon) (types.SyncOp, ssa.Value, bool) {
	if common.IsInvoke() {
		op, ok := syncMethods[methodKey(common.Method)]
		return op, common.Value, ok
	}
	callee := common.StaticCallee()
	if callee == nil {
		return "", nil, false
	}
	fn, ok := callee.Object().(*gotypes.Func)
	if !ok || fn.Pkg() == nil || len(common.Args) == 0 {
		return "", nil, false
	}
	if sig, _ := fn.Type().(*gotypes.Signature); sig != nil && sig.Recv() != nil {
		op, ok := syncMethods[methodKey(fn)]
		if !ok && fn.Pkg().Path() == atomicPkgPath {
			op, ok = atomicOp(fn.Name())
		}
		return op, common.Args[0], ok
	}
	switch {
	case fn.Pkg().Path() == atomicPkgPath:
		op, ok := atomicOp(fn.Name())
		return op, common.Args[0], ok
	case fn.Pkg().Path() == timePkgPath && fn.Name() == "AfterFunc":
		return types.SyncAfterFunc, nil, true
	}
	return "", nil, false
}

// flowSources are the sources a value is followed to (see the rule above).
type flowSources struct {
	nodes []*types.Node
	// unresolved counts the values no source is named for.
	unresolved int
	// joined says the flow crossed a parameter, a cell or a phi: the
	// sources are those of every call, not of one run.
	joined bool
}

func (s *flowSources) add(o flowSources) {
	s.nodes = append(s.nodes, o.nodes...)
	s.unresolved += o.unresolved
	s.joined = s.joined || o.joined
}

// uses reads op at site on the sources of v, when the syntax left the
// operation unnamed (decide emits or marks it).
func (f *syncFlow) uses(site *types.Node, line int, v ssa.Value, op types.SyncOp) {
	if site == nil || !f.syn.unnamed[unnamedKey(site.ID, op)] {
		return
	}
	r := f.reading(site, line, op)
	src := f.sources(v, 0, map[ssa.Value]bool{})
	if src.unresolved > 0 || len(src.nodes) == 0 {
		f.st.SyncUnresolved += max(src.unresolved, 1)
		u := f.why(v, op)
		if r.unknown == nil || widerReading(u, *r.unknown) {
			r.unknown = &u
		}
		return
	}
	r.nodes = append(r.nodes, src.nodes...)
	r.joined = r.joined || src.joined
}

// widerReading reports whether the unresolved reading a says more than b:
// it sees more callers (a test variant's function sees its tests' calls
// too), then more resolved ones, then by reason. Which reading is kept
// must not depend on the order the functions are read in, which follows
// token positions that change from one load to the next.
func widerReading(a, b types.SyncUnknown) bool {
	if a.CallersTotal != b.CallersTotal {
		return a.CallersTotal > b.CallersTotal
	}
	if a.CallersResolved != b.CallersResolved {
		return a.CallersResolved > b.CallersResolved
	}
	return a.Reason < b.Reason
}

// why says why the sources of v do not resolve: through the parameter v
// comes from, how many of its callers resolve; otherwise no origin.
func (f *syncFlow) why(v ssa.Value, op types.SyncOp) types.SyncUnknown {
	p := f.paramOf(v, map[ssa.Value]bool{})
	if p == nil {
		return types.SyncUnknown{Op: op, Reason: types.SyncNoOrigin}
	}
	args := f.callerArgs(p)
	resolved := 0
	for _, a := range args {
		if src := f.sources(a, 1, map[ssa.Value]bool{}); src.unresolved == 0 && len(src.nodes) > 0 {
			resolved++
		}
	}
	reason := types.SyncIncompleteCallers
	if len(args) == 0 || openToUnseenCallers(p.Parent()) {
		reason = types.SyncOpenCallers
	}
	return types.SyncUnknown{Op: op, Reason: reason, CallersResolved: resolved, CallersTotal: len(args)}
}

// paramOf returns the parameter v is read from, through conversions,
// loads, the cells of captured variables and the closures that bind them;
// nil when v does not come from one.
func (f *syncFlow) paramOf(v ssa.Value, busy map[ssa.Value]bool) *ssa.Parameter {
	if v == nil || busy[v] {
		return nil
	}
	busy[v] = true
	switch x := v.(type) {
	case *ssa.Parameter:
		return x
	case *ssa.UnOp:
		if x.Op == token.MUL {
			return f.paramOf(x.X, busy)
		}
	case *ssa.ChangeType:
		return f.paramOf(x.X, busy)
	case *ssa.Convert:
		return f.paramOf(x.X, busy)
	case *ssa.MakeInterface:
		return f.paramOf(x.X, busy)
	case *ssa.ChangeInterface:
		return f.paramOf(x.X, busy)
	case *ssa.FreeVar:
		for _, b := range f.bindings(x) {
			if p := f.paramOf(b, busy); p != nil {
				return p
			}
		}
	case *ssa.Alloc:
		if refs := x.Referrers(); refs != nil {
			for _, r := range *refs {
				if st, ok := r.(*ssa.Store); ok && st.Addr == x {
					if p := f.paramOf(st.Val, busy); p != nil {
						return p
					}
				}
			}
		}
	}
	return nil
}

// sources follows v to its sources.
func (f *syncFlow) sources(v ssa.Value, depth int, busy map[ssa.Value]bool) flowSources {
	unresolved := flowSources{unresolved: 1}
	if v == nil || depth > syncFlowDepth {
		return unresolved
	}
	if busy[v] {
		return flowSources{} // a cycle adds no source of its own
	}
	busy[v] = true
	defer delete(busy, v)
	named := func(n *types.Node) flowSources {
		if n == nil {
			return unresolved
		}
		return flowSources{nodes: []*types.Node{n}}
	}
	var out flowSources
	switch x := v.(type) {
	case *ssa.Parameter:
		args := f.callerArgs(x)
		if len(args) == 0 || openToUnseenCallers(x.Parent()) {
			return unresolved
		}
		out.joined = true
		for _, a := range args {
			out.add(f.sources(a, depth+1, busy))
		}
	case *ssa.FreeVar:
		bound := f.bindings(x)
		if len(bound) == 0 {
			return unresolved
		}
		out.joined = true
		for _, b := range bound {
			out.add(f.sources(b, depth, busy))
		}
	case *ssa.FieldAddr:
		return named(f.c.field(x.X.Type(), x.Field))
	case *ssa.Global:
		return named(f.c.values[goCanonicalID(x.Object())])
	case *ssa.Alloc:
		return f.cell(x, depth, busy)
	case *ssa.UnOp:
		if x.Op != token.MUL {
			return unresolved
		}
		return f.sources(x.X, depth, busy)
	case *ssa.Phi:
		out.joined = true
		for _, e := range x.Edges {
			out.add(f.sources(e, depth, busy))
		}
	case *ssa.MakeInterface:
		return f.sources(x.X, depth, busy)
	case *ssa.ChangeType:
		return f.sources(x.X, depth, busy)
	case *ssa.ChangeInterface:
		return f.sources(x.X, depth, busy)
	case *ssa.Convert:
		return f.sources(x.X, depth, busy)
	case *ssa.Extract:
		if c, ok := x.Tuple.(*ssa.Call); ok {
			return named(f.made(c, x.Index))
		}
		return unresolved
	case *ssa.Call:
		return named(f.made(x, 0))
	default:
		return unresolved
	}
	return out
}

// openToUnseenCallers reports whether code outside the graph can call fn: a
// declared function or method with an exported name (a closure is reached
// only through its parent's values).
func openToUnseenCallers(fn *ssa.Function) bool {
	if fn.Parent() != nil {
		return false
	}
	obj := fn.Object()
	return obj != nil && obj.Exported()
}

// cell returns the sources of a local cell: the variable when it has a
// node, the variable a composite literal or new(T) is assigned to, else
// what is stored in it (a captured parameter is spilled to a cell).
func (f *syncFlow) cell(a *ssa.Alloc, depth int, busy map[ssa.Value]bool) flowSources {
	if n := f.locals.pos.declaredAt(a.Comment, a.Pos()); n != nil {
		return flowSources{nodes: []*types.Node{n}}
	}
	if n := f.assignedAt(a.Pos(), 0); n != nil {
		return flowSources{nodes: []*types.Node{n}}
	}
	var out flowSources
	stored := false
	if refs := a.Referrers(); refs != nil {
		for _, r := range *refs {
			if st, ok := r.(*ssa.Store); ok && st.Addr == a {
				stored = true
				out.add(f.sources(st.Val, depth, busy))
			}
		}
	}
	if !stored {
		return flowSources{unresolved: 1}
	}
	out.joined = true
	return out
}

// made returns the source a call's result is: the local variable it is
// assigned to, or for a call of package context or time no variable with
// a node takes, the call's site; nil otherwise.
func (f *syncFlow) made(c *ssa.Call, result int) *types.Node {
	if n := f.assignedAt(c.Pos(), result); n != nil {
		return n
	}
	callee := c.Call.StaticCallee()
	if callee == nil || callee.Pkg == nil {
		return nil
	}
	switch callee.Pkg.Pkg.Path() {
	case contextPkgPath, timePkgPath:
		return f.ix.site(f.c.prog.Fset.Position(c.Pos()), "", false)
	}
	return nil
}

// assignedAt returns the local variable the value made at pos (result
// result of it) is assigned to.
func (f *syncFlow) assignedAt(pos token.Pos, result int) *types.Node {
	if !pos.IsValid() {
		return nil
	}
	p := f.c.prog.Fset.Position(pos)
	return f.syn.assigned[valueAt{file: p.Filename, offset: p.Offset, result: result}]
}

// bindings returns the values the closures of fv's function are made with
// for fv.
func (f *syncFlow) bindings(fv *ssa.FreeVar) []ssa.Value {
	fn := fv.Parent()
	idx := slices.Index(fn.FreeVars, fv)
	parent := fn.Parent()
	if idx < 0 || parent == nil {
		return nil
	}
	var out []ssa.Value
	for _, b := range parent.Blocks {
		for _, in := range b.Instrs {
			if mc, ok := in.(*ssa.MakeClosure); ok && mc.Fn == fn && idx < len(mc.Bindings) {
				out = append(out, mc.Bindings[idx])
			}
		}
	}
	return out
}

// runs records the declared functions the value v of a call's function
// argument can be, when the syntax cannot name it (a parameter, a phi).
func (f *syncFlow) runs(site *types.Node, line int, v ssa.Value, op types.SyncOp) {
	switch v.(type) {
	case *ssa.Function, *ssa.MakeClosure:
		return // a function or a method value named where it is passed (syncops.go)
	}
	if site == nil {
		return
	}
	funcs := f.funcs(v, 0, map[ssa.Value]bool{})
	if len(funcs) == 0 {
		f.st.SyncNoOrigin++
		return
	}
	for _, fn := range funcs {
		if k := [3]string{site.ID, fn.ID, "runs " + string(op)}; !f.seen[k] {
			f.seen[k] = true
			f.st.SyncFollowed++
			f.out = append(f.out, concfacts.RunsLater{Site: site.ID, Func: fn.ID, Op: op, Line: line, File: site.FilePath,
				Certainty: concfacts.Approx(concfacts.ReasonAliasing)})
		}
	}
}

// callerArgs returns the values the callers pass for parameter p.
func (f *syncFlow) callerArgs(p *ssa.Parameter) []ssa.Value {
	fn := p.Parent()
	idx := -1
	for i, q := range fn.Params {
		if q == p {
			idx = i
		}
	}
	node := f.c.cg.Nodes[fn]
	if idx < 0 || node == nil {
		return nil
	}
	var out []ssa.Value
	for _, e := range node.In {
		if e.Site == nil {
			continue
		}
		common := e.Site.Common()
		switch {
		case common.IsInvoke() && idx == 0:
			out = append(out, common.Value)
		case common.IsInvoke() && idx-1 < len(common.Args):
			out = append(out, common.Args[idx-1])
		case !common.IsInvoke() && idx < len(common.Args):
			out = append(out, common.Args[idx])
		}
	}
	return out
}

// funcs returns the declared functions and methods the value v can be.
func (f *syncFlow) funcs(v ssa.Value, depth int, busy map[ssa.Value]bool) []*types.Node {
	if v == nil || depth > syncFlowDepth || busy[v] {
		return nil
	}
	busy[v] = true
	defer delete(busy, v)
	var out []*types.Node
	switch x := v.(type) {
	case *ssa.Function:
		if x.Parent() == nil {
			if n := f.c.nodeOf(x); n != nil {
				out = append(out, n)
			}
		}
	case *ssa.MakeClosure:
		// A method value (a bound wrapper) names its method; a literal is
		// code without a node of its own.
		if fn, ok := x.Fn.(*ssa.Function); ok && fn.Parent() == nil {
			if n := f.c.nodeOf(fn); n != nil {
				out = append(out, n)
			}
		}
	case *ssa.Parameter:
		for _, a := range f.callerArgs(x) {
			out = append(out, f.funcs(a, depth+1, busy)...)
		}
	case *ssa.Phi:
		for _, e := range x.Edges {
			out = append(out, f.funcs(e, depth, busy)...)
		}
	case *ssa.ChangeType:
		out = append(out, f.funcs(x.X, depth, busy)...)
	}
	return dedupNodes(out)
}

// dedupNodes returns ns without repeats, in order.
func dedupNodes(ns []*types.Node) []*types.Node {
	seen := map[string]bool{}
	out := ns[:0]
	for _, n := range ns {
		if !seen[n.ID] {
			seen[n.ID] = true
			out = append(out, n)
		}
	}
	return out
}
