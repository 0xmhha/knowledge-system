package golang

import (
	"fmt"
	"go/ast"
	"go/token"
	gotypes "go/types"
	"maps"
	"slices"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/types/typeutil"

	"github.com/0xmhha/knowledge-system/internal/graph/concfacts"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// lock_edges.go ties code to the Mutex nodes it uses:
//
//	acquires_lock        Function/Method -> Mutex, for mu.Lock() / mu.RLock()
//	releases_lock        Function/Method -> Mutex, for mu.Unlock() / mu.RUnlock()
//	accessed_under_lock  Field -> Mutex, for a field used where the mutex
//	                     is held (lock_held.go)
//
// The pass runs after Resolve, over all files. It used to run file by file
// with the mutexes of that one file, so a lock on a mutex declared in another
// file of the package had no edge: 171 of the 171 such sites of go-stablenet,
// against 1,438 of 1,438 right ones in the declaring file. A mutex is found
// by where it is declared (posIndex), which is the same in a package and in
// its test variant.
//
// A call in a function literal counts for the function that holds the
// literal, and one in a package-level initialiser for the variable.
//
// The read/write variant (RLock) is on the Mutex node's sub kind (rwmutex),
// not on the edge.
//
// accessed_under_lock is recorded for a field used at a point of a function
// where the mutex is held (lock_held.go). Cross-function propagation is
// buildpipe's lock propagation pass.

// mutexIndex finds the Mutex node of a mutex by where it is declared.
type mutexIndex struct {
	pos posIndex
	// funcs holds the declared functions by the position of their name, so
	// that a mutex reached through a call (s.guard().Lock()) can be read off
	// what the callee returns.
	funcs map[token.Position]funcSource
}

// funcSource is a function declaration and the type information of its file.
type funcSource struct {
	decl *ast.FuncDecl
	info *gotypes.Info
}

// maxResultDepth bounds how many calls are followed from a lock receiver to
// the mutex it returns (a getter that calls another getter).
const maxResultDepth = 2

func newMutexIndex(pkgs []*packages.Package, nodes []types.Node) mutexIndex {
	ix := mutexIndex{pos: newPosIndex(fsetOf(pkgs), nodes, ofTypes(types.NodeMutex)), funcs: map[token.Position]funcSource{}}
	for _, pkg := range pkgs {
		if pkg == nil || pkg.TypesInfo == nil || pkg.Fset == nil {
			continue
		}
		for _, f := range pkg.Syntax {
			if f == nil {
				continue
			}
			for _, d := range f.Decls {
				if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
					k := ix.key(fd.Name.Pos())
					if _, dup := ix.funcs[k]; !dup {
						ix.funcs[k] = funcSource{decl: fd, info: pkg.TypesInfo}
					}
				}
			}
		}
	}
	return ix
}

// key identifies a position by file and offset: a package and its test
// variant read the same file, and need not share token positions.
func (ix mutexIndex) key(p token.Pos) token.Position {
	if ix.pos.fset == nil {
		return token.Position{}
	}
	at := ix.pos.fset.Position(p)
	return token.Position{Filename: at.Filename, Offset: at.Offset}
}

// node returns the Mutex node of the variable or field obj, or nil: the
// node of that name whose span, the declaration's, holds the identifier. An
// embedded mutex is named after its type and positioned at the type name.
func (ix mutexIndex) node(obj gotypes.Object) *types.Node {
	v, ok := obj.(*gotypes.Var)
	if !ok {
		return nil
	}
	v = v.Origin()
	return ix.pos.at(v.Name(), v.Pos())
}

// receiver returns the Mutex node that the receiver expression of a Lock or
// Unlock call denotes, or nil.
//
// The receiver names the mutex (mu, s.mu, *p, &s.mu), or it is a value whose
// struct type embeds one (s.Lock() with struct{ sync.Mutex }), or it is a call
// of a function that returns one such mutex (s.guard().Lock() with
// func (s *S) guard() *sync.Mutex { return &s.mu }; see result). A mutex that
// no declaration names (an element of a map of mutexes, l.locks[addr]) has no
// node, and a lock on it has no edge. A Lock method of an unrelated type
// matches nothing, because only a declaration go/types types as sync.Mutex or
// sync.RWMutex has a Mutex node.
func (ix mutexIndex) receiver(info *gotypes.Info, recv ast.Expr, depth int) *types.Node {
	var obj gotypes.Object
	switch x := ast.Unparen(recv).(type) {
	case *ast.Ident:
		obj = info.ObjectOf(x)
	case *ast.SelectorExpr:
		obj = info.ObjectOf(x.Sel)
	case *ast.StarExpr:
		return ix.receiver(info, x.X, depth)
	case *ast.UnaryExpr:
		if x.Op == token.AND {
			return ix.receiver(info, x.X, depth)
		}
	case *ast.CallExpr:
		return ix.result(info, x, depth)
	}
	if n := ix.node(obj); n != nil {
		return n
	}
	t := info.TypeOf(recv)
	if t == nil || classifyMutexType(t) != "" {
		return nil
	}
	return ix.embedded(t)
}

