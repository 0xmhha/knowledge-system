package verify

import (
	"fmt"
	"go/ast"
	"go/token"
	gotypes "go/types"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// dispatch.go checks two relations the build derives from the program:
//
//   - dispatches_to of an interface method (kinds interface_method and
//     type_hierarchy): the truth is every concrete method that a named type
//     declared in scope selects for an interface method some covered call
//     invokes, when the type implements the interface. A method of an
//     interface is never a target: it has no body.
//   - sends_to / recvs_from from a Goroutine to a Channel: the truth is every
//     channel operation written in the function literal of a go statement
//     (outside go statements nested in it) whose operand is a local variable
//     that one make(chan ...) sets and nothing else assigns.
//
// Both are read from go/types and the syntax here, sharing no code with the
// extraction. Declarations are named by file, line and name, as for
// accessed_under_lock; a Goroutine and a Channel by the byte offset of the
// go statement and of the make call, as the graph names them.

// The goroutine and message checks read the relations the concurrency
// contract names (internal/graph/concfacts: ChanOp, Handles, Trigger); their
// truth is read from the syntax here.

// Dispatch kinds of the interface dispatches compared here.
const (
	dispatchInterfaceMethod = "interface_method"
	dispatchTypeHierarchy   = "type_hierarchy"
)

// programFacts walks the covered files once for dispatchTruth and
// goroutineChanTruth.
type programFacts struct {
	root  string
	scope map[string]bool
	fset  *token.FileSet
	// invoked are the interface methods covered calls invoke, by key.
	invoked map[string]*gotypes.Func
	// named are the named, non-generic, non-interface types declared in
	// covered files.
	named map[string]*gotypes.Named
	// localChan are the (go statement, make call) pairs of a go statement
	// and a channel made in the function that holds it: the links this
	// check decides. A Goroutine link to a channel made elsewhere (reached
	// through a call, a parameter or a field) is left out.
	localChan map[[2]string]bool
	// msg are the handles and triggers facts (messages.go).
	msg *messageFacts
	// chanLocks names the channels used as locks (chanlock.go): their
	// receives carry no message.
	chanLocks chanLockNames
	// goChan are the expected Goroutine -> Channel links; undecidedChan the
	// Goroutine links whose operand's origin is not one make.
	goChan, undecidedChan map[[3]string]bool
}

func collectProgramFacts(pkgs []*packages.Package, root string, scope map[string]bool, chanLocks chanLockNames) *programFacts {
	pf := &programFacts{root: root, scope: scope, invoked: map[string]*gotypes.Func{}, named: map[string]*gotypes.Named{},
		goChan: map[[3]string]bool{}, undecidedChan: map[[3]string]bool{}, localChan: map[[2]string]bool{}, msg: newMessageFacts(), chanLocks: chanLocks}
	seen := map[string]bool{}
	for _, pkg := range pkgs {
		if pkg.TypesInfo == nil {
			continue
		}
		for i, f := range pkg.Syntax {
			if i >= len(pkg.CompiledGoFiles) {
				continue
			}
			pf.fset = pkg.Fset
			rel, ok := relPath(root, pkg.CompiledGoFiles[i])
			if !ok || !scope[rel] {
				// A call outside the covered files still invokes the
				// method: the build's program holds every loaded body.
				pf.invokes(pkg.TypesInfo, f)
				continue
			}
			if seen[rel] {
				continue
			}
			seen[rel] = true
			pf.file(pkg.TypesInfo, f, filepath.ToSlash(rel))
		}
	}
	return pf
}

// key names a declaration by its file, line and name; "" outside scope.
func (pf *programFacts) key(o gotypes.Object) string {
	if o == nil || !o.Pos().IsValid() {
		return ""
	}
	p := pf.fset.Position(o.Pos())
	rel, ok := relPath(pf.root, p.Filename)
	if !ok || !pf.scope[rel] {
		return ""
	}
	return fmt.Sprintf("%s:%d:%s", filepath.ToSlash(rel), p.Line, o.Name())
}

func (pf *programFacts) file(info *gotypes.Info, f *ast.File, rel string) {
	for id, obj := range info.Defs {
		if obj == nil || pf.fset.File(id.Pos()) != pf.fset.File(f.Pos()) {
			continue
		}
		o, ok := obj.(*gotypes.TypeName)
		if !ok || o.IsAlias() {
			continue
		}
		if n, ok := o.Type().(*gotypes.Named); ok && n.TypeParams().Len() == 0 && !gotypes.IsInterface(n) {
			if k := pf.key(o); k != "" {
				pf.named[k] = n
			}
		}
	}
	pf.invokes(info, f)
	pf.messages(info, f, rel)
	for _, d := range f.Decls {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
			pf.goroutines(info, fd.Body, rel)
		}
	}
}

// invokes records the interface methods the calls of f invoke.
func (pf *programFacts) invokes(info *gotypes.Info, f *ast.File) {
	ast.Inspect(f, func(n ast.Node) bool {
		x, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		if sel, ok := ast.Unparen(x.Fun).(*ast.SelectorExpr); ok {
			if s := info.Selections[sel]; s != nil && s.Kind() == gotypes.MethodVal {
				if fn, ok := s.Obj().(*gotypes.Func); ok && isInterfaceMethod(fn) {
					if k := pf.key(fn); k != "" {
						pf.invoked[k] = fn
					}
				}
			}
		}
		return true
	})
}

