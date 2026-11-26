package golang

import (
	"fmt"
	"go/ast"
	"go/token"
	gotypes "go/types"

	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// emitFunctionBodyPos walks a function/method body and emits Pass-1 logic
// blocks (5 kinds), CallSite nodes, Goroutines, and channel send/recv edges.
// Cross-file call resolution is left to Pass 2.
//
// parentID must be the ID already minted for the enclosing function/method
// node — we accept it from the caller so we don't have to re-derive the
// parent's start byte offset here.
func (v *declVisitor) emitFunctionBodyPos(parentQname, parentID string, body *ast.BlockStmt) {
	if body == nil {
		return
	}
	v.emitBodyPos(parentQname, parentID, body)
}

// emitBodyPos is emitFunctionBodyPos for any piece of code that runs on
// behalf of the node parentID: a function body, or the initialiser of a
// package-level variable (`var Default = New().Inc()`), whose statements and
// calls belong to the variable.
func (v *declVisitor) emitBodyPos(parentQname, parentID string, body ast.Node) {
	// Reset channel variable scope for this function — chanVarIDs is function-scoped.
	v.chanVarIDs = make(map[any]string)
	// assignedMakeChan tracks token.Pos of make(chan ...) calls that were already
	// handled by the AssignStmt path, so the subsequent CallExpr event on the
	// same expression doesn't emit a duplicate Channel node.
	assignedMakeChan := map[token.Pos]bool{}
	ast.Inspect(body, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		switch s := n.(type) {
		case *ast.AssignStmt:
			// Capture `ch := make(chan T, n)` before the generic CallExpr case fires.
			// Maps LHS variable name → Channel node ID so sends_to/recvs_from edges
			// can point at the actual Channel node rather than an anonymous CallSite.
			// Note: channel parameters (e.g. `out chan<- int`) are not captured here;
			// those fall back to CallSite as destination (best-effort only).
			if len(s.Lhs) == 1 && len(s.Rhs) == 1 {
				if call, cm, ok := v.chanMake(s.Rhs[0]); ok {
					chanID := v.emitChannelFromMake(parentID, call, cm)
					if key := v.chanVar(s.Lhs[0]); key != nil {
						v.chanVarIDs[key] = chanID
					}
					assignedMakeChan[call.Pos()] = true
				}
			}
		case *ast.CallExpr:
			id := v.appendLogicBlockPos(parentID, parentQname, types.NodeCallSite, "", s.Pos(), s.End())
			// Pending edge: CallSite -(calls|invokes)-> callee — resolved in Pass 2.
			// Classify dispatch kind so non-static dispatches
			// surface as `invokes` with a populated dispatch_kind metadata
			// column instead of being conflated with static `calls`.
			v.pending = append(v.pending, v.parsePendingFromCall(id, s))
			// For dispatch kinds whose target is unresolvable
			// (closure literal, func value, method value) — Resolve would
			// drop the PendingRef because TargetQName has no matching
			// Function/Method node — we emit a direct self-loop on the
			// parent function so the edge isn't lost. Self-loop semantics
			// mirror timeout_path / cancellation_path: marker edges that
			// say "this function performs <kind> dispatch" without naming
			// a runtime target. interface_method dispatch resolves cleanly
			// to a Method node so we leave it in the PendingRef path.
			v.maybeEmitInvokesSelfLoop(parentID, s)
			// Lock and unlock edges are made after Resolve (lock_edges.go):
			// the mutex may be declared in another file.
			//
			// Concurrency: emit Channel node for make(chan ...) calls
			// that were NOT already handled by the AssignStmt path above
			// (prevents duplicate Channel nodes for `ch := make(chan T, n)`).
			if _, cm, ok := v.chanMake(s); ok && !assignedMakeChan[s.Pos()] {
				v.emitChannelFromMake(parentID, s, cm)
			}
		case *ast.GoStmt:
			goroutineID := v.appendLogicBlockPos(parentID, parentQname, types.NodeGoroutine, "", s.Pos(), s.End())
			v.edges = append(v.edges, types.Edge{
				Src: parentID, Dst: goroutineID, Type: types.EdgeSpawns, Count: 1,
				Confidence: types.ConfExtracted,
			})
			// Emit sends_to/recvs_from from goroutine body to known channels.
			v.emitGoroutineChannelEdges(goroutineID, s.Call)
			// Named-function goroutines like
			// `go x.method()` previously emitted no calls/invokes edge,
			// so the cross-function lock propagator skipped the body of
			// x.method entirely (it could only see anonymous goroutine
			// literals via the intra-fn parent attribution path). Emit
			// a PendingRef here so Pass 2 Resolve materialises a calls
			// (or invokes, for interface dispatch) edge from the parent
			// function to the goroutine's target. Anonymous `go func(){}()`
			// is skipped — the FuncLit body has no resolvable target
			// qname and the existing parent-attribution path already
			// covers field touches inside it.
			v.emitGoCall(parentID, parentQname, s)
			return false // goroutine body handled by emitGoroutineChannelEdges; prevent double-walk
		default:
			v.emitStatement(parentID, parentQname, n)
		}
		return true
	})
}