// result returns the Mutex node of the mutex that the call returns, or nil:
// the callee is a declared function whose every return statement returns the
// same mutex declaration. Anything else (a mutex picked at run time, a
// function value, a callee outside the loaded packages) identifies no mutex.
func (ix mutexIndex) result(info *gotypes.Info, call *ast.CallExpr, depth int) *types.Node {
	if depth >= maxResultDepth {
		return nil
	}
	fn := typeutil.StaticCallee(info, call)
	if fn == nil {
		return nil
	}
	src, ok := ix.funcs[ix.key(fn.Origin().Pos())]
	if !ok {
		return nil
	}
	var found *types.Node
	same := true
	ast.Inspect(src.decl.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			return false // its returns are its own
		case *ast.ReturnStmt:
			var mu *types.Node
			if len(x.Results) == 1 {
				mu = ix.receiver(src.info, x.Results[0], depth+1)
			}
			if mu == nil || found != nil && found.ID != mu.ID {
				same = false
			}
			found = mu
		}
		return same
	})
	if !same {
		return nil
	}
	return found
}

// embedded returns the Mutex node of the mutex that the struct type t (or
// the struct t points to) embeds, or nil.
func (ix mutexIndex) embedded(t gotypes.Type) *types.Node {
	t = gotypes.Unalias(t)
	if ptr, ok := t.(*gotypes.Pointer); ok {
		t = gotypes.Unalias(ptr.Elem())
	}
	st, ok := t.Underlying().(*gotypes.Struct)
	if !ok {
		return nil
	}
	for f := range st.Fields() {
		if f.Embedded() && classifyMutexType(f.Type()) != "" {
			if n := ix.node(f); n != nil {
				return n
			}
		}
	}
	return nil
}

// LockStats counts what LockFacts could not decide.
type LockStats struct {
	// UndecidedFunctions are the functions that lock a mutex and whose
	// field uses got no accessed_under_lock edge because the rule cannot
	// follow their control flow (a goto, a jump to an unknown target).
	UndecidedFunctions int
	// PropagatedEdges are the accessed_under_lock edges found in callees
	// (LockOptions.Propagate); UndecidedCallees the callees left out.
	PropagatedEdges, UndecidedCallees int
}

// LockOptions tunes LockFacts.
type LockOptions struct {
	// Propagate follows the calls made while a mutex is held into the
	// declared functions they run, up to PropagationDepth calls deep, and
	// records the fields those functions use while the mutex is still held
	// (accessed_under_lock with DispatchKind PropagatedLockKind).
	Propagate bool
	// ChanLocks are the operations on channels used as locks
	// (DispatchStats.ChanLockOps): taking and returning their token are
	// lock and unlock calls of the lock, the Channel node.
	ChanLocks ChanLocks
}

// PropagatedLockKind is the dispatch kind of an accessed_under_lock edge
// found in a callee: the field is used in a function called while the mutex
// is held. A direct edge (a use in the function that holds the mutex) has
// none.
const PropagatedLockKind = concfacts.PropagatedKind

// PropagationDepth bounds how many calls deep propagation follows.
const PropagationDepth = 5

