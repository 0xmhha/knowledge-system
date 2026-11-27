package verify

import (
	"fmt"
	"go/ast"
	"go/token"
	gotypes "go/types"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/cfg"
	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/types/typeutil"
)

// underlock.go is the truth side of accessed_under_lock: the struct fields
// each function uses while it holds a sync.Mutex or sync.RWMutex, read off
// the control-flow graph of every function body (golang.org/x/tools/go/cfg)
// by a must-hold data flow. It shares no code with the extractor, which
// walks the syntax tree instead.
//
// A use of a field is a selector that names it, or an element of a struct
// literal, which sets it (T{F: v}, T{v}).
//
// A mutex is held at a point when every path from the function's entry to
// that point locks it (Lock or RLock) and does not unlock it (Unlock or
// RUnlock) after. A deferred call does not run where it is written, so
// `defer mu.Unlock()` keeps the mutex held to the end. A function literal
// is a function of its own: it starts with nothing held, unless it is called
// where it is written (`func() { ... }()`), and then it starts with what is
// held at the call. A mutex is named by its declaration: the variable or the
// field, also an embedded one (s.Lock() with struct{ sync.Mutex }), or the
// one a getter returns (s.guard().Lock(), returnedMutex). The receiver of a
// Lock or Unlock call is a use of the mutex field under the other mutexes
// held, not under the mutex itself.

// syncPackage is the import path of the package that declares the mutexes.
const syncPackage = "sync"

// The methods that take and release a mutex.
const (
	truthLock    = "Lock"
	truthRLock   = "RLock"
	truthUnlock  = "Unlock"
	truthRUnlock = "RUnlock"
)

// heldSet is the set of mutexes held at a point, by the key of their
// declaration (declKey: file, line and name, the same in a package and in its
// test variant, whose type checks give one field two objects); nil stands
// for "not reached yet", the top of the must-hold lattice.
type heldSet map[string]bool

// meet returns the mutexes both a and b hold; nil is the identity.
func meet(a, b heldSet) heldSet {
	if a == nil {
		return copyHeld(b)
	}
	if b == nil {
		return copyHeld(a)
	}
	out := heldSet{}
	for m := range a {
		if b[m] {
			out[m] = true
		}
	}
	return out
}

func copyHeld(a heldSet) heldSet {
	if a == nil {
		return nil
	}
	out := make(heldSet, len(a))
	for m := range a {
		out[m] = true
	}
	return out
}

func sameHeld(a, b heldSet) bool {
	if (a == nil) != (b == nil) || len(a) != len(b) {
		return false
	}
	for m := range a {
		if !b[m] {
			return false
		}
	}
	return true
}

// lockTruth collects the (field, mutex) pairs of one file.
type lockTruth struct {
	info  *gotypes.Info
	fset  *token.FileSet
	root  string
	scope map[string]bool
	out   map[[2]string]bool
	// sites, when set, collects the (site, mutex) pairs of the calls, sends
	// and receives made while a mutex is held (runs_under_lock).
	sites map[[2]string]bool
	// only, when set, keeps only the pairs of these mutexes: a callee walked
	// for propagation records the uses under its caller's mutexes alone.
	only heldSet
	// chanLocks names the channels used as locks (chanlock.go).
	chanLocks chanLockNames
	// comms are the cases of selects: their lock effect is their path's.
	comms map[ast.Node]bool
	// funcs finds the declaration a static call runs; calls collects, when
	// set, each call of a declared function made while a mutex is held.
	funcs map[token.Position]truthFunc
	calls *[]truthCall
}

// truthFunc is a declared function and the type information of its file.
type truthFunc struct {
	decl *ast.FuncDecl
	info *gotypes.Info
	fset *token.FileSet
}

// truthCall is a call of a declared function made while held is held.
type truthCall struct {
	callee truthFunc
	held   heldSet
}