// emitStatement emits the node of n when n is a control statement (if, for,
// range, switch, return) or a channel operation (send, receive, range over a
// channel); other nodes are left to the caller. Both body walks call it: the
// one for a function body and the one for the literal of a go statement.
func (v *declVisitor) emitStatement(parentID, parentQname string, n ast.Node) {
	switch s := n.(type) {
	case *ast.IfStmt:
		v.appendLogicBlockPos(parentID, parentQname, types.NodeIfStmt, "", s.Pos(), s.End())
	case *ast.ForStmt:
		v.appendLogicBlockPos(parentID, parentQname, types.NodeLoopStmt, "for", s.Pos(), s.End())
	case *ast.RangeStmt:
		v.appendLogicBlockPos(parentID, parentQname, types.NodeLoopStmt, "range", s.Pos(), s.End())
		// A range over a channel receives from it until it is closed. The
		// site spans `range ch`: the expression alone may be a call, whose
		// site has that span already. 10 such loops of go-stablenet had no
		// representation, the others only an inferred edge.
		if v.isChannel(s.X) {
			v.emitChannelOp(parentID, parentQname, types.EdgeRecvsFrom, subKindRecv, s.X, s.Range, s.X.End())
		}
	case *ast.SwitchStmt:
		v.appendLogicBlockPos(parentID, parentQname, types.NodeSwitchStmt, "", s.Pos(), s.End())
	case *ast.TypeSwitchStmt:
		v.appendLogicBlockPos(parentID, parentQname, types.NodeSwitchStmt, "type", s.Pos(), s.End())
	case *ast.ReturnStmt:
		v.appendLogicBlockPos(parentID, parentQname, types.NodeReturnStmt, "", s.Pos(), s.End())
	case *ast.SendStmt:
		v.emitChannelOp(parentID, parentQname, types.EdgeSendsTo, subKindSend, s.Chan, s.Pos(), s.End())
	case *ast.UnaryExpr:
		if s.Op == token.ARROW {
			v.emitChannelOp(parentID, parentQname, types.EdgeRecvsFrom, subKindRecv, s.X, s.Pos(), s.End())
		}
	}
}

// isChannel reports whether e is of channel type. Without type information
// it cannot tell and returns false.
func (v *declVisitor) isChannel(e ast.Expr) bool {
	if v.typesInfo == nil {
		return false
	}
	t := v.typesInfo.TypeOf(e)
	if t == nil {
		return false
	}
	_, ok := t.Underlying().(*gotypes.Chan)
	return ok
}

// Sub kinds of the CallSite node of a channel operation.
const (
	subKindSend = "send"
	subKindRecv = "recv"
)

