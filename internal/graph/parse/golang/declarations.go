package golang

import (
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	gotypes "go/types"
	"path"
	"path/filepath"
	"strings"

	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// declVisitor walks the AST and emits Pass 1 nodes and edges.
//
// typesInfo, when non-nil, enables go/types-aware extraction (used by the
// concurrency pass to resolve sync.Mutex receivers via *types.Object
// identity rather than name matching). Stays nil when the parser was
// invoked in AST-only mode (no SetPackages call) — callers must check
// before dereferencing.
type declVisitor struct {
	fset      *token.FileSet
	relPath   string
	pkgName   string
	pkgID     string
	fileID    string
	nodes     []types.Node
	edges     []types.Edge
	pending   []parse.PendingRef
	typesInfo *gotypes.Info
	// scope is the function whose body is being walked; zero at package
	// level. A type, var or const declared inside a body is named under the
	// function and defined by it (see declOwner).
	scope declScope
	// structsDone holds the struct type expressions whose fields already
	// have nodes, so the walk does not emit them a second time when it
	// reaches the expression itself (see emitStructFields).
	structsDone map[*ast.StructType]bool
	// inInitialiser is set while a package-level initialiser is walked: the
	// code then belongs to a Variable, not to a function.
	inInitialiser bool
	// netRPC says the build uses net/rpc, so a method with the net/rpc
	// handler shape may be a handler (see maybeEmitJSONRPCHandler).
	netRPC bool
	// mutexNodeIDs maps a *types.Object (the var/field declaration of a
	// sync.Mutex/RWMutex) to the Mutex node ID emitted by emitConcurrencyDecls.
	// Populated during decl walk; consumed by Lock/Unlock detection so
	// acquires_lock edges resolve to the same Mutex node the field declared.
	mutexNodeIDs map[gotypes.Object]string
	// fieldNodeIDs maps a *types.Object (a struct Field declaration) to its
	// NodeField ID. Populated during emitFields when typesInfo is available.
	// Consumed by the accessed_under_lock pass to translate
	// `x.field` references inside locked functions into edges anchored at the
	// owning Field node.
	fieldNodeIDs map[gotypes.Object]string
	// endpointNodeIDs maps an Endpoint qname (e.g. "http:GET /users" or
	// "http:* /users") to its node ID, deduping repeat HandleFunc calls on
	// the same (method, route) pair within a file.
	// Schema 1.9 §6.2 — cross-language qname format shared with the TS parser.
	endpointNodeIDs map[string]string
	// messageNodeIDs maps a MessageType qname (e.g. "pkg.Args" or
	// "rpc:Service.Method") to its node ID, deduping handles_message and
	// rpc_calls targets that resolve to the same logical message.
	messageNodeIDs map[string]string
	// httpClientPlaceholderIDs maps an HTTP-client-call target qname
	// ("http:METHOD /path") to the AMBIGUOUS placeholder Endpoint ID emitted
	// in upsertHTTPClientPlaceholder. Distinct from endpointNodeIDs because
	// placeholder Endpoints use Language="external" — they live in a separate
	// ID space until the link pass (internal/link/http_match.go) either
	// rewires the http_calls edge to a real Endpoint or leaves the placeholder
	// in place as an external-API marker (schema 1.9 §6.3 (B), §6.9).
	httpClientPlaceholderIDs map[string]string
	// grpcServerImpls tracks which (file, service) pairs have already had
	// grpc_listens_on edges emitted, so duplicate `pb.RegisterXXXServer(s, ...)`
	// calls for the same service in one file don't multiply the edge count.
	// Key = "<relPath>::<service>". Schema 1.9.
	grpcServerImpls map[string]struct{}
	// grpcClientStubs maps a local variable name (within the current function
	// scope) to the gRPC service name extracted from its
	// `pb.NewXXXClient(...)` RHS assignment. Populated by
	// scanFuncBodyForGRPCStubs (which runs before the CallExpr walk for the
	// same body) and consumed by maybeEmitGRPCClientCall when typesInfo
	// cannot resolve the stub's receiver type directly. Re-initialized per
	// function scope. Schema 1.9.
	grpcClientStubs map[string]string
	// grpcClientPlaceholderIDs maps a gRPC-client-call target qname
	// ("grpc:Service.Method") to the AMBIGUOUS placeholder Endpoint ID
	// emitted in upsertGRPCClientPlaceholder. Mirrors httpClientPlaceholderIDs
	// — language="external" placeholder Endpoints live in a separate ID
	// space from real `language="go"` Endpoints minted by the server-side
	// RegisterXXXServer pass. Schema 1.9.
	grpcClientPlaceholderIDs map[string]string
	// chanVarIDs maps a channel variable name (within the current function scope)
	// to the Channel node ID emitted by emitChannelFromMake. Used to wire
	// sends_to/recvs_from edges to the actual Channel node instead of an
	// anonymous CallSite. Key = variable name string (AST-level, not qualified).
	// Re-initialized per function scope in emitFunctionBodyPos.
	chanVarIDs map[string]string
	// funcFieldTouches records, for each Function/Method node ID emitted by
	// visitFuncDecl, the set of struct-Field node IDs whose values are
	// read or written anywhere inside the body. Populated even when the
	// function holds NO lock — buildpipe's cross-function lock propagation
	// pass (opt-in via --lock-propagation) walks the call graph from
	// lock-holding callers into their callees and uses this map to discover
	// "what fields does this callee touch?" without re-parsing.
	//
	// Stored as a set (map[string]struct{}) per func so DFS dedup is O(1)
	// and accidental double-emit from repeated field references inside the
	// same body (e.g. `x.f++; x.f++`) collapses to a single entry.
	//
	// Nil-safe: callers must check map presence before iteration. Only
	// populated when typesInfo != nil — AST-only mode has no reliable
	// way to distinguish field references from method receivers.
	funcFieldTouches map[string]map[string]struct{}
}

func newDeclVisitor(fset *token.FileSet, relPath, pkgName string) *declVisitor {
	v := &declVisitor{fset: fset, relPath: relPath, pkgName: pkgName}
	pkgQ := pkgName
	// One package node per package directory: files of one package share it,
	// two packages that declare the same name do not.
	v.pkgID = parse.NodeID("go", path.Dir(filepath.ToSlash(relPath)), pkgQ, 0)
	v.nodes = append(v.nodes, types.Node{
		ID: v.pkgID, Type: types.NodePackage,
		Name: pkgName, QualifiedName: pkgQ,
		FilePath: relPath, StartLine: 1, EndLine: 1,
		Language: "go", Confidence: types.ConfExtracted,
	})
	fileQ := pkgQ + "/" + relPath
	v.fileID = MakeID(fileQ, "go", 0)
	v.nodes = append(v.nodes, types.Node{
		ID: v.fileID, Type: types.NodeFile,
		Name: relPath, QualifiedName: fileQ,
		FilePath: relPath, StartLine: 1, EndLine: 1,
		Language: "go", Confidence: types.ConfExtracted,
	})
	v.edges = append(v.edges, types.Edge{
		Src: v.pkgID, Dst: v.fileID,
		Type: types.EdgeContains, Count: 1, Confidence: types.ConfExtracted,
	})
	return v
}

// declScope is what a local declaration belongs to: the function whose body
// holds it, or the package-level variable whose initialiser holds it.
type declScope struct {
	id    string // node ID of the function, method or variable
	qname string // its qualified name
}

func (v *declVisitor) Visit(n ast.Node) ast.Visitor {
	switch d := n.(type) {
	case *ast.GenDecl:
		v.visitGenDecl(d)
		if v.scope.id == "" {
			// Package level: emitValueSpec walks each initialiser itself,
			// with the variable as scope, and a type holds no declarations.
			return nil
		}
	case *ast.FuncDecl:
		v.visitFuncDecl(d)
		// The body is walked here, with the function as scope, so the
		// declarations inside it are not taken for package-level ones.
		// The signature is walked with it: a parameter or result may be
		// of a struct type written on the spot.
		id, qname := v.funcDeclIDQname(d)
		outer := v.scope
		v.scope = declScope{id: id, qname: qname}
		if d.Recv != nil {
			ast.Walk(v, d.Recv)
		}
		ast.Walk(v, d.Type)
		if d.Body != nil {
			ast.Walk(v, d.Body)
		}
		v.scope = outer
		return nil
	case *ast.StructType:
		v.emitAnonymousStruct(d)
	case *ast.AssignStmt:
		if d.Tok == token.DEFINE {
			for _, lhs := range d.Lhs {
				v.emitLocalName(lhs, d.End())
			}
		}
	case *ast.RangeStmt:
		if d.Tok == token.DEFINE {
			v.emitLocalName(d.Key, d.X.End())
			v.emitLocalName(d.Value, d.X.End())
		}
	}
	return v
}

// emitStructFields emits the Field nodes of the struct type st under the
// node parentID and records st as done.
func (v *declVisitor) emitStructFields(parentID, parentQname, parentCanonical string, st *ast.StructType) {
	if st == nil || st.Fields == nil {
		return
	}
	if v.structsDone == nil {
		v.structsDone = map[*ast.StructType]bool{}
	}
	v.structsDone[st] = true
	for _, f := range st.Fields.List {
		v.emitFields(parentID, parentQname, parentCanonical, f)
	}
}

// emitAnonymousStruct emits the fields of a struct type that is written
// where it is used and so belongs to no type or variable declaration: the
// element type of a composite literal (`[]struct{ in, want int }{...}`), a
// parameter type, a conversion. The fields are named
// "<owner>.struct@<start byte>.<Field>" and defined by the owner, the
// function or variable whose code holds the type (the file at package level).
// They have no canonical id; a use finds them by position (fieldIndex).
func (v *declVisitor) emitAnonymousStruct(st *ast.StructType) {
	if v.structsDone[st] {
		return
	}
	prefix, definer := v.declOwner()
	_, startByte := v.pos(st.Pos())
	v.emitStructFields(definer, fmt.Sprintf("%s.struct@%d", prefix, startByte), "", st)
}

// emitLocalName emits a LocalVariable node for a name that a short variable
// declaration (x := ...) or a range clause (for k, v := range ...) declares.
// The node is named "<function>.<name>", spans the name up to end, and is
// defined by the function, or by the variable whose initialiser holds it.
//
// Only a name that go/types records as a definition gets a node: in
// `a, err := f()` an err that already exists in the scope is assigned, not
// declared. Without type information the two cannot be told apart, so an
// AST-only build emits no local names. The blank identifier names nothing.
func (v *declVisitor) emitLocalName(e ast.Expr, end token.Pos) {
	id, ok := e.(*ast.Ident)
	if !ok || id.Name == "_" || v.typesInfo == nil || v.typesInfo.Defs[id] == nil || v.scope.id == "" {
		return
	}
	qname := v.scope.qname + "." + id.Name
	startLine, startByte := v.pos(id.Pos())
	endLine, endByte := v.pos(end)
	nodeID := v.declID(qname, startByte)
	v.appendNode(nodeID, types.NodeLocalVariable, id.Name, qname,
		startLine, endLine, startByte, endByte, "", "", "")
	v.edges = append(v.edges, types.Edge{
		Src: v.scope.id, Dst: nodeID, Type: types.EdgeDefines, Count: 1, Confidence: types.ConfExtracted,
	})
}

// declOwner returns the qualified-name prefix of a type, var or const
// declared at the current position and the node that defines it: the package
// name and the file at package level, the function's name and the function
// inside a body. Named after the package alone, a local `var buf` was
// "pkg.buf", the name a package-level buf would have, and 3,194 local
// variables of go-stablenet shared a qualified name with another node.
func (v *declVisitor) declOwner() (prefix, definer string) {
	if v.scope.id != "" {
		return v.scope.qname, v.scope.id
	}
	return v.pkgName, v.fileID
}

func (v *declVisitor) pos(p token.Pos) (line, byteOff int) {
	pos := v.fset.Position(p)
	return pos.Line, pos.Offset
}

func (v *declVisitor) visitGenDecl(d *ast.GenDecl) {
	for _, spec := range d.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			v.emitTypeSpec(s, d.Doc)
		case *ast.ValueSpec:
			v.emitValueSpec(s, d.Tok)
		case *ast.ImportSpec:
			v.emitImportSpec(s)
		}
	}
}

