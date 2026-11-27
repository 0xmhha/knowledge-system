package golang

import (
	"fmt"
	"go/ast"
	"go/token"
	gotypes "go/types"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// concurrency.go implements the declaration side of the Go concurrency pass
// (spec §2): Mutex nodes for sync.Mutex / sync.RWMutex declarations and
// Channel nodes for make(chan T) with direction, buffer and element type (in
// Signature).
//
// The edges that tie a Lock or Unlock call and a field access to a Mutex
// node are made after Resolve (lock_edges.go): the mutex may be declared in
// another file than the call. Cross-function lock chain analysis lives in
// buildpipe's lock propagation pass.
//
// With typesInfo, a declaration is a mutex when go/types says its type is
// sync.Mutex or sync.RWMutex (through pointers and aliases). Without it
// (AST-only, used by tests that call ParseFile without packages) the type
// expression is matched by name and the node is INFERRED.

// emitConcurrencyDecls is the per-file entry point for the concurrency
// pass: it emits a Mutex node for every sync.Mutex / sync.RWMutex struct
// field, package-level variable and local variable the file declares.
func (v *declVisitor) emitConcurrencyDecls(f *ast.File) {
	for _, decl := range f.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					v.scanStructForMutex(s)
				case *ast.ValueSpec:
					v.scanValueSpecForMutex(v.pkgName, s, spanOf(d, s))
				}
			}
		case *ast.FuncDecl:
			if d.Body != nil {
				v.scanFuncBodyForMutexLocals(d)
			}
		}
	}
}

// scanStructForMutex inspects a TypeSpec; if it's a struct type, every
// field whose underlying type resolves to *sync.Mutex or *sync.RWMutex
// becomes a NodeMutex. Embedded mutexes (anonymous fields) get a Name
// derived from the type's basename.
func (v *declVisitor) scanStructForMutex(s *ast.TypeSpec) {
	st, ok := s.Type.(*ast.StructType)
	if !ok || st.Fields == nil {
		return
	}
	structQname := v.pkgName + "." + s.Name.Name
	for _, f := range st.Fields.List {
		kind := v.classifyMutexExpr(f.Type)
		if kind == "" {
			continue
		}
		if len(f.Names) == 0 {
			// Embedded mutex: synthesise a name from the underlying type.
			// Suffix qname with #mutex to disambiguate from the Field node
			// emitFields will emit at the same source position (the
			// previous shared-qname collision caused emitFields to overwrite
			// the Mutex node, leaving acquires_lock edges pointing at a Field
			// node instead of the Mutex node).
			name := embeddedMutexName(f.Type)
			v.emitMutexNode(name, structQname+"."+name+mutexQnameSuffix, kind, f.Pos(), fieldSpan(f))
			continue
		}
		for _, name := range f.Names {
			// Same #mutex suffix as the embedded branch above — see the
			// comment there for the collision-avoidance rationale.
			v.emitMutexNode(name.Name, structQname+"."+name.Name+mutexQnameSuffix, kind, name.Pos(), fieldSpan(f))
		}
	}
}

// mutexQnameSuffix ends the qualified name of a struct field's Mutex node,
// which stands at the same place as the field's Field node.
const mutexQnameSuffix = "#mutex"

// scanValueSpecForMutex emits a Mutex node for each name of a var
// declaration that declares a mutex; prefix is the package name for a
// package-level declaration and the function's qualified name for a local one.
//
// The declaration says so by its type (`var mu sync.Mutex`) or, when it has
// none, by its value (`var mu = sync.RWMutex{}`): read by the type alone, two
// package-level mutexes of go-stablenet had no node and their 4 lock sites no
// edge.
//
// sp is the span of the specification, which the Mutex node shares with the
// Variable node of the same name.
func (v *declVisitor) scanValueSpecForMutex(prefix string, s *ast.ValueSpec, sp declSpan) {
	for i, name := range s.Names {
		kind := ""
		switch {
		case s.Type != nil:
			kind = v.classifyMutexExpr(s.Type)
		case len(s.Values) == len(s.Names):
			kind = v.mutexValueKind(s.Values[i])
		}
		if kind != "" && name.Name != "_" {
			v.emitMutexNode(name.Name, prefix+"."+name.Name, kind, name.Pos(), sp)
		}
	}
}

