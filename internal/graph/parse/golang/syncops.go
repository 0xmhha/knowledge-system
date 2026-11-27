package golang

import (
	"go/ast"
	"go/token"
	gotypes "go/types"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/types/typeutil"

	"github.com/0xmhha/knowledge-system/internal/graph/concfacts"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// syncops.go records the operations on sync primitives (schema 1.44) by one
// rule: a call (or a receive) on a sync primitive is linked from its
// CallSite to the declaration that holds the primitive (operates_on), with
// what it does (types.SyncOp). The declaration is the field, the
// package-level variable or the local variable the receiver or the first
// argument names (s.wg.Add(1), atomic.AddInt64(&s.n, 1)); through an
// embedded field, the embedded field. A primitive held by a parameter, a
// map element or a call result has no declaration with a node and is
// counted (SyncStats.NoDeclaration), and the SSA pass (sync_flow.go) follows
// it to its sources: syncSyntax tells that pass which sites the syntax could
// not name and which declaration each value assigned to a local variable
// went to.
//
// The primitives: sync.WaitGroup, sync.Once, sync.Cond, the functions and
// types of sync/atomic, a context and its cancel function (made together by
// context.WithCancel and its kin, assigned to two names), and timers
// (time.AfterFunc, NewTimer, NewTicker and the channel of time.After and
// time.Tick, whose holder is the site of the call that makes the timer). A
// timer made where its channel is received, <-time.NewTimer(d).C, is held by
// no declaration either: its holder is the site of that call too. A
// mutex has acquires_lock and releases_lock instead.
//
// A function that one of these calls runs later (sync.Once.Do,
// time.AfterFunc, sync.WaitGroup.Go) is linked from the site too
// (runs_func) when it is a declared function or method; the calls of a
// literal passed there are told by literal_passed_to.

// Import paths of the packages whose types and functions are primitives.
const (
	syncPkgPath    = "sync"
	atomicPkgPath  = "sync/atomic"
	contextPkgPath = "context"
	timePkgPath    = "time"
)

// syncMethods says what a method of a primitive type does, by
// "<import path>.<type>.<method>".
var syncMethods = map[string]types.SyncOp{
	"sync.WaitGroup.Add": types.SyncWaitGroupAdd, "sync.WaitGroup.Done": types.SyncWaitGroupDone,
	"sync.WaitGroup.Wait": types.SyncWaitGroupWait, "sync.WaitGroup.Go": types.SyncWaitGroupGo,
	"sync.Once.Do":   types.SyncOnceDo,
	"sync.Cond.Wait": types.SyncCondWait, "sync.Cond.Signal": types.SyncCondSignal, "sync.Cond.Broadcast": types.SyncCondBroadcast,
	"time.Timer.Stop": types.SyncTimerStop, "time.Timer.Reset": types.SyncTimerReset,
	"time.Ticker.Stop": types.SyncTimerStop, "time.Ticker.Reset": types.SyncTimerReset,
	"context.Context.Done": types.SyncContextDone,
}

// atomicOps says what a function or method of sync/atomic does, by the
// prefix of its name (AddInt64, Int64.Add, CompareAndSwapPointer, ...).
var atomicOps = []struct {
	prefix string
	op     types.SyncOp
}{
	{"CompareAndSwap", types.SyncAtomicCAS}, {"Load", types.SyncAtomicLoad}, {"Store", types.SyncAtomicStore},
	{"Add", types.SyncAtomicAdd}, {"Swap", types.SyncAtomicSwap}, {"And", types.SyncAtomicAnd}, {"Or", types.SyncAtomicOr},
}

// contextMakers are the functions of package context that return a context
// and the function that cancels it.
var contextMakers = map[string]bool{"WithCancel": true, "WithTimeout": true, "WithDeadline": true,
	"WithCancelCause": true, "WithTimeoutCause": true, "WithDeadlineCause": true}

// timerMakers are the functions of package time that return a timer.
var timerMakers = map[string]bool{"AfterFunc": true, "NewTimer": true, "NewTicker": true}

// timerChans are the functions of package time that return a timer's
// channel, which no declaration holds.
var timerChans = map[string]bool{"After": true, "Tick": true}

// cancelFuncTypes are the types of package context a cancel function has.
var cancelFuncTypes = map[string]bool{"CancelFunc": true, "CancelCauseFunc": true}

// timerChanField is the field of time.Timer and time.Ticker that the
// runtime sends on.
const timerChanField = "C"

// afterFuncArg is the index of the function time.AfterFunc runs.
const afterFuncArg = 1

// SyncStats counts what SyncFacts found and what it could not name.
type SyncStats struct {
	Uses     int // operates_on facts
	RunsFunc int // runs_func facts
	// NoDeclaration counts operations on a primitive no declaration with a
	// node holds (a parameter, a map element, a call result).
	NoDeclaration int
	// NoSite counts operations whose call or receive has no CallSite.
	NoSite int
}

// syncWalker finds the nodes the facts of one piece of code name.
type syncWalker struct {
	info   *gotypes.Info
	fset   *token.FileSet
	sites  *siteIndex
	fields fieldIndex
	locals localIndex
	decls  declIndex
	values map[string]*types.Node // package-level variables and External nodes by canonical id
	st     *SyncStats
	out    concfacts.Facts
	syn    syncSyntaxFacts
}

// syncSyntaxFacts is what the syntax pass hands the SSA pass (sync_flow.go).
type syncSyntaxFacts struct {
	// unnamed are the operations whose primitive no declaration with a node
	// holds, by unnamedKey (site and operation): only those are followed
	// through the value flow, so a site the syntax names keeps its one edge.
	unnamed map[string]bool
	// assigned are the local variables (with a node) that the value an
	// expression makes is assigned to, by the expression's place (see
	// valuePlace) and the index of the result.
	assigned map[valueAt]*types.Node
}

// valueAt is the place of an expression that makes a value, and which of
// its results is meant: the file and byte offset SSA gives the value (the
// opening parenthesis of a call, the opening brace of a composite literal).
// File and offset, not token.Pos: a package and its test variant read one
// file at different positions.
type valueAt struct {
	file   string
	offset int
	result int
}

// unnamedKey names an operation the syntax could not name.
func unnamedKey(site string, op types.SyncOp) string { return site + "|" + string(op) }

// SyncFacts returns the operations on sync primitives in the loaded
// packages (operates_on, runs_func), proved from the calls and go/types.
func SyncFacts(pkgs []*packages.Package, nodes []types.Node) (concfacts.Facts, SyncStats) {
	facts, st, _ := syncSyntax(pkgs, nodes)
	return facts, st
}

// syncSyntax is SyncFacts with what the SSA pass needs from the syntax.
func syncSyntax(pkgs []*packages.Package, nodes []types.Node) (concfacts.Facts, SyncStats, syncSyntaxFacts) {
	var st SyncStats
	syn := syncSyntaxFacts{unnamed: map[string]bool{}, assigned: map[valueAt]*types.Node{}}
	if len(pkgs) == 0 {
		return nil, st, syn
	}
	fset := fsetOf(pkgs)
	values := map[string]*types.Node{}
	for i := range nodes {
		n := &nodes[i]
		if n.Type == types.NodeVariable && n.CanonicalID != "" {
			if _, ok := values[n.CanonicalID]; !ok {
				values[n.CanonicalID] = n
			}
		}
	}
	w := &syncWalker{fset: fset, sites: newSiteIndex(nodes), fields: newFieldIndex(fset, nodes), locals: newLocalIndex(fset, nodes),
		decls: newDeclIndex(nodes), values: values, st: &st, syn: syn}
	forEachOwnedCode(pkgs, nodes, func(c ownedCode) {
		if !c.part.runs() || c.tok == token.CONST {
			return
		}
		w.info = c.info()
		ast.Inspect(c.root, w.visit)
	})
	return w.out, st, syn
}

// visit records the facts of one syntax node.
func (w *syncWalker) visit(n ast.Node) bool {
	switch x := n.(type) {
	case *ast.AssignStmt:
		if len(x.Rhs) == 1 {
			w.assigned(x.Rhs[0], x.Lhs)
		}
		w.noteAssigned(x.Lhs, x.Rhs)
	case *ast.ValueSpec:
		lhs := make([]ast.Expr, len(x.Names))
		for i, id := range x.Names {
			lhs[i] = id
		}
		if len(x.Values) == 1 {
			w.assigned(x.Values[0], lhs)
		}
		w.noteAssigned(lhs, x.Values)
	case *ast.UnaryExpr:
		if x.Op == token.ARROW {
			w.receive(x)
		}
	case *ast.CallExpr:
		w.call(x)
	}
	return true
}

// assigned records what a maker of contexts or timers makes: the names its
// results are assigned to.
func (w *syncWalker) assigned(rhs ast.Expr, lhs []ast.Expr) {
	call, ok := ast.Unparen(rhs).(*ast.CallExpr)
	if !ok {
		return
	}
	fn := w.pkgFunc(call)
	switch {
	case fn == nil:
	case fn.Pkg().Path() == contextPkgPath && contextMakers[fn.Name()] && len(lhs) == 2:
		w.use(call, lhs[0], types.SyncContextMake)
		w.use(call, lhs[1], types.SyncCancelMake)
	case fn.Pkg().Path() == timePkgPath && timerMakers[fn.Name()] && len(lhs) == 1:
		w.use(call, lhs[0], types.SyncTimerMake)
	}
}

// noteAssigned records, for the SSA pass, the local variables of lhs that
// take the value a call or a composite literal of rhs makes: one result
// each when the two lists pair up, every result of the one call otherwise.
func (w *syncWalker) noteAssigned(lhs, rhs []ast.Expr) {
	for i, l := range lhs {
		id, ok := ast.Unparen(l).(*ast.Ident)
		if !ok {
			continue
		}
		v, ok := w.info.ObjectOf(id).(*gotypes.Var)
		if !ok || v.IsField() || v.Pkg() == nil || v.Parent() == v.Pkg().Scope() {
			continue
		}
		n := w.locals.node(v)
		if n == nil {
			continue
		}
		var e ast.Expr
		result := 0
		switch {
		case len(rhs) == len(lhs):
			e = rhs[i]
		case len(rhs) == 1:
			e, result = rhs[0], i
		default:
			continue
		}
		if pos, ok := valuePlace(e); ok {
			p := w.fset.Position(pos)
			w.syn.assigned[valueAt{file: p.Filename, offset: p.Offset, result: result}] = n
		}
	}
}

// valuePlace returns the position SSA gives the value e makes: the opening
// parenthesis of a call (new(T) included), the opening brace of a
// composite literal or of the one whose address e takes.
func valuePlace(e ast.Expr) (token.Pos, bool) {
	switch x := ast.Unparen(e).(type) {
	case *ast.CallExpr:
		return x.Lparen, true
	case *ast.CompositeLit:
		return x.Lbrace, true
	case *ast.UnaryExpr:
		if lit, ok := ast.Unparen(x.X).(*ast.CompositeLit); ok && x.Op == token.AND {
			return lit.Lbrace, true
		}
	}
	return token.NoPos, false
}

// receive records a receive on a timer's channel or on a context's Done.
func (w *syncWalker) receive(u *ast.UnaryExpr) {
	switch ch := ast.Unparen(u.X).(type) {
	case *ast.SelectorExpr:
		// t.C of a time.Timer or time.Ticker.
		s := w.info.Selections[ch]
		if s == nil || s.Kind() != gotypes.FieldVal || ch.Sel.Name != timerChanField {
			return
		}
		if pkg, typ := namedOf(s.Recv()); pkg == timePkgPath && (typ == "Timer" || typ == "Ticker") {
			w.siteUse(u.Pos(), u.End(), w.timerOf(ch.X), types.SyncTimerRecv)
		}
	case *ast.CallExpr:
		if fn := w.pkgFunc(ch); fn != nil && fn.Pkg().Path() == timePkgPath && timerChans[fn.Name()] {
			// No declaration holds the channel: the call that makes the
			// timer does (its site).
			w.siteUse(u.Pos(), u.End(), w.sites.span(w.fset.Position(ch.Pos()), w.fset.Position(ch.End())), types.SyncTimerRecv)
			return
		}
		if fn, recv := w.method(ch); fn != nil && syncMethods[methodKey(fn)] == types.SyncContextDone {
			w.siteUse(u.Pos(), u.End(), w.declOf(recv), types.SyncDoneRecv)
		}
	}
}

// timerOf returns the holder of the timer e is: the declaration e names, or
// for a timer made in place (time.NewTimer(d) in <-time.NewTimer(d).C) the
// site of the call that makes it, as for the channel of time.After.
func (w *syncWalker) timerOf(e ast.Expr) *types.Node {
	call, ok := ast.Unparen(e).(*ast.CallExpr)
	if !ok {
		return w.declOf(e)
	}
	if fn := w.pkgFunc(call); fn != nil && fn.Pkg().Path() == timePkgPath && timerMakers[fn.Name()] {
		return w.sites.span(w.fset.Position(call.Pos()), w.fset.Position(call.End()))
	}
	return nil
}

// call records a method call on a primitive, a sync/atomic function on the
// address of a field or variable, a call of a cancel function and the
// function a call runs later.
func (w *syncWalker) call(call *ast.CallExpr) {
	if fn, recv := w.method(call); fn != nil {
		op, ok := syncMethods[methodKey(fn)]
		if !ok && fn.Pkg().Path() == atomicPkgPath {
			op, ok = atomicOp(fn.Name())
		}
		if !ok {
			return
		}
		w.use(call, recv, op)
		if (op == types.SyncOnceDo || op == types.SyncWaitGroupGo) && len(call.Args) == 1 {
			w.runs(call, call.Args[0], op)
		}
		return
	}
	if fn := w.pkgFunc(call); fn != nil {
		switch {
		case fn.Pkg().Path() == atomicPkgPath && len(call.Args) > 0:
			if op, ok := atomicOp(fn.Name()); ok {
				w.use(call, call.Args[0], op)
			}
		case fn.Pkg().Path() == timePkgPath && fn.Name() == "AfterFunc" && len(call.Args) > afterFuncArg:
			w.runs(call, call.Args[afterFuncArg], types.SyncAfterFunc)
		}
		return
	}
	// A call of a variable or field of a cancel function type.
	if t := w.info.TypeOf(call.Fun); t != nil {
		if pkg, typ := namedOf(t); pkg == contextPkgPath && cancelFuncTypes[typ] {
			w.use(call, call.Fun, types.SyncCancel)
		}
	}
}

// use records op of the call at its site on the primitive e names.
func (w *syncWalker) use(call *ast.CallExpr, e ast.Expr, op types.SyncOp) {
	w.siteUse(call.Pos(), call.End(), w.declOf(e), op)
}

// siteUse records op of the site spanning [from, to) on the declaration
// decl.
// decl nil: no declaration with a node holds the primitive; the site is
// left to the SSA pass.
func (w *syncWalker) siteUse(from, to token.Pos, decl *types.Node, op types.SyncOp) {
	p := w.fset.Position(from)
	site := w.sites.span(p, w.fset.Position(to))
	if decl == nil {
		w.st.NoDeclaration++
		if site != nil {
			w.syn.unnamed[unnamedKey(site.ID, op)] = true
		}
		return
	}
	if site == nil {
		w.st.NoSite++
		return
	}
	w.st.Uses++
	w.out = append(w.out, concfacts.SyncUse{Site: site.ID, Primitive: decl.ID, Op: op, Line: p.Line, File: site.FilePath,
		Certainty: concfacts.Proof})
}

// runs records the declared function arg names as run later by call.
func (w *syncWalker) runs(call *ast.CallExpr, arg ast.Expr, op types.SyncOp) {
	var obj gotypes.Object
	switch a := ast.Unparen(arg).(type) {
	case *ast.Ident:
		obj = w.info.Uses[a]
	case *ast.SelectorExpr:
		obj = w.info.Uses[a.Sel]
	}
	fn, ok := obj.(*gotypes.Func)
	if !ok {
		return
	}
	target := w.decls.at(goCanonicalID(fn), "", w.fset.Position(fn.Pos()))
	p := w.fset.Position(call.Pos())
	site := w.sites.span(p, w.fset.Position(call.End()))
	if target == nil || site == nil {
		return
	}
	w.st.RunsFunc++
	w.out = append(w.out, concfacts.RunsLater{Site: site.ID, Func: target.ID, Op: op, Line: p.Line, File: site.FilePath,
		Certainty: concfacts.Proof})
}

// method returns the method a call calls and the receiver expression, or
// nil for a call that is not a method call.
func (w *syncWalker) method(call *ast.CallExpr) (*gotypes.Func, ast.Expr) {
	sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil, nil
	}
	s := w.info.Selections[sel]
	if s == nil || s.Kind() != gotypes.MethodVal {
		return nil, nil
	}
	fn, ok := s.Obj().(*gotypes.Func)
	if !ok || fn.Pkg() == nil {
		return nil, nil
	}
	if len(s.Index()) > 1 {
		// Promoted through an embedded field: the primitive is that field.
		return fn, sel
	}
	return fn, sel.X
}

