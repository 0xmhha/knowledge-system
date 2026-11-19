package golang

import (
	"go/ast"
	"go/token"
	gotypes "go/types"

	"golang.org/x/tools/go/packages"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// statement_links.go connects the statement nodes of a function body, so a
// reader can follow what happens inside a function: which condition decides
// a write, what a loop or a switch depends on, and what each call site calls.
//
// The declaration pass makes one node per IfStmt, LoopStmt, SwitchStmt,
// ReturnStmt, CallSite and Goroutine, each with only a contains edge from its
// function; calls are lifted to the function. This pass adds (types.Edge*):
//
//	site_calls   CallSite or Goroutine -> the declaration it calls, resolved
//	             with go/types (dispatch_kind static or interface_method)
//	cond_reads   IfStmt, LoopStmt, SwitchStmt -> each field, package-level
//	             variable or constant its condition reads; the condition is
//	             the init statement, condition, post statement, range
//	             expression, switch tag and case expressions
//	cond_calls   the same -> each function or method its condition calls
//	body_writes  IfStmt, LoopStmt, SwitchStmt, Goroutine -> each value its
//	             body writes outside every statement nested in it
//	             (dispatch_kind then, else, body, case, go)
//	encloses     a statement -> each statement node directly inside it
//	             (dispatch_kind cond, then, else, body, case, go)
//
// Line is the line of the use. A function literal's body is walked as its
// own top level: it runs when it is called, not where it is written; the
// literal of a go statement belongs to its Goroutine node. A statement
// without a node (none is made inside a goroutine literal except call
// sites) is transparent: what lies inside it belongs to the statement
// around it.

// StatementStats summarises one run of the pass for the build log.
type StatementStats struct {
	Functions  int // function bodies walked
	NoNode     int // statements of a kind that has nodes but whose node was not found
	Unresolved int // calls whose callee has no node (builtins, conversions, outside code, function values)
}

type stmtKey struct {
	file string
	off  int
	typ  types.NodeType
}

// EmitStatementEdges returns the statement edges of the loaded packages.
func EmitStatementEdges(pkgs []*packages.Package, nodes []types.Node) ([]types.Edge, StatementStats) {
	var st StatementStats
	if len(pkgs) == 0 {
		return nil, st
	}
	byCanonical := make(map[string]*types.Node, len(nodes))
	stmts := map[stmtKey]*types.Node{}
	for i := range nodes {
		n := &nodes[i]
		switch n.Type {
		case types.NodeField, types.NodeVariable, types.NodeConstant, types.NodeFunction, types.NodeMethod:
			if n.CanonicalID != "" {
				if _, ok := byCanonical[n.CanonicalID]; !ok {
					byCanonical[n.CanonicalID] = n
				}
			}
		case types.NodeIfStmt, types.NodeLoopStmt, types.NodeSwitchStmt, types.NodeReturnStmt, types.NodeGoroutine:
			stmts[stmtKey{n.FilePath, n.StartByte, n.Type}] = n
		case types.NodeCallSite:
			if n.SubKind == "" { // send and recv sites are channel operations, not calls
				stmts[stmtKey{n.FilePath, n.StartByte, n.Type}] = n
			}
		}
	}
	decls := newDeclIndex(nodes)
	c := &stmtCollector{seen: map[stmtEdgeKey]int{}}
	for _, pkg := range pkgs {
		if pkg == nil || pkg.Types == nil || pkg.TypesInfo == nil {
			continue
		}
		for _, f := range pkg.Syntax {
			for _, decl := range f.Decls {
				d, ok := decl.(*ast.FuncDecl)
				if !ok || d.Body == nil {
					continue
				}
				src := decls.at(goCanonicalID(pkg.TypesInfo.Defs[d.Name]), funcDeclQname(pkg.Types.Name(), d), pkg.Fset.Position(d.Pos()))
				if src == nil {
					continue
				}
				st.Functions++
				w := &stmtWalker{c: c, st: &st, info: pkg.TypesInfo, fset: pkg.Fset, file: src.FilePath,
					stmts: stmts, byCanonical: byCanonical, lhs: writtenIn(d.Body)}
				w.region(d.Body, ctl{})
			}
		}
	}
	return c.edges, st
}

// ctl is the statement a region of code lies in, and which part of it.
type ctl struct {
	node *types.Node
	kind string // cond, then, else, body, case, go
}

type stmtEdgeKey struct {
	src, dst string
	typ      types.EdgeType
	kind     string
	line     int
}

type stmtCollector struct {
	seen  map[stmtEdgeKey]int
	edges []types.Edge
}

func (c *stmtCollector) emit(src, dst *types.Node, typ types.EdgeType, kind string, line int) {
	if src == nil || dst == nil || src.ID == dst.ID {
		return
	}
	k := stmtEdgeKey{src.ID, dst.ID, typ, kind, line}
	if i, ok := c.seen[k]; ok {
		c.edges[i].Count++
		return
	}
	c.seen[k] = len(c.edges)
	c.edges = append(c.edges, types.Edge{Src: src.ID, Dst: dst.ID, Type: typ, DispatchKind: kind,
		FilePath: src.FilePath, Line: line, Count: 1, Confidence: types.ConfExtracted})
}

type stmtWalker struct {
	c           *stmtCollector
	st          *StatementStats
	info        *gotypes.Info
	fset        *token.FileSet
	file        string
	stmts       map[stmtKey]*types.Node
	byCanonical map[string]*types.Node
	lhs         map[ast.Expr]bool // the left sides of assignments and increments in the body
}

// writtenIn marks what the assignments and increments of body write
// (x.F, pkg.V, V, and m[k] into m), so a condition's reads leave them out:
// "if w.v, err = f(); err != nil" writes w.v.
func writtenIn(body ast.Node) map[ast.Expr]bool {
	out := map[ast.Expr]bool{}
	var mark func(e ast.Expr)
	mark = func(e ast.Expr) {
		switch x := e.(type) {
		case *ast.Ident:
			out[x] = true
		case *ast.SelectorExpr:
			out[x] = true
			out[x.Sel] = true
		case *ast.IndexExpr:
			mark(x.X)
		case *ast.ParenExpr:
			mark(x.X)
		case *ast.StarExpr:
			mark(x.X)
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			if x.Tok != token.DEFINE {
				for _, l := range x.Lhs {
					mark(l)
				}
			}
		case *ast.IncDecStmt:
			mark(x.X)
		}
		return true
	})
	return out
}