// callAt records call as a truth call when a mutex is held and its static
// callee is a declared function.
func (lt *lockTruth) callAt(call *ast.CallExpr, held heldSet) {
	if lt.calls == nil || len(held) == 0 || lt.funcs == nil {
		return
	}
	fn := typeutil.StaticCallee(lt.info, call)
	if fn == nil {
		return
	}
	p := lt.fset.Position(fn.Origin().Pos())
	if f, ok := lt.funcs[token.Position{Filename: p.Filename, Offset: p.Offset}]; ok {
		*lt.calls = append(*lt.calls, truthCall{callee: f, held: copyHeld(held)})
	}
}

// function analyses one function body that starts with entry held (nil:
// nothing held).
func (lt *lockTruth) function(body *ast.BlockStmt, entry heldSet) {
	if body == nil {
		return
	}
	g := cfg.New(body, lt.mayReturn)
	preds := make([][]int32, len(g.Blocks))
	for _, b := range g.Blocks {
		for _, s := range b.Succs {
			preds[s.Index] = append(preds[s.Index], b.Index)
		}
	}
	if entry == nil {
		entry = heldSet{}
	}
	// The cfg evaluates every case of a select before it branches; a case
	// that takes a channel lock takes it on its own path only, so its
	// effect is applied where its body starts (caseEntry), not with the
	// select's other cases.
	ast.Inspect(body, func(n ast.Node) bool {
		if cc, ok := n.(*ast.CommClause); ok && cc.Comm != nil {
			if lt.comms == nil {
				lt.comms = map[ast.Node]bool{}
			}
			lt.comms[cc.Comm] = true
		}
		return true
	})
	caseEntry := func(b *cfg.Block, h heldSet) heldSet {
		cc, ok := b.Stmt.(*ast.CommClause)
		if !ok || b.Kind != cfg.KindSelectCaseBody || cc.Comm == nil {
			return h
		}
		next := copyHeld(h)
		ast.Inspect(cc.Comm, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.UnaryExpr:
				if m := lt.chanLock(x); m != "" {
					lt.chanEffect(m, true)(next)
				}
			case *ast.SendStmt:
				if m := lt.chanLockOf(x.Chan); m != "" {
					lt.chanEffect(m, false)(next)
				}
			}
			return true
		})
		return next
	}
	out := make([]heldSet, len(g.Blocks))
	// start is what every path into b holds; nil while no path reached it.
	start := func(b *cfg.Block) heldSet {
		var h heldSet
		if b.Index == 0 {
			h = copyHeld(entry)
		}
		for _, p := range preds[b.Index] {
			if out[p] != nil {
				h = meet(h, out[p])
			}
		}
		return h
	}
	// Iterate to the fixed point: a block's set only shrinks once reached.
	for changed := true; changed; {
		changed = false
		for _, b := range g.Blocks {
			h := start(b)
			if !b.Live || h == nil {
				continue
			}
			h = caseEntry(b, h)
			for _, n := range b.Nodes {
				h = lt.node(n, h, false)
			}
			if !sameHeld(h, out[b.Index]) {
				out[b.Index] = h
				changed = true
			}
		}
	}
	// One recording pass with the final states.
	var exit heldSet
	for _, b := range g.Blocks {
		h := start(b)
		if !b.Live || h == nil {
			continue
		}
		h = caseEntry(b, h)
		for _, n := range b.Nodes {
			h = lt.node(n, h, true)
		}
		if lt.returns(b) {
			exit = meet(exit, h)
		}
	}
	if exit != nil {
		lt.deferred(body, exit)
	}
}

// returns reports whether the function returns at the end of block b: b
// has no successor and does not end in a panic.
func (lt *lockTruth) returns(b *cfg.Block) bool {
	if len(b.Succs) != 0 {
		return false
	}
	if n := len(b.Nodes); n > 0 {
		if es, ok := b.Nodes[n-1].(*ast.ExprStmt); ok {
			if c, ok := ast.Unparen(es.X).(*ast.CallExpr); ok && !lt.mayReturn(c) {
				return false
			}
		}
	}
	return true
}

