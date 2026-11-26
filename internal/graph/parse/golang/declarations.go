package golang

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/printer"
	"go/token"
	gotypes "go/types"
	"sort"
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
	// src and comments are the file's bytes and its comment groups in
	// source order; funcSpan reads the comment after a function from them.
	// src is nil when the caller has no source, and a function then ends
	// at its last token.
	src      []byte
	comments []*ast.CommentGroup
	// scope is the function whose body is being walked; zero at package
	// level. A type, var or const declared inside a body is named under the
	// function and defined by it (see declOwner).
	scope declScope
	// structsDone holds the struct type expressions whose fields already
	// have nodes, so the walk does not emit them a second time when it
	// reaches the expression itself (see emitStructFields).
	structsDone map[*ast.StructType]bool
	// ifacesDone holds the interface type expressions whose methods already
	// have nodes (see emitInterfaceMethods).
	ifacesDone map[*ast.InterfaceType]bool
	// netRPC says the build uses net/rpc, so a method with the net/rpc
	// handler shape may be a handler (see maybeEmitJSONRPCHandler).
	netRPC bool
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
	// chanVarIDs maps a channel variable of the current function to the
	// Channel node ID emitted by emitChannelFromMake. Used to wire
	// sends_to/recvs_from edges to the actual Channel node instead of an
	// anonymous CallSite. The key is chanVar's: the variable's object, or its
	// name without type information. Re-initialized per function scope in
	// emitBodyPos.
	chanVarIDs map[any]string
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
	v.pkgID = packageNodeID(relPath, pkgName)
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
	case *ast.InterfaceType:
		v.emitAnonymousInterface(d)
	case *ast.AssignStmt:
		if d.Tok == token.DEFINE {
			for _, lhs := range d.Lhs {
				v.emitLocalName(lhs, d.End(), false)
			}
		}
	case *ast.RangeStmt:
		if d.Tok == token.DEFINE {
			v.emitLocalName(d.Key, d.X.End(), false)
			v.emitLocalName(d.Value, d.X.End(), false)
		}
	case *ast.TypeSwitchStmt:
		// switch v := x.(type): v is declared by the header, and every case
		// clause has a variable of its own type at that one identifier.
		if guard, ok := d.Assign.(*ast.AssignStmt); ok && guard.Tok == token.DEFINE && len(guard.Lhs) == 1 {
			v.emitLocalName(guard.Lhs[0], guard.End(), true)
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

// emitAnonymousInterface emits the methods of an interface type that is
// written where it is used and so belongs to no type declaration: a parameter
// type (func Slot(b interface{ Bytes() []byte })), the type of an assertion.
// The methods are named "<owner>.interface@<start byte>.<Method>" and defined
// by the owner, as the fields of an anonymous struct are. They have no
// canonical id; a call finds them by position (posIndex). go-stablenet has 12
// such methods, called at 9 sites.
func (v *declVisitor) emitAnonymousInterface(it *ast.InterfaceType) {
	if v.ifacesDone[it] {
		return
	}
	prefix, definer := v.declOwner()
	v.emitInterfaceMethods(definer, anonymousInterfaceQname(prefix, v, it), "", it)
}

// anonymousInterfaceQname names an interface type written without a name
// under the node whose qualified name is prefix.
func anonymousInterfaceQname(prefix string, v *declVisitor, it *ast.InterfaceType) string {
	_, startByte := v.pos(it.Pos())
	return fmt.Sprintf("%s.interface@%d", prefix, startByte)
}

// emitInterfaceMethods emits the Method nodes of the interface type it under
// the node parentID and records it as done.
func (v *declVisitor) emitInterfaceMethods(parentID, parentQname, parentCanonical string, it *ast.InterfaceType) {
	if it == nil || it.Methods == nil {
		return
	}
	if v.ifacesDone == nil {
		v.ifacesDone = map[*ast.InterfaceType]bool{}
	}
	v.ifacesDone[it] = true
	for _, f := range it.Methods.List {
		v.emitInterfaceMethod(parentID, parentQname, parentCanonical, f)
	}
}

// emitInterfacesUnder emits the methods of the unnamed interface types
// written under root, a part of the declaration named parentQname: the type
// of a struct field, an element type, the constraint of a type parameter
// (type Set[T interface{ Key() string }]). Package-level declarations are not
// walked by Visit, so their type expressions are read here.
//
// The methods are named under the declaration and defined by it, as the
// methods of an interface written in a function's signature are defined by
// the function: every method of an unnamed interface belongs to the
// declaration that holds the interface.
func (v *declVisitor) emitInterfacesUnder(parentID, parentQname string, root ast.Node) {
	ast.Inspect(root, func(n ast.Node) bool {
		if it, ok := n.(*ast.InterfaceType); ok && !v.ifacesDone[it] {
			v.emitInterfaceMethods(parentID, anonymousInterfaceQname(parentQname, v, it), "", it)
		}
		return true
	})
}

// A short variable declaration has no keyword: its node starts at the name
// (the first token of what declares it) and its id is keyed there too.
//
// emitLocalName emits a LocalVariable node for a name that a short variable
// declaration (x := ...), a range clause (for k, v := range ...) or the
// header of a type switch (switch v := x.(type), with guard set) declares.
// The node is named "<function>.<name>", spans the name up to end, and is
// defined by the function, or by the variable whose initialiser holds it.
//
// Only a name that go/types records as a definition gets a node: in
// `a, err := f()` an err that already exists in the scope is assigned, not
// declared. Without type information the two cannot be told apart, so an
// AST-only build emits no local names. The blank identifier names nothing.
//
// go/types records no definition for the name in a type switch header: each
// case clause has an implicit variable instead, all positioned at the header
// identifier. The node stands there, so the uses in every clause reach it
// (83 such variables with 602 uses in go-stablenet had neither node nor edge).
func (v *declVisitor) emitLocalName(e ast.Expr, end token.Pos, guard bool) {
	id, ok := e.(*ast.Ident)
	if !ok || id.Name == "_" || v.typesInfo == nil || v.scope.id == "" {
		return
	}
	if !guard && v.typesInfo.Defs[id] == nil {
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
			v.emitTypeSpec(s, spanOf(d, s), specDoc(d, s.Doc))
		case *ast.ValueSpec:
			v.emitValueSpec(s, d.Tok, spanOf(d, s), specDoc(d, s.Doc))
		case *ast.ImportSpec:
			v.emitImportSpec(s, spanOf(d, s))
		}
	}
}

