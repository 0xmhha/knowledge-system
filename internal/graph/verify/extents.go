package verify

import (
	"fmt"
	"go/ast"
	gotypes "go/types"
	"strings"

	"golang.org/x/tools/go/types/typeutil"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// extents.go checks two schema 1.45 facts against the syntax and go/types:
//
//   - the extent of each break, continue, goto and fallthrough, case clause
//     (of a switch, type switch or select), defer statement and function
//     literal: a node of its type, sub kind, first and last line;
//   - flows_to of kind sync: a sync primitive (a context, a cancel
//     function, a *time.Timer or *Ticker, a *sync.WaitGroup, *Once or *Cond)
//     held by a local variable or a field, assigned to a field or a
//     package-level variable (x.f = v, T{f: v}); the build reads the same
//     from SSA stores. A local is followed only when every assignment to it
//     is a call's result (a value made there); one assigned from anything
//     else, a parameter in particular, is left out (the build follows a
//     parameter to its callers). A primitive made in place by a call of
//     package context or time (c.ctx, c.cancel = context.WithCancel(p)) is
//     held by that call's site.

// extentKey names one extent fact.
func extentKey(file string, t types.NodeType, sub string, from, to int) string {
	return fmt.Sprintf("%s:%s:%s:L%d-L%d", file, t, sub, from, to)
}

// extentNodeTypes are the node types whose extent is checked.
var extentNodeTypes = map[types.NodeType]bool{types.NodeBranchStmt: true, types.NodeCaseClause: true,
	types.NodeDeferStmt: true, types.NodeFuncLit: true}

// extentOf returns the extent fact of n, "" for a node of another kind.
func (w *fileWalker) extentOf(n ast.Node) string {
	t := stmtKind(n)
	if !extentNodeTypes[t] {
		return ""
	}
	sub := ""
	switch x := n.(type) {
	case *ast.BranchStmt:
		sub = x.Tok.String()
	case *ast.CaseClause:
		sub = types.SubKindCase
		if x.List == nil {
			sub = types.SubKindDefault
		}
	case *ast.CommClause:
		sub = types.SubKindCase
		if x.Comm == nil {
			sub = types.SubKindDefault
		}
	}
	return extentKey(w.rel, t, sub, w.fset.Position(n.Pos()).Line, w.fset.Position(n.End()).Line)
}

// compareExtents compares the extent facts with the graph's nodes of those
// types, by file, type, sub kind and lines.
func compareExtents(truth map[string]bool, idx nodeIndex, samples int) UnderLockStats {
	t := map[[2]string]bool{}
	for k := range truth {
		t[[2]string{k, ""}] = true
	}
	g := map[[2]string]bool{}
	for _, n := range idx.byID {
		if extentNodeTypes[n.Type] {
			g[[2]string{extentKey(n.FilePath, n.Type, n.SubKind, n.StartLine, n.EndLine), ""}] = true
		}
	}
	return comparePairs(t, g, nil, samples)
}

// syncValueTypes are the sync primitive types a store is followed for, by
// "<import path>.<name>" and whether a pointer to it is meant.
var syncValueTypes = map[string]bool{
	"context.Context": false, "context.CancelFunc": false, "context.CancelCauseFunc": false,
	"time.Timer": true, "time.Ticker": true, "sync.WaitGroup": true, "sync.Once": true, "sync.Cond": true,
}

// isSyncValue reports whether t is the type of a sync primitive value.
func isSyncValue(t gotypes.Type) bool {
	if t == nil {
		return false
	}
	_, isPtr := gotypes.Unalias(t).(*gotypes.Pointer)
	named, ok := gotypes.Unalias(derefType(t)).(*gotypes.Named)
	if !ok || named.Obj().Pkg() == nil {
		return false
	}
	want, known := syncValueTypes[named.Obj().Pkg().Path()+"."+named.Obj().Name()]
	return known && want == isPtr
}

// syncAliases records the sync stores of the function d (see the top).
func (pf *programFacts) syncAliases(info *gotypes.Info, d *ast.FuncDecl, rel string) {
	madeLocals := callAssignedLocals(info, d.Body)
	// A parameter holds what its callers pass, even when the body assigns
	// it a call's result: not a value made here.
	ast.Inspect(d, func(n ast.Node) bool {
		var ft *ast.FuncType
		switch x := n.(type) {
		case *ast.FuncDecl:
			ft = x.Type
		case *ast.FuncLit:
			ft = x.Type
		}
		if ft != nil && ft.Params != nil {
			for _, f := range ft.Params.List {
				for _, id := range f.Names {
					if v, ok := info.Defs[id].(*gotypes.Var); ok {
						delete(madeLocals, v)
					}
				}
			}
		}
		return true
	})
	holderOf := func(e ast.Expr) (string, bool) {
		switch x := ast.Unparen(e).(type) {
		case *ast.Ident:
			v, ok := info.ObjectOf(x).(*gotypes.Var)
			if !ok {
				return "", false
			}
			if isPackageLevel(v) {
				return pf.key(v), true
			}
			if !madeLocals[v] {
				return "", false
			}
			return pf.key(v), true
		case *ast.SelectorExpr:
			if s := info.Selections[x]; s != nil && s.Kind() == gotypes.FieldVal {
				return pf.key(s.Obj().(*gotypes.Var).Origin()), true
			}
		case *ast.CallExpr:
			if madeInPlace(info, x) || isDoneCall(info, x) {
				return pf.span(rel, x.Pos(), x.End()), true
			}
		}
		return "", false
	}
	field := func(l ast.Expr) string {
		switch lv := ast.Unparen(l).(type) {
		case *ast.SelectorExpr:
			if s := info.Selections[lv]; s != nil && s.Kind() == gotypes.FieldVal {
				return pf.key(s.Obj().(*gotypes.Var).Origin())
			}
		case *ast.Ident:
			if v, ok := info.ObjectOf(lv).(*gotypes.Var); ok && isPackageLevel(v) {
				return pf.key(v)
			}
		}
		return ""
	}
	store := func(dst string, value ast.Expr, line int) {
		if call, ok := ast.Unparen(value).(*ast.CallExpr); ok && dst != "" && isDoneCall(info, call) {
			pf.noteDone(info, dst, call)
		} else if dst == "" || !isSyncValue(info.TypeOf(value)) {
			return
		}
		if named, ok := gotypes.Unalias(info.TypeOf(value)).(*gotypes.Named); ok && named.Obj().Pkg() != nil &&
			named.Obj().Pkg().Path() == "context" && strings.HasPrefix(named.Obj().Name(), "Cancel") {
			pf.exec.cancelFields[dst] = true
		}
		at := fmt.Sprintf("%s|%d", dst, line)
		h, ok := holderOf(value)
		if !ok || h == "" {
			pf.exec.syncUndecided[at] = true
			return
		}
		if h != dst {
			pf.exec.syncAlias[[2]string{at, h}] = true
		}
	}
	ast.Inspect(d.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			line := pf.fset.Position(x.Pos()).Line
			if len(x.Rhs) == 1 && len(x.Lhs) > 1 {
				// a.ctx, a.cancel = context.WithCancel(p): each result is
				// made at the call.
				call, ok := ast.Unparen(x.Rhs[0]).(*ast.CallExpr)
				tuple, isTuple := info.TypeOf(x.Rhs[0]).(*gotypes.Tuple)
				for i, l := range x.Lhs {
					dst := field(l)
					if dst == "" || !isTuple || i >= tuple.Len() || !isSyncValue(tuple.At(i).Type()) {
						continue
					}
					if named, ok := gotypes.Unalias(tuple.At(i).Type()).(*gotypes.Named); ok && named.Obj().Pkg() != nil &&
						named.Obj().Pkg().Path() == "context" && strings.HasPrefix(named.Obj().Name(), "Cancel") {
						pf.exec.cancelFields[dst] = true
					}
					at := fmt.Sprintf("%s|%d", dst, line)
					if ok && madeInPlace(info, call) {
						pf.exec.syncAlias[[2]string{at, pf.span(rel, call.Pos(), call.End())}] = true
					} else {
						pf.exec.syncUndecided[at] = true
					}
				}
				return true
			}
			if len(x.Lhs) != len(x.Rhs) {
				return true
			}
			for i, l := range x.Lhs {
				store(field(l), x.Rhs[i], line)
			}
		case *ast.CompositeLit:
			st, ok := derefStruct(info.TypeOf(x))
			if !ok {
				return true
			}
			for i, elt := range x.Elts {
				if kv, isKV := elt.(*ast.KeyValueExpr); isKV {
					if id, ok := kv.Key.(*ast.Ident); ok {
						if v, ok := info.ObjectOf(id).(*gotypes.Var); ok && v.IsField() {
							store(pf.key(v.Origin()), kv.Value, pf.fset.Position(kv.Value.Pos()).Line)
						}
					}
				} else if i < st.NumFields() {
					store(pf.key(st.Field(i).Origin()), elt, pf.fset.Position(elt.Pos()).Line)
				}
			}
		}
		return true
	})
}