func (w *stmtWalker) line(p token.Pos) int { return w.fset.Position(p).Line }

func (w *stmtWalker) node(t types.NodeType, p token.Pos) *types.Node {
	n := w.stmts[stmtKey{w.file, w.fset.Position(p).Offset, t}]
	if n == nil {
		w.st.NoNode++
	}
	return n
}

// inner is the context for the parts of a statement: the statement itself
// when it has a node, otherwise the context around it.
func inner(n *types.Node, outer ctl, kind string) ctl {
	if n == nil {
		return outer
	}
	return ctl{n, kind}
}

func (w *stmtWalker) enclose(c ctl, child *types.Node, p token.Pos) {
	if c.node != nil && child != nil {
		w.c.emit(c.node, child, types.EdgeEncloses, c.kind, w.line(p))
	}
}

// region walks root, handling each statement with a node itself and
// recording what root reads, writes and calls on behalf of c.
func (w *stmtWalker) region(root ast.Node, c ctl) {
	if root == nil {
		return
	}
	ast.Inspect(root, func(n ast.Node) bool {
		switch x := n.(type) {
		case nil:
			return false
		case *ast.FuncLit:
			w.region(x.Body, ctl{})
			return false
		case *ast.IfStmt:
			w.ifStmt(x, c)
			return false
		case *ast.ForStmt:
			s := w.node(types.NodeLoopStmt, x.Pos())
			w.enclose(c, s, x.Pos())
			cc := inner(s, c, "cond")
			w.region(x.Init, cc)
			w.region(x.Cond, cc)
			w.region(x.Post, cc)
			w.region(x.Body, inner(s, c, "body"))
			return false
		case *ast.RangeStmt:
			s := w.node(types.NodeLoopStmt, x.Pos())
			w.enclose(c, s, x.Pos())
			w.region(x.X, inner(s, c, "cond"))
			w.region(x.Body, inner(s, c, "body"))
			return false
		case *ast.SwitchStmt:
			s := w.node(types.NodeSwitchStmt, x.Pos())
			w.enclose(c, s, x.Pos())
			cc := inner(s, c, "cond")
			w.region(x.Init, cc)
			w.region(x.Tag, cc)
			w.caseClauses(x.Body, s, c)
			return false
		case *ast.TypeSwitchStmt:
			s := w.node(types.NodeSwitchStmt, x.Pos())
			w.enclose(c, s, x.Pos())
			cc := inner(s, c, "cond")
			w.region(x.Init, cc)
			w.region(x.Assign, cc)
			w.caseClauses(x.Body, s, c)
			return false
		case *ast.GoStmt:
			w.goStmt(x, c)
			return false
		case *ast.ReturnStmt:
			w.enclose(c, w.node(types.NodeReturnStmt, x.Pos()), x.Pos())
		case *ast.CallExpr:
			callee, kind := w.callee(x)
			if s := w.node(types.NodeCallSite, x.Pos()); s != nil {
				w.enclose(c, s, x.Pos())
				w.c.emit(s, callee, types.EdgeSiteCalls, kind, w.line(x.Pos()))
			}
			if callee == nil {
				w.st.Unresolved++
			} else if c.kind == "cond" {
				w.c.emit(c.node, callee, types.EdgeCondCalls, "", w.line(x.Pos()))
			}
		case *ast.AssignStmt:
			if c.node != nil && c.kind != "cond" && x.Tok != token.DEFINE {
				for _, lhs := range x.Lhs {
					w.written(lhs, c)
				}
			}
		case *ast.IncDecStmt:
			if c.node != nil && c.kind != "cond" {
				w.written(x.X, c)
			}
		case *ast.SelectorExpr:
			if c.kind == "cond" && !w.lhs[x] {
				if sel := w.info.Selections[x]; sel != nil && sel.Kind() == gotypes.FieldVal {
					w.c.emit(c.node, w.byCanonical[selectedFieldCanonical(sel)], types.EdgeCondReads, "", w.line(x.Sel.Pos()))
				}
			}
		case *ast.Ident:
			if c.kind == "cond" && !w.lhs[x] {
				if v := w.pkgValue(x); v != nil {
					w.c.emit(c.node, v, types.EdgeCondReads, "", w.line(x.Pos()))
				}
			}
		}
		return true
	})
}