// dispatchTruth returns the expected (interface method, concrete method)
// pairs.
func (pf *programFacts) dispatchTruth() map[[2]string]bool {
	out := map[[2]string]bool{}
	byIface := map[*gotypes.Interface][]gotypes.Type{}
	keys := make([]string, 0, len(pf.named))
	for k := range pf.named {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for mk, m := range pf.invoked {
		it, ok := m.Type().(*gotypes.Signature).Recv().Type().Underlying().(*gotypes.Interface)
		if !ok {
			continue
		}
		impls, done := byIface[it]
		if !done {
			for _, k := range keys {
				n := pf.named[k]
				if gotypes.Implements(n, it) {
					impls = append(impls, n)
				} else if p := gotypes.NewPointer(n); gotypes.Implements(p, it) {
					impls = append(impls, p)
				}
			}
			byIface[it] = impls
		}
		for _, t := range impls {
			obj, _, _ := gotypes.LookupFieldOrMethod(t, true, m.Pkg(), m.Name())
			fn, ok := obj.(*gotypes.Func)
			if !ok || isInterfaceMethod(fn) {
				continue
			}
			if dk := pf.key(fn); dk != "" && dk != mk {
				out[[2]string{mk, dk}] = true
			}
		}
	}
	return out
}

// goroutines records the Goroutine -> Channel links of the go statements in
// body that run a function literal.
func (pf *programFacts) goroutines(info *gotypes.Info, body *ast.BlockStmt, rel string) {
	// origins counts the assignments of each local variable and keeps the
	// make call of the one that makes a channel.
	type origin struct {
		sets int
		make *ast.CallExpr
	}
	origins := map[gotypes.Object]*origin{}
	var makes []string // the channels made in body, by file and offset
	set := func(lhs ast.Expr, rhs ast.Expr) {
		id, ok := ast.Unparen(lhs).(*ast.Ident)
		if !ok {
			return
		}
		obj := info.ObjectOf(id)
		if obj == nil {
			return
		}
		o := origins[obj]
		if o == nil {
			o = &origin{}
			origins[obj] = o
		}
		o.sets++
		if c, ok := ast.Unparen(rhs).(*ast.CallExpr); ok && isMakeChan(info, c) {
			o.make = c
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			if isMakeChan(info, x) {
				makes = append(makes, fmt.Sprintf("%s@%d", rel, pf.fset.Position(x.Pos()).Offset))
			}
		case *ast.AssignStmt:
			for i, l := range x.Lhs {
				var r ast.Expr
				if len(x.Lhs) == len(x.Rhs) {
					r = x.Rhs[i]
				}
				set(l, r)
			}
		case *ast.ValueSpec:
			for i, name := range x.Names {
				var r ast.Expr
				if len(x.Names) == len(x.Values) {
					r = x.Values[i]
				}
				set(name, r)
			}
		}
		return true
	})
	var walkGo func(n ast.Node, nested bool)
	walkGo = func(n ast.Node, nested bool) {
		ast.Inspect(n, func(c ast.Node) bool {
			g, ok := c.(*ast.GoStmt)
			if !ok {
				return true
			}
			lit, isLit := ast.Unparen(g.Call.Fun).(*ast.FuncLit)
			if isLit && !nested {
				goKey := fmt.Sprintf("%s@%d", rel, pf.fset.Position(g.Pos()).Offset)
				for _, m := range makes {
					pf.localChan[[2]string{goKey, m}] = true
				}
				pf.goroutineOps(info, lit, g, rel, func(operand ast.Expr) (*ast.CallExpr, bool) {
					id, ok := ast.Unparen(operand).(*ast.Ident)
					if !ok {
						return nil, false
					}
					o := origins[info.ObjectOf(id)]
					if o == nil {
						return nil, false
					}
					return o.make, o.sets == 1 && o.make != nil
				})
			}
			if isLit {
				walkGo(lit.Body, true)
			}
			return false
		})
	}
	walkGo(body, false)
}

// goroutineOps records the channel operations of lit, the literal of go
// statement g, outside go statements nested in it. originOf returns the make
// call of a local operand and whether it is the operand's only origin.
func (pf *programFacts) goroutineOps(info *gotypes.Info, lit *ast.FuncLit, g *ast.GoStmt, rel string,
	originOf func(ast.Expr) (*ast.CallExpr, bool)) {
	goKey := fmt.Sprintf("%s@%d", rel, pf.fset.Position(g.Pos()).Offset)
	record := func(et types.EdgeType, operand ast.Expr) {
		if !isChanTyped(info, operand) {
			return
		}
		call, ok := originOf(operand)
		if call == nil {
			return // no make origin: a parameter, a field, a received channel
		}
		k := [3]string{goKey, fmt.Sprintf("%s@%d", rel, pf.fset.Position(call.Pos()).Offset), string(et)}
		if ok {
			pf.goChan[k] = true
		} else {
			pf.undecidedChan[k] = true
		}
	}
	ast.Inspect(lit.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.GoStmt:
			return false
		case *ast.SendStmt:
			record(types.EdgeSendsTo, x.Chan)
		case *ast.UnaryExpr:
			if x.Op == token.ARROW {
				record(types.EdgeRecvsFrom, x.X)
			}
		case *ast.RangeStmt:
			record(types.EdgeRecvsFrom, x.X)
		}
		return true
	})
}

