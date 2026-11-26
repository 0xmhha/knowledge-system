package verify

import (
	"fmt"
	"go/ast"
	"go/token"
	gotypes "go/types"
	"path/filepath"

	"golang.org/x/tools/go/cfg"
)

// underlock.go is the truth side of accessed_under_lock: the struct fields
// each function uses while it holds a sync.Mutex or sync.RWMutex, read off
// the control-flow graph of every function body (golang.org/x/tools/go/cfg)
// by a must-hold data flow. It shares no code with the extractor, which
// walks the syntax tree instead.
//
// A mutex is held at a point when every path from the function's entry to
// that point locks it (Lock or RLock) and does not unlock it (Unlock or
// RUnlock) after. A deferred call does not run where it is written, so
// `defer mu.Unlock()` keeps the mutex held to the end. A function literal
// is a function of its own: it starts with nothing held, unless it is called
// where it is written (`func() { ... }()`), and then it starts with what is
// held at the call. A mutex is named by its declaration: the variable or the
// field, also an embedded one (s.Lock() with struct{ sync.Mutex }); a mutex
// reached through a call (s.guard().Lock()) names no declaration and is not
// followed. The receiver of a Lock or Unlock call is a use of the mutex
// field under the other mutexes held, not under the mutex itself.

// syncPackage is the import path of the package that declares the mutexes.
const syncPackage = "sync"

// The methods that take and release a mutex.
const (
	truthLock    = "Lock"
	truthRLock   = "RLock"
	truthUnlock  = "Unlock"
	truthRUnlock = "RUnlock"
)

// heldSet is the set of mutexes held at a point, by declaration; nil stands
// for "not reached yet", the top of the must-hold lattice.
type heldSet map[gotypes.Object]bool

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
	for _, b := range g.Blocks {
		h := start(b)
		if !b.Live || h == nil {
			continue
		}
		for _, n := range b.Nodes {
			h = lt.node(n, h, true)
		}
	}
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
		// receiver of a deferred Unlock is the mutex, not a use.
		if _, _, ok := lt.lockCall(x.Call); ok {
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
			case *ast.SelectorExpr:
				if record && !receivers[x] {
					lt.access(x, held)
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
func (lt *lockTruth) unlockedIn(body *ast.BlockStmt) []gotypes.Object {
	var out []gotypes.Object
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
			case *ast.SelectorExpr:
				if record {
					lt.access(x, held)
				}
			}
			return true
		})
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
	field := lt.declKey(v.Origin())
	if field == "" {
		return
	}
	for m := range held {
		if mu := lt.declKey(m); mu != "" {
			lt.out[[2]string{field, mu}] = true
		}
	}
}

// lockCall reports whether call is mu.Lock(), RLock(), Unlock() or
// RUnlock() of a sync mutex, and which mutex declaration mu is.
func (lt *lockTruth) lockCall(call *ast.CallExpr) (gotypes.Object, string, bool) {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || len(call.Args) != 0 {
		return nil, "", false
	}
	switch sel.Sel.Name {
	case truthLock, truthRLock, truthUnlock, truthRUnlock:
	default:
		return nil, "", false
	}
	fn, ok := lt.info.ObjectOf(sel.Sel).(*gotypes.Func)
	if !ok || fn.Pkg() == nil || fn.Pkg().Path() != syncPackage {
		return nil, "", false
	}
	m := lt.mutexOf(sel)
	return m, sel.Sel.Name, m != nil
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
	}
	return nil
}

func derefStruct(t gotypes.Type) (*gotypes.Struct, bool) {
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
