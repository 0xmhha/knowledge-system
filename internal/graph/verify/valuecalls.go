package verify

import (
	"fmt"
	"go/ast"
	"go/token"
	gotypes "go/types"
	"sort"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// valuecalls.go is the truth side of the calls through a function value,
// read from go/types and the syntax, sharing no code with the extraction.
//
// A call through a local variable is a call of one known function when the
// variable is assigned once, where it is declared, a function or a
// concrete method (f := inc; g := b.get; var h = pkg.F), its address is
// never taken, and the call is in the function (or literal) that declares
// it. That is a static call and the graph must have its calls edge.
//
// Every other call through a value (DynamicSite) is classified by what the
// value is, and compared with the graph: a site has a target when its
// caller has a calls or dispatches_to edge to another node on its line.

// Causes of a dynamic call site: what the called value is.
const (
	causeOwnLiteral = "own_literal" // a literal of the caller, called by name: the caller's own code
	causeLocal      = "local"       // a local variable set more than once or from a value
	causeParam      = "param"       // a parameter or result
	causeField      = "field"       // a struct field
	causeGlobal     = "global"      // a package-level variable
	causeResult     = "result"      // a local set from a call's results (ctx, cancel := ...): its callee may be outside the build
	causeOther      = "other"       // a call result, an index, a type assertion
)

// DynamicSite is a call through a function value the truth cannot name the
// callee of.
type DynamicSite struct {
	Caller string // the caller's key
	File   string
	Line   int
	Cause  string
}

// TargetStats counts the dynamic call sites of one cause and how many of
// them have a target in the graph.
type TargetStats struct {
	Sites      int      `json:"sites"`
	WithTarget int      `json:"with_target"`
	Samples    []string `json:"samples_without_target,omitempty"`
}

// localValue is what the truth knows of one local variable of a file.
type localValue struct {
	sets  int
	value ast.Expr
	home  ast.Node // the function or literal that declares it
	addr  bool
	param bool
}

// localValues indexes the local variables of f.
func (w *fileWalker) localValues(f *ast.File) {
	w.locals = map[*gotypes.Var]*localValue{}
	w.funcSpans = nil
	get := func(id *ast.Ident) *localValue {
		v, ok := w.info.ObjectOf(id).(*gotypes.Var)
		if !ok || v.IsField() || v.Pkg() == nil || v.Parent() == v.Pkg().Scope() {
			return nil
		}
		l := w.locals[v]
		if l == nil {
			l = &localValue{}
			w.locals[v] = l
		}
		return l
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl, *ast.FuncLit:
			w.funcSpans = append(w.funcSpans, x)
			var ft *ast.FuncType
			if d, ok := x.(*ast.FuncDecl); ok {
				ft = d.Type
				if d.Recv != nil {
					w.markParams(d.Recv, get)
				}
			} else {
				ft = x.(*ast.FuncLit).Type
			}
			w.markParams(ft.Params, get)
			w.markParams(ft.Results, get)
		case *ast.AssignStmt:
			for i, lhs := range x.Lhs {
				id, ok := ast.Unparen(lhs).(*ast.Ident)
				if !ok {
					continue
				}
				if l := get(id); l != nil {
					l.sets++
					if len(x.Lhs) == len(x.Rhs) && x.Tok == token.DEFINE {
						l.value = x.Rhs[i]
						l.home = w.funcAt(id.Pos())
					}
				}
			}
		case *ast.ValueSpec:
			for i, id := range x.Names {
				if l := get(id); l != nil && len(x.Values) == len(x.Names) {
					l.sets++
					l.value = x.Values[i]
					l.home = w.funcAt(id.Pos())
				}
			}
		case *ast.RangeStmt:
			for _, e := range []ast.Expr{x.Key, x.Value} {
				if id, ok := e.(*ast.Ident); ok {
					if l := get(id); l != nil {
						l.sets += 2 // set on every iteration
					}
				}
			}
		case *ast.UnaryExpr:
			if id, ok := ast.Unparen(x.X).(*ast.Ident); ok && x.Op == token.AND {
				if l := get(id); l != nil {
					l.addr = true
				}
			}
		}
		return true
	})
}

func (w *fileWalker) markParams(fl *ast.FieldList, get func(*ast.Ident) *localValue) {
	if fl == nil {
		return
	}
	for _, f := range fl.List {
		for _, id := range f.Names {
			if l := get(id); l != nil {
				l.param = true
			}
		}
	}
}

