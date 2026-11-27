package verify

import (
	"go/ast"
	"go/token"
	gotypes "go/types"
	"sort"

	"golang.org/x/tools/go/types/typeutil"
)

// paramflow.go is the truth of operates_on for a sync primitive a function
// receives as a parameter. The build follows the value through SSA and the
// VTA call graph (sync_flow.go); this check follows the syntax instead, the
// static calls go/types resolves, and shares no code with it.
//
// The rule both sides keep: the edge from the operation's site names every
// source of the parameter, or nothing. The sources of a parameter are what
// every call of its function passes for it, followed through the callers'
// own parameters up to paramFlowDepth calls: a field, a package-level or a
// local variable (its declaration), a call of package context or time (its
// site). Anything else (a value another function returns, a function no
// call reaches, a parameter of an exported function, which code outside the
// program can call) is unresolved and makes the expected set empty: the
// site must have no edge.
//
// What the syntax cannot decide is left out on both sides (undecided): a
// parameter of a method or of a function literal (a method may be called
// through an interface, a literal through a function value), a function
// used as a value, a variadic parameter, a parameter its function assigns
// or takes the address of, a call that spreads a slice (f(xs...)).

// paramFlowDepth bounds how many calls a parameter is followed through, as
// the build's syncFlowDepth.
const paramFlowDepth = 5

// paramInfo is a parameter of a plain function declaration.
type paramInfo struct {
	fn       string // the function's key
	index    int
	variadic bool
	assigned bool
	exported bool // of an exported function
}

// paramCall is a static call of a plain function, in a file of rel ("" when
// the file is outside the covered files).
type paramCall struct {
	info *gotypes.Info
	call *ast.CallExpr
	rel  string
}

// paramOp is an operation on a parameter, by its "site|op" key.
type paramOp struct {
	key   string
	param string
}

// paramFlow collects the calls and parameters of the program and the
// operations to decide.
type paramFlow struct {
	params map[string]paramInfo
	calls  map[string][]paramCall
	// others are the parameters of methods and function literals, and
	// receivers: not decided.
	others map[string]bool
	// values are the functions used other than as the function of a call;
	// callees are the identifiers that name the function of a call, by
	// package, to tell the two apart.
	values  map[string]bool
	callees map[*gotypes.Info]map[*ast.Ident]bool
	seen    map[token.Position]bool
	ops     []paramOp
}

func newParamFlow() *paramFlow {
	return &paramFlow{params: map[string]paramInfo{}, calls: map[string][]paramCall{}, others: map[string]bool{},
		values: map[string]bool{}, callees: map[*gotypes.Info]map[*ast.Ident]bool{}, seen: map[token.Position]bool{}}
}

// collect records the plain functions, their parameters and their static
// calls in file f, whose path is rel ("" outside the covered files).
func (pf *programFacts) collectParams(info *gotypes.Info, f *ast.File, rel string) {
	fl := pf.pflow
	callees := fl.callees[info]
	if callees == nil {
		callees = map[*ast.Ident]bool{}
		fl.callees[info] = callees
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncDecl:
			if x.Recv == nil && x.Body != nil {
				pf.declareParams(info, x)
			} else {
				pf.otherParams(info, x.Recv, x.Type)
			}
		case *ast.FuncLit:
			pf.otherParams(info, nil, x.Type)
		case *ast.CallExpr:
			switch fun := ast.Unparen(x.Fun).(type) {
			case *ast.Ident:
				callees[fun] = true
			case *ast.SelectorExpr:
				callees[fun.Sel] = true
			}
			fn := typeutil.StaticCallee(info, x)
			if fn == nil || fn.Type().(*gotypes.Signature).Recv() != nil {
				return true
			}
			k := pf.key(fn.Origin())
			at := pf.fset.Position(x.Lparen)
			at = token.Position{Filename: at.Filename, Offset: at.Offset}
			if k == "" || fl.seen[at] {
				return true
			}
			fl.seen[at] = true
			fl.calls[k] = append(fl.calls[k], paramCall{info: info, call: x, rel: rel})
		}
		return true
	})
}

