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
// Line is the line of the use. A statement without a node is transparent:
// what lies inside it belongs to the statement around it.
//
// The parts of a statement are regions of the source text, and a function
// literal does not end a region. A statement written in a literal that is
// written in a loop lies in that loop, and encloses says so (1,136 statements
// of go-stablenet had no encloses edge because the literal cut them off). A
// value read or a function called in a literal that stands in a statement's
// header (for _, tc := range []test{{run: func() { use(x) }}}) is in the
// header: cond_reads and cond_calls start at the innermost statement whose
// header holds the use, across literals and the statements inside them (13
// such reads of go-stablenet had no edge). body_writes alone stops at a
// literal: a branch that only defines a function writes nothing when it runs. The literal of a go
// statement belongs to its Goroutine node, and so do the calls the go
// statement makes itself to evaluate its function and its arguments.
//
// The pass also resolves the calls the declaration pass cannot: a method of
// an interface declared inside a function, or written without a name, has no
// canonical id, so its node is found by position (posIndex). Such a call gets
// its site_calls edge here and its invokes edge from the function as well.

// StatementStats summarises one run of the pass for the build log.
type StatementStats struct {
	Functions  int // function bodies walked
	NoNode     int // statements of a kind that has nodes but whose node was not found
	Unresolved int // calls whose callee has no node (builtins, conversions, outside code, function values)
}

// stmtKey finds the node of a statement. A call site is also keyed by its
// end byte, because the calls of a chain start at the same byte; for the
// other kinds end is 0.
type stmtKey struct {
	file string
	off  int
	end  int
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
			stmts[stmtKey{n.FilePath, n.StartByte, 0, n.Type}] = n
		case types.NodeCallSite:
			// A call and a channel operation never share a span: a send
			// ends after its value, a receive starts at the arrow, and a
			// range over a channel starts at the range keyword.
			stmts[stmtKey{n.FilePath, n.StartByte, n.EndByte, n.Type}] = n
		}
	}
	fields := newFieldIndex(fsetOf(pkgs), nodes)
	// Methods without a canonical id: those of interfaces declared inside a
	// function or written without a name.
	methods := newPosIndex(fsetOf(pkgs), nodes, func(n *types.Node) bool {
		return n.Type == types.NodeMethod && n.CanonicalID == ""
	})
	c := &stmtCollector{seen: map[stmtEdgeKey]int{}}
	// The initialiser of a package-level variable holds statements and call
	// sites of its own (schema 1.38); a constant's holds no code that runs.
	forEachOwnedCode(pkgs, nodes, func(oc ownedCode) {
		if !oc.part.runs() || oc.tok == token.CONST {
			return
		}
		if oc.part == partBody {
			st.Functions++
		}
		w := &stmtWalker{c: c, st: &st, info: oc.info(), fset: oc.pkg.Fset, file: oc.owner.FilePath, owner: oc.owner,
			stmts: stmts, byCanonical: byCanonical, fields: fields, methods: methods, lhs: assignedExprs(oc.root, oc.info(), true)}
		w.region(oc.root, ctl{})
	})
	return c.edges, st
}

// ctl is the statement a region of code lies in, and which part of it.
type ctl struct {
	node *types.Node
	kind string // cond, then, else, body, case, go
	// lit is set inside a function literal written in the statement: the
	// statement still encloses what the literal holds, but the literal's
	// writes are not the statement's.
	lit bool
	// hdr is the statement whose header holds this region from further out:
	// set for the parts of a statement that is itself written inside a
	// function literal in that header.
	hdr *types.Node
}

// writes reports whether what the region writes is recorded for the
// statement's body.
func (c ctl) writes() bool { return c.node != nil && !c.lit && c.kind != "cond" }

// header returns the innermost statement whose header holds the region, or
// nil: the statement itself in its own header, otherwise the one from
// further out.
func (c ctl) header() *types.Node {
	if c.kind == "cond" && c.node != nil {
		return c.node
	}
	return c.hdr
}

// inLiteral is c for the body of a function literal written in the region.
func (c ctl) inLiteral() ctl { return ctl{node: c.node, kind: c.kind, lit: true, hdr: c.hdr} }

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
	owner       *types.Node // the function or variable whose code is walked
	stmts       map[stmtKey]*types.Node
	byCanonical map[string]*types.Node
	fields      fieldIndex
	methods     posIndex
	lhs         map[ast.Expr]bool // what the assignments and increments in the body write (assignedExprs)
}

func (w *stmtWalker) line(p token.Pos) int { return w.fset.Position(p).Line }

func (w *stmtWalker) node(t types.NodeType, p token.Pos) *types.Node {
	n := w.stmts[stmtKey{w.file, w.fset.Position(p).Offset, 0, t}]
	if n == nil {
		w.st.NoNode++
	}
	return n
}

// site returns the CallSite node of the call x.
func (w *stmtWalker) site(x *ast.CallExpr) *types.Node {
	n := w.stmts[stmtKey{w.file, w.fset.Position(x.Pos()).Offset, w.fset.Position(x.End()).Offset, types.NodeCallSite}]
	if n == nil {
		w.st.NoNode++
	}
	return n
}