func (w *stmtWalker) ifStmt(x *ast.IfStmt, c ctl) {
	s := w.node(types.NodeIfStmt, x.Pos())
	w.enclose(c, s, x.Pos())
	cc := inner(s, c, "cond")
	w.region(x.Init, cc)
	w.region(x.Cond, cc)
	w.region(x.Body, inner(s, c, "then"))
	w.region(x.Else, inner(s, c, "else")) // an else-if is a nested IfStmt
}

func (w *stmtWalker) caseClauses(body *ast.BlockStmt, s *types.Node, c ctl) {
	if body == nil {
		return
	}
	for _, cl := range body.List {
		cc, ok := cl.(*ast.CaseClause)
		if !ok {
			continue
		}
		for _, e := range cc.List {
			w.region(e, inner(s, c, "cond"))
		}
		for _, b := range cc.Body {
			w.region(b, inner(s, c, "case"))
		}
	}
}

func (w *stmtWalker) goStmt(x *ast.GoStmt, c ctl) {
	g := w.node(types.NodeGoroutine, x.Pos())
	w.enclose(c, g, x.Pos())
	if x.Call == nil {
		return
	}
	if lit, ok := x.Call.Fun.(*ast.FuncLit); ok {
		w.region(lit.Body, inner(g, ctl{}, "go"))
	} else {
		callee, kind := w.callee(x.Call)
		if callee == nil {
			w.st.Unresolved++
		}
		w.c.emit(g, callee, types.EdgeSiteCalls, kind, w.line(x.Pos()))
		w.region(x.Call.Fun, c) // a receiver or function expression evaluated here
	}
	for _, a := range x.Call.Args { // arguments are evaluated by the go statement itself
		w.region(a, c)
	}
}