// isMakeChan reports whether c is the builtin make of a channel.
func isMakeChan(info *gotypes.Info, c *ast.CallExpr) bool {
	id, ok := ast.Unparen(c.Fun).(*ast.Ident)
	if !ok || len(c.Args) == 0 {
		return false
	}
	if _, builtin := info.ObjectOf(id).(*gotypes.Builtin); !builtin || id.Name != "make" {
		return false
	}
	return isChanTyped(info, c.Args[0])
}

// isChanTyped reports whether e has (or, for a type expression, is) a
// channel type.
func isChanTyped(info *gotypes.Info, e ast.Expr) bool {
	t := info.TypeOf(e)
	if t == nil {
		return false
	}
	_, ok := t.Underlying().(*gotypes.Chan)
	return ok
}

// compareDispatch compares the interface dispatches_to edges of g with the
// truth's pairs; an edge that ends at an interface method is unsupported
// whatever the truth says.
func compareDispatch(truth map[[2]string]bool, idx nodeIndex, g GraphReader, samples int) (UnderLockStats, error) {
	edges, err := g.QueryEdgesByType(string(types.EdgeDispatchesTo))
	if err != nil {
		return UnderLockStats{}, fmt.Errorf("verify: read dispatches_to: %w", err)
	}
	key := func(id string) string {
		n, ok := idx.byID[id]
		if !ok {
			return ""
		}
		return fmt.Sprintf("%s:%d:%s", filepath.ToSlash(n.FilePath), n.StartLine, n.Name)
	}
	graph := map[[2]string]bool{}
	for _, e := range edges {
		if e.DispatchKind == dispatchInterfaceMethod || e.DispatchKind == dispatchTypeHierarchy {
			graph[[2]string{key(e.Src), key(e.Dst)}] = true
		}
	}
	return comparePairs(truth, graph, nil, samples), nil
}

// compareGoroutineChans compares the sends_to / recvs_from edges from a
// Goroutine to a Channel with the truth's links. Links whose operand has
// several origins are neither expected nor unsupported.
//
// Only links to a channel made in the function that holds the go statement
// (local) are decided; the others are left out of Edges.
func compareGoroutineChans(truth, undecided map[[3]string]bool, local map[[2]string]bool, idx nodeIndex, g GraphReader, samples int) (UnderLockStats, error) {
	graph := map[[2]string]bool{}
	for _, et := range []types.EdgeType{types.EdgeSendsTo, types.EdgeRecvsFrom} {
		edges, err := g.QueryEdgesByType(string(et))
		if err != nil {
			return UnderLockStats{}, fmt.Errorf("verify: read %s: %w", et, err)
		}
		for _, e := range edges {
			src, dst := idx.byID[e.Src], idx.byID[e.Dst]
			if src.Type != types.NodeGoroutine || dst.Type != types.NodeChannel {
				continue
			}
			graph[[2]string{placeOf(src), placeOf(dst) + "|" + string(et)}] = true
		}
	}
	flat := func(m map[[3]string]bool) map[[2]string]bool {
		out := make(map[[2]string]bool, len(m))
		for k := range m {
			out[[2]string{k[0], k[1] + "|" + k[2]}] = true
		}
		return out
	}
	skip := flat(undecided)
	decided := func(k [2]string) bool {
		ch, _, _ := strings.Cut(k[1], "|")
		return local[[2]string{k[0], ch}] && !skip[k]
	}
	return comparePairs(flat(truth), graph, func(k [2]string) bool { return !decided(k) }, samples), nil
}

// placeOf names a Goroutine or Channel node by its file and start byte.
func placeOf(n types.Node) string {
	return fmt.Sprintf("%s@%d", filepath.ToSlash(n.FilePath), n.StartByte)
}

// comparePairs counts the graph pairs the truth has and the truth pairs the
// graph lacks; skip leaves graph pairs out of the unsupported ones.
func comparePairs(truth, graph map[[2]string]bool, skip func([2]string) bool, samples int) UnderLockStats {
	st := UnderLockStats{Truth: len(truth)}
	var missing, unsupported []string
	for k := range graph {
		if skip != nil && skip(k) {
			continue
		}
		st.Edges++
		if truth[k] {
			st.Matched++
			continue
		}
		st.Unsupported++
		unsupported = append(unsupported, k[0]+" -> "+k[1])
	}
	for k := range truth {
		if !graph[k] {
			missing = append(missing, k[0]+" -> "+k[1])
		}
	}
	sort.Strings(missing)
	sort.Strings(unsupported)
	st.MissingSamples = firstN(missing, samples)
	st.UnsupportedSamples = firstN(unsupported, samples)
	return st
}
