package solidity

import (
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Sub kinds of the declaration nodes this file adds.
const (
	// subKindStructMember marks a Field node that is a member of a struct,
	// not a state variable: it has no storage slot of its own.
	subKindStructMember = "struct_member"
	// subKindEnumValue marks a Constant node that is a value of an enum.
	subKindEnumValue = "enum_value"
	// subKindConstant marks a Constant node declared with `constant`. A
	// constant is compiled into the code and has no storage slot.
	subKindConstant = "constant"
	// subKindImmutable marks a Field declared with `immutable`. An immutable
	// is written once in the constructor into the code, not into storage.
	subKindImmutable = "immutable"
	// subKindStoragePrefix starts the sub kind of a state variable that has
	// a written visibility (`storage_public`).
	subKindStoragePrefix = "storage_"
)

// Tree-sitter node kinds and fields read while emitting declarations.
const (
	kindComment            = "comment"
	kindVisibility         = "visibility"
	kindModifierInvocation = "modifier_invocation"
	kindStructMember       = "struct_member"
	kindEnumValue          = "enum_value"
	kindImportDirective    = "import_directive"
	kindConstantKeyword    = "constant"
	kindImmutableKeyword   = "immutable"
	kindString             = "string"
	fieldBody              = "body"
	fieldType              = "type"
	fieldSource            = "source"
)

// NatSpec comment markers.
const (
	natSpecLine       = "///"
	natSpecBlockOpen  = "/**"
	natSpecBlockClose = "*/"
)

// declSpec describes one declaration node to emit.
type declSpec struct {
	nt    types.NodeType
	name  string
	qname string
	// decl is the whole declaration; the node spans it.
	decl *sitter.Node
	// idByte is the byte that keys the node id: the start of the name
	// identifier, or the start of the declaration when it has no name.
	idByte int
	// definer is the id of the node that defines this one.
	definer    string
	subKind    string
	signature  string
	visibility string
}

// emitDecl appends the node of one declaration and the defines edge from its
// definer, and returns the node id.
//
// The node spans the whole declaration, from its first token to its closing
// brace or semicolon. It used to span only the name identifier: all 265
// functions of the system contracts had start_line = end_line and the stored
// text of `isValidator` was the 11 bytes of its name. The name position is
// still the id key (spec.idByte), so code that finds a callable from a node
// inside it computes the same id.
func (v *declVisitor) emitDecl(spec declSpec) string {
	id := v.declID(spec.qname, spec.idByte)
	v.nodes = append(v.nodes, types.Node{
		ID: id, Type: spec.nt, Name: spec.name, QualifiedName: spec.qname,
		FilePath:  v.rel,
		StartLine: int(spec.decl.StartPosition().Row) + 1,
		EndLine:   int(spec.decl.EndPosition().Row) + 1,
		StartByte: int(spec.decl.StartByte()), EndByte: int(spec.decl.EndByte()),
		Language: langSol, Confidence: types.ConfExtracted,
		SubKind:    spec.subKind,
		Signature:  spec.signature,
		Visibility: spec.visibility,
		DocComment: natSpecOf(spec.decl, v.src),
	})
	v.edges = append(v.edges, types.Edge{
		Src: spec.definer, Dst: id, Type: types.EdgeDefines,
		Count: 1, Confidence: types.ConfExtracted,
	})
	return id
}

// emitCallable emits the Function node of a function, constructor, fallback
// or receive declaration, or the Modifier node of a modifier, and returns its
// id. subKind tells the roles apart.
func (v *declVisitor) emitCallable(decl *sitter.Node, subKind string) string {
	name, idByte, ok := callableKey(decl, v.src)
	if !ok {
		return ""
	}
	nt := types.NodeFunction
	if decl.Kind() == kindModifierDef {
		nt = types.NodeModifier
	}
	return v.emitDecl(declSpec{
		nt: nt, name: name, qname: v.memberQName(decl, name), decl: decl,
		idByte: idByte, definer: v.definerOf(decl), subKind: subKind,
		signature:  declHeader(decl, v.src),
		visibility: visibilityOf(decl, v.src),
	})
}

// lastNode returns the node emitDecl appended last, so a caller can set the
// fields only its kind has.
func (v *declVisitor) lastNode() *types.Node {
	return &v.nodes[len(v.nodes)-1]
}

// declHeader returns the text of a declaration up to its body, or up to its
// closing semicolon when it has no body, with whitespace collapsed and with
// modifier invocations and comments left out. For a function that is its
// name, parameter types and names, visibility, mutability, virtual or
// override, and return types; the modifiers it applies are has_modifier
// edges. For a contract it is its kind, its name and its bases. Pass 2 reads
// the bases and the parameter count back from this text (basesOf,
// parameterCount), so a comment inside the header must not be part of it.
func declHeader(decl *sitter.Node, src []byte) string {
	end := decl.EndByte()
	if body := decl.ChildByFieldName(fieldBody); body != nil {
		end = body.StartByte()
	}
	start := decl.StartByte()
	text := []byte(string(src[start:end]))
	var blank func(n *sitter.Node)
	blank = func(n *sitter.Node) {
		for i := uint(0); i < n.ChildCount(); i++ {
			c := n.Child(i)
			if c == nil || c.StartByte() >= end {
				continue
			}
			if c.Kind() != kindModifierInvocation && c.Kind() != kindComment {
				blank(c)
				continue
			}
			for b := c.StartByte(); b < min(c.EndByte(), end); b++ {
				text[b-start] = ' '
			}
		}
	}
	blank(decl)
	header := strings.Join(strings.Fields(string(text)), " ")
	header = strings.TrimSuffix(header, ";")
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(header), "{"))
}