// declSpan is the part of a file that a declaration node covers.
type declSpan struct {
	start, end token.Pos
}

// spanOf returns the span of the node of the specification spec of the
// declaration d. It applies the span rule of every Go declaration node:
//
//	A declaration's span starts at its first token and runs through the
//	comment that follows it on its last line.
//
// Where it starts. For a function the first token is `func`. For a type,
// var, const or import that stands alone (`type T struct{...}`, `var x = 1`,
// also inside a function body) it is the keyword too. In a parenthesised
// group (`var ( a = 1; b = 2 )`) each specification is a declaration of its
// own and starts at its first token, the name. A struct field and an
// interface method start at their first token as well.
//
// Where it ends. The comment after a declaration on its last line
// (`MaxSkeletonSize = 128 // Number of ...`) describes that declaration and
// belongs to no other node, so the span includes it (withComment). The
// syntax tree attaches such a comment to a specification, a struct field and
// an interface method. It attaches none to a function, so for a function or
// method the file's comment list decides (funcSpan): the span runs through
// the comments that start on the line of its last token with only blanks
// between (`func (s *Ethereum) IsListening() bool { return true } // Always
// listening`), as the syntax tree groups a line comment. A comment on a
// later line is not one of them: it is the next declaration's doc or stands
// alone.
//
// Names that share a declaration. Every name of one specification
// (`var a, b = f()`) and of one field declaration (`rmu, wmu sync.Mutex`)
// spans the whole of it, so the text stored for each name is the complete
// declaration. Such nodes have the same span, and blank names (`var _, _ =
// f()`) the same name and qualified name too: the Signature of each says
// which name it is, "name 2 of 4" (ordinalSignature). A Mutex node has the
// span of the Variable or Field that declares the mutex.
//
// While type, var and const nodes started at their name, the keyword was in
// no node's text: a search for `type \w+ struct` found 0 of the 1,091
// production struct declarations of go-stablenet, and a name after the first
// in a specification held a text that began in the middle of it. While they
// ended at their last token, the comment after 367 package-level
// specifications was in no node's text.
//
// A node's id does not follow its span: it stays keyed by the offset of the
// declaring identifier (declID), which is unique where two names share one
// span, and which is how a use finds a declaration without a canonical id
// (posIndex.declaredAt).
func spanOf(d *ast.GenDecl, spec ast.Spec) declSpan {
	sp := declSpan{start: d.Pos(), end: spec.End()}
	if d.Lparen.IsValid() {
		sp.start = spec.Pos()
	}
	switch s := spec.(type) {
	case *ast.ValueSpec:
		sp.end = withComment(sp.end, s.Comment)
	case *ast.TypeSpec:
		sp.end = withComment(sp.end, s.Comment)
	case *ast.ImportSpec:
		sp.end = withComment(sp.end, s.Comment)
	}
	return sp
}