func (v *declVisitor) emitTypeSpec(s *ast.TypeSpec, doc *ast.CommentGroup) {
	prefix, definer := v.declOwner()
	qname := prefix + "." + s.Name.Name
	startLine, startByte := v.pos(s.Pos())
	endLine, endByte := v.pos(s.End())
	id := v.declID(qname, startByte)
	// canonical id for the type itself: <importpath>.<Type> (symbol
	// identity). Empty in AST-only mode. Passed down so fields and interface
	// methods qualify against the type's canonical id, not the short pkg name.
	var cid string
	if v.typesInfo != nil {
		cid = goCanonicalID(v.typesInfo.ObjectOf(s.Name))
	}
	var nodeType types.NodeType
	switch t := s.Type.(type) {
	case *ast.StructType:
		nodeType = types.NodeStruct
		v.appendNode(id, nodeType, s.Name.Name, qname, startLine, endLine, startByte, endByte, exported(s.Name.Name), commentText(doc), "")
		v.setLastCanonicalID(cid)
		v.emitStructFields(id, qname, cid, t)
	case *ast.InterfaceType:
		nodeType = types.NodeInterface
		v.appendNode(id, nodeType, s.Name.Name, qname, startLine, endLine, startByte, endByte, exported(s.Name.Name), commentText(doc), "")
		v.setLastCanonicalID(cid)
		for _, f := range t.Methods.List {
			v.emitInterfaceMethod(id, qname, cid, f)
		}
	default:
		nodeType = types.NodeTypeAlias
		v.appendNode(id, nodeType, s.Name.Name, qname, startLine, endLine, startByte, endByte, exported(s.Name.Name), commentText(doc), "")
		v.setLastCanonicalID(cid)
		// A defined type over an anonymous struct (`type Rows []struct{..}`):
		// the struct's fields belong to the type.
		v.emitStructFields(id, qname, cid, anonymousStruct(s.Type))
	}
	v.edges = append(v.edges, types.Edge{
		Src: definer, Dst: id, Type: types.EdgeDefines, Count: 1, Confidence: types.ConfExtracted,
	})
}

