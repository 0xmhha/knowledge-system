package verify

import (
	"go/ast"
	"go/token"
	gotypes "go/types"
	"strings"

	"golang.org/x/tools/go/types/typeutil"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// literaluse.go is the truth of the literal_passed_to edges from a FuncLit
// (schema 1.45): what the callee does with the literal it is passed, read
// off the callee's syntax, not its SSA as the build does. Each use of the
// parameter is classified by where it stands: the function of a call
// (call), of a go statement (go), of a defer statement (defer); the right
// side of an assignment to anything but a local variable, an element of a
// composite literal, the value of a send (store); an argument, a result, a
// use inside a function literal (pass, where the literal stands). A use
// runs on every path when no branch (if, for, range, switch, select, a
// clause) lies between the callee's body and it (or the literal it is in)
// and no return ends before it; the kind then carries EXTRACTED, else
// INFERRED. A panic is not a return: code after if err != nil { panic(err) }
// runs on every path that returns. An assignment to a local variable is
// not followed (the build follows the register; neither names it).

// litKindMark separates the use kinds of a literal_passed_to fact from its
// confidence ("call/EXTRACTED"), so the comparison checks both.
const litKindMark = "/"

// literalUse returns the fact kind of the literal passed as argument i of
// call, "" when the callee has no declaration here or does nothing with it.
func (w *fileWalker) literalUse(call *ast.CallExpr, i int) string {
	fn := typeutil.StaticCallee(w.info, call)
	if fn == nil || w.funcs == nil {
		return ""
	}
	p := w.fset.Position(fn.Origin().Pos())
	f, ok := w.funcs[token.Position{Filename: p.Filename, Offset: p.Offset}]
	if !ok || f.decl.Body == nil {
		return ""
	}
	param := paramAt(f, i)
	if param == nil {
		return ""
	}
	uses := map[string]bool{}
	always := false
	var stack []ast.Node
	firstReturn := token.NoPos // the end of the first return outside literals
	ast.Inspect(f.decl.Body, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		if r, ok := n.(*ast.ReturnStmt); ok && innermostLiteral(stack) == nil && !firstReturn.IsValid() {
			firstReturn = r.End()
		}
		if id, ok := n.(*ast.Ident); ok && f.info.Uses[id] == param {
			if k, branched := useKind(stack, id, f.info, f.decl.Body); k != "" {
				uses[k] = true
				if !branched && (!firstReturn.IsValid() || id.Pos() < firstReturn) {
					always = true
				}
			}
		}
		stack = append(stack, n)
		return true
	})
	var kinds []string
	for _, k := range types.LiteralUses() {
		if uses[k] {
			kinds = append(kinds, k)
		}
	}
	if len(kinds) == 0 {
		return ""
	}
	conf := types.ConfInferred
	if always {
		conf = types.ConfExtracted
	}
	return strings.Join(kinds, types.LiteralUseSep) + litKindMark + string(conf)
}

// literalVars returns the local variables of body set once, to a function
// literal (f := func() {...}), with that literal; a variable assigned again
// is left out.
func literalVars(info *gotypes.Info, body ast.Node) map[gotypes.Object]*ast.FuncLit {
	out := map[gotypes.Object]*ast.FuncLit{}
	again := map[gotypes.Object]bool{}
	set := func(l ast.Expr, r ast.Expr) {
		id, ok := ast.Unparen(l).(*ast.Ident)
		if !ok {
			return
		}
		obj := info.ObjectOf(id)
		if obj == nil {
			return
		}
		lit, isLit := ast.Unparen(r).(*ast.FuncLit)
		if _, seen := out[obj]; seen || !isLit || again[obj] {
			again[obj] = true
			delete(out, obj)
			return
		}
		out[obj] = lit
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			for i, l := range x.Lhs {
				var r ast.Expr
				if len(x.Rhs) == len(x.Lhs) {
					r = x.Rhs[i]
				}
				set(l, r)
			}
		case *ast.ValueSpec:
			for i, id := range x.Names {
				var r ast.Expr
				if len(x.Values) == len(x.Names) {
					r = x.Values[i]
				}
				set(id, r)
			}
		}
		return true
	})
	return out
}

// paramAt returns the object of parameter i of f, nil for a variadic one
// or one past the end.
func paramAt(f truthFunc, i int) gotypes.Object {
	n := 0
	list := f.decl.Type.Params.List
	for fi, field := range list {
		if _, variadic := field.Type.(*ast.Ellipsis); variadic && fi == len(list)-1 {
			return nil
		}
		for _, id := range field.Names {
			if n == i {
				return f.info.Defs[id]
			}
			n++
		}
		if len(field.Names) == 0 {
			n++
		}
	}
	return nil
}

