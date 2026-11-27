package golang

import (
	"go/ast"
	"go/token"
	gotypes "go/types"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/tools/go/ssa"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// literal_uses.go says what a callee does with a function literal it is
// passed (schema 1.45): a literal_passed_to edge from the literal's FuncLit
// node to the callee, with the kinds of the callee's uses of the parameter
// as its dispatch kind (types.LiteralUse*, joined by "+"): call (the callee
// calls it before it returns), go, defer, store (into a field, a variable,
// a map, a channel) and pass (as an argument, in a closure, as a result).
// It is read off the SSA of the callee: the referrers of the parameter,
// through phis, conversions and the cell a captured parameter is spilled
// to; a call of it inside a closure of the callee takes the kind of how the
// closure runs (go func() { f() }() is go). Instances of generic functions
// count as their origin. The edge is EXTRACTED when one of those uses is in a block that
// dominates every return of the callee (it happens on every path), and
// INFERRED when each depends on a branch. A callee without an SSA body (a
// function value, code outside the build) gives no edge; a literal whose
// callee does nothing with it gives none either.
//
// The calls written in the literal keep their literal_passed_to edges of
// kind arg to the callee (statement_links.go); this edge says when they
// run.

// literalUseDepth bounds how many copies of the parameter are followed.
const literalUseDepth = 8

// literalUses returns the literal_passed_to edges of the literals funcs
// pass to callees with SSA bodies.
func (c *chanLinker) literalUses(funcs map[*ssa.Function]bool, nodes []types.Node) []types.Edge {
	lits := map[valueAt]*types.Node{}
	for i := range nodes {
		if n := &nodes[i]; n.Type == types.NodeFuncLit {
			lits[valueAt{file: n.FilePath, offset: n.StartByte}] = n
		}
	}
	files := newSiteIndex(nodes)
	fns := make([]*ssa.Function, 0, len(funcs))
	for fn := range funcs {
		fns = append(fns, fn)
	}
	sort.Slice(fns, func(i, j int) bool {
		return fns[i].Pos() < fns[j].Pos() || fns[i].Pos() == fns[j].Pos() && fns[i].String() < fns[j].String()
	})
	seen := map[[4]string]bool{}
	var out []types.Edge
	for _, fn := range fns {
		for _, b := range fn.Blocks {
			for _, in := range b.Instrs {
				call, ok := in.(ssa.CallInstruction)
				if !ok {
					continue
				}
				common := call.Common()
				callee := common.StaticCallee()
				if callee == nil || len(callee.Blocks) == 0 || callee.Parent() != nil || callee.Synthetic != "" && callee.Origin() == nil {
					continue
				}
				dst := c.nodeOf(callee)
				if dst == nil {
					continue
				}
				for i, a := range common.Args {
					lit := c.literalNode(a, lits, files)
					if lit == nil || i >= len(callee.Params) {
						continue
					}
					kind, always := paramUses(callee, callee.Params[i])
					if kind == "" {
						continue
					}
					line := c.prog.Fset.Position(common.Pos()).Line
					if k := [4]string{lit.ID, dst.ID, kind, strconv.Itoa(line)}; !seen[k] {
						seen[k] = true
						conf := types.ConfInferred
						if always {
							conf = types.ConfExtracted
						}
						out = append(out, types.Edge{Src: lit.ID, Dst: dst.ID, Type: types.EdgeLiteralPassedTo, DispatchKind: kind,
							Line: line, Count: 1, Confidence: conf, FilePath: lit.FilePath})
					}
				}
			}
		}
	}
	return out
}

// literalNode returns the FuncLit node of the literal v is, or nil.
func (c *chanLinker) literalNode(v ssa.Value, lits map[valueAt]*types.Node, files *siteIndex) *types.Node {
	var fn *ssa.Function
	// A literal passed as a value of a named function type (or an
	// interface) is converted on the way.
	for {
		switch x := v.(type) {
		case *ssa.ChangeType:
			v = x.X
			continue
		case *ssa.MakeInterface:
			v = x.X
			continue
		case *ssa.Phi:
			// var f F; if c { f = func() {...} }: the one value that is
			// not nil.
			if only := onlyNonNil(x); only != nil {
				v = only
				continue
			}
		case *ssa.UnOp:
			// Read from the cell of a captured variable set once, here or
			// in the function a closure is made in.
			if s := onlyStore(x); s != nil {
				v = s.Val
				continue
			}
			if fv, ok := x.X.(*ssa.FreeVar); ok && x.Op == token.MUL {
				if b := freeVarBindings(fv); len(b) == 1 {
					v = &ssa.UnOp{Op: token.MUL, X: b[0]}
					if s := onlyStore(v.(*ssa.UnOp)); s != nil {
						v = s.Val
						continue
					}
				}
			}
		}
		break
	}
	switch x := v.(type) {
	case *ssa.MakeClosure:
		fn, _ = x.Fn.(*ssa.Function)
	case *ssa.Function:
		fn = x
	}
	if fn == nil || fn.Parent() == nil {
		return nil
	}
	lit, ok := fn.Syntax().(*ast.FuncLit)
	if !ok {
		return nil
	}
	p := c.prog.Fset.Position(lit.Pos())
	return lits[valueAt{file: files.file(p), offset: p.Offset}]
}

// freeVarBindings returns the values the closures of fv's function are
// made with for fv.
func freeVarBindings(fv *ssa.FreeVar) []ssa.Value {
	fn := fv.Parent()
	idx := -1
	for i, f := range fn.FreeVars {
		if f == fv {
			idx = i
		}
	}
	parent := fn.Parent()
	if idx < 0 || parent == nil {
		return nil
	}
	var out []ssa.Value
	for _, b := range parent.Blocks {
		for _, in := range b.Instrs {
			if mc, ok := in.(*ssa.MakeClosure); ok && mc.Fn == fn && idx < len(mc.Bindings) {
				out = append(out, mc.Bindings[idx])
			}
		}
	}
	return out
}

// onlyNonNil returns the one edge of the phi that is not the nil constant,
// nil when there are several.
func onlyNonNil(p *ssa.Phi) ssa.Value {
	var only ssa.Value
	for _, e := range p.Edges {
		if k, isConst := e.(*ssa.Const); isConst && k.IsNil() {
			continue
		}
		if only != nil {
			return nil
		}
		only = e
	}
	return only
}

// onlyStore returns the one store into the cell a load u reads, nil when
// u is no load of a cell or the cell is stored more than once.
func onlyStore(u *ssa.UnOp) *ssa.Store {
	cell, ok := u.X.(*ssa.Alloc)
	if u.Op != token.MUL || !ok || cell.Referrers() == nil {
		return nil
	}
	var only *ssa.Store
	for _, r := range *cell.Referrers() {
		if s, isStore := r.(*ssa.Store); isStore && s.Addr == cell {
			if only != nil {
				return nil
			}
			only = s
		}
	}
	return only
}

// paramUses returns the kinds of fn's uses of the parameter p, joined in
// the order of types.LiteralUses, and whether one of them is on every path
// to a return of fn.
func paramUses(fn *ssa.Function, p *ssa.Parameter) (string, bool) {
	uses := map[string][]ssa.Instruction{}
	collectUses(p, uses, 0, map[ssa.Value]bool{})
	var kinds []string
	always := false
	rets := returnBlocks(fn)
	for _, k := range types.LiteralUses() {
		ins := uses[k]
		if len(ins) == 0 {
			continue
		}
		kinds = append(kinds, k)
		for _, in := range ins {
			if sel, isSel := in.(*ssa.Select); isSel && (len(sel.States) > 1 || !sel.Blocking) {
				continue // one case of a select: a branch
			}
			if in.Parent() == fn && dominatesAll(in.Block(), rets) {
				always = true
			}
		}
	}
	return strings.Join(kinds, types.LiteralUseSep), always
}

// collectUses adds the uses of v by kind.
func collectUses(v ssa.Value, uses map[string][]ssa.Instruction, depth int, busy map[ssa.Value]bool) {
	if depth > literalUseDepth || busy[v] || v.Referrers() == nil {
		return
	}
	busy[v] = true
	add := func(k string, in ssa.Instruction) { uses[k] = append(uses[k], in) }
	for _, r := range *v.Referrers() {
		switch x := r.(type) {
		case *ssa.Call:
			callUse(&x.Call, v, types.LiteralUseCall, x, add)
		case *ssa.Go:
			callUse(&x.Call, v, types.LiteralUseGo, x, add)
		case *ssa.Defer:
			callUse(&x.Call, v, types.LiteralUseDefer, x, add)
		case *ssa.Store:
			if x.Val != v {
				continue
			}
			if cell, ok := x.Addr.(*ssa.Alloc); ok {
				// A captured parameter is spilled to a cell: its uses are
				// those of what is loaded from it.
				cellUses(cell, uses, depth, busy, add)
				continue
			}
			if isVarargs(x.Addr) {
				add(types.LiteralUsePass, x)
				continue
			}
			add(types.LiteralUseStore, x)
		case *ssa.MapUpdate:
			if x.Value == v {
				add(types.LiteralUseStore, x)
			}
		case *ssa.Send:
			if x.X == v {
				add(types.LiteralUseSend, x)
			}
		case *ssa.Select:
			for _, s := range x.States {
				if s.Dir == gotypes.SendOnly && s.Send == v {
					add(types.LiteralUseSend, x)
				}
			}
		case *ssa.MakeClosure:
			closureUses(x, v, uses, depth, busy, add)
		case *ssa.Return:
			add(types.LiteralUseStore, r)
		case *ssa.Phi:
			collectUses(x, uses, depth+1, busy)
		case *ssa.ChangeType:
			collectUses(x, uses, depth+1, busy)
		case *ssa.MakeInterface:
			collectUses(x, uses, depth+1, busy)
		}
	}
}

// cellUses adds the uses of the values loaded from cell; a closure made
// with the cell passes it on.
func cellUses(cell *ssa.Alloc, uses map[string][]ssa.Instruction, depth int, busy map[ssa.Value]bool, add func(string, ssa.Instruction)) {
	if cell.Referrers() == nil {
		return
	}
	for _, r := range *cell.Referrers() {
		switch x := r.(type) {
		case *ssa.UnOp:
			if x.Op == token.MUL {
				collectUses(x, uses, depth+1, busy)
			}
		case *ssa.MakeClosure:
			closureUses(x, cell, uses, depth, busy, add)
		}
	}
}

// closureUses adds the uses of v that the closure mc, made with v bound,
// makes: what the closure does with its free variable, where a call of it
// takes the kind of how mc runs (runKinds).
func closureUses(mc *ssa.MakeClosure, v ssa.Value, uses map[string][]ssa.Instruction, depth int, busy map[ssa.Value]bool,
	add func(string, ssa.Instruction)) {
	fn, ok := mc.Fn.(*ssa.Function)
	if !ok {
		add(types.LiteralUsePass, mc)
		return
	}
	inner := map[string][]ssa.Instruction{}
	for i, b := range mc.Bindings {
		if b != v || i >= len(fn.FreeVars) {
			continue
		}
		fv := fn.FreeVars[i]
		collectUses(fv, inner, depth+1, busy)
		if fv.Referrers() != nil {
			for _, r := range *fv.Referrers() {
				if load, isLoad := r.(*ssa.UnOp); isLoad && load.Op == token.MUL {
					collectUses(load, inner, depth+1, busy)
				}
			}
		}
	}
	runs := runKinds(mc, 0)
	rets := returnBlocks(fn)
	for _, k := range types.LiteralUses() {
		ins := inner[k]
		if len(ins) == 0 {
			continue
		}
		// A use that does not run on every path through the closure stays
		// conditional: its own instruction (in the closure) is recorded,
		// which dominates nothing of the callee.
		at := func(outer ssa.Instruction) ssa.Instruction {
			for _, in := range ins {
				if dominatesAll(in.Block(), rets) {
					return outer
				}
			}
			return ins[0]
		}
		if k != types.LiteralUseCall {
			add(k, at(mc))
			continue
		}
		for _, run := range runs {
			add(run.kind, at(run.at))
		}
	}
}

// closureRun is one way a closure runs and the instruction that runs it.
type closureRun struct {
	kind string
	at   ssa.Instruction
}

// runKinds returns how the closure value v runs: started by go, deferred,
// called, stored or returned, sent, passed on; through conversions and the
// cell of a local variable that holds it (f := func() {...}; f()).
func runKinds(v ssa.Value, depth int) []closureRun {
	var out []closureRun
	if v.Referrers() == nil || depth > literalUseDepth {
		return out
	}
	for _, r := range *v.Referrers() {
		switch x := r.(type) {
		case *ssa.Go:
			out = append(out, closureRun{callRunKind(&x.Call, v, types.LiteralUseGo), x})
		case *ssa.Defer:
			out = append(out, closureRun{callRunKind(&x.Call, v, types.LiteralUseDefer), x})
		case *ssa.Call:
			out = append(out, closureRun{callRunKind(&x.Call, v, types.LiteralUseCall), x})
		case *ssa.Store:
			if isVarargs(x.Addr) {
				out = append(out, closureRun{types.LiteralUsePass, r}) // f(a, lit) for a ...func()
				continue
			}
			cell, isCell := x.Addr.(*ssa.Alloc)
			if !isCell || x.Val != v || cell.Referrers() == nil {
				out = append(out, closureRun{types.LiteralUseStore, r})
				continue
			}
			for _, cr := range *cell.Referrers() {
				if load, ok := cr.(*ssa.UnOp); ok && load.Op == token.MUL {
					out = append(out, runKinds(load, depth+1)...)
				}
			}
		case *ssa.MapUpdate, *ssa.Return:
			out = append(out, closureRun{types.LiteralUseStore, r})
		case *ssa.Send:
			out = append(out, closureRun{types.LiteralUseSend, r})
		case *ssa.ChangeType:
			out = append(out, runKinds(x, depth+1)...)
		case *ssa.MakeInterface:
			out = append(out, runKinds(x, depth+1)...)
		case *ssa.DebugRef:
		default:
			out = append(out, closureRun{types.LiteralUsePass, r})
		}
	}
	return out
}

// varargsComment is how go/ssa names the array it builds the arguments of
// a variadic parameter in.
const varargsComment = "varargs"

// isVarargs reports whether addr is an element of the array the arguments
// of a variadic call are built in: storing there passes the value on.
func isVarargs(addr ssa.Value) bool {
	ia, ok := addr.(*ssa.IndexAddr)
	if !ok {
		return false
	}
	a, ok := ia.X.(*ssa.Alloc)
	return ok && a.Comment == varargsComment
}

// callRunKind is kind when the call runs mc, pass when mc is an argument.
func callRunKind(common *ssa.CallCommon, mc ssa.Value, kind string) string {
	if common.Value == mc {
		return kind
	}
	return types.LiteralUsePass
}

// callUse adds the use of v by a call: kind when v is the function called,
// pass when it is an argument.
func callUse(common *ssa.CallCommon, v ssa.Value, kind string, in ssa.Instruction, add func(string, ssa.Instruction)) {
	if !common.IsInvoke() && common.Value == v {
		add(kind, in)
		return
	}
	for _, a := range common.Args {
		if a == v {
			add(types.LiteralUsePass, in)
			return
		}
	}
}

// returnBlocks returns the blocks of fn that end in a return, but the
// block a deferred call's recover returns from (it runs only on a panic).
func returnBlocks(fn *ssa.Function) []*ssa.BasicBlock {
	var out []*ssa.BasicBlock
	for _, b := range fn.Blocks {
		if b == fn.Recover {
			continue
		}
		if n := len(b.Instrs); n > 0 {
			if _, ok := b.Instrs[n-1].(*ssa.Return); ok {
				out = append(out, b)
			}
		}
	}
	return out
}

// dominatesAll reports whether b dominates every block of rets (false for
// none: a function that never returns).
func dominatesAll(b *ssa.BasicBlock, rets []*ssa.BasicBlock) bool {
	if b == nil || len(rets) == 0 {
		return false
	}
	for _, r := range rets {
		if !b.Dominates(r) {
			return false
		}
	}
	return true
}