// setLastCanonicalID assigns cid to the most recently appended node when cid is
// non-empty. Centralises the "set canonical id on the node we just appended"
// pattern used across the decl emitters.
func (v *declVisitor) setLastCanonicalID(cid string) {
	if cid != "" && len(v.nodes) > 0 {
		v.nodes[len(v.nodes)-1].CanonicalID = cid
	}
}

// subKindEmbedded marks the Field node of an embedded field.
const subKindEmbedded = "embedded"

// emitFields emits one Field node per name of the field declaration f, under
// the struct or the field parentID.
//
// An embedded field has no name in the syntax; Go names it after its type
// (x.Key for an embedded *Key), and that name gets the node. When the field's
// type is an anonymous struct, the struct's own fields are emitted under the
// field, so `Sig struct{ R, S int }` in Wire gives Wire.Sig, Wire.Sig.R and
// Wire.Sig.S.
func (v *declVisitor) emitFields(parentID, parentQname, parentCanonical string, f *ast.Field) {
	names := f.Names
	embedded := len(names) == 0
	if embedded {
		id := asTypeIdent(f.Type)
		if id == nil {
			return
		}
		names = []*ast.Ident{id}
	}
	for _, name := range names {
		qname := parentQname + "." + name.Name
		startLine, startByte := v.pos(f.Pos())
		endLine, endByte := v.pos(f.End())
		id := v.declID(qname, startByte)
		v.appendNode(id, types.NodeField, name.Name, qname,
			startLine, endLine, startByte, endByte,
			exported(name.Name), commentText(f.Doc), "")
		if embedded {
			v.nodes[len(v.nodes)-1].SubKind = subKindEmbedded
		}
		// field canonical id: <importpath>.<Type>.<Field>, derived from the
		// owning type's canonical id (a field *types.Var carries no receiver).
		// Skip the blank identifier `_` (struct padding fields) — see goCanonicalID.
		canonical := ""
		if parentCanonical != "" && name.Name != "_" {
			canonical = parentCanonical + "." + name.Name
			v.setLastCanonicalID(canonical)
		}
		v.emitStructFields(id, qname, canonical, anonymousStruct(f.Type))
		v.edges = append(v.edges, types.Edge{
			Src: parentID, Dst: id, Type: types.EdgeDefines, Count: 1, Confidence: types.ConfExtracted,
		})
		// Index by *types.Object so the accessed_under_lock pass can
		// resolve `recv.field` references back to this NodeField. Empty when
		// typesInfo is nil — that path emits nothing (avoids false
		// positives on AST-only mode where field receivers are ambiguous).
		if v.typesInfo != nil {
			if obj := v.typesInfo.Defs[name]; obj != nil {
				if v.fieldNodeIDs == nil {
					v.fieldNodeIDs = map[gotypes.Object]string{}
				}
				v.fieldNodeIDs[obj] = id
			}
		}
	}
}