// opSite returns the site node of the channel operation spanning [from, to),
// or nil: an operation on a local channel bound to its make site has an edge
// to the Channel node and no site.
func (w *stmtWalker) opSite(from, to token.Pos) *types.Node {
	return w.stmts[stmtKey{w.file, w.fset.Position(from).Offset, w.fset.Position(to).Offset, types.NodeCallSite}]
}

// inner is the context for the parts of a statement: the statement itself
// when it has a node, otherwise the context around it.
func inner(n *types.Node, outer ctl, kind string) ctl {
	if n == nil {
		return outer
	}
	return ctl{node: n, kind: kind, hdr: outer.header()}
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
			w.region(x.Body, c.inLiteral())
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
			cc := inner(s, c, "cond")
			// A range over a channel receives; its site is part of the
			// loop header, as a call in the range expression is.
			w.enclose(cc, w.opSite(x.Range, x.X.End()), x.Range)
			w.region(x.X, cc)
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
		case *ast.SendStmt:
			// Send and receive sites lie in the statement around them
			// like call sites (841 of go-stablenet had no encloses edge).
			w.enclose(c, w.opSite(x.Pos(), x.End()), x.Pos())
		case *ast.UnaryExpr:
			if x.Op == token.ARROW {
				w.enclose(c, w.opSite(x.Pos(), x.End()), x.Pos())
			}
		case *ast.CallExpr:
			callee, kind := w.resolve(x)
			if s := w.site(x); s != nil {
				w.enclose(c, s, x.Pos())
				w.c.emit(s, callee, types.EdgeSiteCalls, kind, w.line(x.Pos()))
			}
			if callee == nil {
				w.st.Unresolved++
			} else if h := c.header(); h != nil {
				w.c.emit(h, callee, types.EdgeCondCalls, "", w.line(x.Pos()))
			}
		case *ast.AssignStmt:
			if c.writes() && x.Tok != token.DEFINE {
				for _, lhs := range x.Lhs {
					w.written(lhs, c)
				}
			}
		case *ast.IncDecStmt:
			if c.writes() {
				w.written(x.X, c)
			}
		case *ast.SelectorExpr:
			if h := c.header(); h != nil && !w.lhs[x] {
				if sel := w.info.Selections[x]; sel != nil && sel.Kind() == gotypes.FieldVal {
					w.c.emit(h, w.fields.node(sel.Obj()), types.EdgeCondReads, "", w.line(x.Sel.Pos()))
				}
			}
		case *ast.Ident:
			if h := c.header(); h != nil && !w.lhs[x] {
				if v := w.pkgValue(x); v != nil {
					w.c.emit(h, v, types.EdgeCondReads, "", w.line(x.Pos()))
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
	// The go statement evaluates its function expression and its arguments
	// itself, before the goroutine starts: the calls in them are its own,
	// not the surrounding statement's (13 encloses edges of go-stablenet
	// started at the statement around the go statement).
	own := inner(g, c, "go")
	if lit, ok := x.Call.Fun.(*ast.FuncLit); ok {
		w.region(lit.Body, own)
	} else {
		callee, kind := w.resolve(x.Call)
		if callee == nil {
			w.st.Unresolved++
		}
		w.c.emit(g, callee, types.EdgeSiteCalls, kind, w.line(x.Pos()))
		w.region(x.Call.Fun, own)
	}
	for _, a := range x.Call.Args {
		w.region(a, own)
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
			w.c.emit(c.node, w.fields.node(sel.Obj()), types.EdgeBodyWrites, c.kind, w.line(x.Sel.Pos()))
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

// Dispatch kinds of a site_calls edge.
const (
	dispatchStatic          = "static"
	dispatchInterfaceMethod = "interface_method"
)

// resolve is callee, and records the call for the owner when the declaration
// pass could not: the callee has no canonical id for Resolve to match (a
// method of a local or unnamed interface, always an interface call).
func (w *stmtWalker) resolve(call *ast.CallExpr) (*types.Node, string) {
	callee, kind := w.callee(call)
	if callee != nil && callee.CanonicalID == "" && kind == dispatchInterfaceMethod {
		w.c.emit(w.owner, callee, types.EdgeInvokes, kind, w.line(call.Pos()))
	}
	return callee, kind
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
	kind := dispatchStatic
	switch x := fun.(type) {
	case *ast.Ident:
		obj = w.info.Uses[x]
	case *ast.SelectorExpr:
		if sel := w.info.Selections[x]; sel != nil {
			if sel.Kind() != gotypes.MethodVal {
				return nil, ""
			}
			obj = sel.Obj()
			if f, ok := obj.(*gotypes.Func); ok && isInterfaceMethod(f) {
				kind = dispatchInterfaceMethod
			}
		} else {
			obj = w.info.Uses[x.Sel] // pkg.Func
		}
	}
	fn, ok := obj.(*gotypes.Func)
	if !ok {
		return nil, ""
	}
	cid := goCanonicalID(fn)
	if n := w.byCanonical[cid]; n != nil {
		return n, kind
	}
	if cid == "" {
		if n := w.methods.declaredAt(fn.Name(), fn.Pos()); n != nil {
			return n, kind
		}
	}
	return nil, ""
}