// emitChannelOp records one send (et sends_to) or receive (et recvs_from) by
// the function parentID; ch is the channel operand and [from, to) the span of
// the operation.
//
// A channel that is a local variable bound to a make site is linked to its
// Channel node. Any other channel (a parameter, a field, a call result) gets a
// site node of sub kind subKind and an edge to it. The edge carries the line
// of the operation, as a call edge does.
//
// The same holds inside the literal of a go statement: the operation is
// written in the function, which gets the edge. emitGoroutineChannelEdges adds
// the Goroutine's own inferred edge to the channel; with that edge alone, 76
// operations of go-stablenet on a local channel had no edge from their
// function and no line.
func (v *declVisitor) emitChannelOp(parentID, parentQname string, et types.EdgeType, subKind string, ch ast.Expr, from, to token.Pos) {
	line, _ := v.pos(from)
	if key := v.chanVar(ch); key != nil {
		if chanID, ok := v.chanVarIDs[key]; ok {
			v.edges = append(v.edges, types.Edge{
				Src: parentID, Dst: chanID, Type: et, Line: line,
				Count: 1, Confidence: types.ConfExtracted,
			})
			return
		}
	}
	id := v.appendLogicBlockPos(parentID, parentQname, types.NodeCallSite, subKind, from, to)
	v.edges = append(v.edges, types.Edge{
		Src: parentID, Dst: id, Type: et, Line: line,
		Count: 1, Confidence: types.ConfExtracted,
	})
}

// chanVar returns the key of the variable that the channel operand e names
// (ch in "ch <- v" or "<-ch"), or nil when e is not a plain identifier.
//
// The key is the variable itself, as go/types resolves it: a parameter of a
// function literal that is called like a local channel (go func(errc chan
// error) { errc <- f() }(errc)) is another variable, and an operation on it
// is not an operation on the local one. Keyed by name, 6 such sends of
// go-stablenet were tied to the channel of the hidden local. Without type
// information the name is all there is.
func (v *declVisitor) chanVar(e ast.Expr) any {
	id, ok := e.(*ast.Ident)
	if !ok {
		return nil
	}
	if v.typesInfo != nil {
		if obj := v.typesInfo.ObjectOf(id); obj != nil {
			return obj
		}
	}
	return id.Name
}

// appendLogicBlockPos creates a logic-block (or CallSite/Goroutine) node and
// a contains-edge from the enclosing parent. Returns the new node's ID.
//
// The node is named "<parent>#<Type>@<start byte>". A CallSite adds its end
// byte, "<parent>#CallSite@<start>-<end>": the calls of a chain start at the
// same byte (New().Inc().Val() is three calls from the N of New), and so does
// a send whose channel is a call result (f() <- v). Named by the start alone
// they shared one node ID, the graph kept one node per ID, and 3,245 calls of
// go-stablenet had no site of their own.
func (v *declVisitor) appendLogicBlockPos(parentID, parentQname string, t types.NodeType, subKind string, startPos, endPos token.Pos) string {
	startLn, startBy := v.pos(startPos)
	endLn, endBy := v.pos(endPos)
	qname := fmt.Sprintf("%s#%s@%d", parentQname, t, startBy)
	if t == types.NodeCallSite {
		qname = fmt.Sprintf("%s-%d", qname, endBy)
	}
	id := v.declID(qname, startBy)
	v.nodes = append(v.nodes, types.Node{
		ID: id, Type: t, Name: string(t), QualifiedName: qname,
		FilePath: v.relPath, StartLine: startLn, EndLine: endLn,
		StartByte: startBy, EndByte: endBy,
		Language: "go", Confidence: types.ConfExtracted, SubKind: subKind,
	})
	v.edges = append(v.edges, types.Edge{
		Src: parentID, Dst: id, Type: types.EdgeContains, Count: 1, Confidence: types.ConfExtracted,
	})
	return id
}