// pkgFunc returns the package-level function of sync, sync/atomic, context
// or time that call calls, or nil.
func (w *syncWalker) pkgFunc(call *ast.CallExpr) *gotypes.Func {
	fn, ok := typeutil.Callee(w.info, call).(*gotypes.Func)
	if !ok || fn.Pkg() == nil {
		return nil
	}
	if sig, ok := fn.Type().(*gotypes.Signature); !ok || sig.Recv() != nil {
		return nil
	}
	switch fn.Pkg().Path() {
	case syncPkgPath, atomicPkgPath, contextPkgPath, timePkgPath:
		return fn
	}
	return nil
}

// declOf returns the node of the declaration e names: a field (through an
// embedded field, the embedded one), a package-level variable or a local
// variable, after & and *; nil for anything else.
func (w *syncWalker) declOf(e ast.Expr) *types.Node {
	for {
		switch x := ast.Unparen(e).(type) {
		case *ast.UnaryExpr:
			if x.Op != token.AND {
				return nil
			}
			e = x.X
			continue
		case *ast.StarExpr:
			e = x.X
			continue
		case *ast.SelectorExpr:
			if s := w.info.Selections[x]; s != nil {
				if s.Kind() == gotypes.MethodVal {
					return w.embedded(s)
				}
				if v, ok := s.Obj().(*gotypes.Var); ok && s.Kind() == gotypes.FieldVal {
					return w.fields.node(v.Origin())
				}
				return nil
			}
			// A qualified identifier: pkg.V.
			return w.variable(w.info.ObjectOf(x.Sel))
		case *ast.Ident:
			return w.variable(w.info.ObjectOf(x))
		}
		return nil
	}
}