// anonymousStruct returns the struct type literal that e is, points to or
// holds as an element (struct{..}, *struct{..}, []struct{..}, [N]struct{..},
// map[K]struct{..}, chan struct{..}); nil for any other type expression.
func anonymousStruct(e ast.Expr) *ast.StructType {
	switch t := e.(type) {
	case *ast.StructType:
		return t
	case *ast.ParenExpr:
		return anonymousStruct(t.X)
	case *ast.StarExpr:
		return anonymousStruct(t.X)
	case *ast.ArrayType:
		return anonymousStruct(t.Elt)
	case *ast.MapType:
		return anonymousStruct(t.Value)
	case *ast.ChanType:
		return anonymousStruct(t.Value)
	}
	return nil
}

func (v *declVisitor) emitInterfaceMethod(parentID, parentQname, parentCanonical string, f *ast.Field) {
	for _, name := range f.Names {
		qname := parentQname + "." + name.Name
		startLine, startByte := v.pos(f.Pos())
		endLine, endByte := v.pos(f.End())
		id := v.declID(qname, startByte)
		v.appendNode(id, types.NodeMethod, name.Name, qname,
			startLine, endLine, startByte, endByte,
			exported(name.Name), commentText(f.Doc), "")
		// interface-method canonical id: <importpath>.<Interface>.<Method>,
		// distinct from any concrete impl's <importpath>.(*T).<Method>.
		if parentCanonical != "" && name.Name != "_" {
			v.setLastCanonicalID(parentCanonical + "." + name.Name)
		}
		v.edges = append(v.edges, types.Edge{
			Src: parentID, Dst: id, Type: types.EdgeDefines, Count: 1, Confidence: types.ConfExtracted,
		})
	}
}