// LockFacts returns the lock facts of the loaded packages (concfacts: the
// acquires and releases, proved from the calls; the fields used under a
// mutex, approximated because mutexes are told apart by declaration, not
// instance), and what it could not decide. The graph writer stores them as
// acquires_lock, releases_lock and accessed_under_lock edges
// (concfacts.Facts.Edges).
//
// With opt.Propagate a callee's use counts when the mutex is held at the call
// by the must-hold rule (lock_held.go) and, in the callee, still held at the
// use: the callee is walked with the mutexes held at the call as its entry
// state, so a use after the callee's own Unlock is not under it. Only calls
// that run where they are written count: not go statements (the goroutine
// runs without the caller's locks), not deferred calls, not calls through an
// interface or a function value (the code they run is not known here). A
// callee the rule cannot follow (a goto) gives no edge and is counted.
func LockFacts(pkgs []*packages.Package, nodes []types.Node, opt LockOptions) (concfacts.Facts, LockStats) {
	var st LockStats
	if len(pkgs) == 0 {
		return nil, st
	}
	mutexes := newMutexIndex(pkgs, nodes)
	fields := newFieldIndex(fsetOf(pkgs), nodes)
	var edges concfacts.Facts
	under := map[edgeDedupKey]bool{} // (field, mutex) pairs already recorded
	held := siteLocks{fset: fsetOf(pkgs), sites: newSiteIndex(nodes), seen: map[edgeDedupKey]bool{}}
	var pending []heldCall // calls made under a lock, for propagation
	calleeOf := func(info *gotypes.Info) func(*ast.CallExpr) (funcSource, bool) {
		if !opt.Propagate {
			return nil
		}
		return func(call *ast.CallExpr) (funcSource, bool) {
			fn := typeutil.StaticCallee(info, call)
			if fn == nil {
				return funcSource{}, false
			}
			src, ok := mutexes.funcs[mutexes.key(fn.Origin().Pos())]
			return src, ok
		}
	}
	forEachOwnedCode(pkgs, nodes, func(c ownedCode) {
		if !c.part.runs() {
			return
		}
		var locked []*types.Node // mutexes the code locks, in source order
		seen := map[string]bool{}
		ast.Inspect(c.root, func(n ast.Node) bool {
			if pos, ok := chanOpPos(n); ok {
				if op, isLock := opt.ChanLocks[pos]; isLock {
					if op.Kind == ChanLockAcquire && !seen[op.Lock.ID] {
						seen[op.Lock.ID] = true
						locked = append(locked, op.Lock)
					}
					edges = append(edges, concfacts.LockOp{Function: c.owner.ID, Mutex: op.Lock.ID, Acquire: op.Kind == ChanLockAcquire,
						Close: op.Kind == ChanLockClose, Line: c.line(pos), File: c.owner.FilePath,
						Certainty: concfacts.Approx(concfacts.ReasonAliasing)})
				}
			}
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			method, ok := lockMethodName(call)
			if !ok {
				return true
			}
			sel := call.Fun.(*ast.SelectorExpr) // lockMethodName checked the form
			mu := mutexes.receiver(c.info(), sel.X, 0)
			if mu == nil {
				return true
			}
			et := types.EdgeAcquiresLock
			if method == lockMethodUnlock || method == lockMethodRUnlock {
				et = types.EdgeReleasesLock
			} else if !seen[mu.ID] {
				seen[mu.ID] = true
				locked = append(locked, mu)
			}
			edges = append(edges, concfacts.LockOp{Function: c.owner.ID, Mutex: mu.ID, Acquire: et == types.EdgeAcquiresLock,
				Line: c.line(call.Pos()), File: c.owner.FilePath, Certainty: concfacts.Proof})
			return true
		})
		if len(locked) == 0 || c.part != partBody {
			return
		}
		body, ok := c.root.(*ast.BlockStmt)
		if !ok {
			return
		}
		info := c.info()
		flow := &lockFlow{info: info,
			mutexOf:   func(recv ast.Expr) *types.Node { return mutexes.receiver(info, recv, 0) },
			fieldOf:   func(f *gotypes.Var) *types.Node { return fields.node(f) },
			calleeOf:  calleeOf(info),
			chanLocks: opt.ChanLocks,
		}
		flow.body(body, heldMutexes{}, true)
		if flow.undecided {
			st.UndecidedFunctions++
			return
		}
		pending = append(pending, flow.calls...)
		edges = append(edges, held.facts(flow.sites, nil)...)
		for _, use := range flow.uses {
			field, mu := use[0], use[1]
			k := edgeDedupKey{src: field.ID, dst: mu.ID}
			if under[k] {
				continue
			}
			under[k] = true
			edges = append(edges, concfacts.UnderLock{Field: field.ID, Mutex: mu.ID, File: c.owner.FilePath,
				Certainty: concfacts.Approx(concfacts.ReasonAliasing)})
		}
	})
	if opt.Propagate {
		edges = append(edges, propagateLocks(pending, mutexes, fields, under, &held, calleeOf, opt.ChanLocks, &st)...)
	}
	return edges, st
}

// siteLocks turns the sites a lock flow records into SiteLock facts, once
// per (site, mutex): a site whose own function holds the mutex is not also
// recorded as held there by a caller.
type siteLocks struct {
	fset  *token.FileSet
	sites *siteIndex
	seen  map[edgeDedupKey]bool
}