// deferred records the calls of the defer statements of body (not of the
// literals in it) as run with what is held when it returns, exit, less what
// a defer written after them releases: defers run last written first. A
// deferred literal's call is a site like any call, and its body runs then,
// starting with what is held at that moment; what it releases anywhere in
// its body (an Unlock, a channel lock's token given back, a close of a
// channel lock) is not held by the defers that run after it. The extractor
// walks the literal's body for what it leaves held instead; the two agree
// unless a literal releases and takes back a mutex it was entered with.
func (lt *lockTruth) deferred(body *ast.BlockStmt, exit heldSet) {
	var defers []*ast.CallExpr
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.DeferStmt:
			defers = append(defers, x.Call)
		}
		return true
	})
	held := copyHeld(exit)
	for i := len(defers) - 1; i >= 0; i-- {
		c := defers[i]
		lt.callSite(c, held)
		if lit, ok := ast.Unparen(c.Fun).(*ast.FuncLit); ok {
			lt.function(lit.Body, copyHeld(held))
			for _, m := range lt.releasedIn(lit.Body) {
				delete(held, m)
			}
			continue
		}
		if m, method, ok := lt.lockCall(c); ok && (method == truthUnlock || method == truthRUnlock) {
			delete(held, m)
		}
		if ch, ok := closeArg(lt.info, c); ok {
			if m := lt.chanLockOf(ch); m != "" {
				delete(held, m)
			}
		}
	}
}

// releasedIn returns the locks body releases anywhere outside the literals
// inside it: the mutexes it unlocks, and the channel locks whose token it
// gives back or that it closes.
func (lt *lockTruth) releasedIn(body *ast.BlockStmt) []string {
	out := lt.unlockedIn(body)
	ast.Inspect(body, func(c ast.Node) bool {
		switch x := c.(type) {
		case *ast.FuncLit:
			return false
		case *ast.SendStmt:
			if m := lt.chanLockOf(x.Chan); m != "" && !lt.chanLocks.inverted[m] {
				out = append(out, m)
			}
		case *ast.UnaryExpr:
			if m := lt.chanLock(x); m != "" && lt.chanLocks.inverted[m] {
				out = append(out, m)
			}
		case *ast.CallExpr:
			if ch, ok := closeArg(lt.info, x); ok {
				if m := lt.chanLockOf(ch); m != "" {
					out = append(out, m)
				}
			}
		}
		return true
	})
	return out
}

// callSite records call as a site under held when it calls code: a
// conversion or a builtin (but close, a channel operation) has no callee.
// A close under a lock is a site under it, as any call. Sites are keyed by
// the call's span, so new(T).M(), two calls from one byte, stay two: the
// call of new is not a site, the call of M is.
func (lt *lockTruth) callSite(call *ast.CallExpr, held heldSet) {
	if tv, ok := lt.info.Types[call.Fun]; ok {
		if tv.IsType() {
			return
		}
		if id, isID := ast.Unparen(call.Fun).(*ast.Ident); tv.IsBuiltin() && (!isID || id.Name != "close") {
			return
		}
	}
	lt.site(call.Pos(), call.End(), held)
}

// mayReturn is false for a call of the builtin panic.
func (lt *lockTruth) mayReturn(call *ast.CallExpr) bool {
	id, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok {
		return true
	}
	_, builtin := lt.info.Uses[id].(*gotypes.Builtin)
	return !builtin || id.Name != "panic"
}