// otherParams records the receiver and parameters of a method or a
// function literal as undecided.
func (pf *programFacts) otherParams(info *gotypes.Info, recv *ast.FieldList, ft *ast.FuncType) {
	for _, fl := range []*ast.FieldList{recv, ft.Params} {
		if fl == nil {
			continue
		}
		for _, field := range fl.List {
			for _, id := range field.Names {
				if k := pf.key(info.Defs[id]); k != "" {
					pf.pflow.others[k] = true
				}
			}
		}
	}
}

// functionValues records the plain functions used other than as the
// function of a call, once every file is collected.
func (pf *programFacts) functionValues() {
	for info, callees := range pf.pflow.callees {
		for id, obj := range info.Uses {
			fn, ok := obj.(*gotypes.Func)
			if !ok || callees[id] {
				continue
			}
			if sig, _ := fn.Type().(*gotypes.Signature); sig != nil && sig.Recv() == nil {
				if k := pf.key(fn.Origin()); k != "" {
					pf.pflow.values[k] = true
				}
			}
		}
	}
}

// declareParams records the parameters of the plain function d.
func (pf *programFacts) declareParams(info *gotypes.Info, d *ast.FuncDecl) {
	fn, ok := info.Defs[d.Name].(*gotypes.Func)
	if !ok {
		return
	}
	k := pf.key(fn)
	if k == "" {
		return
	}
	assigned := assignedVars(info, d.Body)
	i := 0
	list := d.Type.Params.List
	for fi, field := range list {
		_, variadic := field.Type.(*ast.Ellipsis)
		for _, id := range field.Names {
			if obj := info.Defs[id]; obj != nil {
				if pk := pf.key(obj); pk != "" {
					pf.pflow.params[pk] = paramInfo{fn: k, index: i, variadic: variadic && fi == len(list)-1, assigned: assigned[obj],
						exported: fn.Exported()}
				}
			}
			i++
		}
		if len(field.Names) == 0 {
			i++
		}
	}
}

// assignedVars returns the variables body assigns, increments or takes the
// address of.
func assignedVars(info *gotypes.Info, body *ast.BlockStmt) map[gotypes.Object]bool {
	out := map[gotypes.Object]bool{}
	mark := func(e ast.Expr) {
		if id, ok := ast.Unparen(e).(*ast.Ident); ok {
			if obj := info.ObjectOf(id); obj != nil {
				out[obj] = true
			}
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			for _, l := range x.Lhs {
				mark(l)
			}
		case *ast.IncDecStmt:
			mark(x.X)
		case *ast.UnaryExpr:
			if x.Op == token.AND {
				mark(x.X)
			}
		}
		return true
	})
	return out
}

// paramKey returns the key of the variable e names (after &, * and
// conversions), "" when e names none.
func (pf *programFacts) paramKey(info *gotypes.Info, e ast.Expr) string {
	for {
		switch x := stripConversions(info, e).(type) {
		case *ast.UnaryExpr:
			if x.Op != token.AND {
				return ""
			}
			e = x.X
			continue
		case *ast.StarExpr:
			e = x.X
			continue
		case *ast.Ident:
			return pf.key(info.ObjectOf(x))
		}
		return ""
	}
}

// stripConversions returns e without parentheses and type conversions.
func stripConversions(info *gotypes.Info, e ast.Expr) ast.Expr {
	for {
		e = ast.Unparen(e)
		call, ok := e.(*ast.CallExpr)
		if !ok || len(call.Args) != 1 {
			return e
		}
		if tv, ok := info.Types[call.Fun]; !ok || !tv.IsType() {
			return e
		}
		e = call.Args[0]
	}
}