// parsePendingFromCall extracts a best-effort callee qname from a *ast.CallExpr.
// The result is consumed in Pass 2 (Resolve) to materialize a `calls` (static)
// or `invokes` (non-static dispatch) edge.
//
// Classifies dispatch kind via types.Info when available and
// stamps the result on PendingRef.DispatchKind. The five outcomes:
//
//	dispatch_kind == ""                  → static call (EdgeCalls)
//	dispatch_kind == "interface_method"  → callee.Recv() is *types.Interface
//	dispatch_kind == "func_value"        → CallExpr.Fun is *ast.Ident bound to
//	                                       a var/param of *types.Signature type
//	dispatch_kind == "method_value"      → CallExpr.Fun is *ast.SelectorExpr
//	                                       whose Sel is a Field of *types.Signature
//	dispatch_kind == "closure"           → CallExpr.Fun is an *ast.FuncLit
//
// AST-only fallback (typesInfo == nil): only the closure case is detectable
// without type info; everything else falls back to EdgeCalls. Real graphs
// run the typed path, so the AST-only mode is mainly a test convenience.
func (v *declVisitor) parsePendingFromCall(srcID string, c *ast.CallExpr) parse.PendingRef {
	target := exprName(c.Fun)
	pos := v.fset.Position(c.Pos())
	pr := parse.PendingRef{
		SrcID:       srcID,
		EdgeType:    types.EdgeCalls,
		TargetQName: target,
		HintFile:    pos.Filename,
		Line:        pos.Line,
	}
	// Builtins (len/cap/make/append/new/delete/...) have no graph node.
	// Leaving the bare name ("len") lets Pass 2's bare-name fallback bind it
	// to an unrelated method that happens to share the name (e.g. a type with
	// a method literally named len) — a false cross-subsystem call edge. Clear
	// the target so Resolve drops it instead of guessing.
	if v.isBuiltinCall(c.Fun) || v.isConversion(c.Fun) {
		pr.TargetQName = ""
		return pr
	}
	dispatchKind := v.classifyCallDispatch(c)
	switch dispatchKind {
	case "interface_method":
		// Interface dispatch resolves to the interface's Method node. Qualify
		// it to "pkg.Interface.Method" (via go/types) so Pass 2 matches the
		// exact interface method rather than bare-name binding to a same-named
		// concrete method on an unrelated type.
		pr.EdgeType = types.EdgeInvokes
		pr.DispatchKind = dispatchKind
		if q := v.qualifiedStaticTarget(c.Fun); q != "" {
			pr.TargetQName = q
		}
		pr.TargetCanonicalID = v.calleeCanonicalID(c.Fun)
	case "closure", "func_value", "method_value":
		// Target is unresolvable (closure literal / runtime func value /
		// stored callback). The direct self-loop emission in
		// maybeEmitInvokesSelfLoop covers this case. Clear the target so
		// Resolve drops the PendingRef: the bare variable name (`cancel`,
		// `f`, `callback`) used to suffix-match an unrelated method.
		if v.typesInfo != nil {
			pr.TargetQName = ""
		}
	default:
		// Static dispatch (concrete method / package function). exprName
		// captured only the bare callee name (e.g. "Size"), which Pass 2
		// resolved by bare-name suffix match with last-write-wins — so
		// `valSet.Size()` could bind to an unrelated `pathdb.Database.Size`
		// in another package. When go/types is available, replace the bare
		// name with the callee's fully-qualified qname so it matches the
		// exact definition node. Falls back to the bare name (AST-only
		// builds, or callees we can't qualify) so behaviour is unchanged
		// when type info is absent.
		if q := v.qualifiedStaticTarget(c.Fun); q != "" {
			pr.TargetQName = q
		}
		pr.TargetCanonicalID = v.calleeCanonicalID(c.Fun)
	}
	return pr
}

