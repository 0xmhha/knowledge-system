package golang

import (
	"go/ast"
	"go/token"
	gotypes "go/types"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/types/typeutil"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// value_calls.go keeps one rule for calls: every call site has a calls or
// invokes edge to its callee, or a dispatches_to edge to what it may run.
// A call through a function value has no callee in go/types, so the syntax
// gives it only a marker (an invokes edge from the caller to itself); the
// value flow (VTA) gives its targets, but skips a call whose one callee SSA
// knows, as it would skip a static call: a local set once (f := inc; f(1);
// defer f(1)) or a method value (g := b.get; g()). Such a call gets the
// plain calls edge here, at its line: which function runs is a static fact.
//
// A literal the caller calls (twice := func(...){...}; twice(5)) runs the
// caller's own code; it has no node of its own, and no edge.

// dynamicCalls returns the opening parentheses of the calls of pkgs that
// go/types gives no callee and that are not calls of an interface method,
// a conversion, a builtin or a literal written in place.
func dynamicCalls(pkgs []*packages.Package) map[token.Pos]bool {
	out := map[token.Pos]bool{}
	for _, pkg := range pkgs {
		info := pkg.TypesInfo
		if info == nil {
			continue
		}
		for _, f := range pkg.Syntax {
			ast.Inspect(f, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok || typeutil.StaticCallee(info, call) != nil {
					return true
				}
				if _, lit := ast.Unparen(call.Fun).(*ast.FuncLit); lit {
					return true
				}
				if tv, ok := info.Types[call.Fun]; ok && (tv.IsType() || tv.IsBuiltin()) {
					return true
				}
				if sel, ok := ast.Unparen(call.Fun).(*ast.SelectorExpr); ok {
					if s := info.Selections[sel]; s != nil && s.Kind() == gotypes.MethodVal {
						if fn, ok := s.Obj().(*gotypes.Func); ok && abstractMethod(fn) {
							return true // an interface method call: invokes
						}
					}
				}
				out[call.Lparen] = true
				return true
			})
		}
	}
	return out
}

// valueCalls returns the calls edges of the calls through a function value
// whose one callee SSA knows (see the file comment), one per caller,
// callee and line, with the uses counted.
func valueCalls(prog *ssa.Program, funcs map[*ssa.Function]bool, pkgs []*packages.Package,
	nodeOf func(*ssa.Function) *types.Node) []types.Edge {
	dynamic := dynamicCalls(pkgs)
	type key struct {
		src, dst string
		line     int
	}
	at := map[key]int{}
	var out []types.Edge
	for fn := range funcs {
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				ci, ok := in.(ssa.CallInstruction)
				if !ok {
					continue
				}
				common := ci.Common()
				callee := common.StaticCallee()
				if callee == nil || common.IsInvoke() || !dynamic[common.Pos()] {
					continue
				}
				if m, ok := callee.Object().(*gotypes.Func); ok && abstractMethod(m) {
					continue
				}
				src, dst := nodeOf(fn), nodeOf(callee)
				if src == nil || dst == nil || src.ID == dst.ID {
					continue
				}
				k := key{src.ID, dst.ID, prog.Fset.Position(common.Pos()).Line}
				if i, ok := at[k]; ok {
					out[i].Count++
					continue
				}
				at[k] = len(out)
				out = append(out, types.Edge{Src: src.ID, Dst: dst.ID, Type: types.EdgeCalls, Line: k.line, Count: 1,
					Confidence: types.ConfExtracted, FilePath: src.FilePath})
			}
		}
	}
	return out
}