// mutexValueKind returns "mutex" / "rwmutex" when the expression e creates a
// mutex (sync.Mutex{}, &sync.Mutex{}, new(sync.Mutex)), "" otherwise. A value
// that merely refers to a mutex declared elsewhere (mu := &s.mu) creates
// none.
func (v *declVisitor) mutexValueKind(e ast.Expr) string {
	e = ast.Unparen(e)
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.AND {
		e = ast.Unparen(u.X)
	}
	switch x := e.(type) {
	case *ast.CompositeLit:
		if x.Type != nil {
			return v.classifyMutexExpr(x.Type)
		}
	case *ast.CallExpr:
		if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "new" && len(x.Args) == 1 &&
			(v.typesInfo == nil || v.isBuiltinCall(x.Fun)) {
			return v.classifyMutexExpr(x.Args[0])
		}
	}
	return ""
}

// scanFuncBodyForMutexLocals walks a function body for local declarations
// (`var mu sync.Mutex`, `mu := sync.Mutex{}`, `mu := new(sync.Mutex)`) and
// emits Mutex nodes. The qualified-name uses pkg.func.localVar to
// disambiguate same-named locals across different functions.
func (v *declVisitor) scanFuncBodyForMutexLocals(fd *ast.FuncDecl) {
	funcQname := funcDeclQname(v.pkgName, fd)
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.DeclStmt:
			gen, ok := s.Decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				return true
			}
			for _, spec := range gen.Specs {
				if vs, ok := spec.(*ast.ValueSpec); ok {
					v.scanValueSpecForMutex(funcQname, vs, spanOf(gen, vs))
				}
			}
		case *ast.AssignStmt:
			if s.Tok != token.DEFINE || len(s.Lhs) != len(s.Rhs) {
				return true
			}
			for i, lhs := range s.Lhs {
				id, ok := lhs.(*ast.Ident)
				if !ok || id.Name == "_" || (v.typesInfo != nil && v.typesInfo.Defs[id] == nil) {
					continue // not a new name
				}
				if kind := v.mutexValueKind(s.Rhs[i]); kind != "" {
					// The span of the LocalVariable node of the same name.
					v.emitMutexNode(id.Name, funcQname+"."+id.Name, kind, id.Pos(), declSpan{start: id.Pos(), end: s.End()})
				}
			}
		}
		return true
	})
}

// emitMutexNode appends a NodeMutex to v.nodes and a defines-edge from the
// enclosing file. Returns the new node's ID. Confidence is EXTRACTED when
// typesInfo confirmed the type, INFERRED in AST-only mode.
//
// sp is the span of the declaration that declares the mutex: a Mutex node
// describes the same declaration as its Variable or Field and has its span
// (see spanOf). namePos keys the id: the mutex's own identifier, or the start
// of an embedded field.
func (v *declVisitor) emitMutexNode(name, qname, subKind string, namePos token.Pos, sp declSpan) string {
	startLn, startBy := v.pos(sp.start)
	endLn, endBy := v.pos(sp.end)
	_, nameBy := v.pos(namePos)
	// The node kind is part of the id key: a package-level `var mu
	// sync.Mutex` is also a Variable node with the same qualified name and
	// identifier, and sharing one id made graph.Build keep only one of them.
	id := v.declID("mutex:"+qname, nameBy)
	conf := types.ConfExtracted
	if v.typesInfo == nil {
		conf = types.ConfInferred
	}
	v.nodes = append(v.nodes, types.Node{
		ID: id, Type: types.NodeMutex,
		Name: name, QualifiedName: qname,
		FilePath: v.relPath, StartLine: startLn, EndLine: endLn,
		StartByte: startBy, EndByte: endBy,
		Language: "go", Confidence: conf, SubKind: subKind,
	})
	v.edges = append(v.edges, types.Edge{
		Src: v.fileID, Dst: id, Type: types.EdgeDefines, Count: 1,
		Confidence: conf,
	})
	return id
}

