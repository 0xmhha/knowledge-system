package golang

import (
	"go/ast"
	"go/token"
	gotypes "go/types"
	"maps"
	"slices"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// lock_held.go decides which mutexes a function holds where it uses a field,
// for accessed_under_lock (lock_edges.go). The rule:
//
//	A field use is under mutex L when L is held at that point of the
//	function: every path from the function's start to the use locks L
//	(Lock or RLock) and does not unlock it (Unlock or RUnlock) after.
//
// A field use is what reads_field and writes_field count (fieldUsesAt): a
// selector that names the field and an element of a struct literal.
//
// It is read off the syntax tree, statement by statement, the paths of a
// branch met at its end (a mutex is held after an if, a switch or a select
// only when every branch that goes on holds it; a branch that returns, panics
// or jumps away does not count), and a loop's state found by repeating its
// body until the state at its head stops changing. `defer L.Unlock()` runs at
// the end, so L stays held to the end of the function. A function literal is
// a function of its own and starts with nothing held, so a lock it takes does
// not cover the code around it and the reverse; a literal called where it is
// written (`func() { ... }()`) runs there and starts with what is held at the
// call, and after it only what it keeps holding to its end is held.
//
// The receiver of a Lock or Unlock call is a use of the mutex field under
// the other mutexes held, not under itself: `a.mu.Lock(); a.other.Lock()`
// records other under mu, the order the two are taken in.
//
// Undecided code gets no edge: a function with a goto, or with a break or
// continue whose target is not a loop, switch or select around it, is left
// out whole and counted (LockStats.UndecidedFunctions). A lock taken through
// a helper that is passed the mutex (lockAll(&s.mu)) is not seen, so no use
// is taken for being under it.
//
// The rule used to be "any field used in a function that locks a mutex
// anywhere is under every mutex the function locks": on go-stablenet 545 of
// 1,761 accessed_under_lock edges named a use the source does not hold the
// mutex at (a field read before Lock, after Unlock, or in another branch).

// heldMutexes is the set of Mutex nodes held at a point, by node id.
type heldMutexes map[string]*types.Node

func (h heldMutexes) clone() heldMutexes {
	out := make(heldMutexes, len(h))
	for id, n := range h {
		out[id] = n
	}
	return out
}

func (h heldMutexes) equal(o heldMutexes) bool {
	if len(h) != len(o) {
		return false
	}
	for id := range h {
		if _, ok := o[id]; !ok {
			return false
		}
	}
	return true
}

// meetHeld returns what every state of states holds, and false when states
// is empty: no path reaches the point.
func meetHeld(states []heldMutexes) (heldMutexes, bool) {
	if len(states) == 0 {
		return nil, false
	}
	out := states[0].clone()
	for _, s := range states[1:] {
		for id := range out {
			if _, ok := s[id]; !ok {
				delete(out, id)
			}
		}
	}
	return out, true
}

// jumpTarget is a loop, switch or select a break or continue can leave,
// with the states that reach it that way.
type jumpTarget struct {
	label     string
	loop      bool
	breaks    []heldMutexes
	continues []heldMutexes
}

// lockFlow walks one function body (and the literals inside it) for the
// accessed_under_lock edges.
type lockFlow struct {
	info *gotypes.Info
	// mutexOf returns the Mutex node a Lock or Unlock receiver denotes.
	mutexOf func(recv ast.Expr) *types.Node
	// fieldOf returns the Field node of a struct field.
	fieldOf func(f *gotypes.Var) *types.Node
	// chanLocks are the channel lock operations by position (chanlock.go).
	chanLocks ChanLocks
	// uses are the (field, mutex) pairs found; undecided says the function
	// holds code the rule cannot decide, and its uses are dropped.
	uses      [][2]*types.Node
	undecided bool
	// calleeOf, when set, returns the declared function a call runs (a
	// static callee with source); calls records each such call made while
	// a mutex is held, with what is held there, for propagation.
	calleeOf func(call *ast.CallExpr) (funcSource, bool)
	calls    []heldCall
}

// heldCall is a call of a declared function made while held is held.
type heldCall struct {
	callee funcSource
	held   heldMutexes
}

// funcWalk is the state of one function or literal being walked.
type funcWalk struct {
	flow    *lockFlow
	targets []*jumpTarget
	returns []heldMutexes
	// deferredUnlocks are the mutexes a deferred call unlocks at the end.
	deferredUnlocks map[string]bool
	// record says whether uses are recorded: off while a loop's state is
	// still being found, on for the pass with the final state.
	record bool
}

// body walks a function body that starts with entry held and returns what
// it holds at its end: after its last statement or at a return, with its
// deferred unlocks applied; ok is false when it never ends.
func (f *lockFlow) body(b *ast.BlockStmt, entry heldMutexes, record bool) (heldMutexes, bool) {
	if b == nil {
		return entry, true
	}
	w := &funcWalk{flow: f, deferredUnlocks: map[string]bool{}, record: record}
	out, live := w.stmts(b.List, entry.clone())
	ends := w.returns
	if live {
		ends = append(ends, out)
	}
	end, ok := meetHeld(ends)
	for id := range w.deferredUnlocks {
		delete(end, id)
	}
	return end, ok
}

func (w *funcWalk) stmts(list []ast.Stmt, held heldMutexes) (heldMutexes, bool) {
	for _, s := range list {
		var live bool
		held, live = w.stmt(s, held, "")
		if !live {
			return held, false
		}
	}
	return held, true
}

// stmt applies one statement to held; label is the label written on it.
func (w *funcWalk) stmt(s ast.Stmt, held heldMutexes, label string) (heldMutexes, bool) {
	switch x := s.(type) {
	case *ast.LabeledStmt:
		return w.stmt(x.Stmt, held, x.Label.Name)
	case *ast.ExprStmt:
		if u, ok := ast.Unparen(x.X).(*ast.UnaryExpr); ok && u.Op == token.ARROW {
			return w.chanLock(u.OpPos, w.exprs(x.X, held)), true
		}
		if call, ok := ast.Unparen(x.X).(*ast.CallExpr); ok {
			if next, ok := w.lockCall(call, held); ok {
				return next, true
			}
			if _, ok := w.flow.chanLocks[call.Lparen]; ok {
				return w.chanLock(call.Lparen, w.exprs(x.X, held)), true
			}
			if w.isPanic(call) {
				w.exprs(call, held)
				return held, false
			}
		}
		return w.exprs(x.X, held), true
	case *ast.SendStmt:
		return w.chanLock(x.Arrow, w.exprs(x, held)), true
	case *ast.AssignStmt:
		// v := <-ch and v, ok := <-ch take a lock's token.
		after := w.exprs(x, held)
		if len(x.Rhs) == 1 {
			if u, ok := ast.Unparen(x.Rhs[0]).(*ast.UnaryExpr); ok && u.Op == token.ARROW {
				after = w.chanLock(u.OpPos, after)
			}
		}
		return after, true
	case *ast.IncDecStmt, *ast.DeclStmt:
		return w.exprs(x, held), true
	case *ast.GoStmt:
		w.callApart(x.Call, held)
		return held, true
	case *ast.DeferStmt:
		if method, ok := lockMethodName(x.Call); ok && (method == lockMethodUnlock || method == lockMethodRUnlock) {
			if mu := w.flow.mutexOf(x.Call.Fun.(*ast.SelectorExpr).X); mu != nil {
				w.deferredUnlocks[mu.ID] = true
			}
		}
		w.callApart(x.Call, held)
		return held, true
	case *ast.ReturnStmt:
		for _, r := range x.Results {
			held = w.exprs(r, held)
		}
		w.returns = append(w.returns, held.clone())
		return held, false
	case *ast.BlockStmt:
		return w.stmts(x.List, held)
	case *ast.IfStmt:
		return w.ifStmt(x, held)
	case *ast.ForStmt:
		return w.forStmt(x, held, label)
	case *ast.RangeStmt:
		return w.rangeStmt(x, held, label)
	case *ast.SwitchStmt:
		if x.Init != nil {
			held, _ = w.stmt(x.Init, held, "")
		}
		if x.Tag != nil {
			held = w.exprs(x.Tag, held)
		}
		return w.clauses(x.Body, held, label)
	case *ast.TypeSwitchStmt:
		if x.Init != nil {
			held, _ = w.stmt(x.Init, held, "")
		}
		held = w.exprs(x.Assign, held)
		return w.clauses(x.Body, held, label)
	case *ast.SelectStmt:
		return w.clauses(x.Body, held, label)
	case *ast.BranchStmt:
		return w.branch(x, held)
	}
	return held, true
}

func (w *funcWalk) ifStmt(x *ast.IfStmt, held heldMutexes) (heldMutexes, bool) {
	if x.Init != nil {
		var live bool
		if held, live = w.stmt(x.Init, held, ""); !live {
			return held, false
		}
	}
	held = w.exprs(x.Cond, held)
	var ends []heldMutexes
	if out, live := w.stmts(x.Body.List, held.clone()); live {
		ends = append(ends, out)
	}
	if x.Else == nil {
		ends = append(ends, held)
	} else if out, live := w.stmt(x.Else, held.clone(), ""); live {
		ends = append(ends, out)
	}
	out, ok := meetHeld(ends)
	if !ok {
		return held, false
	}
	return out, true
}

// loop finds the state at the head of a loop whose body pass runs from a
// head state: the meet of entry and every state that goes back to the head
// (pass returns those, and the states that break out of the loop). It runs
// the passes with recording off until the head state stops changing (it only
// shrinks), then once more with recording as it was, and returns the head
// state and that pass's breaks.
func (w *funcWalk) loop(entry heldMutexes, pass func(head heldMutexes) (back, breaks []heldMutexes)) (heldMutexes, []heldMutexes) {
	record := w.record
	w.record = false
	head := entry.clone()
	for {
		back, _ := pass(head)
		next, _ := meetHeld(append([]heldMutexes{entry}, back...))
		if next.equal(head) {
			break
		}
		head = next
	}
	w.record = record
	_, breaks := pass(head)
	return head, breaks
}

// bodyPass runs a loop body (and its post statement) from head with the
// loop's target on the stack, and returns the states that reach the head
// again, the end of the body and every continue, and those that break out.
func (w *funcWalk) bodyPass(body *ast.BlockStmt, head heldMutexes, label string, post ast.Stmt) (back, breaks []heldMutexes) {
	t := &jumpTarget{label: label, loop: true}
	w.targets = append(w.targets, t)
	out, live := w.stmts(body.List, head.clone())
	w.targets = w.targets[:len(w.targets)-1]
	back = t.continues
	if live {
		back = append(back, out)
	}
	if post != nil {
		for i, b := range back {
			back[i], _ = w.stmt(post, b, "")
		}
	}
	return back, t.breaks
}

func (w *funcWalk) forStmt(x *ast.ForStmt, held heldMutexes, label string) (heldMutexes, bool) {
	if x.Init != nil {
		var live bool
		if held, live = w.stmt(x.Init, held, ""); !live {
			return held, false
		}
	}
	head, exits := w.loop(held, func(head heldMutexes) ([]heldMutexes, []heldMutexes) {
		if x.Cond != nil {
			w.exprs(x.Cond, head)
		}
		return w.bodyPass(x.Body, head, label, x.Post)
	})
	if x.Cond != nil {
		exits = append(exits, head)
	}
	out, ok := meetHeld(exits)
	if !ok {
		return head, false
	}
	return out, true
}

func (w *funcWalk) rangeStmt(x *ast.RangeStmt, held heldMutexes, label string) (heldMutexes, bool) {
	held = w.exprs(x.X, held)
	head, breaks := w.loop(held, func(head heldMutexes) ([]heldMutexes, []heldMutexes) {
		for _, e := range []ast.Expr{x.Key, x.Value} {
			if e != nil {
				w.exprs(e, head)
			}
		}
		return w.bodyPass(x.Body, head, label, nil)
	})
	out, _ := meetHeld(append([]heldMutexes{head}, breaks...))
	return out, true
}

// clauses walks the clauses of a switch, type switch or select from held.
// A clause that ends in fallthrough goes on into the next one. Without a
// default clause a switch can take none.
func (w *funcWalk) clauses(body *ast.BlockStmt, held heldMutexes, label string) (heldMutexes, bool) {
	t := &jumpTarget{label: label}
	w.targets = append(w.targets, t)
	var ends []heldMutexes
	var carry heldMutexes
	hasDefault, isSelect := false, false
	for _, c := range body.List {
		entry := held.clone()
		var list []ast.Stmt
		switch cc := c.(type) {
		case *ast.CaseClause:
			if cc.List == nil {
				hasDefault = true
			}
			for _, e := range cc.List {
				entry = w.exprs(e, entry)
			}
			list = cc.Body
		case *ast.CommClause:
			isSelect = true
			if cc.Comm != nil {
				entry, _ = w.stmt(cc.Comm, entry, "")
			}
			list = cc.Body
		}
		if carry != nil {
			entry, _ = meetHeld([]heldMutexes{entry, carry})
			carry = nil
		}
		through := false
		if n := len(list); n > 0 {
			if b, ok := list[n-1].(*ast.BranchStmt); ok && b.Tok == token.FALLTHROUGH {
				through, list = true, list[:n-1]
			}
		}
		out, live := w.stmts(list, entry)
		switch {
		case live && through:
			carry = out
		case live:
			ends = append(ends, out)
		}
	}
	w.targets = w.targets[:len(w.targets)-1]
	if !hasDefault && !isSelect {
		ends = append(ends, held)
	}
	out, ok := meetHeld(append(ends, t.breaks...))
	if !ok {
		return held, false
	}
	return out, true
}

// branch handles break, continue and goto. A jump whose target the walk
// does not know, and every goto, makes the function undecided.
func (w *funcWalk) branch(x *ast.BranchStmt, held heldMutexes) (heldMutexes, bool) {
	label := ""
	if x.Label != nil {
		label = x.Label.Name
	}
	switch x.Tok {
	case token.BREAK, token.CONTINUE:
		for i := len(w.targets) - 1; i >= 0; i-- {
			t := w.targets[i]
			if label != "" && t.label != label || x.Tok == token.CONTINUE && !t.loop {
				continue
			}
			if x.Tok == token.BREAK {
				t.breaks = append(t.breaks, held.clone())
			} else {
				t.continues = append(t.continues, held.clone())
			}
			return held, false
		}
	}
	w.flow.undecided = true
	return held, false
}

// lockCall applies call to held when it is mu.Lock(), RLock(), Unlock() or
// RUnlock() of a mutex with a node; ok is false for any other call.
func (w *funcWalk) lockCall(call *ast.CallExpr, held heldMutexes) (heldMutexes, bool) {
	method, ok := lockMethodName(call)
	if !ok {
		return held, false
	}
	recv := call.Fun.(*ast.SelectorExpr).X // lockMethodName checked the form
	mu := w.flow.mutexOf(recv)
	if mu == nil {
		return held, false
	}
	held = w.receiver(recv, mu, held)
	next := held.clone()
	if method == lockMethodLock || method == lockMethodRLock {
		next[mu.ID] = mu
	} else {
		delete(next, mu.ID)
	}
	return next, true
}

// chanLock applies the channel lock operation at pos, if there is one, to
// held: a receive takes the lock's token (holds it), a send returns it and
// close closes it (neither holds it after).
func (w *funcWalk) chanLock(pos token.Pos, held heldMutexes) heldMutexes {
	op, ok := w.flow.chanLocks[pos]
	if !ok {
		return held
	}
	next := held.clone()
	if op.Kind == ChanLockAcquire {
		next[op.Lock.ID] = op.Lock
	} else {
		delete(next, op.Lock.ID)
	}
	return next
}

// receiver records the uses of the receiver of a Lock or Unlock call of mu:
// the path to the mutex (s.inner in s.inner.mu.Lock()) is used under held,
// and the mutex field itself under the other mutexes held (the order in
// which locks are taken), not under mu. It returns what is held after.
func (w *funcWalk) receiver(recv ast.Expr, mu *types.Node, held heldMutexes) heldMutexes {
	sel, ok := ast.Unparen(recv).(*ast.SelectorExpr)
	if !ok {
		return w.exprs(recv, held)
	}
	held = w.exprs(sel.X, held)
	others := held.clone()
	delete(others, mu.ID)
	w.use(sel, others)
	return held
}

// callApart handles the call of a go or defer statement: its function and
// arguments are evaluated here, with what is held here, and only the call
// runs later; a function literal runs elsewhere, with nothing held. So the
// receiver path of `defer s.Wallet.lock.Unlock()` is read where the defer
// stands (accounts/scwallet/wallet.go:919 read s.Wallet under the lock and
// had no edge).
func (w *funcWalk) callApart(call *ast.CallExpr, held heldMutexes) {
	if lit, ok := ast.Unparen(call.Fun).(*ast.FuncLit); ok {
		w.flow.body(lit.Body, heldMutexes{}, w.record)
	} else if _, isLock := lockMethodName(call); !isLock {
		w.exprs(call.Fun, held)
	} else if recv := call.Fun.(*ast.SelectorExpr).X; w.flow.mutexOf(recv) != nil {
		w.receiver(recv, w.flow.mutexOf(recv), held)
	} else {
		w.exprs(call.Fun, held)
	}
	for _, a := range call.Args {
		w.exprs(a, held)
	}
}

func (w *funcWalk) isPanic(call *ast.CallExpr) bool {
	id, ok := ast.Unparen(call.Fun).(*ast.Ident)
	if !ok || id.Name != "panic" {
		return false
	}
	_, builtin := w.flow.info.Uses[id].(*gotypes.Builtin)
	return builtin
}

// exprs records the field uses under n with held and returns what is held
// after it: less than held only when n calls a literal in place that ends
// without some of held.
func (w *funcWalk) exprs(n ast.Node, held heldMutexes) heldMutexes {
	if n == nil {
		return held
	}
	after := held
	ast.Inspect(n, func(c ast.Node) bool {
		switch x := c.(type) {
		case *ast.FuncLit:
			w.flow.body(x.Body, heldMutexes{}, w.record)
			return false
		case *ast.CallExpr:
			lit, ok := ast.Unparen(x.Fun).(*ast.FuncLit)
			if !ok {
				// A call evaluated here runs here, with what is held
				// (calls of go and defer statements do not come here).
				if w.record && len(held) > 0 && w.flow.calleeOf != nil {
					if callee, ok := w.flow.calleeOf(x); ok {
						w.flow.calls = append(w.flow.calls, heldCall{callee: callee, held: held.clone()})
					}
				}
				return true
			}
			for _, a := range x.Args {
				w.exprs(a, held)
			}
			end, ends := w.flow.body(lit.Body, held, w.record)
			if ends {
				kept := heldMutexes{}
				for id, mu := range after {
					if _, still := end[id]; still {
						kept[id] = mu
					}
				}
				after = kept
			}
			return false
		}
		w.use(c, held)
		return true
	})
	return after
}

// declares reports whether field is the field that declares the mutex mu
// (a Mutex node has its field's place and name): no field is under its own
// mutex, as when a getter returns &s.mu while s.mu is held.
func declares(field, mu *types.Node) bool {
	return field.FilePath == mu.FilePath && field.StartLine == mu.StartLine && field.Name == mu.Name
}

// use records the field uses that n makes itself (fieldUsesAt: a selector,
// the elements of a struct literal) as uses under every mutex of held.
func (w *funcWalk) use(n ast.Node, held heldMutexes) {
	if !w.record || len(held) == 0 {
		return
	}
	for _, u := range fieldUsesAt(n, w.flow.info) {
		field := w.flow.fieldOf(u.field)
		if field == nil {
			continue
		}
		// In id order: the edges come out in the same order on every build.
		for _, id := range slices.Sorted(maps.Keys(held)) {
			if mu := held[id]; !declares(field, mu) && !w.flow.chanLocks.holds(mu, field) {
				w.flow.uses = append(w.flow.uses, [2]*types.Node{field, mu})
			}
		}
	}
}