func (v *declVisitor) emitValueSpec(s *ast.ValueSpec, tok token.Token) {
	prefix, definer := v.declOwner()
	owners := make([]declScope, 0, len(s.Names))
	for _, name := range s.Names {
		qname := prefix + "." + name.Name
		startLine, startByte := v.pos(name.Pos())
		endLine, endByte := v.pos(s.End())
		id := v.declID(qname, startByte)
		nt := types.NodeVariable
		if tok == token.CONST {
			nt = types.NodeConstant
		}
		v.appendNode(id, nt, name.Name, qname, startLine, endLine, startByte, endByte,
			exported(name.Name), commentText(s.Doc), "")
		// canonical id only for PACKAGE-LEVEL const/var: the AST walk also
		// reaches `var x = …` declared inside function bodies, whose id
		// <importpath>.<name> is neither unique (many funcs declare `gspec`,
		// `err`, …) nor a useful retrieval target. go/types tells them apart:
		// a package-level object's parent scope is the package scope.
		if v.typesInfo != nil {
			if obj := v.typesInfo.ObjectOf(name); obj != nil && obj.Pkg() != nil &&
				obj.Parent() == obj.Pkg().Scope() {
				v.setLastCanonicalID(goCanonicalID(obj))
			}
		}
		v.edges = append(v.edges, types.Edge{
			Src: definer, Dst: id, Type: types.EdgeDefines, Count: 1, Confidence: types.ConfExtracted,
		})
		owners = append(owners, declScope{id: id, qname: qname})
		// A variable declared with an anonymous struct type
		// (`var Config struct{..}`): the struct's fields belong to it.
		if s.Type != nil {
			v.emitStructFields(id, qname, "", anonymousStruct(s.Type))
		}
	}
	// A package-level initialiser runs outside any function: its statements,
	// call sites and local declarations belong to the variable it
	// initialises. Inside a function the body walk already covers it.
	if v.scope.id != "" {
		return
	}
	for i, value := range s.Values {
		owner := owners[initialiserOwner(i, len(owners))]
		v.scope, v.inInitialiser = owner, true
		v.emitBodyPos(owner.qname, owner.id, value)
		ast.Walk(v, value)
		v.scope, v.inInitialiser = declScope{}, false
	}
}

