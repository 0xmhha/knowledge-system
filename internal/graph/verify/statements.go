package verify

import (
	"fmt"
	"go/ast"
	"go/token"
	gotypes "go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/types/typeutil"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// statements.go checks the statement links of schema 1.32 (site_calls,
// cond_reads, cond_calls, body_writes, encloses) against facts computed here
// from go/types and the syntax tree.
//
// The build walks a body region by region (parse/golang/statement_links.go).
// The truth here is computed another way, so that the two can disagree: one
// flat walk keeps the stack of syntax nodes above each node, and the
// innermost control statement and its part (cond, then, else, body, case,
// go) are read off that stack. Canonical ids are formatted by this package
// (canonicalID, fieldID), not the parser's.
//
// A truth fact whose statement has no node in the graph (the build makes
// only call-site nodes inside a goroutine's function literal, and none for
// the call of a go statement itself) is counted apart, as is a fact whose
// value or callee has no node (outside the scope).

// StmtFact is one statement link: from a statement to a value, a callee or a
// statement, in one part of the source statement, on a line.
type StmtFact struct {
	Rel  string `json:"rel"`  // the edge type
	File string `json:"file"` // the source statement's file
	Src  string `json:"src"`  // "<NodeType>@L<line>"
	Dst  string `json:"dst"`  // canonical id, or "<NodeType>@L<line>" for a statement
	Kind string `json:"kind"` // dispatch_kind
	Line int    `json:"line"`
}

func (f StmtFact) String() string {
	return fmt.Sprintf("%s:%s %s %s %s L%d", f.File, f.Src, f.Rel, f.Dst, f.Kind, f.Line)
}

// StmtRelStats compares one statement-link type.
type StmtRelStats struct {
	Truth   int `json:"truth"`
	Matched int `json:"matched"`
	Missing int `json:"missing"`
	// NoStatementNode: a statement of the fact has no node in the graph.
	NoStatementNode int `json:"no_statement_node"`
	// EndpointMissing: the value or callee has no node in the graph.
	EndpointMissing int `json:"endpoint_missing"`
	// Unexpected: a graph edge with no truth fact.
	Unexpected int `json:"unexpected"`
	// OuterStatement: a graph edge with no truth fact whose truth fact (same
	// value or statement, line) starts at a statement without a node: the
	// build gives it to the statement around that one.
	OuterStatement    int      `json:"outer_statement"`
	MissingSamples    []string `json:"missing_samples,omitempty"`
	UnexpectedSamples []string `json:"unexpected_samples,omitempty"`
}

// Precision is Matched over the graph edges compared (matched + unexpected).
func (s StmtRelStats) Precision() float64 {
	if s.Matched+s.Unexpected == 0 {
		return 0
	}
	return float64(s.Matched) / float64(s.Matched+s.Unexpected)
}

// Recall is Matched over the truth facts the graph could hold.
func (s StmtRelStats) Recall() float64 {
	d := s.Matched + s.Missing
	if d == 0 {
		return 0
	}
	return float64(s.Matched) / float64(d)
}

var stmtRels = []types.EdgeType{types.EdgeSiteCalls, types.EdgeCondReads, types.EdgeCondCalls, types.EdgeBodyWrites, types.EdgeEncloses}

var stmtNodeTypes = map[types.NodeType]bool{
	types.NodeIfStmt: true, types.NodeLoopStmt: true, types.NodeSwitchStmt: true,
	types.NodeReturnStmt: true, types.NodeCallSite: true, types.NodeGoroutine: true,
}

// stmtKind is the graph node type a statement of the syntax tree becomes.
func stmtKind(n ast.Node) types.NodeType {
	switch n.(type) {
	case *ast.IfStmt:
		return types.NodeIfStmt
	case *ast.ForStmt, *ast.RangeStmt:
		return types.NodeLoopStmt
	case *ast.SwitchStmt, *ast.TypeSwitchStmt:
		return types.NodeSwitchStmt
	case *ast.ReturnStmt:
		return types.NodeReturnStmt
	case *ast.CallExpr:
		return types.NodeCallSite
	case *ast.GoStmt:
		return types.NodeGoroutine
	}
	return ""
}

// statements adds the statement facts of one function body.
func (w *fileWalker) statements(d *ast.FuncDecl) {
	if w.stmt == nil {
		return
	}
	writes := writtenExprs(d.Body)
	key := func(n ast.Node) string {
		return fmt.Sprintf("%s@L%d", stmtKind(n), w.fset.Position(n.Pos()).Line)
	}
	add := func(rel types.EdgeType, src ast.Node, dst, kind string, p token.Pos) {
		w.stmt[StmtFact{Rel: string(rel), File: w.rel, Src: key(src), Dst: dst, Kind: kind, Line: w.fset.Position(p).Line}] = true
	}
	var stack []ast.Node
	ast.Inspect(d.Body, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		ctl, part := enclosingControl(stack, n)
		if stmtKind(n) != "" && !isGoCall(stack, n) {
			if ctl != nil && part != "" {
				add(types.EdgeEncloses, ctl, key(n), part, n.Pos())
			}
		}
		switch x := n.(type) {
		case *ast.CallExpr:
			if dst, kind := w.callTarget(x); dst != "" {
				if !isGoCall(stack, n) {
					add(types.EdgeSiteCalls, x, dst, kind, x.Pos())
				}
				if part == "cond" {
					add(types.EdgeCondCalls, ctl, dst, "", x.Pos())
				}
			}
		case *ast.GoStmt:
			if x.Call != nil {
				if _, lit := x.Call.Fun.(*ast.FuncLit); !lit {
					if dst, kind := w.callTarget(x.Call); dst != "" {
						add(types.EdgeSiteCalls, x, dst, kind, x.Pos())
					}
				}
			}
		case *ast.SelectorExpr:
			if s := w.info.Selections[x]; s != nil && s.Kind() == gotypes.FieldVal {
				if id := fieldID(s); id != "" && w.inScopeID(s.Obj()) {
					w.valueFact(add, ctl, part, writes[x], id, x.Sel.Pos())
				}
			}
		case *ast.Ident:
			obj := w.info.Uses[x]
			if v, ok := obj.(*gotypes.Var); ok && v.IsField() {
				break
			}
			switch obj.(type) {
			case *gotypes.Var, *gotypes.Const:
				if id := canonicalID(obj); id != "" && w.inScope(obj) {
					w.valueFact(add, ctl, part, writes[x], id, x.Pos())
				}
			}
		}
		stack = append(stack, n)
		return true
	})
}