// visibilityOf returns the visibility keyword written on a declaration
// (external, public, internal, private), or "" when none is written.
func visibilityOf(decl *sitter.Node, src []byte) string {
	for i := uint(0); i < decl.ChildCount(); i++ {
		c := decl.Child(i)
		if c != nil && c.Kind() == kindVisibility {
			return strings.TrimSpace(c.Utf8Text(src))
		}
	}
	return ""
}

// hasKeywordChild reports whether a declaration has a direct child token of
// the given kind. `constant` is an anonymous token and `immutable` a named
// one, so the scan covers every child.
func hasKeywordChild(decl *sitter.Node, kind string) bool {
	for i := uint(0); i < decl.ChildCount(); i++ {
		if c := decl.Child(i); c != nil && c.Kind() == kind {
			return true
		}
	}
	return false
}

// natSpecOf returns the NatSpec comment written directly above a declaration:
// the run of `///` lines or the `/** */` block that ends on the line before
// it. Comment markers are removed and lines are joined by newlines. A plain
// `//` comment or a blank line ends the run.
func natSpecOf(decl *sitter.Node, src []byte) string {
	var blocks []string
	nextRow := decl.StartPosition().Row
	for c := decl.PrevSibling(); c != nil && c.Kind() == kindComment; c = c.PrevSibling() {
		if c.EndPosition().Row+1 < nextRow {
			break
		}
		text := c.Utf8Text(src)
		if !strings.HasPrefix(text, natSpecLine) && !strings.HasPrefix(text, natSpecBlockOpen) {
			break
		}
		blocks = append([]string{cleanNatSpec(text)}, blocks...)
		nextRow = c.StartPosition().Row
	}
	return strings.TrimSpace(strings.Join(blocks, "\n"))
}

