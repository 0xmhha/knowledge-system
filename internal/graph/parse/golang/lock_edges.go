package golang

import (
	"go/ast"
	"go/token"
	gotypes "go/types"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/types/typeutil"

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

// LockStats counts what EmitLockEdges could not decide.
type LockStats struct {
	// UndecidedFunctions are the functions that lock a mutex and whose
	// field uses got no accessed_under_lock edge because the rule cannot
	// follow their control flow (a goto, a jump to an unknown target).
	UndecidedFunctions int
}

// EmitLockEdges returns the acquires_lock, releases_lock and
// accessed_under_lock edges of the loaded packages, and what it could not
// decide.
func EmitLockEdges(pkgs []*packages.Package, nodes []types.Node) ([]types.Edge, LockStats) {
	var st LockStats
	if len(pkgs) == 0 {
		return nil, st
	}
	mutexes := newMutexIndex(pkgs, nodes)
	fields := newFieldIndex(fsetOf(pkgs), nodes)
	var edges []types.Edge
	under := map[edgeDedupKey]bool{} // (field, mutex) pairs already recorded
	forEachOwnedCode(pkgs, nodes, func(c ownedCode) {
		if !c.part.runs() {
			return
		}
		var held []*types.Node // mutexes the code locks, in source order
		seen := map[string]bool{}
		ast.Inspect(c.root, func(n ast.Node) bool {
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
				held = append(held, mu)
			}
			edges = append(edges, types.Edge{
				Src: c.owner.ID, Dst: mu.ID, Type: et,
				Line: c.line(call.Pos()), Count: 1, Confidence: types.ConfExtracted,
				FilePath: c.owner.FilePath,
			})
			return true
		})
		if len(held) == 0 || c.part != partBody {
			return
		}
		body, ok := c.root.(*ast.BlockStmt)
		if !ok {
			return
		}
		info := c.info()
		flow := &lockFlow{info: info,
			mutexOf: func(recv ast.Expr) *types.Node { return mutexes.receiver(info, recv, 0) },
			fieldOf: func(f *gotypes.Var) *types.Node { return fields.node(f) },
		}
		flow.body(body, heldMutexes{}, true)
		if flow.undecided {
			st.UndecidedFunctions++
			return
		}
		for _, use := range flow.uses {
			field, mu := use[0], use[1]
			k := edgeDedupKey{src: field.ID, dst: mu.ID}
			if under[k] {
				continue
			}
			under[k] = true
			edges = append(edges, types.Edge{
				Src: field.ID, Dst: mu.ID, Type: types.EdgeAccessedUnderLock,
				Count: 1, Confidence: types.ConfInferred,
				FilePath: c.owner.FilePath,
			})
		}
	})
	return edges, st
}