func (w *fileWalker) valueFact(add func(types.EdgeType, ast.Node, string, string, token.Pos), ctl ast.Node, part string, write bool, id string, p token.Pos) {
	switch {
	case ctl == nil || part == "":
	case part == "cond":
		if !write {
			add(types.EdgeCondReads, ctl, id, "", p)
		}
	case write:
		add(types.EdgeBodyWrites, ctl, id, part, p)
	}
}

func (w *fileWalker) inScopeID(obj gotypes.Object) bool { return w.inScope(obj) }

// callTarget is the canonical id of what a call names in scope, and
// "static" or "interface_method".
func (w *fileWalker) callTarget(c *ast.CallExpr) (string, string) {
	if fn := typeutil.StaticCallee(w.info, c); fn != nil {
		if fn.Origin() != nil {
			fn = fn.Origin()
		}
		if w.inScope(fn) {
			return canonicalID(fn), "static"
		}
		return "", ""
	}
	if sel, ok := ast.Unparen(c.Fun).(*ast.SelectorExpr); ok {
		if s := w.info.Selections[sel]; s != nil && s.Kind() == gotypes.MethodVal {
			if fn, ok := s.Obj().(*gotypes.Func); ok && isInterfaceMethod(fn) && w.inScope(fn) {
				return canonicalID(fn), "interface_method"
			}
		}
	}
	return "", ""
}

