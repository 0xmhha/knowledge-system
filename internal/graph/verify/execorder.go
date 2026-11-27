package verify

import (
	"fmt"
	"go/ast"
	"go/token"
	gotypes "go/types"
	"strings"

	"golang.org/x/tools/go/types/typeutil"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// execorder.go checks the execution-order facts of schema 1.44 against the
// syntax and go/types, sharing no code with the extraction:
//
//   - closes: a closer -> the field or package-level variable whose
//     channel close(x) closes, when x names one. The closer is the function
//     the close is written in, and the goroutine whose literal holds it.
//     A close of a channel reached any other way (a parameter, a call
//     result) is not decided, and its closer's edges are left out.
//   - operates_on: a call (or a receive) on a sync primitive -> the
//     declaration that holds it (for <-time.After(d), the site of the call
//     that makes the timer). The operation is read off the callee's
//     full name (go/types Func.FullName), "(*sync.WaitGroup).Add".
//   - runs_func: the declared function passed to sync.Once.Do,
//     time.AfterFunc or sync.WaitGroup.Go.
//
// runs_under_lock is checked with accessed_under_lock (underlock.go): the
// must-hold data flow over the control-flow graph records the sites as it
// records the fields.

// execFacts are the closes, operates_on and runs_func truth.
type execFacts struct {
	// closes are the expected (closer, channel) pairs; closers the closers
	// whose every close the check decides.
	closes, sync, runs map[[2]string]bool
	closers            map[string]bool
	undecidedCloser    map[string]bool
	// undecided are the "site|op" keys of operations on a parameter (or a
	// function run later held in a variable): not checked.
	undecided map[string]bool
	// unknown are the "site|op" keys of operations whose primitive no
	// declaration names: the graph marks their site (types.SyncUnknown).
	unknown map[string]bool
	// syncAlias are the ("store|line", holder) pairs of sync stores
	// (extents.go); syncUndecided the "store|line" keys left out.
	syncAlias     map[[2]string]bool
	syncUndecided map[string]bool
	// fieldCalls and fieldRecvs are calls through, and receives from, a
	// field ("site|op", field); cancelFields the fields of a plain function
	// type a cancel function is stored into; doneFields the fields a
	// context's Done is stored into, with the contexts' holders, or nil
	// when one is not decided (heldUses).
	fieldCalls, fieldRecvs [][2]string
	cancelFields           map[string]bool
	doneFields             map[string][]string
	doneUndecided          map[string]bool
}

func newExecFacts() *execFacts {
	return &execFacts{closes: map[[2]string]bool{}, sync: map[[2]string]bool{}, runs: map[[2]string]bool{},
		closers: map[string]bool{}, undecidedCloser: map[string]bool{}, undecided: map[string]bool{}, unknown: map[string]bool{},
		syncAlias: map[[2]string]bool{}, syncUndecided: map[string]bool{}, cancelFields: map[string]bool{}, doneFields: map[string][]string{},
		doneUndecided: map[string]bool{}}
}

// fullNameOps says what a call does, by the callee's full name.
var fullNameOps = map[string]types.SyncOp{
	"(*sync.WaitGroup).Add": types.SyncWaitGroupAdd, "(*sync.WaitGroup).Done": types.SyncWaitGroupDone,
	"(*sync.WaitGroup).Wait": types.SyncWaitGroupWait, "(*sync.WaitGroup).Go": types.SyncWaitGroupGo,
	"(*sync.Once).Do":   types.SyncOnceDo,
	"(*sync.Cond).Wait": types.SyncCondWait, "(*sync.Cond).Signal": types.SyncCondSignal, "(*sync.Cond).Broadcast": types.SyncCondBroadcast,
	"(*time.Timer).Stop": types.SyncTimerStop, "(*time.Timer).Reset": types.SyncTimerReset,
	"(*time.Ticker).Stop": types.SyncTimerStop, "(*time.Ticker).Reset": types.SyncTimerReset,
	"(context.Context).Done": types.SyncContextDone,
}

// atomicPrefixes are the operations of sync/atomic, by the start of a
// function's or method's name; the longer prefix first.
var atomicPrefixes = []struct {
	name string
	op   types.SyncOp
}{
	{"CompareAndSwap", types.SyncAtomicCAS}, {"Swap", types.SyncAtomicSwap}, {"Load", types.SyncAtomicLoad},
	{"Store", types.SyncAtomicStore}, {"Add", types.SyncAtomicAdd}, {"And", types.SyncAtomicAnd}, {"Or", types.SyncAtomicOr},
}

// Full names of the makers of a context with its cancel function, of
// timers, and of the timer channels no declaration holds.
var (
	contextMakerNames = map[string]bool{"context.WithCancel": true, "context.WithTimeout": true, "context.WithDeadline": true,
		"context.WithCancelCause": true, "context.WithTimeoutCause": true, "context.WithDeadlineCause": true}
	timerMakerNames = map[string]bool{"time.AfterFunc": true, "time.NewTimer": true, "time.NewTicker": true}
	timerChanNames  = map[string]bool{"time.After": true, "time.Tick": true}
	// runsLater: the index of the function argument a call runs later.
	runsLater = map[string]struct {
		arg int
		op  types.SyncOp
	}{
		"(*sync.Once).Do":      {0, types.SyncOnceDo},
		"(*sync.WaitGroup).Go": {0, types.SyncWaitGroupGo},
		"time.AfterFunc":       {1, types.SyncAfterFunc},
	}
)

// execOrder records the execution-order facts of one function declaration.
func (pf *programFacts) execOrder(info *gotypes.Info, d *ast.FuncDecl, rel string) {
	params := map[gotypes.Object]bool{} // parameters and results: no node holds them
	ast.Inspect(d, func(n ast.Node) bool {
		var ft *ast.FuncType
		switch x := n.(type) {
		case *ast.FuncDecl:
			ft = x.Type
			if x.Recv != nil {
				for _, f := range x.Recv.List {
					for _, id := range f.Names {
						params[info.Defs[id]] = true
					}
				}
			}
		case *ast.FuncLit:
			ft = x.Type
		}
		if ft != nil {
			for _, fl := range []*ast.FieldList{ft.Params, ft.Results} {
				if fl == nil {
					continue
				}
				for _, f := range fl.List {
					for _, id := range f.Names {
						params[info.Defs[id]] = true
					}
				}
			}
		}
		return true
	})
	// declKey names the holder of the primitive e: a declaration, the
	// site of a call of package context or time that makes it in place
	// (<-time.NewTimer(d).C), or a parameter (paramflow.go), through type
	// conversions; "" for none.
	declKey := func(e ast.Expr) string {
		e = stripConversions(info, e)
		switch k := pf.holder(info, e, params); k {
		case paramHolder:
			if pk := pf.paramKey(info, e); pk != "" {
				return paramHolder + ":" + pk
			}
			return k
		case "":
			return pf.source(info, rel, e)
		default:
			return k
		}
	}
	site := func(from, to token.Pos) string { return pf.span(rel, from, to) }
	addSync := func(s string, op types.SyncOp, decl string) {
		key := s + "|" + string(op)
		switch {
		case decl == "":
			pf.exec.unknown[key] = true
		case strings.HasPrefix(decl, paramHolder+":") && followedOps[op]:
			// The parameter's sources, decided once every call is known.
			pf.pflow.ops = append(pf.pflow.ops, paramOp{key: key, param: strings.TrimPrefix(decl, paramHolder+":")})
		case strings.HasPrefix(decl, paramHolder+":") && madeOps[op]:
			// A context or timer made into a parameter: held by the
			// fields and package-level variables the parameter is stored
			// into, all or none.
			holders, complete := pf.storesOf(info, d.Body, strings.TrimPrefix(decl, paramHolder+":"))
			if !complete || len(holders) == 0 {
				pf.exec.unknown[key] = true
				return
			}
			for _, h := range holders {
				pf.exec.sync[[2]string{key, h}] = true
			}
		case strings.HasPrefix(decl, paramHolder):
			pf.exec.undecided[key] = true
		default:
			pf.exec.sync[[2]string{key, decl}] = true
		}
	}
	fnKey := fmt.Sprintf("%s:%d:%s", rel, pf.fset.Position(d.Pos()).Line, d.Name.Name)
	var walk func(n ast.Node, goroutine string)
	walk = func(n ast.Node, goroutine string) {
		ast.Inspect(n, func(x ast.Node) bool {
			switch y := x.(type) {
			case *ast.GoStmt:
				if lit, ok := ast.Unparen(y.Call.Fun).(*ast.FuncLit); ok {
					walk(lit.Body, fmt.Sprintf("%s@%d", rel, pf.fset.Position(y.Pos()).Offset))
					for _, a := range y.Call.Args {
						walk(a, goroutine)
					}
					return false
				}
			case *ast.FuncLit:
				// Not a goroutine's own code: its closes are the function's.
				walk(y.Body, "")
				return false
			case *ast.AssignStmt:
				if len(y.Rhs) == 1 {
					pf.made(info, y.Rhs[0], y.Lhs, site, declKey, addSync)
				}
			case *ast.ValueSpec:
				if len(y.Values) == 1 {
					lhs := make([]ast.Expr, 0, len(y.Names))
					for _, id := range y.Names {
						lhs = append(lhs, id)
					}
					pf.made(info, y.Values[0], lhs, site, declKey, addSync)
				}
			case *ast.UnaryExpr:
				if y.Op == token.ARROW {
					pf.received(info, rel, y, site(y.Pos(), y.End()), declKey, addSync)
				}
			case *ast.CallExpr:
				if ch, ok := closeArg(info, y); ok {
					pf.closed(info, ch, fnKey, goroutine)
					return true
				}
				pf.called(info, y, site(y.Pos(), y.End()), declKey, addSync)
			}
			return true
		})
	}
	walk(d.Body, "")
}

// closeArg returns the operand of a call of the builtin close.
func closeArg(info *gotypes.Info, call *ast.CallExpr) (ast.Expr, bool) {
	id, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok || id.Name != "close" || len(call.Args) != 1 {
		return nil, false
	}
	_, builtin := info.Uses[id].(*gotypes.Builtin)
	return call.Args[0], builtin
}

// closed records a close of ch by the function fnKey and, inside a go
// statement's literal, by that goroutine.
func (pf *programFacts) closed(info *gotypes.Info, ch ast.Expr, fnKey, goroutine string) {
	var origin string
	switch x := ast.Unparen(ch).(type) {
	case *ast.SelectorExpr:
		if s := info.Selections[x]; s != nil && s.Kind() == gotypes.FieldVal {
			if v, ok := s.Obj().(*gotypes.Var); ok {
				origin = pf.key(v.Origin())
			}
		} else if s == nil {
			origin = pf.packageVar(info.Uses[x.Sel])
		}
	case *ast.Ident:
		origin = pf.packageVar(info.Uses[x])
	}
	closers := []string{fnKey}
	if goroutine != "" {
		closers = append(closers, goroutine)
	}
	for _, c := range closers {
		if origin == "" {
			pf.exec.undecidedCloser[c] = true
			continue
		}
		pf.exec.closers[c] = true
		pf.exec.closes[[2]string{c, origin}] = true
	}
}

// packageVar is the key of a package-level variable, "" for anything else.
func (pf *programFacts) packageVar(obj gotypes.Object) string {
	v, ok := obj.(*gotypes.Var)
	if !ok || v.Pkg() == nil || v.Parent() != v.Pkg().Scope() {
		return ""
	}
	return pf.key(v)
}

// holder returns the key of the declaration that holds the primitive e
// names (a field, through an embedded field the embedded one; a variable),
// "" when none with a node does (a parameter, a call result).
func (pf *programFacts) holder(info *gotypes.Info, e ast.Expr, params map[gotypes.Object]bool) string {
	switch x := ast.Unparen(e).(type) {
	case *ast.UnaryExpr:
		if x.Op == token.AND {
			return pf.holder(info, x.X, params)
		}
	case *ast.StarExpr:
		return pf.holder(info, x.X, params)
	case *ast.SelectorExpr:
		s := info.Selections[x]
		switch {
		case s == nil:
			return pf.packageVar(info.Uses[x.Sel])
		case s.Kind() == gotypes.FieldVal:
			if v, ok := s.Obj().(*gotypes.Var); ok {
				return pf.key(v.Origin())
			}
		case s.Kind() == gotypes.MethodVal && len(s.Index()) > 1:
			// x.M() promoted from an embedded field: that field.
			return pf.key(embeddedField(s))
		}
	case *ast.Ident:
		v, ok := info.ObjectOf(x).(*gotypes.Var)
		if !ok || v.IsField() {
			return ""
		}
		if params[v] {
			return paramHolder
		}
		return pf.key(v)
	}
	return ""
}

// paramHolder stands for a primitive (or a function) a parameter holds.
const paramHolder = "param"

// madeOps are the operations that make a primitive into a variable; made
// into a parameter (no node), they name where it is stored. followedOps are
// the operations on a primitive the build follows from a parameter to its
// sources (a call of a cancel function held in a parameter is not one).
var (
	madeOps     = map[types.SyncOp]bool{types.SyncContextMake: true, types.SyncCancelMake: true, types.SyncTimerMake: true}
	followedOps = map[types.SyncOp]bool{
		types.SyncWaitGroupAdd: true, types.SyncWaitGroupDone: true, types.SyncWaitGroupWait: true, types.SyncWaitGroupGo: true,
		types.SyncOnceDo: true, types.SyncCondWait: true, types.SyncCondSignal: true, types.SyncCondBroadcast: true,
		types.SyncTimerStop: true, types.SyncTimerReset: true, types.SyncTimerRecv: true,
		types.SyncContextDone: true, types.SyncDoneRecv: true,
		types.SyncAtomicAdd: true, types.SyncAtomicLoad: true, types.SyncAtomicStore: true, types.SyncAtomicSwap: true,
		types.SyncAtomicCAS: true, types.SyncAtomicAnd: true, types.SyncAtomicOr: true,
	}
)

// embeddedField returns the field a promoted method is selected through.
func embeddedField(s *gotypes.Selection) gotypes.Object {
	t := s.Recv()
	var f *gotypes.Var
	for _, i := range s.Index()[:len(s.Index())-1] {
		st, ok := derefStruct(t)
		if !ok || i >= st.NumFields() {
			return nil
		}
		f = st.Field(i)
		t = f.Type()
	}
	if f == nil {
		return nil
	}
	return f.Origin()
}

// calleeOf returns the full name of the function or method call calls, and
// the expression of the primitive it operates on (the receiver; for a
// promoted method the selector, which names the embedded field).
func calleeOf(info *gotypes.Info, call *ast.CallExpr) (string, ast.Expr, *gotypes.Func) {
	fn, ok := typeutil.Callee(info, call).(*gotypes.Func)
	if !ok {
		return "", nil, nil
	}
	sel, isSel := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !isSel {
		return fn.FullName(), nil, fn
	}
	if s := info.Selections[sel]; s != nil && s.Kind() == gotypes.MethodVal {
		if len(s.Index()) > 1 {
			return fn.FullName(), sel, fn
		}
		return fn.FullName(), sel.X, fn
	}
	return fn.FullName(), nil, fn
}

// atomicOf returns the operation of a sync/atomic function or method by its
// full name.
func atomicOf(name string) (types.SyncOp, bool) {
	if !strings.HasPrefix(name, "sync/atomic.") && !strings.HasPrefix(name, "(*sync/atomic.") && !strings.HasPrefix(name, "(sync/atomic.") {
		return "", false
	}
	short := name[strings.LastIndex(name, ".")+1:]
	for _, a := range atomicPrefixes {
		if strings.HasPrefix(short, a.name) {
			return a.op, true
		}
	}
	return "", false
}

// called records a call on a primitive and the function it runs later.
func (pf *programFacts) called(info *gotypes.Info, call *ast.CallExpr, site string, declKey func(ast.Expr) string,
	addSync func(string, types.SyncOp, string)) {
	name, recv, fn := calleeOf(info, call)
	if fn == nil {
		// A call of a cancel function held in a variable or field.
		if named, ok := gotypes.Unalias(info.TypeOf(call.Fun)).(*gotypes.Named); ok && named.Obj().Pkg() != nil &&
			named.Obj().Pkg().Path() == "context" && (named.Obj().Name() == "CancelFunc" || named.Obj().Name() == "CancelCauseFunc") {
			addSync(site, types.SyncCancel, declKey(call.Fun))
		} else if sel, isSel := ast.Unparen(call.Fun).(*ast.SelectorExpr); isSel {
			// A call through a field of a plain function type: a cancel
			// when a cancel function is stored into the field (decided
			// once every store is known, heldUses).
			if s := info.Selections[sel]; s != nil && s.Kind() == gotypes.FieldVal {
				pf.exec.fieldCalls = append(pf.exec.fieldCalls, [2]string{site + "|" + string(types.SyncCancel), pf.key(s.Obj().(*gotypes.Var).Origin())})
			}
		}
		return
	}
	if op, ok := fullNameOps[name]; ok && recv != nil {
		addSync(site, op, declKey(recv))
	} else if op, ok := atomicOf(name); ok {
		if recv != nil {
			addSync(site, op, declKey(recv))
		} else if len(call.Args) > 0 {
			addSync(site, op, declKey(call.Args[0]))
		}
	}
	if r, ok := runsLater[name]; ok && r.arg < len(call.Args) {
		var obj gotypes.Object
		switch a := ast.Unparen(call.Args[r.arg]).(type) {
		case *ast.Ident:
			obj = info.Uses[a]
		case *ast.SelectorExpr:
			obj = info.Uses[a.Sel]
		}
		if f, ok := obj.(*gotypes.Func); ok {
			if k := pf.key(f.Origin()); k != "" {
				pf.exec.runs[[2]string{site + "|" + string(r.op), k}] = true
			}
		} else if v, ok := obj.(*gotypes.Var); ok && !v.IsField() {
			// A function in a variable or parameter: the build follows the
			// value flow; the check leaves the site out.
			pf.exec.undecided[site+"|"+string(r.op)] = true
		}
	}
}

// made records a context or timer a maker's results are assigned to.
func (pf *programFacts) made(info *gotypes.Info, rhs ast.Expr, lhs []ast.Expr, site func(token.Pos, token.Pos) string,
	declKey func(ast.Expr) string, addSync func(string, types.SyncOp, string)) {
	call, ok := ast.Unparen(rhs).(*ast.CallExpr)
	if !ok {
		return
	}
	name, _, fn := calleeOf(info, call)
	if fn == nil {
		return
	}
	s := site(call.Pos(), call.End())
	switch {
	case contextMakerNames[name] && len(lhs) == 2:
		addSync(s, types.SyncContextMake, declKey(lhs[0]))
		addSync(s, types.SyncCancelMake, declKey(lhs[1]))
	case timerMakerNames[name] && len(lhs) == 1:
		addSync(s, types.SyncTimerMake, declKey(lhs[0]))
	}
}

// received records a receive on a timer's channel or a context's Done.
func (pf *programFacts) received(info *gotypes.Info, rel string, u *ast.UnaryExpr, site string, declKey func(ast.Expr) string,
	addSync func(string, types.SyncOp, string)) {
	switch ch := ast.Unparen(u.X).(type) {
	case *ast.SelectorExpr:
		s := info.Selections[ch]
		if s != nil && s.Kind() == gotypes.FieldVal && ch.Sel.Name != "C" {
			// A receive from a field a context's Done may be stored into
			// (heldUses).
			pf.exec.fieldRecvs = append(pf.exec.fieldRecvs, [2]string{site + "|" + string(types.SyncDoneRecv), pf.key(s.Obj().(*gotypes.Var).Origin())})
		}
		if s == nil || s.Kind() != gotypes.FieldVal || ch.Sel.Name != "C" {
			return
		}
		if named, ok := gotypes.Unalias(derefType(s.Recv())).(*gotypes.Named); ok && named.Obj().Pkg() != nil &&
			named.Obj().Pkg().Path() == "time" && (named.Obj().Name() == "Timer" || named.Obj().Name() == "Ticker") {
			addSync(site, types.SyncTimerRecv, declKey(ch.X))
		}
	case *ast.CallExpr:
		name, recv, fn := calleeOf(info, ch)
		switch {
		case fn == nil:
		case timerChanNames[name]:
			// The timer the call makes: its site.
			addSync(site, types.SyncTimerRecv, pf.span(rel, ch.Pos(), ch.End()))
		case fullNameOps[name] == types.SyncContextDone && recv != nil:
			addSync(site, types.SyncDoneRecv, declKey(recv))
		}
	}
}

// derefType is *t's element for a pointer, else t.
func derefType(t gotypes.Type) gotypes.Type {
	if p, ok := gotypes.Unalias(t).(*gotypes.Pointer); ok {
		return p.Elem()
	}
	return t
}

// compareSyncUnknown compares the operations the graph marks on their site
// as of an unknown primitive (types.Node.SyncUnknown) with the truth's,
// leaving the undecided ones out on both sides. A pair is (site|op,
// "unknown"); the reason is not compared.
func compareSyncUnknown(m *execFacts, idx nodeIndex, samples int) UnderLockStats {
	const unknown = "unknown"
	truth := map[[2]string]bool{}
	for k := range m.unknown {
		truth[[2]string{k, unknown}] = true
	}
	graph := map[[2]string]bool{}
	for _, n := range idx.byID {
		for _, u := range n.SyncUnknown {
			graph[[2]string{placeKey(n) + "|" + string(u.Op), unknown}] = true
		}
	}
	return comparePairs(truth, graph, func(k [2]string) bool { return m.undecided[k[0]] }, samples)
}

// compareExecOrder compares the closes, operates_on and runs_func edges of
// g with the truth.
func compareExecOrder(m *execFacts, idx nodeIndex, g GraphReader, samples int) (closes, syncOps, runs UnderLockStats, err error) {
	key := placeKey
	read := func(t types.EdgeType, keep func(types.Edge) bool, withKind bool) (map[[2]string]bool, error) {
		edges, err := g.QueryEdgesByType(string(t))
		if err != nil {
			return nil, fmt.Errorf("verify: read %s: %w", t, err)
		}
		out := map[[2]string]bool{}
		for _, e := range edges {
			if keep != nil && !keep(e) {
				continue
			}
			src := key(idx.byID[e.Src])
			if withKind {
				src += "|" + e.DispatchKind
			}
			out[[2]string{src, key(idx.byID[e.Dst])}] = true
		}
		return out, nil
	}
	// The channel linker's edges name the origin (field, global); the
	// syntax's edge to the close's own site carries no kind.
	cl, err := read(types.EdgeCloses, func(e types.Edge) bool { return e.DispatchKind == "field" || e.DispatchKind == "global" }, false)
	if err != nil {
		return closes, syncOps, runs, err
	}
	so, err := read(types.EdgeOperatesOn, nil, true)
	if err != nil {
		return closes, syncOps, runs, err
	}
	rf, err := read(types.EdgeRunsFunc, nil, true)
	if err != nil {
		return closes, syncOps, runs, err
	}
	// A closer with a close the check does not decide is left out on both
	// sides.
	undecided := func(k [2]string) bool { return !m.closers[k[0]] || m.undecidedCloser[k[0]] }
	decided := map[[2]string]bool{}
	for k := range m.closes {
		if !undecided(k) {
			decided[k] = true
		}
	}
	closes = comparePairs(decided, cl, undecided, samples)
	skip := func(k [2]string) bool { return m.undecided[k[0]] }
	return closes, comparePairs(m.sync, so, skip, samples), comparePairs(m.runs, rf, skip, samples), nil
}