// fieldSpan is the span of a struct field or an interface method (see
// spanOf for the rule).
func fieldSpan(f *ast.Field) declSpan {
	return declSpan{start: f.Pos(), end: withComment(f.End(), f.Comment)}
}

// funcSpan is the span of a function or method declaration (see spanOf for
// the rule): from `func` through the comments that follow its last token on
// that line, with only blanks between.
func (v *declVisitor) funcSpan(d *ast.FuncDecl) declSpan {
	sp := declSpan{start: d.Pos(), end: d.End()}
	tf := v.fset.File(sp.end)
	if tf == nil || len(v.src) == 0 {
		return sp
	}
	// The first comment group after the declaration's last token.
	i := sort.Search(len(v.comments), func(i int) bool { return v.comments[i].Pos() >= sp.end })
	if i == len(v.comments) {
		return sp
	}
	line := tf.Line(sp.end)
	for _, c := range v.comments[i].List {
		if tf.Line(c.Pos()) != line || !blankBetween(v.src, tf.Offset(sp.end), tf.Offset(c.Pos())) {
			break
		}
		sp.end = c.End()
		line = tf.Line(c.End())
	}
	return sp
}

// lineBlanks are the bytes that may stand between a function's last token
// and the comment after it (funcSpan).
const lineBlanks = " \t"

// blankBetween reports whether src holds only spaces and tabs from byte from
// up to byte to.
func blankBetween(src []byte, from, to int) bool {
	if from < 0 || to > len(src) || from > to {
		return false
	}
	return len(bytes.Trim(src[from:to], lineBlanks)) == 0
}

// withComment returns the end of a declaration that ends at end and is
// followed on its last line by the comment c (nil for none).
func withComment(end token.Pos, c *ast.CommentGroup) token.Pos {
	if c != nil && c.End() > end {
		return c.End()
	}
	return end
}

// ordinalSignature is the Signature of name i (from 0) of the n names that
// one declaration declares: "" for a single name, else "name 2 of 4". It is
// what tells apart nodes that share a span (see spanOf).
func ordinalSignature(i, n int) string {
	if n < 2 {
		return ""
	}
	return fmt.Sprintf("name %d of %d", i+1, n)
}

// specDoc returns the doc comment of one specification of the declaration d,
// whose own comment is own.
//
// A comment above a single declaration (`// Limit is ...` over `const Limit =
// 10`) is attached to the declaration, not to its specification. In a group
// (`type ( ... )`) the declaration's comment describes the group and each
// member has its own or none. Reading only the specification lost the comment
// of 658 single var and const declarations of go-stablenet; reading only the
// declaration gave 13 grouped types the group's comment and lost 12 others.
func specDoc(d *ast.GenDecl, own *ast.CommentGroup) *ast.CommentGroup {
	if own != nil || d.Lparen.IsValid() {
		return own
	}
	return d.Doc
}