// classifyMutexExpr returns "mutex" / "rwmutex" / "" based on whether the
// type expression resolves to sync.Mutex or sync.RWMutex.
//
// With typesInfo: walks the underlying types.Type chain, dereferences any
// pointer/named layers, returns the package+name match. This handles type
// aliases and embedded mutexes correctly (R2.3 in spec).
//
// Without typesInfo (AST-only fallback): pattern-matches `sync.Mutex` /
// `sync.RWMutex` selector expressions by name. False positives possible if
// a different package is also imported as "sync" — flagged INFERRED on
// emit so downstream consumers can distinguish.
func (v *declVisitor) classifyMutexExpr(e ast.Expr) string {
	if v.typesInfo != nil {
		t := v.typesInfo.TypeOf(e)
		return classifyMutexType(t)
	}
	return classifyMutexExprByName(e)
}

// classifyMutexType walks the types.Type to find sync.Mutex / sync.RWMutex
// at any depth (pointer, named alias, embedded). Returns the sub_kind tag
// ("mutex" / "rwmutex") or "" when not a mutex.
func classifyMutexType(t gotypes.Type) string {
	if t == nil {
		return ""
	}
	// Strip pointer layers — `*sync.Mutex` and `sync.Mutex` are both mutexes —
	// and aliases.
	t = gotypes.Unalias(t)
	if ptr, ok := t.(*gotypes.Pointer); ok {
		return classifyMutexType(ptr.Elem())
	}
	named, ok := t.(*gotypes.Named)
	if !ok {
		return ""
	}
	obj := named.Obj()
	if obj == nil || obj.Pkg() == nil {
		return ""
	}
	if obj.Pkg().Path() != "sync" {
		return ""
	}
	switch obj.Name() {
	case "Mutex":
		return "mutex"
	case "RWMutex":
		return "rwmutex"
	}
	return ""
}

// classifyMutexExprByName is the AST-only fallback: a SelectorExpr of form
// `sync.Mutex` / `sync.RWMutex` (or starred forms) is treated as the
// matching mutex. Will produce false positives if another import is also
// aliased to "sync"; emitMutexNode flags these with INFERRED confidence.
func classifyMutexExprByName(e ast.Expr) string {
	if star, ok := e.(*ast.StarExpr); ok {
		return classifyMutexExprByName(star.X)
	}
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	pkgIdent, ok := sel.X.(*ast.Ident)
	if !ok || pkgIdent.Name != "sync" {
		return ""
	}
	switch sel.Sel.Name {
	case "Mutex":
		return "mutex"
	case "RWMutex":
		return "rwmutex"
	}
	return ""
}

// embeddedMutexName synthesises a field name for an anonymous embedded
// mutex field. `sync.Mutex` → "Mutex", `*sync.RWMutex` → "RWMutex".
func embeddedMutexName(e ast.Expr) string {
	if star, ok := e.(*ast.StarExpr); ok {
		return embeddedMutexName(star.X)
	}
	if sel, ok := e.(*ast.SelectorExpr); ok {
		return sel.Sel.Name
	}
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	return "embedded"
}

// asTypeIdent extracts the *ast.Ident representing a type expression, used
// to look up the field's *types.Object for embedded fields. Returns nil
// when the type expression isn't a simple selector.
func asTypeIdent(e ast.Expr) *ast.Ident {
	if star, ok := e.(*ast.StarExpr); ok {
		return asTypeIdent(star.X)
	}
	// An instantiated generic type: Box[T] and Pair[K, V] are named Box, Pair.
	if ix, ok := e.(*ast.IndexExpr); ok {
		return asTypeIdent(ix.X)
	}
	if ix, ok := e.(*ast.IndexListExpr); ok {
		return asTypeIdent(ix.X)
	}
	if sel, ok := e.(*ast.SelectorExpr); ok {
		return sel.Sel
	}
	if id, ok := e.(*ast.Ident); ok {
		return id
	}
	return nil
}

// The methods of sync.Mutex and sync.RWMutex that take or give up the lock.
const (
	lockMethodLock    = "Lock"
	lockMethodUnlock  = "Unlock"
	lockMethodRLock   = "RLock"
	lockMethodRUnlock = "RUnlock"
)