// isDoneCall reports whether call is ctx.Done() of a context.Context.
func isDoneCall(info *gotypes.Info, call *ast.CallExpr) bool {
	name, _, fn := calleeOf(info, call)
	return fn != nil && fullNameOps[name] == types.SyncContextDone
}

// noteDone records that the field dst holds the Done channel of the
// context call is made on: its holder, or undecided (nil) when the
// context is a parameter or no declaration names it.
func (pf *programFacts) noteDone(info *gotypes.Info, dst string, call *ast.CallExpr) {
	_, recv, _ := calleeOf(info, call)
	h := ""
	if recv != nil {
		h = pf.holder(info, stripConversions(info, recv), nil)
	}
	if v, ok := info.ObjectOf(identOf(recv)).(*gotypes.Var); ok && !v.IsField() && !isPackageLevel(v) && pf.pflow.others[pf.key(v)] ||
		h == "" || pf.pflow.params[h] != (paramInfo{}) {
		pf.exec.doneFields[dst] = nil
		pf.exec.doneUndecided[dst] = true
		return
	}
	if !pf.exec.doneUndecided[dst] {
		pf.exec.doneFields[dst] = append(pf.exec.doneFields[dst], h)
	}
}

// identOf returns e as an identifier, nil when it is none.
func identOf(e ast.Expr) *ast.Ident {
	if e == nil {
		return nil
	}
	id, _ := ast.Unparen(e).(*ast.Ident)
	return id
}