// initialiserOwner is the index of the name that value i of a value
// specification initialises: its own name, or the first name when there are
// fewer names than values or one value for several names (`var a, b = f()`).
func initialiserOwner(i, names int) int {
	if i < names {
		return i
	}
	return 0
}

func (v *declVisitor) emitImportSpec(s *ast.ImportSpec) {
	pathLit := strings.Trim(s.Path.Value, "\"")
	qname := "import:" + pathLit
	startLine, startByte := v.pos(s.Pos())
	endLine, endByte := v.pos(s.End())
	id := v.declID(qname, startByte)
	v.appendNode(id, types.NodeImport, pathLit, qname,
		startLine, endLine, startByte, endByte, "", "", "")
	v.edges = append(v.edges, types.Edge{
		Src: v.fileID, Dst: id, Type: types.EdgeImports, Count: 1, Confidence: types.ConfExtracted,
	})
}

func (v *declVisitor) visitFuncDecl(d *ast.FuncDecl) {
	nt := types.NodeFunction
	if d.Recv != nil && len(d.Recv.List) > 0 {
		nt = types.NodeMethod
	}
	id, qname := v.funcDeclIDQname(d)
	startLine, startByte := v.pos(d.Pos())
	endLine, endByte := v.pos(d.End())
	sig := v.formatSignature(d)
	v.appendNode(id, nt, d.Name.Name, qname, startLine, endLine, startByte, endByte,
		exported(d.Name.Name), commentText(d.Doc), sig)
	if v.typesInfo != nil {
		v.setLastCanonicalID(goCanonicalID(v.typesInfo.ObjectOf(d.Name)))
	}
	v.edges = append(v.edges, types.Edge{
		Src: v.fileID, Dst: id, Type: types.EdgeDefines, Count: 1, Confidence: types.ConfExtracted,
	})
	v.emitFunctionBodyPos(qname, id, d.Body)
	// Emit accessed_under_lock(field, mutex) for fields
	// referenced inside a function that holds at least one lock. No-op when
	// typesInfo is nil or the body holds no lock — keeps AST-only mode safe.
	v.emitAccessedUnderLock(id, d.Body)
	// Lock propagation (opt-in via --lock-propagation): record funcID → fields
	// touched for ALL functions (lock-holding or not). buildpipe's
	// propagateLockedFieldAccessDFS reads this map after Pass 2 to walk the
	// call graph and emit accessed_under_lock for callee field accesses under
	// caller-held locks. No-op when typesInfo is nil — declarations.go's
	// fieldNodeIDs is empty in AST-only mode and collectFieldAccesses would
	// return an empty set anyway.
	v.recordFuncFieldTouches(id, d.Body)
}

// helpers

func (v *declVisitor) appendNode(id string, t types.NodeType, name, qname string,
	startLine, endLine, startByte, endByte int, vis, doc, sig string) {
	v.nodes = append(v.nodes, types.Node{
		ID: id, Type: t, Name: name, QualifiedName: qname,
		FilePath: v.relPath, StartLine: startLine, EndLine: endLine,
		StartByte: startByte, EndByte: endByte,
		Language: "go", Visibility: vis, DocComment: doc, Signature: sig,
		Confidence: types.ConfExtracted,
	})
}

func exported(name string) string {
	if name == "" {
		return "private"
	}
	if name[0] >= 'A' && name[0] <= 'Z' {
		return "exported"
	}
	return "private"
}

func commentText(g *ast.CommentGroup) string {
	if g == nil {
		return ""
	}
	return strings.TrimSpace(g.Text())
}