// calleeCanonicalID returns the canonical id of the function or method a call
// expression names, or "" without type information or for callees that are
// not a declared *types.Func (function values, closures).
func (v *declVisitor) calleeCanonicalID(fun ast.Expr) string {
	if v.typesInfo == nil {
		return ""
	}
	var obj gotypes.Object
	switch f := ast.Unparen(fun).(type) {
	case *ast.SelectorExpr:
		obj = v.typesInfo.ObjectOf(f.Sel)
	case *ast.Ident:
		obj = v.typesInfo.ObjectOf(f)
	case *ast.IndexExpr: // explicit instantiation: F[T](x)
		return v.calleeCanonicalID(f.X)
	case *ast.IndexListExpr:
		return v.calleeCanonicalID(f.X)
	}
	if fn, ok := obj.(*gotypes.Func); ok {
		return goCanonicalID(fn)
	}
	return ""
}

// isConversion reports whether fun names a type, so the call is a conversion
// such as uint(x) or Header(h), not a call to code. Without type information
// it returns false.
func (v *declVisitor) isConversion(fun ast.Expr) bool {
	if v.typesInfo == nil {
		return false
	}
	tv, ok := v.typesInfo.Types[fun]
	return ok && tv.IsType()
}

// emitCallsInFuncLit records the calls inside a goroutine's function literal
// (`go func() { ... }()`) as calls of the enclosing function, the same way the
// body walker records calls elsewhere: a CallSite node, a pending calls or
// invokes ref, and the dispatch self-loop. The GoStmt case stops the body walk
// so channel and lock edges are not emitted twice; without this, every call in
// a goroutine literal was missing from the graph (276 of the 306 missing
// function-literal calls on go-stablenet).
//
// The literal's control statements and channel operations get their nodes
// too (emitStatement), so the statement links inside a goroutine have the
// same shape as elsewhere; a go statement inside the literal gets its
// Goroutine node and spawns edge.
func (v *declVisitor) emitCallsInFuncLit(parentID, parentQname string, lit *ast.FuncLit) {
	if lit == nil || lit.Body == nil {
		return
	}
	ast.Inspect(lit.Body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.GoStmt:
			gid := v.appendLogicBlockPos(parentID, parentQname, types.NodeGoroutine, "", s.Pos(), s.End())
			v.edges = append(v.edges, types.Edge{Src: parentID, Dst: gid, Type: types.EdgeSpawns, Count: 1,
				Confidence: types.ConfExtracted})
			v.emitGoCall(parentID, parentQname, s)
			return false
		case *ast.CallExpr:
			v.emitCallSite(parentID, parentQname, s)
			// A channel made inside the literal has its node like any other
			// (9 of the 10 make sites of go-stablenet without a Channel node
			// were in a go statement's literal). It is not bound to its
			// variable here: the operations on it get sites.
			if _, cm, ok := v.chanMake(s); ok {
				v.emitChannelFromMake(parentID, s, cm)
			}
		default:
			v.emitStatement(parentID, parentQname, n)
		}
		return true
	})
}

// emitCallSite records one call of the enclosing function: a CallSite node,
// the pending calls or invokes ref, and the dispatch self-loop.
func (v *declVisitor) emitCallSite(parentID, parentQname string, c *ast.CallExpr) {
	id := v.appendLogicBlockPos(parentID, parentQname, types.NodeCallSite, "", c.Pos(), c.End())
	v.pending = append(v.pending, v.parsePendingFromCall(id, c))
	v.maybeEmitInvokesSelfLoop(parentID, c)
}

// emitGoCall records what a go statement runs: the literal's body, or the
// named function as a call of the enclosing function, and the calls in its
// arguments and in the function expression, which the go statement
// evaluates itself (go e.refresh(load(3)) calls load).
func (v *declVisitor) emitGoCall(parentID, parentQname string, s *ast.GoStmt) {
	if s.Call == nil {
		return
	}
	calls := func(n ast.Node) {
		ast.Inspect(n, func(x ast.Node) bool {
			switch c := x.(type) {
			case *ast.FuncLit:
				v.emitCallsInFuncLit(parentID, parentQname, c)
				return false
			case *ast.CallExpr:
				v.emitCallSite(parentID, parentQname, c)
			}
			return true
		})
	}
	if lit, isFuncLit := s.Call.Fun.(*ast.FuncLit); isFuncLit {
		v.emitCallsInFuncLit(parentID, parentQname, lit)
	} else {
		v.pending = append(v.pending, v.parsePendingFromCall(parentID, s.Call))
		calls(s.Call.Fun)
	}
	for _, a := range s.Call.Args {
		calls(a)
	}
}