// written records a write of the value e names (x.F, pkg.V, m[k] into m).
func (w *stmtWalker) written(e ast.Expr, c ctl) {
	switch x := e.(type) {
	case *ast.ParenExpr:
		w.written(x.X, c)
	case *ast.IndexExpr:
		w.written(x.X, c)
	case *ast.StarExpr:
		w.written(x.X, c)
	case *ast.SelectorExpr:
		if sel := w.info.Selections[x]; sel != nil && sel.Kind() == gotypes.FieldVal {
			w.c.emit(c.node, w.byCanonical[selectedFieldCanonical(sel)], types.EdgeBodyWrites, c.kind, w.line(x.Sel.Pos()))
			return
		}
		if v := w.pkgValue(x.Sel); v != nil { // pkg.Var
			w.c.emit(c.node, v, types.EdgeBodyWrites, c.kind, w.line(x.Sel.Pos()))
		}
	case *ast.Ident:
		if v := w.pkgValue(x); v != nil {
			w.c.emit(c.node, v, types.EdgeBodyWrites, c.kind, w.line(x.Pos()))
		}
	}
}

// pkgValue is the node of a package-level variable or constant an
// identifier uses.
func (w *stmtWalker) pkgValue(id *ast.Ident) *types.Node {
	obj := w.info.Uses[id]
	if obj == nil || obj.Pkg() == nil || obj.Parent() != obj.Pkg().Scope() {
		return nil
	}
	switch o := obj.(type) {
	case *gotypes.Var:
		if o.IsField() {
			return nil
		}
		return w.byCanonical[goCanonicalID(o)]
	case *gotypes.Const:
		return w.byCanonical[goCanonicalID(o)]
	}
	return nil
}

// callee resolves the declaration a call names: a function, a method, or
// an interface method. Function values, builtins and conversions give nil.
func (w *stmtWalker) callee(call *ast.CallExpr) (*types.Node, string) {
	fun := ast.Unparen(call.Fun)
	switch x := fun.(type) {
	case *ast.IndexExpr: // f[T](...)
		fun = x.X
	case *ast.IndexListExpr:
		fun = x.X
	}
	var obj gotypes.Object
	kind := "static"
	switch x := fun.(type) {
	case *ast.Ident:
		obj = w.info.Uses[x]
	case *ast.SelectorExpr:
		if sel := w.info.Selections[x]; sel != nil {
			if sel.Kind() != gotypes.MethodVal {
				return nil, ""
			}
			obj = sel.Obj()
			// Declared by an interface: a call on an interface value, or on a
			// struct that embeds one (b.Write() with HookedBatch{Batch}).
			if f, ok := obj.(*gotypes.Func); ok {
				if sig, ok := f.Type().(*gotypes.Signature); ok && sig.Recv() != nil && gotypes.IsInterface(sig.Recv().Type()) {
					kind = "interface_method"
				}
			}
		} else {
			obj = w.info.Uses[x.Sel] // pkg.Func
		}
	}
	fn, ok := obj.(*gotypes.Func)
	if !ok {
		return nil, ""
	}
	if n := w.byCanonical[goCanonicalID(fn)]; n != nil {
		return n, kind
	}
	return nil, ""
}