// funcAt returns the innermost function declaration or literal of the file
// that holds p. funcSpans is in source order, outer before inner.
func (w *fileWalker) funcAt(p token.Pos) ast.Node {
	var in ast.Node
	for _, n := range w.funcSpans {
		if n.Pos() <= p && p < n.End() {
			in = n
		}
	}
	return in
}

// valueCallee returns the one function a call through a value calls, or
// nil, and otherwise what the value is.
func (w *fileWalker) valueCallee(c *ast.CallExpr) (*gotypes.Func, string) {
	switch x := ast.Unparen(c.Fun).(type) {
	case *ast.Ident:
		v, ok := w.info.ObjectOf(x).(*gotypes.Var)
		if !ok {
			return nil, causeOther
		}
		if v.Pkg() != nil && v.Parent() == v.Pkg().Scope() {
			return nil, causeGlobal
		}
		l := w.locals[v]
		if l == nil || l.param {
			return nil, causeParam
		}
		if _, lit := ast.Unparen(l.value).(*ast.FuncLit); lit {
			return nil, causeOwnLiteral
		}
		if l.sets == 1 && l.value == nil {
			return nil, causeResult
		}
		if l.sets != 1 || l.addr || l.value == nil || l.home != w.funcAt(c.Pos()) {
			return nil, causeLocal
		}
		if fn := w.funcValue(l.value); fn != nil {
			return fn, ""
		}
		return nil, causeLocal
	case *ast.SelectorExpr:
		if s := w.info.Selections[x]; s != nil && s.Kind() == gotypes.FieldVal {
			return nil, causeField
		}
		if v, ok := w.info.Uses[x.Sel].(*gotypes.Var); ok && !v.IsField() {
			return nil, causeGlobal
		}
	}
	return nil, causeOther
}

// funcValue returns the function or concrete method e denotes: a function
// name, a method value or a method expression.
func (w *fileWalker) funcValue(e ast.Expr) *gotypes.Func {
	var fn *gotypes.Func
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		fn, _ = w.info.Uses[x].(*gotypes.Func)
	case *ast.SelectorExpr:
		if s := w.info.Selections[x]; s != nil {
			if s.Kind() == gotypes.FieldVal {
				return nil
			}
			fn, _ = s.Obj().(*gotypes.Func)
		} else {
			fn, _ = w.info.Uses[x.Sel].(*gotypes.Func)
		}
	}
	if fn == nil || isInterfaceMethod(fn) {
		return nil
	}
	return fn.Origin()
}

// markStaticLine notes that caller makes a static call on line.
func (t *Truth) markStaticLine(caller string, line int) {
	if t.StaticLines == nil {
		t.StaticLines = map[string]map[int]bool{}
	}
	if t.StaticLines[caller] == nil {
		t.StaticLines[caller] = map[int]bool{}
	}
	t.StaticLines[caller][line] = true
}

// compareDynamicSites counts, per cause, the dynamic sites whose caller has
// a dispatches_to edge to another node on the site's line, or a calls edge
// on a line where it makes no static call.
func compareDynamicSites(t *Truth, declNode map[string]string, g GraphReader, samples int) (map[string]TargetStats, error) {
	sites := t.DynamicSites
	staticAt := map[string]map[int]bool{} // by caller node
	for k, lines := range t.StaticLines {
		staticAt[declNode[k]] = lines
	}
	type at struct {
		src  string
		line int
	}
	target := map[at]bool{}
	for _, et := range []types.EdgeType{types.EdgeCalls, types.EdgeDispatchesTo} {
		es, err := g.QueryEdgesByType(string(et))
		if err != nil {
			return nil, fmt.Errorf("verify: read %s: %w", et, err)
		}
		for _, e := range es {
			if e.Src == e.Dst || e.Line == 0 || et == types.EdgeCalls && staticAt[e.Src][e.Line] {
				continue
			}
			target[at{e.Src, e.Line}] = true
		}
	}
	sort.Slice(sites, func(i, j int) bool {
		a, b := sites[i], sites[j]
		if a.File != b.File {
			return a.File < b.File
		}
		return a.Line < b.Line
	})
	out := map[string]TargetStats{}
	for _, s := range sites {
		st := out[s.Cause]
		st.Sites++
		if target[at{declNode[s.Caller], s.Line}] {
			st.WithTarget++
		} else if len(st.Samples) < samples {
			st.Samples = append(st.Samples, fmt.Sprintf("%s:%d", s.File, s.Line))
		}
		out[s.Cause] = st
	}
	return out, nil
}