// isBuiltinCall reports whether c.Fun is a Go builtin (len, cap, make, append,
// new, delete, copy, close, panic, recover, print, println). Builtins are
// called as a plain identifier that resolves to a *types.Builtin object.
// Requires type info; without it we cannot distinguish a builtin from a
// same-named user function, so we conservatively return false.
func (v *declVisitor) isBuiltinCall(fun ast.Expr) bool {
	if v.typesInfo == nil {
		return false
	}
	id, ok := fun.(*ast.Ident)
	if !ok {
		return false
	}
	_, isBuiltin := v.typesInfo.ObjectOf(id).(*gotypes.Builtin)
	return isBuiltin
}

// qualifiedStaticTarget builds the fully-qualified callee qname for a
// statically-dispatched call using go/types, in the same form the callee's
// definition node carries (see visitFuncDecl): "pkg.RecvType.Method" for a
// concrete method and "pkg.Func" for a package function. Returns "" when type
// info is unavailable or the callee is not a concrete *types.Func with a
// package, so the caller keeps the bare-name fallback.
func (v *declVisitor) qualifiedStaticTarget(fun ast.Expr) string {
	if v.typesInfo == nil {
		return ""
	}
	var obj gotypes.Object
	switch f := fun.(type) {
	case *ast.SelectorExpr:
		obj = v.typesInfo.ObjectOf(f.Sel)
	case *ast.Ident:
		obj = v.typesInfo.ObjectOf(f)
	default:
		return ""
	}
	fn, ok := obj.(*gotypes.Func)
	if !ok || fn.Pkg() == nil {
		return ""
	}
	sig, ok := fn.Type().(*gotypes.Signature)
	if !ok {
		return ""
	}
	pkg := fn.Pkg().Name()
	if recv := sig.Recv(); recv != nil {
		named, _ := receiverBase(recv.Type())
		if named == nil || named.Obj() == nil || !isPackageLevel(named.Obj()) {
			return ""
		}
		return pkg + "." + named.Obj().Name() + "." + fn.Name()
	}
	return pkg + "." + fn.Name()
}

// maybeEmitInvokesSelfLoop emits a self-loop `invokes` edge on the parent
// function for dispatch kinds whose runtime target cannot be resolved at
// AST time. Dst == Src by design — self-loop semantics signal "this
// function performs <dispatch_kind> dispatch" without claiming a callee.
//
// The parentID passed by the caller is a CallSite ID (appendLogicBlockPos
// returns the call-site, not the enclosing function). We climb to the
// enclosing function via the same callSiteParent map Resolve uses — but
// inline at AST time, by deriving the parent function's qname from the
// call-site qname suffix construction "<parentQname>#<Kind>@<offset>".
//
// Implementation note: at this AST visitor call site we already know the
// enclosing function's ID (the v.declVisitor's currently-walked function),
// but the existing emitFunctionBodyPos signature accepts a parentID that
// happens to be the function ID (not the call-site ID — the call-site is
// minted inside the case body and assigned to local var `id`). So we use
// parentID directly. Reading emitFunctionBodyPos confirms parentID is the
// function ID (visitFuncDecl passes `id` from MakeID(funcQname, ...)).
func (v *declVisitor) maybeEmitInvokesSelfLoop(parentFuncID string, c *ast.CallExpr) {
	// In a package-level initialiser the parent is the Variable it
	// initialises (var Version = func() string { ... }()). The marker names
	// no callee, so the variable is not called by it; without it 4 closure
	// calls of go-stablenet had no trace.
	if c == nil || parentFuncID == "" {
		return
	}
	dispatchKind := v.classifyCallDispatch(c)
	switch dispatchKind {
	case "closure", "func_value", "method_value":
		// fall through
	default:
		return
	}
	pos := v.fset.Position(c.Pos())
	conf := types.ConfExtracted
	if v.typesInfo == nil {
		conf = types.ConfInferred
	}
	v.edges = append(v.edges, types.Edge{
		Src: parentFuncID, Dst: parentFuncID,
		Type:         types.EdgeInvokes,
		Line:         pos.Line,
		Count:        1,
		Confidence:   conf,
		FilePath:     v.relPath,
		DispatchKind: dispatchKind,
	})
}