// storesOf returns the fields and package-level variables body stores the
// variable param (a key) into (x.f = v, T{f: v}, T{v}, pkgVar = v), and
// whether every such store names one. Assignments to locals are not stores
// of a holder.
func (pf *programFacts) storesOf(info *gotypes.Info, body *ast.BlockStmt, param string) ([]string, bool) {
	isParam := func(e ast.Expr) bool {
		id, ok := ast.Unparen(e).(*ast.Ident)
		return ok && pf.key(info.ObjectOf(id)) == param
	}
	var out []string
	complete := true
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			if len(x.Lhs) != len(x.Rhs) {
				return true
			}
			for i, r := range x.Rhs {
				if !isParam(r) || isParam(x.Lhs[i]) {
					continue
				}
				switch l := ast.Unparen(x.Lhs[i]).(type) {
				case *ast.SelectorExpr:
					if k := pf.holder(info, l, nil); k != "" {
						out = append(out, k)
					} else {
						complete = false
					}
				case *ast.Ident:
					if k := pf.packageVar(info.ObjectOf(l)); k != "" {
						out = append(out, k)
					}
				default:
					complete = false
				}
			}
		case *ast.CompositeLit:
			st, ok := derefStruct(info.TypeOf(x))
			if !ok {
				return true
			}
			for i, elt := range x.Elts {
				var f gotypes.Object
				v := elt
				if kv, isKV := elt.(*ast.KeyValueExpr); isKV {
					v = kv.Value
					if id, isID := kv.Key.(*ast.Ident); isID {
						f = info.ObjectOf(id)
					}
				} else if i < st.NumFields() {
					f = st.Field(i)
				}
				if !isParam(v) {
					continue
				}
				if fv, isVar := f.(*gotypes.Var); isVar {
					if k := pf.key(fv.Origin()); k != "" {
						out = append(out, k)
						continue
					}
				}
				complete = false
			}
		}
		return true
	})
	return out, complete
}

// paramResult is the outcome of following a parameter.
type paramResult int

const (
	paramResolved paramResult = iota
	paramUnresolved
	paramUndecided
)

// resolveParams decides the operations on parameters: their expected
// sources, an empty set, or left out.
func (pf *programFacts) resolveParams() {
	pf.functionValues()
	for _, op := range pf.pflow.ops {
		srcs := map[string]bool{}
		switch pf.follow(op.param, 0, srcs, map[string]bool{}) {
		case paramUndecided:
			pf.exec.undecided[op.key] = true
		case paramUnresolved:
			// Expected: no edge at all, and the site marked.
			pf.exec.unknown[op.key] = true
		case paramResolved:
			keys := make([]string, 0, len(srcs))
			for k := range srcs {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				pf.exec.sync[[2]string{op.key, k}] = true
			}
		}
	}
}

// follow adds the sources of the parameter param to srcs.
func (pf *programFacts) follow(param string, depth int, srcs, busy map[string]bool) paramResult {
	p, ok := pf.pflow.params[param]
	switch {
	case !ok || p.variadic || p.assigned || pf.pflow.values[p.fn]:
		return paramUndecided
	case depth > paramFlowDepth || p.exported:
		return paramUnresolved
	case busy[param]:
		return paramResolved
	}
	busy[param] = true
	defer delete(busy, param)
	calls := pf.pflow.calls[p.fn]
	if len(calls) == 0 {
		return paramUnresolved
	}
	result := paramResolved
	for _, c := range calls {
		if c.call.Ellipsis.IsValid() || p.index >= len(c.call.Args) {
			return paramUndecided
		}
		switch r := pf.argSource(c, c.call.Args[p.index], depth, srcs, busy); r {
		case paramUndecided:
			return r
		case paramUnresolved:
			result = r
		}
	}
	return result
}

// argSource adds the source of the argument e of call c to srcs.
func (pf *programFacts) argSource(c paramCall, e ast.Expr, depth int, srcs, busy map[string]bool) paramResult {
	if c.rel == "" {
		return paramUnresolved
	}
	if pk := pf.paramKey(c.info, e); pk != "" {
		if _, isParam := pf.pflow.params[pk]; isParam {
			return pf.follow(pk, depth+1, srcs, busy)
		}
		if pf.pflow.others[pk] {
			return paramUndecided // a parameter of a method or a literal
		}
	}
	if k := pf.source(c.info, c.rel, e); k != "" {
		srcs[k] = true
		return paramResolved
	}
	return paramUnresolved
}

// source returns the key of the source e is, other than a parameter: the
// declaration that holds it, or the site of a call of package context or
// time; "" when there is none.
func (pf *programFacts) source(info *gotypes.Info, rel string, e ast.Expr) string {
	e = stripConversions(info, e)
	if k := pf.holder(info, e, nil); k != "" {
		return k
	}
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return ""
	}
	fn, ok := typeutil.Callee(info, call).(*gotypes.Func)
	if !ok || fn.Pkg() == nil {
		return ""
	}
	switch fn.Pkg().Path() {
	case "context", "time":
		return pf.span(rel, call.Pos(), call.End())
	}
	return ""
}