// goCanonicalID builds the globally-unique import-path-qualified identity of a
// declared symbol from its go/types object, per docs/symbol-identity-design.md:
//
//	function:         <importpath>.<Func>
//	method:           <importpath>.(<*?RecvType>).<Method>   (pointer star preserved)
//	interface method: <importpath>.<Interface>.<Method>
//	type/const/var:   <importpath>.<Name>
//
// Only package-level declarations and methods get an id. A type, const or var
// declared inside a function body shares its name with package-level ones
// (gencodec's `type Genesis struct` inside (Genesis).MarshalJSON next to the
// package-level core.Genesis), so giving it <importpath>.<Name> made callers
// of the package-level type resolve to the local one.
//
// Generic functions and methods resolve to their origin, so a call on an
// instantiated type (Prque[int, string].Push) has the declaration's id.
//
// Returns "" when the object has no package (builtins), is the blank
// identifier, or is not package level, so callers leave the node's CanonicalID
// empty rather than emitting an ambiguous id.
func goCanonicalID(obj gotypes.Object) string {
	if obj == nil || obj.Pkg() == nil {
		return ""
	}
	// The blank identifier `_` is not an addressable symbol — many package-level
	// `var _ = ...` / `_ "import"` declarations share the name, so an id like
	// `<pkg>._` is intentionally non-unique. Leave it empty.
	if obj.Name() == "_" {
		return ""
	}
	pkgPath := obj.Pkg().Path()
	fn, ok := obj.(*gotypes.Func)
	if !ok {
		if obj.Parent() != obj.Pkg().Scope() {
			return ""
		}
		return pkgPath + "." + obj.Name()
	}
	fn = fn.Origin()
	sig, ok := fn.Type().(*gotypes.Signature)
	if !ok || sig.Recv() == nil {
		if fn.Parent() != fn.Pkg().Scope() {
			return ""
		}
		return pkgPath + "." + fn.Name()
	}
	rt := sig.Recv().Type()
	star := ""
	if ptr, ok := rt.(*gotypes.Pointer); ok {
		rt = ptr.Elem()
		star = "*"
	}
	named, ok := rt.(*gotypes.Named)
	if !ok || named.Obj() == nil {
		return ""
	}
	if gotypes.IsInterface(named) {
		return pkgPath + "." + named.Obj().Name() + "." + fn.Name()
	}
	return pkgPath + ".(" + star + named.Obj().Name() + ")." + fn.Name()
}

// recvTypeName is the type name of a method receiver: T for (t T), (t *T),
// and for the generic receivers (t *T[K]) and (t *T[K, V]), whose type
// expression is an index over the type name.
func recvTypeName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.StarExpr:
		return recvTypeName(t.X)
	case *ast.ParenExpr:
		return recvTypeName(t.X)
	case *ast.IndexExpr:
		return recvTypeName(t.X)
	case *ast.IndexListExpr:
		return recvTypeName(t.X)
	}
	return exprName(e)
}

func exprName(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return exprName(t.X)
	case *ast.SelectorExpr:
		return t.Sel.Name
	}
	return ""
}

// formatSignature renders a one-line Go signature with the real parameter
// and result lists, e.g. "func (Core) handle(x int) (bool, error)". The
// parameter/result lists are printed from the AST via go/printer (accurate
// for variadics, multi-name fields, qualified types) and then whitespace-
// collapsed so a multi-line source signature still yields a single line.
//
// The signature is the highest-signal per-symbol field a retrieval consumer
// reads to answer "what does this check"; the historical
// "func name(...) ..." placeholder hid every parameter (e.g. isJustified's
// targetView), which defeated that purpose.
func (v *declVisitor) formatSignature(d *ast.FuncDecl) string {
	var b strings.Builder
	b.WriteString("func ")
	if d.Recv != nil && len(d.Recv.List) > 0 {
		_, _ = fmt.Fprintf(&b, "(%s) ", recvTypeName(d.Recv.List[0].Type))
	}
	b.WriteString(d.Name.Name)

	var ft strings.Builder
	if err := printer.Fprint(&ft, v.fset, d.Type); err != nil {
		// Fall back to the old placeholder if the type fails to print
		// (should not happen for a well-formed FuncDecl).
		b.WriteString("(...)")
		if d.Type.Results != nil {
			b.WriteString(" ...")
		}
		return strings.Join(strings.Fields(b.String()), " ")
	}
	// printer renders the FuncType as "func(params) results" — drop the
	// leading "func" so it joins onto the receiver+name we already wrote.
	b.WriteString(strings.TrimPrefix(ft.String(), "func"))

	// Collapse any newlines/tabs/runs of spaces (multi-line source
	// signatures, gofmt alignment) into single spaces, then trim the
	// artifacts a multi-line field list leaves behind: a space just inside
	// the parens and the trailing comma the printer keeps before a
	// newline-preceded close paren.
	s := strings.Join(strings.Fields(b.String()), " ")
	s = strings.ReplaceAll(s, "( ", "(")
	s = strings.ReplaceAll(s, " )", ")")
	s = strings.ReplaceAll(s, ",)", ")")
	return s
}