// facts returns the SiteLock facts of sites; with from set, only of the
// mutexes in from (a callee walked with its callers' mutexes), marked
// propagated. Every one is approximated: mutexes are told apart by
// declaration, and the must-hold rule reads the paths of a branch together.
func (sl *siteLocks) facts(sites []heldSite, from heldMutexes) concfacts.Facts {
	var out concfacts.Facts
	for _, s := range sites {
		p := sl.fset.Position(s.from)
		site := sl.sites.span(p, sl.fset.Position(s.to))
		if site == nil {
			continue
		}
		for _, id := range slices.Sorted(maps.Keys(s.held)) {
			if _, fromCaller := from[id]; from != nil && !fromCaller {
				continue
			}
			k := edgeDedupKey{src: site.ID, dst: id}
			if sl.seen[k] {
				continue
			}
			sl.seen[k] = true
			out = append(out, concfacts.SiteLock{Site: site.ID, Mutex: id, Propagated: from != nil, Line: p.Line,
				File: site.FilePath, Certainty: concfacts.Approx(concfacts.ReasonBranches)})
		}
	}
	return out
}

// chanOpPos returns the position a channel lock operation is keyed by when
// n is one: the arrow of a receive or a send, the opening parenthesis of a
// call (close(ch)).
func chanOpPos(n ast.Node) (token.Pos, bool) {
	switch x := n.(type) {
	case *ast.UnaryExpr:
		if x.Op == token.ARROW {
			return x.OpPos, true
		}
	case *ast.SendStmt:
		return x.Arrow, true
	case *ast.CallExpr:
		return x.Lparen, true
	}
	return token.NoPos, false
}

// propagateLocks walks the calls made under a lock into their callees (see
// LockFacts) and returns the UnderLock facts of the pairs that
// under does not hold yet, marking them PropagatedLockKind. A callee is
// walked once per set of mutexes held at its calls.
func propagateLocks(calls []heldCall, mutexes mutexIndex, fields fieldIndex, under map[edgeDedupKey]bool, sites *siteLocks,
	calleeOf func(*gotypes.Info) func(*ast.CallExpr) (funcSource, bool), chanLocks ChanLocks, st *LockStats) concfacts.Facts {
	type item struct {
		call  heldCall
		depth int
	}
	work := make([]item, 0, len(calls))
	for _, c := range calls {
		work = append(work, item{c, 1})
	}
	done := map[string]bool{}
	var edges concfacts.Facts
	for len(work) > 0 {
		it := work[0]
		work = work[1:]
		callee, held := it.call.callee, it.call.held
		at := mutexes.key(callee.decl.Pos())
		key := fmt.Sprintf("%s|%d|%s", at.Filename, at.Offset, strings.Join(slices.Sorted(maps.Keys(held)), ","))
		if done[key] || callee.decl.Body == nil {
			continue
		}
		done[key] = true
		info := callee.info
		flow := &lockFlow{info: info,
			mutexOf:   func(recv ast.Expr) *types.Node { return mutexes.receiver(info, recv, 0) },
			fieldOf:   func(f *gotypes.Var) *types.Node { return fields.node(f) },
			calleeOf:  calleeOf(info),
			chanLocks: chanLocks,
		}
		flow.body(callee.decl.Body, held, true)
		if flow.undecided {
			st.UndecidedCallees++
			continue
		}
		edges = append(edges, sites.facts(flow.sites, held)...)
		for _, use := range flow.uses {
			field, mu := use[0], use[1]
			k := edgeDedupKey{src: field.ID, dst: mu.ID}
			if _, fromCaller := held[mu.ID]; !fromCaller || under[k] {
				continue // the callee's own lock: a direct edge of the callee
			}
			under[k] = true
			edges = append(edges, concfacts.UnderLock{Field: field.ID, Mutex: mu.ID, Propagated: true, File: field.FilePath,
				Certainty: concfacts.Approx(concfacts.ReasonAliasing)})
			st.PropagatedEdges++
		}
		if it.depth >= PropagationDepth {
			continue
		}
		for _, next := range flow.calls {
			// Only the caller's mutexes go on: the callee's own locks are
			// propagated from the callee as a function that locks them.
			kept := heldMutexes{}
			for id, mu := range next.held {
				if _, fromCaller := held[id]; fromCaller {
					kept[id] = mu
				}
			}
			if len(kept) > 0 {
				work = append(work, item{heldCall{callee: next.callee, held: kept}, it.depth + 1})
			}
		}
	}
	return edges
}