// emitTypeSpec emits the node of a type declaration with the span sp
// (spanOf).
func (v *declVisitor) emitTypeSpec(s *ast.TypeSpec, sp declSpan, doc *ast.CommentGroup) {
	prefix, definer := v.declOwner()
	qname := prefix + "." + s.Name.Name
	startLine, startByte := v.pos(sp.start)
	endLine, endByte := v.pos(sp.end)
	_, nameByte := v.pos(s.Name.Pos())
	id := v.declID(qname, nameByte)
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
		v.emitInterfaceMethods(id, qname, cid, t)
	default:
		// Every other named type is a TypeAlias node; the sub kind says
		// whether it is another name for a type (type A = B) or a type of
		// its own (type A B), which has its own method set.
		nodeType = types.NodeTypeAlias
		v.appendNode(id, nodeType, s.Name.Name, qname, startLine, endLine, startByte, endByte, exported(s.Name.Name), commentText(doc), "")
		v.nodes[len(v.nodes)-1].SubKind = types.SubKindDefinedType
		if s.Assign.IsValid() {
			v.nodes[len(v.nodes)-1].SubKind = types.SubKindAlias
		}
		v.setLastCanonicalID(cid)
		// A defined type over an anonymous struct (`type Rows []struct{..}`):
		// the struct's fields belong to the type.
		v.emitStructFields(id, qname, cid, anonymousStruct(s.Type))
	}
	if s.TypeParams != nil {
		v.emitInterfacesUnder(id, qname, s.TypeParams)
	}
	v.emitInterfacesUnder(id, qname, s.Type)
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
	sp := fieldSpan(f)
	for i, name := range names {
		qname := parentQname + "." + name.Name
		startLine, startByte := v.pos(sp.start)
		endLine, endByte := v.pos(sp.end)
		id := v.declID(qname, startByte)
		v.appendNode(id, types.NodeField, name.Name, qname,
			startLine, endLine, startByte, endByte,
			exported(name.Name), commentText(f.Doc), ordinalSignature(i, len(names)))
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
	sp := fieldSpan(f)
	for _, name := range f.Names {
		qname := parentQname + "." + name.Name
		startLine, startByte := v.pos(sp.start)
		endLine, endByte := v.pos(sp.end)
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

// emitValueSpec emits one node per name of a var or const specification, all
// with the span sp (spanOf).
func (v *declVisitor) emitValueSpec(s *ast.ValueSpec, tok token.Token, sp declSpan, doc *ast.CommentGroup) {
	prefix, definer := v.declOwner()
	owners := make([]declScope, 0, len(s.Names))
	for i, name := range s.Names {
		qname := prefix + "." + name.Name
		startLine, startByte := v.pos(sp.start)
		endLine, endByte := v.pos(sp.end)
		_, nameByte := v.pos(name.Pos())
		id := v.declID(qname, nameByte)
		nt := types.NodeVariable
		if tok == token.CONST {
			nt = types.NodeConstant
		}
		v.appendNode(id, nt, name.Name, qname, startLine, endLine, startByte, endByte,
			exported(name.Name), commentText(doc), ordinalSignature(i, len(s.Names)))
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
			v.emitInterfacesUnder(id, qname, s.Type)
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
		v.scope = owner
		v.emitBodyPos(owner.qname, owner.id, value)
		ast.Walk(v, value)
		v.scope = declScope{}
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

// emitImportSpec emits the node of an import with the span sp (spanOf).
func (v *declVisitor) emitImportSpec(s *ast.ImportSpec, sp declSpan) {
	pathLit := strings.Trim(s.Path.Value, "\"")
	qname := "import:" + pathLit
	startLine, startByte := v.pos(sp.start)
	endLine, endByte := v.pos(sp.end)
	_, specByte := v.pos(s.Pos())
	id := v.declID(qname, specByte)
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
	sp := v.funcSpan(d)
	startLine, startByte := v.pos(sp.start)
	endLine, endByte := v.pos(sp.end)
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
// A method declared on an alias (type E = fe12; func (e *E) One()) belongs to
// the aliased type and has its id, <importpath>.(*fe12).One: the receiver is
// unaliased first. Left as written, the alias is not a named type, the id was
// empty, and the 9 calls of such methods in go-stablenet had no edge.
//
// The Error method of the builtin error interface has no package; it gets
// BuiltinErrorMethodID so that err.Error() has a target (222 calls).
//
// Returns "" when the object has no package (other builtins), is the blank
// identifier, or is not package level, so callers leave the node's CanonicalID
// empty rather than emitting an ambiguous id.
func goCanonicalID(obj gotypes.Object) string {
	if obj == nil {
		return ""
	}
	if obj.Pkg() == nil {
		if obj == builtinErrorMethod {
			return BuiltinErrorMethodID
		}
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
	named, star := receiverBase(sig.Recv().Type())
	if named == nil || named.Obj() == nil || !isPackageLevel(named.Obj()) {
		// A method of an interface declared inside a function has no id,
		// like the interface: <importpath>.<Interface>.<Method> would be the
		// id of a package-level interface of that name.
		return ""
	}
	if gotypes.IsInterface(named) {
		return pkgPath + "." + named.Obj().Name() + "." + fn.Name()
	}
	return pkgPath + ".(" + star + named.Obj().Name() + ")." + fn.Name()
}

// isPackageLevel reports whether obj is declared in the scope of its package.
func isPackageLevel(obj gotypes.Object) bool {
	return obj.Pkg() != nil && obj.Parent() == obj.Pkg().Scope()
}

// receiverBase returns the named type a receiver of type rt belongs to and
// "*" when the receiver is a pointer, looking through aliases on both levels
// ((e *E) with type E = fe12 gives fe12, "*"). nil for an unnamed receiver
// type (a method of an interface written without a name).
func receiverBase(rt gotypes.Type) (named *gotypes.Named, star string) {
	rt = gotypes.Unalias(rt)
	if ptr, ok := rt.(*gotypes.Pointer); ok {
		rt, star = gotypes.Unalias(ptr.Elem()), "*"
	}
	named, _ = rt.(*gotypes.Named)
	return named, star
}

// builtinErrorMethod is the Error method of the builtin error interface.
var builtinErrorMethod = func() gotypes.Object {
	iface, ok := gotypes.Universe.Lookup("error").Type().Underlying().(*gotypes.Interface)
	if !ok || iface.NumMethods() == 0 {
		return nil
	}
	return iface.Method(0)
}()

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