// lockMethodName returns the called method name and true when call is of
// the form `<expr>.Lock()` / `Unlock()` / `RLock()` / `RUnlock()` with no
// arguments. Returns "", false otherwise.
func lockMethodName(call *ast.CallExpr) (string, bool) {
	if call == nil || len(call.Args) != 0 {
		return "", false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return "", false
	}
	switch sel.Sel.Name {
	case lockMethodLock, lockMethodUnlock, lockMethodRLock, lockMethodRUnlock:
		return sel.Sel.Name, true
	}
	return "", false
}

// chanMake describes a make(chan T, n) call.
type chanMake struct {
	dir  string // bidi, send or recv
	elem string // the element type
	// named is the channel type when make is given a type name instead of a
	// `chan T` literal (make(blockingIter)); the signature shows the type as
	// the source writes it.
	named string
	buf   int // 0 unbuffered, -1 when the size is not an integer literal
}

// unqualified writes type names without their package, as the element type
// of a Channel signature always was ("chan Header" for chan *types.Header).
func unqualified(*gotypes.Package) string { return "" }

// chanMake reports whether e is a call of the builtin make that makes a
// channel, and describes the channel.
//
// With type information the argument's type decides, so a channel made
// through a named type (make(blockingIter) with type blockingIter chan
// struct{}) is one, and the element type is the one go/types prints: struct{},
// []byte, *Header. Taken from the syntax as a single name, the element type
// was "?" on 312 of the 708 Channel nodes of go-stablenet. Without type
// information only a literal `chan T` argument is recognised and the element
// is its name.
func (v *declVisitor) chanMake(e ast.Expr) (*ast.CallExpr, chanMake, bool) {
	call, ok := e.(*ast.CallExpr)
	if !ok || len(call.Args) == 0 {
		return nil, chanMake{}, false
	}
	if id, ok := call.Fun.(*ast.Ident); !ok || id.Name != "make" {
		return nil, chanMake{}, false
	}
	cm := chanMake{}
	if len(call.Args) >= 2 {
		cm.buf = literalIntValue(call.Args[1])
	}
	if v.typesInfo == nil {
		chType, ok := call.Args[0].(*ast.ChanType)
		if !ok {
			return nil, chanMake{}, false
		}
		cm.dir, cm.elem = channelDirection(chType.Dir), exprName(chType.Value)
		return call, cm, true
	}
	t := v.typesInfo.TypeOf(call.Args[0])
	if t == nil || !v.isBuiltinCall(call.Fun) {
		return nil, chanMake{}, false
	}
	ch, ok := t.Underlying().(*gotypes.Chan)
	if !ok {
		return nil, chanMake{}, false
	}
	switch ch.Dir() {
	case gotypes.SendOnly:
		cm.dir = chanDirSend
	case gotypes.RecvOnly:
		cm.dir = chanDirRecv
	default:
		cm.dir = chanDirBidi
	}
	cm.elem = gotypes.TypeString(ch.Elem(), unqualified)
	if _, literal := ast.Unparen(call.Args[0]).(*ast.ChanType); !literal {
		cm.named = gotypes.TypeString(t, unqualified)
	}
	return call, cm, true
}

// emitChannelFromMake emits the Channel node of the make(chan T, n) call that
// cm describes (see chanMake), with direction, element type and buffer size
// encoded in Signature, and returns its ID.
func (v *declVisitor) emitChannelFromMake(parentID string, call *ast.CallExpr, cm chanMake) string {
	sig := formatChannelSignature(cm)
	startLn, startBy := v.pos(call.Pos())
	endLn, endBy := v.pos(call.End())
	qname := fmt.Sprintf("chan@%s:%d", v.relPath, startBy)
	nodeID := v.declID(qname, startBy)
	v.nodes = append(v.nodes, types.Node{
		ID: nodeID, Type: types.NodeChannel,
		Name: "channel", QualifiedName: qname,
		FilePath: v.relPath, StartLine: startLn, EndLine: endLn,
		StartByte: startBy, EndByte: endBy,
		Language: "go", Confidence: types.ConfExtracted,
		SubKind: cm.dir, Signature: sig,
	})
	v.edges = append(v.edges, types.Edge{
		Src: parentID, Dst: nodeID, Type: types.EdgeContains, Count: 1,
		Confidence: types.ConfExtracted,
	})
	return nodeID
}