// node applies one CFG node to held: the accesses it makes are recorded
// under held (when record is set), then its lock calls change held. A range
// statement stands for its header only; its body has blocks of its own.
func (lt *lockTruth) node(n ast.Node, held heldSet, record bool) heldSet {
	var parts []ast.Node
	switch x := n.(type) {
	case *ast.RangeStmt:
		for _, e := range []ast.Expr{x.Key, x.Value, x.X} {
			if e != nil {
				parts = append(parts, e)
			}
		}
	case *ast.DeferStmt:
		// The call runs at the end: only its function value and arguments
		// are evaluated here, and a literal is a function of its own. The
		// receiver path of a deferred Unlock is evaluated here, with what
		// is held here; the mutex itself is used under the others.
		if m, _, ok := lt.lockCall(x.Call); ok {
			recv := ast.Unparen(x.Call.Fun.(*ast.SelectorExpr).X)
			if sel, isSel := recv.(*ast.SelectorExpr); isSel && record {
				others := copyHeld(held)
				delete(others, m)
				lt.access(sel, others)
				lt.exprs([]ast.Expr{sel.X}, held, record, true)
			} else if record {
				lt.exprs([]ast.Expr{recv}, held, record, true)
			}
			lt.exprs(x.Call.Args, held, record, true)
			return held
		}
		lt.exprs(append([]ast.Expr{x.Call.Fun}, x.Call.Args...), held, record, true)
		return held
	case *ast.GoStmt:
		lt.exprs(append([]ast.Expr{x.Call.Fun}, x.Call.Args...), held, record, true)
		return held
	default:
		parts = []ast.Node{n}
	}
	receivers := map[ast.Expr]bool{}
	var effects []func(heldSet)
	inComm := lt.comms[n]
	for _, p := range parts {
		ast.Inspect(p, func(c ast.Node) bool {
			switch x := c.(type) {
			case *ast.FuncLit:
				if record {
					lt.function(x.Body, nil)
				}
				return false
			case *ast.CallExpr:
				if lit, ok := ast.Unparen(x.Fun).(*ast.FuncLit); ok {
					// Called where it is written: it runs now, with what
					// is held here. Literals are analysed in the recording
					// pass only, with the final state of their call.
					if record {
						lt.site(x.Pos(), x.End(), held)
						lt.function(lit.Body, held)
					}
					// What the literal unlocks, here or deferred, is not
					// held after it.
					for _, m := range lt.unlockedIn(lit.Body) {
						effects = append(effects, func(h heldSet) { delete(h, m) })
					}
					for _, a := range x.Args {
						lt.exprs([]ast.Expr{a}, held, record, false)
					}
					return false
				}
				if record {
					lt.callAt(x, held)
					lt.callSite(x, held)
				}
				if id, ok := ast.Unparen(x.Fun).(*ast.Ident); ok && id.Name == "close" && len(x.Args) == 1 {
					if _, builtin := lt.info.ObjectOf(id).(*gotypes.Builtin); builtin {
						if m := lt.chanLockOf(x.Args[0]); m != "" {
							effects = append(effects, func(h heldSet) { delete(h, m) })
						}
					}
				}
				if m, method, ok := lt.lockCall(x); ok {
					recv := ast.Unparen(x.Fun.(*ast.SelectorExpr).X)
					receivers[recv] = true
					if sel, isSel := recv.(*ast.SelectorExpr); isSel && record {
						others := copyHeld(held)
						delete(others, m)
						lt.access(sel, others)
					}
					effects = append(effects, func(h heldSet) {
						if method == truthLock || method == truthRLock {
							h[m] = true
						} else {
							delete(h, m)
						}
					})
				}
			case *ast.UnaryExpr:
				if x.Op == token.ARROW && record {
					lt.site(x.Pos(), x.End(), held)
				}
				// <-lock takes a channel lock's token (returns it, inverted).
				if m := lt.chanLock(x); m != "" && !inComm {
					effects = append(effects, lt.chanEffect(m, true))
				}
			case *ast.SendStmt:
				if record {
					lt.site(x.Pos(), x.End(), held)
				}
				if m := lt.chanLockOf(x.Chan); m != "" && !inComm {
					effects = append(effects, lt.chanEffect(m, false))
				}
			case *ast.SelectorExpr:
				if record && !receivers[x] {
					lt.access(x, held)
				}
			case *ast.CompositeLit:
				if record {
					lt.literal(x, held)
				}
			}
			return true
		})
	}
	if len(effects) == 0 {
		return held
	}
	next := copyHeld(held)
	for _, e := range effects {
		e(next)
	}
	return next
}