// isGoCall reports whether n is the call of a go statement (go f(x) or
// go func(){}()), which is the Goroutine itself, not a call site.
func isGoCall(stack []ast.Node, n ast.Node) bool {
	if len(stack) == 0 {
		return false
	}
	g, ok := stack[len(stack)-1].(*ast.GoStmt)
	return ok && ast.Node(g.Call) == n
}

// enclosingControl returns the innermost control statement around n, read
// off the stack of its ancestors, and the part of it n lies in. A function
// literal ends the search, except the literal a go statement runs, which
// belongs to that go statement ("go").
func enclosingControl(stack []ast.Node, n ast.Node) (ast.Node, string) {
	at := func(i int) ast.Node {
		if i < len(stack) {
			return stack[i]
		}
		if i == len(stack) {
			return n
		}
		return nil
	}
	for i := len(stack) - 1; i >= 0; i-- {
		child := at(i + 1)
		switch a := stack[i].(type) {
		case *ast.FuncLit:
			if i >= 2 {
				// Only the literal the go statement runs (go func(){...}()),
				// not a literal passed as one of its arguments.
				if g, ok := stack[i-2].(*ast.GoStmt); ok && ast.Node(g.Call) == stack[i-1] && ast.Node(g.Call.Fun) == stack[i] {
					return g, "go"
				}
			}
			return nil, ""
		case *ast.IfStmt:
			switch child {
			case ast.Node(a.Init), ast.Node(a.Cond):
				return a, "cond"
			case ast.Node(a.Body):
				return a, "then"
			case ast.Node(a.Else):
				return a, "else"
			}
		case *ast.ForStmt:
			switch child {
			case ast.Node(a.Init), ast.Node(a.Cond), ast.Node(a.Post):
				return a, "cond"
			case ast.Node(a.Body):
				return a, "body"
			}
		case *ast.RangeStmt:
			switch child {
			case ast.Node(a.X):
				return a, "cond"
			case ast.Node(a.Body):
				return a, "body"
			}
			return nil, "" // key and value: not a part with links
		case *ast.SwitchStmt:
			if child == ast.Node(a.Init) || child == ast.Node(a.Tag) {
				return a, "cond"
			}
			if child == ast.Node(a.Body) {
				return a, casePart(at(i+2), at(i+3))
			}
		case *ast.TypeSwitchStmt:
			if child == ast.Node(a.Init) || child == ast.Node(a.Assign) {
				return a, "cond"
			}
			if child == ast.Node(a.Body) {
				return a, casePart(at(i+2), at(i+3))
			}
		}
	}
	return nil, ""
}

// casePart says whether x, a child of case clause cl, is a case expression
// or a statement of the clause's body.
func casePart(cl, x ast.Node) string {
	cc, ok := cl.(*ast.CaseClause)
	if !ok || x == nil {
		return ""
	}
	for _, e := range cc.List {
		if ast.Node(e) == x {
			return "cond"
		}
	}
	return "case"
}