// cleanNatSpec removes the comment markers of one NatSpec comment.
func cleanNatSpec(text string) string {
	if strings.HasPrefix(text, natSpecLine) {
		return strings.TrimSpace(strings.TrimPrefix(text, natSpecLine))
	}
	text = strings.TrimSuffix(strings.TrimPrefix(text, natSpecBlockOpen), natSpecBlockClose)
	var lines []string
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		line = strings.TrimSpace(strings.TrimPrefix(line, "*"))
		lines = append(lines, line)
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// typeText returns the type expression of a type_name node with whitespace
// collapsed (`mapping(address => uint256)`, `SetLib.Set`, `uint256[]`).
func typeText(typeNode *sitter.Node, src []byte) string {
	if typeNode == nil {
		return ""
	}
	return strings.Join(strings.Fields(string(src[typeNode.StartByte():typeNode.EndByte()])), " ")
}

// emitStructMembers emits one Field node per member of a struct, defined by
// the struct. The system contracts declare 43 struct members and the graph
// had none.
func (v *declVisitor) emitStructMembers(structID, structQName string, decl *sitter.Node) {
	body := decl.ChildByFieldName(fieldBody)
	if body == nil {
		return
	}
	for i := uint(0); i < body.NamedChildCount(); i++ {
		member := body.NamedChild(i)
		if member == nil || member.Kind() != kindStructMember {
			continue
		}
		nameNode := member.ChildByFieldName(fieldName)
		if nameNode == nil {
			continue
		}
		name := nameNode.Utf8Text(v.src)
		v.emitDecl(declSpec{
			nt: types.NodeField, name: name, qname: structQName + qnameSep + name,
			decl: member, idByte: int(nameNode.StartByte()), definer: structID,
			subKind:   subKindStructMember,
			signature: typeText(member.ChildByFieldName(fieldType), v.src),
		})
	}
}

// emitEnumValues emits one Constant node per value of an enum, defined by
// the enum. The system contracts declare 13 enum values and the graph had
// none.
func (v *declVisitor) emitEnumValues(enumID, enumQName string, decl *sitter.Node) {
	body := decl.ChildByFieldName(fieldBody)
	if body == nil {
		return
	}
	for i := uint(0); i < body.NamedChildCount(); i++ {
		value := body.NamedChild(i)
		if value == nil || value.Kind() != kindEnumValue {
			continue
		}
		name := value.Utf8Text(v.src)
		v.emitDecl(declSpec{
			nt: types.NodeConstant, name: name, qname: enumQName + qnameSep + name,
			decl: value, idByte: int(value.StartByte()), definer: enumID,
			subKind: subKindEnumValue,
		})
	}
}

// runImports emits one Import node per import directive, named by the
// imported path, with an imports edge from the file. Pass 2 adds an imports
// edge from the Import node to the imported file when that file is in the
// graph (linkImports). The system contracts have 32 import directives and the graph
// recorded none.
func (v *declVisitor) runImports() {
	if v.root == nil {
		return
	}
	for i := uint(0); i < v.root.NamedChildCount(); i++ {
		directive := v.root.NamedChild(i)
		if directive == nil || directive.Kind() != kindImportDirective {
			continue
		}
		source := directive.ChildByFieldName(fieldSource)
		if source == nil {
			continue
		}
		path := trimStringQuotes(source.Utf8Text(v.src))
		if path == "" {
			continue
		}
		qname := importQNamePrefix + path
		startByte := int(directive.StartByte())
		id := v.declID(qname, startByte)
		v.nodes = append(v.nodes, types.Node{
			ID: id, Type: types.NodeImport, Name: path, QualifiedName: qname,
			FilePath:  v.rel,
			StartLine: int(directive.StartPosition().Row) + 1,
			EndLine:   int(directive.EndPosition().Row) + 1,
			StartByte: startByte, EndByte: int(directive.EndByte()),
			Language: langSol, Confidence: types.ConfExtracted,
			// The directive as written. Pass 2 reads the aliases it
			// declares from here (importAliasesOf).
			Signature: declHeader(directive, v.src),
		})
		v.edges = append(v.edges, types.Edge{
			Src: v.fileID, Dst: id, Type: types.EdgeImports,
			Line: int(directive.StartPosition().Row) + 1, Count: 1, Confidence: types.ConfExtracted,
		})
		// The dependency on the imported file, by the name of its File
		// node: when that file changes, an incremental build re-resolves
		// this file's references, whose targets may live there.
		if target := relativeImportTarget(v.rel, path); target != "" {
			v.queue(v.fileID, types.EdgeImports, refImport, fileQNamePrefix+target, directive)
		}
	}
}