// heldUses decides the calls through and the receives from fields once
// every store is known: a call through a field a cancel function is stored
// into is a cancel on the field; a receive from a field a context's Done is
// stored into is a done_recv of each of those contexts, left out when one
// is undecided.
func (pf *programFacts) heldUses() {
	for _, c := range pf.exec.fieldCalls {
		if pf.exec.cancelFields[c[1]] {
			pf.exec.sync[[2]string{c[0], c[1]}] = true
		}
	}
	for _, r := range pf.exec.fieldRecvs {
		if pf.exec.doneUndecided[r[1]] {
			pf.exec.undecided[r[0]] = true
			continue
		}
		for _, h := range pf.exec.doneFields[r[1]] {
			pf.exec.sync[[2]string{r[0], h}] = true
		}
	}
}

// madeInPlace reports whether call is a call of a function of package
// context or time (a primitive made there).
func madeInPlace(info *gotypes.Info, call *ast.CallExpr) bool {
	fn, ok := typeutil.Callee(info, call).(*gotypes.Func)
	if !ok || fn.Pkg() == nil {
		return false
	}
	switch fn.Pkg().Path() {
	case "context", "time":
		return true
	}
	return false
}

// callAssignedLocals returns the local variables of body every assignment
// of which is a call's result.
func callAssignedLocals(info *gotypes.Info, body *ast.BlockStmt) map[*gotypes.Var]bool {
	ok := map[*gotypes.Var]bool{}
	bad := map[*gotypes.Var]bool{}
	note := func(l ast.Expr, fromCall bool) {
		id, isID := ast.Unparen(l).(*ast.Ident)
		if !isID {
			return
		}
		v, isVar := info.ObjectOf(id).(*gotypes.Var)
		if !isVar || isPackageLevel(v) {
			return
		}
		if fromCall {
			ok[v] = true
		} else {
			bad[v] = true
		}
	}
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			if len(x.Rhs) == 1 && len(x.Lhs) > 1 {
				_, isCall := ast.Unparen(x.Rhs[0]).(*ast.CallExpr)
				for _, l := range x.Lhs {
					note(l, isCall)
				}
				return true
			}
			for i, l := range x.Lhs {
				if i < len(x.Rhs) {
					_, isCall := ast.Unparen(x.Rhs[i]).(*ast.CallExpr)
					note(l, isCall)
				}
			}
		case *ast.ValueSpec:
			for i, id := range x.Names {
				fromCall := false
				if len(x.Values) == 1 && len(x.Names) > 1 {
					_, fromCall = ast.Unparen(x.Values[0]).(*ast.CallExpr)
				} else if i < len(x.Values) {
					_, fromCall = ast.Unparen(x.Values[i]).(*ast.CallExpr)
				}
				note(id, fromCall)
			}
		}
		return true
	})
	for v := range bad {
		delete(ok, v)
	}
	return ok
}

// compareSyncAliases compares the flows_to edges of kind sync with the
// truth, by (store place and line, holder).
func compareSyncAliases(m *execFacts, idx nodeIndex, edges []types.Edge, samples int) UnderLockStats {
	g := map[[2]string]bool{}
	for _, e := range edges {
		if e.DispatchKind != syncAliasKind {
			continue
		}
		src, dst := idx.byID[e.Src], idx.byID[e.Dst]
		g[[2]string{fmt.Sprintf("%s|%d", placeKey(dst), e.Line), placeKey(src)}] = true
	}
	return comparePairs(m.syncAlias, g, func(k [2]string) bool { return m.syncUndecided[k[0]] }, samples)
}

// syncAliasKind is the dispatch kind of a flows_to edge of a sync store.
const syncAliasKind = "sync"