// writtenExprs marks the expressions an assignment or increment writes:
// x.F, pkg.V, V, and m[k] into m.
func writtenExprs(body ast.Node) map[ast.Node]bool {
	out := map[ast.Node]bool{}
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

// fieldID formats the field a selection reaches as <importpath>.<Type>.<Field>,
// following embedded fields to the struct that declares it.
func fieldID(s *gotypes.Selection) string {
	t := s.Recv()
	idx := s.Index()
	for k, i := range idx {
		u := gotypes.Unalias(t)
		if p, ok := u.(*gotypes.Pointer); ok {
			u = gotypes.Unalias(p.Elem())
		}
		st, ok := u.Underlying().(*gotypes.Struct)
		if !ok || i >= st.NumFields() {
			return ""
		}
		f := st.Field(i)
		if k == len(idx)-1 {
			named, ok := u.(*gotypes.Named)
			if !ok || named.Origin().Obj().Pkg() == nil {
				return "" // a field of an anonymous struct
			}
			o := named.Origin().Obj()
			return o.Pkg().Path() + "." + o.Name() + "." + f.Name()
		}
		t = f.Type()
	}
	return ""
}

// compareStatements classifies truth facts and graph edges per type.
func compareStatements(truth map[StmtFact]bool, nodes []types.Node, g GraphReader, samples int) (map[string]StmtRelStats, error) {
	byID := make(map[string]types.Node, len(nodes))
	stmtNodes := map[string]bool{} // file + "|" + Type@L
	canon := map[string]bool{}
	for _, n := range nodes {
		byID[n.ID] = n
		if stmtNodeTypes[n.Type] {
			stmtNodes[n.FilePath+"|"+fmt.Sprintf("%s@L%d", n.Type, n.StartLine)] = true
		}
		if n.CanonicalID != "" {
			canon[baseID(n.CanonicalID)] = true
		}
	}
	ref := func(n types.Node) string {
		if stmtNodeTypes[n.Type] {
			return fmt.Sprintf("%s@L%d", n.Type, n.StartLine)
		}
		return baseID(n.CanonicalID)
	}
	graph := map[StmtFact]bool{}
	edges := 0
	for _, rel := range stmtRels {
		es, err := g.QueryEdgesByType(string(rel))
		if err != nil {
			return nil, fmt.Errorf("verify: read %s edges: %w", rel, err)
		}
		edges += len(es)
		for _, e := range es {
			s, d := byID[e.Src], byID[e.Dst]
			graph[StmtFact{Rel: string(rel), File: s.FilePath, Src: ref(s), Dst: ref(d), Kind: e.DispatchKind, Line: e.Line}] = true
		}
	}
	if edges == 0 {
		return nil, nil // a graph before schema 1.32: nothing to compare
	}
	out := map[string]StmtRelStats{}
	sample := func(list *[]string, f StmtFact) {
		if len(*list) < samples {
			*list = append(*list, f.String())
		}
	}
	isStmt := func(ref string) bool {
		i := strings.Index(ref, "@L")
		return i > 0 && stmtNodeTypes[types.NodeType(ref[:i])]
	}
	facts := make([]StmtFact, 0, len(truth))
	for f := range truth {
		facts = append(facts, f)
	}
	sort.Slice(facts, func(i, j int) bool { return facts[i].String() < facts[j].String() })
	for _, f := range facts {
		s := out[f.Rel]
		s.Truth++
		switch {
		case graph[f]:
			s.Matched++
		case !stmtNodes[f.File+"|"+f.Src] || isStmt(f.Dst) && !stmtNodes[f.File+"|"+f.Dst]:
			s.NoStatementNode++
		case !isStmt(f.Dst) && !canon[f.Dst]:
			s.EndpointMissing++
		default:
			s.Missing++
			sample(&s.MissingSamples, f)
		}
		out[f.Rel] = s
	}
	gf := make([]StmtFact, 0, len(graph))
	for f := range graph {
		gf = append(gf, f)
	}
	sort.Slice(gf, func(i, j int) bool { return gf[i].String() < gf[j].String() })
	// Truth facts whose source statement has no node, by everything but the
	// source and the part.
	type loose struct {
		rel, file, dst string
		line           int
	}
	orphan := map[loose]bool{}
	for f := range truth {
		if !stmtNodes[f.File+"|"+f.Src] {
			orphan[loose{f.Rel, f.File, f.Dst, f.Line}] = true
		}
	}
	for _, f := range gf {
		if truth[f] {
			continue
		}
		s := out[f.Rel]
		if orphan[loose{f.Rel, f.File, f.Dst, f.Line}] {
			s.OuterStatement++
		} else {
			s.Unexpected++
			sample(&s.UnexpectedSamples, f)
		}
		out[f.Rel] = s
	}
	return out, nil
}
