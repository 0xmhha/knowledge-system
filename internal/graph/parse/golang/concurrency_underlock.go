package golang

import (
	"go/ast"
	gotypes "go/types"
)

// concurrency_underlock.go records which struct fields each function body
// touches, for buildpipe's cross-function lock propagation pass (opt-in
// --lock-propagation). The accessed_under_lock edges of a function that locks
// a mutex itself are made after Resolve, with the lock edges (lock_edges.go).

// recordFuncFieldTouches stashes the funcID → set-of-field-IDs touched by
// body, regardless of whether body holds a lock. Consumed by buildpipe's
// cross-function lock propagation pass (opt-in --lock-propagation).
//
// No-op when typesInfo is nil (AST-only mode) or fieldNodeIDs is empty
// (no struct fields declared in this file's scope — see emitFields).
//
// Idempotent per funcID: callers should invoke at most once per FuncDecl
// (visitFuncDecl wires it that way). Re-invocation merges into the existing
// set so deferred passes don't lose data.
func (v *declVisitor) recordFuncFieldTouches(funcID string, body *ast.BlockStmt) {
	if v.typesInfo == nil || body == nil || len(v.fieldNodeIDs) == 0 {
		return
	}
	fields := v.collectFieldAccesses(body)
	if len(fields) == 0 {
		return
	}
	if v.funcFieldTouches == nil {
		v.funcFieldTouches = map[string]map[string]struct{}{}
	}
	dst := v.funcFieldTouches[funcID]
	if dst == nil {
		dst = make(map[string]struct{}, len(fields))
		v.funcFieldTouches[funcID] = dst
	}
	for fid := range fields {
		dst[fid] = struct{}{}
	}
}

// collectFieldAccesses returns the set of Field node IDs referenced
// anywhere inside body. Reference detection is intentionally permissive:
// any *ast.SelectorExpr whose Sel resolves (via typesInfo.ObjectOf) to an
// object in fieldNodeIDs counts.
//
// Why a set (not slice): a field can be touched many times in a function
// body (`x.counter++; x.counter++`), but accessed_under_lock collapses
// that to one edge per (field, mutex) pair.
func (v *declVisitor) collectFieldAccesses(body *ast.BlockStmt) map[string]struct{} {
	out := map[string]struct{}{}
	ast.Inspect(body, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		obj := v.typesInfo.ObjectOf(sel.Sel)
		if obj == nil {
			return true
		}
		// Skip method receivers — `s.Lock()` resolves Sel to a *types.Func,
		// not a *types.Var. fieldNodeIDs is keyed on *types.Var (struct
		// fields), so this rejects naturally, but we early-out for clarity.
		if _, isVar := obj.(*gotypes.Var); !isVar {
			return true
		}
		if id, ok := v.fieldNodeIDs[obj]; ok {
			out[id] = struct{}{}
		}
		return true
	})
	return out
}