// unlockedIn returns the mutexes body unlocks anywhere outside the function
// literals inside it.
func (lt *lockTruth) unlockedIn(body *ast.BlockStmt) []string {
	var out []string
	ast.Inspect(body, func(c ast.Node) bool {
		switch x := c.(type) {
		case *ast.FuncLit:
			return false
		case *ast.CallExpr:
			if m, method, ok := lt.lockCall(x); ok && (method == truthUnlock || method == truthRUnlock) {
				out = append(out, m)
			}
		}
		return true
	})
	return out
}

// exprs records the accesses in es under held; with literalsApart a function
// literal among them starts with nothing held.
func (lt *lockTruth) exprs(es []ast.Expr, held heldSet, record, literalsApart bool) {
	for _, e := range es {
		ast.Inspect(e, func(c ast.Node) bool {
			switch x := c.(type) {
			case *ast.FuncLit:
				switch {
				case !record:
				case literalsApart:
					lt.function(x.Body, nil)
				default:
					lt.function(x.Body, held)
				}
				return false
			case *ast.CallExpr:
				if record {
					lt.callAt(x, held)
					lt.callSite(x, held)
				}
			case *ast.UnaryExpr:
				if x.Op == token.ARROW && record {
					lt.site(x.Pos(), x.End(), held)
				}
			case *ast.SelectorExpr:
				if record {
					lt.access(x, held)
				}
			case *ast.CompositeLit:
				if record {
					lt.literal(x, held)
				}
			}
			return true
		})
	}
}

// site records the call, send or receive spanning [from, to) as made
// while every mutex of held is held (of lt.only, when set).
func (lt *lockTruth) site(from, to token.Pos, held heldSet) {
	if lt.sites == nil || len(held) == 0 {
		return
	}
	p := lt.fset.Position(from)
	rel, ok := relPath(lt.root, p.Filename)
	if !ok || !lt.scope[rel] {
		return
	}
	k := fmt.Sprintf("%s@%d-%d", rel, p.Offset, lt.fset.Position(to).Offset)
	for mu := range held {
		if mu != "" && (lt.only == nil || lt.only[mu]) {
			lt.sites[[2]string{k, mu}] = true
		}
	}
}

// literal records the fields a struct literal sets as used under held: the
// field each key names, or the field at each element's place when the
// literal has no keys.
func (lt *lockTruth) literal(lit *ast.CompositeLit, held heldSet) {
	st, ok := derefStruct(lt.info.TypeOf(lit))
	if !ok || len(held) == 0 {
		return
	}
	for i, elt := range lit.Elts {
		var f *gotypes.Var
		if kv, isKV := elt.(*ast.KeyValueExpr); isKV {
			if id, isID := kv.Key.(*ast.Ident); isID {
				f, _ = lt.info.ObjectOf(id).(*gotypes.Var)
			}
		} else if i < st.NumFields() {
			f = st.Field(i)
		}
		if f != nil && f.IsField() {
			lt.record(f.Origin(), held)
		}
	}
}

// access records the field sel names as used under every mutex of held.
func (lt *lockTruth) access(sel *ast.SelectorExpr, held heldSet) {
	if len(held) == 0 {
		return
	}
	v, ok := lt.info.ObjectOf(sel.Sel).(*gotypes.Var)
	if !ok || !v.IsField() {
		return
	}
	lt.record(v.Origin(), held)
}

// record notes the field v as used under every mutex of held.
func (lt *lockTruth) record(v *gotypes.Var, held heldSet) {
	field := lt.declKey(v)
	if field == "" {
		return
	}
	for mu := range held {
		// No field is under its own mutex (a getter returning &s.mu).
		// Nor is the field that holds a channel lock under that lock.
		if mu != "" && mu != field && lt.chanLocks.lockOf(field) != mu && (lt.only == nil || lt.only[mu]) {
			lt.out[[2]string{field, mu}] = true
		}
	}
}

// chanEffect is what a receive (recv) or a send on the channel lock m does
// to what is held: a receive takes the token and a send returns it, the
// other way round for an inverted lock (chanlock.go).
func (lt *lockTruth) chanEffect(m string, recv bool) func(heldSet) {
	if recv != lt.chanLocks.inverted[m] {
		return func(h heldSet) { h[m] = true }
	}
	return func(h heldSet) { delete(h, m) }
}