// embedded returns the node of the embedded field a promoted method s is
// selected through: the last field of its path.
func (w *syncWalker) embedded(s *gotypes.Selection) *types.Node {
	t := s.Recv()
	var f *gotypes.Var
	for _, i := range s.Index()[:len(s.Index())-1] {
		st, ok := derefStructType(t)
		if !ok || i >= st.NumFields() {
			return nil
		}
		f = st.Field(i)
		t = f.Type()
	}
	if f == nil {
		return nil
	}
	return w.fields.node(f.Origin())
}

// variable returns the node of a package-level or local variable.
func (w *syncWalker) variable(obj gotypes.Object) *types.Node {
	v, ok := obj.(*gotypes.Var)
	if !ok || v.IsField() || v.Pkg() == nil {
		return nil
	}
	if v.Parent() == v.Pkg().Scope() {
		return w.values[goCanonicalID(v)]
	}
	return w.locals.node(v)
}

// methodKey names a method "<import path>.<type>.<method>".
func methodKey(fn *gotypes.Func) string {
	sig, ok := fn.Type().(*gotypes.Signature)
	if !ok || sig.Recv() == nil {
		return ""
	}
	pkg, typ := namedOf(sig.Recv().Type())
	return pkg + "." + typ + "." + fn.Name()
}

// namedOf returns the import path and name of the named type t (or *t)
// denotes, or "".
func namedOf(t gotypes.Type) (string, string) {
	if p, ok := gotypes.Unalias(t).(*gotypes.Pointer); ok {
		t = p.Elem()
	}
	n, ok := gotypes.Unalias(t).(*gotypes.Named)
	if !ok || n.Obj().Pkg() == nil {
		return "", ""
	}
	return n.Obj().Pkg().Path(), n.Obj().Name()
}

// atomicOp returns the operation of a sync/atomic function or method name.
func atomicOp(name string) (types.SyncOp, bool) {
	for _, a := range atomicOps {
		if strings.HasPrefix(name, a.prefix) {
			return a.op, true
		}
	}
	return "", false
}

// derefStructType returns the struct t (or *t) denotes.
func derefStructType(t gotypes.Type) (*gotypes.Struct, bool) {
	if p, ok := gotypes.Unalias(t).(*gotypes.Pointer); ok {
		t = p.Elem()
	}
	st, ok := t.Underlying().(*gotypes.Struct)
	return st, ok
}