// useKind classifies the use id of a parameter by its ancestors.
// A use inside a function literal of the callee takes, when it calls the
// parameter, the kind of how that literal runs; its other uses keep their
// own kind. branched says a branch lies between the callee's body and the
// use: around the use inside the literal, around the literal, or around
// the call that runs a literal held in a local variable.
func useKind(stack []ast.Node, id *ast.Ident, info *gotypes.Info, body ast.Node) (string, bool) {
	for i := len(stack) - 1; i >= 0; i-- {
		lit, ok := stack[i].(*ast.FuncLit)
		if !ok {
			continue
		}
		inner := branchIn(stack[i+1:])
		k := valueKind(stack[i+1:], id, info, body)
		if k != types.LiteralUseCall {
			return k, inner || branchIn(stack[:i])
		}
		outer, runBranched := valueKindRun(stack[:i], lit, info, body)
		return outer, inner || runBranched
	}
	return valueKind(stack, id, info, body), branchIn(stack)
}

// valueKindRun is valueKind for a literal of the callee, and whether a
// branch lies around where it runs (where it stands, or where the local
// variable holding it is called).
func valueKindRun(stack []ast.Node, lit *ast.FuncLit, info *gotypes.Info, body ast.Node) (string, bool) {
	if len(stack) == 0 {
		return "", false
	}
	if p, ok := stack[len(stack)-1].(*ast.AssignStmt); ok {
		for i, r := range p.Rhs {
			if r == ast.Expr(lit) && i < len(p.Lhs) {
				if l, isID := p.Lhs[i].(*ast.Ident); isID && !isPackageLevel(info.ObjectOf(l)) {
					return localRunKind(info, body, info.ObjectOf(l))
				}
			}
		}
	}
	return valueKind(stack, lit, info, body), branchIn(stack)
}

// branchIn reports whether stack holds a statement whose parts do not all
// run (if, for, range, switch, select, a clause).
func branchIn(stack []ast.Node) bool {
	for _, n := range stack {
		switch n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt,
			*ast.CaseClause, *ast.CommClause:
			return true
		}
	}
	return false
}

// valueKind classifies what the code around it does with the value e (the
// parameter, or a literal that uses it), by its ancestors stack.
func valueKind(stack []ast.Node, e ast.Expr, info *gotypes.Info, body ast.Node) string {
	if len(stack) == 0 {
		return ""
	}
	id := e
	switch p := stack[len(stack)-1].(type) {
	case *ast.ParenExpr:
		return valueKind(stack[:len(stack)-1], p, info, body)
	case *ast.CallExpr:
		if ast.Unparen(p.Fun) == id {
			if len(stack) >= 2 {
				switch g := stack[len(stack)-2].(type) {
				case *ast.GoStmt:
					if g.Call == p {
						return types.LiteralUseGo
					}
				case *ast.DeferStmt:
					if g.Call == p {
						return types.LiteralUseDefer
					}
				}
			}
			return types.LiteralUseCall
		}
		return types.LiteralUsePass
	case *ast.AssignStmt:
		for i, r := range p.Rhs {
			if r == id && i < len(p.Lhs) {
				if l, ok := p.Lhs[i].(*ast.Ident); ok && !isPackageLevel(info.ObjectOf(l)) {
					// A local: what is done with it where it is used.
					if _, isLit := e.(*ast.FuncLit); isLit {
						k, _ := localRunKind(info, body, info.ObjectOf(l))
						return k
					}
					return ""
				}
				return types.LiteralUseStore
			}
		}
	case *ast.KeyValueExpr:
		if p.Value == id {
			return types.LiteralUseStore
		}
	case *ast.CompositeLit:
		return types.LiteralUseStore
	case *ast.SendStmt:
		if p.Value == id {
			return types.LiteralUseSend
		}
	case *ast.ReturnStmt:
		return types.LiteralUseStore
	}
	return ""
}

// localRunKind is how the literal held by the local variable obj runs:
// the kind of the first use of obj in body as a call, go or defer, pass
// otherwise; and whether a branch lies around that use.
func localRunKind(info *gotypes.Info, body ast.Node, obj gotypes.Object) (string, bool) {
	kind, branched := types.LiteralUsePass, false
	var stack []ast.Node
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		if id, ok := n.(*ast.Ident); ok && info.Uses[id] == obj && kind == types.LiteralUsePass {
			if k := valueKind(stack, id, info, body); k == types.LiteralUseCall || k == types.LiteralUseGo || k == types.LiteralUseDefer {
				kind, branched = k, branchIn(stack)
			}
		}
		stack = append(stack, n)
		return true
	})
	return kind, branched
}

// isPackageLevel reports whether obj is a package-level variable.
func isPackageLevel(obj gotypes.Object) bool {
	v, ok := obj.(*gotypes.Var)
	return ok && v.Pkg() != nil && v.Parent() == v.Pkg().Scope()
}