// chanLock returns the lock a receive u takes, "" when u is not one.
func (lt *lockTruth) chanLock(u *ast.UnaryExpr) string {
	if u.Op != token.ARROW {
		return ""
	}
	return lt.chanLockOf(u.X)
}

// chanLockOf returns the lock the channel expression e names: a variable
// or field that names a channel lock (findChanLocks), "" otherwise.
func (lt *lockTruth) chanLockOf(e ast.Expr) string {
	if len(lt.chanLocks.names) == 0 {
		return ""
	}
	var obj gotypes.Object
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		obj = lt.info.ObjectOf(x)
	case *ast.SelectorExpr:
		if s := lt.info.Selections[x]; s != nil && s.Kind() == gotypes.FieldVal {
			if v, ok := s.Obj().(*gotypes.Var); ok {
				obj = v.Origin()
			}
		}
	}
	if obj == nil {
		return ""
	}
	return lt.chanLocks.lockOf(lt.declKey(obj))
}

// lockCall reports whether call is mu.Lock(), RLock(), Unlock() or
// RUnlock() of a sync mutex, and which mutex declaration mu is.
func (lt *lockTruth) lockCall(call *ast.CallExpr) (string, string, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) != 0 {
		return "", "", false
	}
	switch sel.Sel.Name {
	case truthLock, truthRLock, truthUnlock, truthRUnlock:
	default:
		return "", "", false
	}
	fn, ok := lt.info.ObjectOf(sel.Sel).(*gotypes.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != syncPackage {
		return "", "", false
	}
	m := lt.mutexOf(sel)
	if m == nil {
		return "", "", false
	}
	return lt.declKey(m), sel.Sel.Name, true
}

// mutexOf returns the declaration of the mutex whose method sel selects:
// the embedded field the method is promoted through, or the variable or
// field the receiver names.
func (lt *lockTruth) mutexOf(sel *ast.SelectorExpr) gotypes.Object {
	if s := lt.info.Selections[sel]; s != nil && len(s.Index()) > 1 {
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
		return f
	}
	x := ast.Unparen(sel.X)
	for {
		switch y := x.(type) {
		case *ast.StarExpr:
			x = ast.Unparen(y.X)
			continue
		case *ast.UnaryExpr:
			if y.Op == token.AND {
				x = ast.Unparen(y.X)
				continue
			}
		}
		break
	}
	switch y := x.(type) {
	case *ast.Ident:
		return lt.info.ObjectOf(y)
	case *ast.SelectorExpr:
		return lt.info.ObjectOf(y.Sel)
	case *ast.CallExpr:
		return lt.returnedMutex(y)
	}
	return nil
}

// returnedMutex returns the mutex a getter returns (s.guard().Lock() with
// func (s *S) guard() *sync.Mutex { return &s.mu }): the declared function
// call runs, when each of its return statements returns the same variable
// or field, or its address. Getters are followed one call deep.
func (lt *lockTruth) returnedMutex(call *ast.CallExpr) gotypes.Object {
	fn := typeutil.StaticCallee(lt.info, call)
	if fn == nil || lt.funcs == nil {
		return nil
	}
	p := lt.fset.Position(fn.Origin().Pos())
	f, ok := lt.funcs[token.Position{Filename: p.Filename, Offset: p.Offset}]
	if !ok {
		return nil
	}
	var got gotypes.Object
	same := true
	ast.Inspect(f.decl.Body, func(n ast.Node) bool {
		if _, lit := n.(*ast.FuncLit); lit {
			return false
		}
		r, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}
		var obj gotypes.Object
		if len(r.Results) == 1 {
			e := ast.Unparen(r.Results[0])
			if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.AND {
				e = ast.Unparen(u.X)
			}
			switch y := e.(type) {
			case *ast.Ident:
				obj = f.info.ObjectOf(y)
			case *ast.SelectorExpr:
				obj = f.info.ObjectOf(y.Sel)
			}
		}
		if obj == nil || got != nil && got != obj {
			same = false
		}
		got = obj
		return true
	})
	if !same {
		return nil
	}
	return got
}