// Directions of a Channel node (its sub kind).
const (
	chanDirBidi = "bidi"
	chanDirSend = "send"
	chanDirRecv = "recv"
)

// channelDirection maps ast.ChanDir to the spec's string tag.
//
//	SEND  → "send"  (chan<- T)
//	RECV  → "recv"  (<-chan T)
//	other → "bidi"  (chan T, the parser bitwise-ORs SEND|RECV for bidi)
func channelDirection(d ast.ChanDir) string {
	switch d {
	case ast.SEND:
		return chanDirSend
	case ast.RECV:
		return chanDirRecv
	}
	return chanDirBidi
}

// literalIntValue parses an *ast.BasicLit as an int. Returns -1 on any
// failure (non-literal expression, non-INT kind, parse error). Used for
// buffer sizes — non-literal sizes lose the constant info but the channel
// node is still emitted with BufferSize=-1 ("unknown") so consumers can
// distinguish "unbuffered" (0) from "size lost" (-1).
func literalIntValue(e ast.Expr) int {
	lit, ok := e.(*ast.BasicLit)
	if !ok || lit.Kind != token.INT {
		return -1
	}
	var n int
	if _, err := fmt.Sscanf(lit.Value, "%d", &n); err != nil {
		return -1
	}
	return n
}

// formatChannelSignature returns a concise human-readable channel
// description suitable for the existing Signature field — schema-free way
// to surface direction/buffer/elem in the viewer without a node.go bump. It
// is the channel type as the make call writes it, with the buffer size:
//
//	make(chan int, 10)   → "chan int (buf=10)"
//	make(chan<- Msg)     → "chan<- Msg"
//	make(<-chan T, n)    → "<-chan T (buf=?)"
//	make(blockingIter)   → "blockingIter"
func formatChannelSignature(cm chanMake) string {
	bufPart := ""
	switch {
	case cm.buf > 0:
		bufPart = fmt.Sprintf(" (buf=%d)", cm.buf)
	case cm.buf < 0:
		bufPart = " (buf=?)"
	}
	if cm.named != "" {
		return cm.named + bufPart
	}
	prefix := "chan"
	switch cm.dir {
	case chanDirSend:
		prefix = "chan<-"
	case chanDirRecv:
		prefix = "<-chan"
	}
	elem := cm.elem
	if elem == "" {
		elem = "?"
	}
	return prefix + " " + elem + bufPart
}

// bindLocalChans binds each variable that n (an assignment, or a var
// declaration, grouped or not) sets to a make(chan ...) to the Channel node
// of that make, emitted here, in chanVarIDs, so the function's operations on
// it point at the Channel (emitChannelOp). made records the make calls
// handled, so the walk does not emit their node again. Several names are
// bound one by one (a, b := make(chan int), make(chan error)); a name set
// from one call that returns several values has no make origin here. The
// Goroutine's edges do not use this: the channel linker follows a channel
// to its origin through parameters, closures and stores (dispatch.go).
func (v *declVisitor) bindLocalChans(parentID string, n ast.Node, made map[token.Pos]bool) {
	bind := func(lhs, rhs []ast.Expr) {
		if len(lhs) != len(rhs) {
			return
		}
		for i, r := range rhs {
			call, cm, ok := v.chanMake(r)
			if !ok {
				continue
			}
			chanID := v.emitChannelFromMake(parentID, call, cm)
			made[call.Pos()] = true
			if key := v.chanVar(lhs[i]); key != nil {
				v.chanVarIDs[key] = chanID
			}
		}
	}
	switch s := n.(type) {
	case *ast.AssignStmt:
		bind(s.Lhs, s.Rhs)
	case *ast.DeclStmt:
		gd, ok := s.Decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			return
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok {
				continue
			}
			names := make([]ast.Expr, len(vs.Names))
			for i, name := range vs.Names {
				names[i] = name
			}
			bind(names, vs.Values)
		}
	}
}