// classifyCallDispatch returns the dispatch_kind tag for c, or "" when c is
// a static call. Uses types.Info when available; falls back to AST-only
// closure detection otherwise (the only kind unambiguously visible at the
// syntactic layer).
//
// Order matters: closure literal beats SelectorExpr beats Ident, because a
// `func(){...}()` call's Fun is *ast.FuncLit (no Selector / no Ident path).
func (v *declVisitor) classifyCallDispatch(c *ast.CallExpr) string {
	if c == nil {
		return ""
	}
	// Closure literal call: `func() { ... }()`. Detectable at AST layer alone.
	if _, ok := c.Fun.(*ast.FuncLit); ok {
		return "closure"
	}
	if v.typesInfo == nil {
		return ""
	}
	switch fun := c.Fun.(type) {
	case *ast.SelectorExpr:
		// Try Selections first — gives precise interface-vs-concrete answer.
		if sel, ok := v.typesInfo.Selections[fun]; ok {
			// Declared by an interface: the call is dispatched at run
			// time also when the receiver is a struct that embeds the
			// interface (p.rw.ReadMsg() with protoRW{MsgReadWriter}).
			// Judged by the receiver alone, 17 such calls of
			// go-stablenet were static calls.
			if fn, ok := sel.Obj().(*gotypes.Func); ok && isInterfaceMethod(fn) {
				return dispatchInterfaceMethod
			}
			recv := sel.Recv()
			if recv == nil {
				return ""
			}
			// Strip pointer to find the underlying type.
			if ptr, ok := recv.(*gotypes.Pointer); ok {
				recv = ptr.Elem()
			}
			if _, isIface := recv.Underlying().(*gotypes.Interface); isIface {
				return "interface_method"
			}
			// A field of function type whose Sel resolves to a *types.Var
			// (struct field) carrying *types.Signature is a "method_value"
			// dispatch (the field is a stored function pointer). Distinguish
			// from a real method by checking the Sel's object kind.
			if obj := v.typesInfo.ObjectOf(fun.Sel); obj != nil {
				if vobj, ok := obj.(*gotypes.Var); ok {
					if _, isSig := vobj.Type().Underlying().(*gotypes.Signature); isSig {
						return "method_value"
					}
				}
			}
			return ""
		}
		// No Selection: Sel might still resolve to a Var-of-Signature
		// (package-level func value referenced via package selector).
		if obj := v.typesInfo.ObjectOf(fun.Sel); obj != nil {
			if vobj, ok := obj.(*gotypes.Var); ok {
				if _, isSig := vobj.Type().Underlying().(*gotypes.Signature); isSig {
					return "func_value"
				}
			}
		}
	case *ast.Ident:
		// Bare identifier call: `cb(arg)` where cb is a func-typed var/param.
		// Distinguish from a regular function call by checking the object
		// kind — *types.Func means static dispatch, *types.Var of signature
		// type means func value.
		if obj := v.typesInfo.ObjectOf(fun); obj != nil {
			if vobj, ok := obj.(*gotypes.Var); ok {
				if _, isSig := vobj.Type().Underlying().(*gotypes.Signature); isSig {
					return "func_value"
				}
			}
		}
	}
	return ""
}