func derefStruct(t gotypes.Type) (*gotypes.Struct, bool) {
	if t == nil {
		return nil, false
	}
	t = gotypes.Unalias(t)
	if p, ok := t.Underlying().(*gotypes.Pointer); ok {
		t = p.Elem()
	}
	st, ok := t.Underlying().(*gotypes.Struct)
	return st, ok
}

// declKey names a declaration by its file, line and name; "" for one
// outside the covered files.
func (lt *lockTruth) declKey(o gotypes.Object) string {
	if o == nil || !o.Pos().IsValid() {
		return ""
	}
	p := lt.fset.Position(o.Pos())
	rel, ok := relPath(lt.root, p.Filename)
	if !ok || !lt.scope[rel] {
		return ""
	}
	return fmt.Sprintf("%s:%d:%s", filepath.ToSlash(rel), p.Line, o.Name())
}

// propagationDepth bounds how many calls deep propagateTruth follows.
const propagationDepth = 5

// truthFuncs indexes the function declarations of pkgs by the position of
// their name (file and offset: a package and its test variant read one
// file), with the type information of their package.
func truthFuncs(pkgs []*packages.Package) map[token.Position]truthFunc {
	out := map[token.Position]truthFunc{}
	for _, pkg := range pkgs {
		if pkg.TypesInfo == nil || pkg.Fset == nil {
			continue
		}
		for _, f := range pkg.Syntax {
			for _, d := range f.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if !ok || fd.Body == nil {
					continue
				}
				p := pkg.Fset.Position(fd.Name.Pos())
				k := token.Position{Filename: p.Filename, Offset: p.Offset}
				if _, dup := out[k]; !dup {
					out[k] = truthFunc{decl: fd, info: pkg.TypesInfo, fset: pkg.Fset}
				}
			}
		}
	}
	return out
}

// propagateTruth walks the calls made while a mutex is held into the
// declared functions they run: each callee is analysed with what is held
// at the call as its entry state, its uses under those mutexes are
// propagated pairs, and its own calls made while one of them is still held
// are followed, up to propagationDepth calls deep. Pairs the direct
// analysis already has are left out.
func propagateTruth(t *Truth, calls []truthCall, funcs map[token.Position]truthFunc, root string, scope map[string]bool, chanLocks chanLockNames) {
	type item struct {
		call  truthCall
		depth int
	}
	work := make([]item, 0, len(calls))
	for _, c := range calls {
		work = append(work, item{c, 1})
	}
	found := map[[2]string]bool{}
	foundSites := map[[2]string]bool{}
	done := map[string]bool{}
	for len(work) > 0 {
		it := work[0]
		work = work[1:]
		keys := make([]string, 0, len(it.call.held))
		for k := range it.call.held {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		key := fmt.Sprintf("%p|%s", it.call.callee.decl, strings.Join(keys, ","))
		if done[key] {
			continue
		}
		done[key] = true
		var next []truthCall
		lt := &lockTruth{info: it.call.callee.info, fset: it.call.callee.fset, root: root, scope: scope, chanLocks: chanLocks,
			out: found, sites: foundSites, only: it.call.held, funcs: funcs, calls: &next}
		lt.function(it.call.callee.decl.Body, it.call.held)
		if it.depth >= propagationDepth {
			continue
		}
		for _, n := range next {
			kept := heldSet{}
			for m := range n.held {
				if it.call.held[m] {
					kept[m] = true
				}
			}
			if len(kept) > 0 {
				work = append(work, item{truthCall{callee: n.callee, held: kept}, it.depth + 1})
			}
		}
	}
	for k := range found {
		if !t.UnderLock[k] {
			t.UnderLockPropagated[k] = true
		}
	}
	for k := range foundSites {
		if !t.SiteLocks[k] {
			t.SiteLocksPropagated[k] = true
		}
	}
}
