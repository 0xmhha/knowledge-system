package solidity

import (
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Tree-sitter node kinds whose type names are not type uses of their own:
// a using directive and an `is` clause have their own edges, `override(A,
// B)` names contracts as bases, and `type T is uint256` declares T.
const (
	kindUsingDirective     = "using_directive"
	kindOverrideSpecifier  = "override_specifier"
	kindUserTypeDefinition = "user_defined_type_definition"
	kindEventDefinition    = "event_definition"
	kindErrorDeclaration   = "error_declaration"
	kindStructDeclaration  = "struct_declaration"
	kindStructBody         = "struct_body"
)

// runTypeUses queues one reference per place a user-defined type is named as
// a type: the type of a state variable, a constant, a struct member, a
// parameter, a return value, a local variable, an event or error parameter,
// the key or value of a mapping, the element of an array, and `new T`. The
// reference starts at the declaration that names the type: the variable, the
// struct member, the event, the error, or the callable for everything inside
// it. Pass 2 keeps it when the name is a struct, an enum, a contract or an
// interface of the graph and emits a uses_type edge. A cast (`IToken(x)`)
// and a struct constructor (`Item(...)`) are calls in the syntax tree; the
// call resolver emits their uses_type edge.
//
// Before this only `defines` reached a struct or an enum: nothing told which
// declarations use a type.
func (v *declVisitor) runTypeUses() {
	if v.root == nil {
		return
	}
	var visit func(n *sitter.Node)
	visit = func(n *sitter.Node) {
		switch n.Kind() {
		case kindAssemblyStatement, kindUsingDirective, kindInheritanceSpecifier,
			kindOverrideSpecifier, kindUserTypeDefinition:
			return
		case kindUserDefinedType:
			if owner, ok := v.typeUseOwner(n); ok {
				v.queueTypeUses(owner, n)
			}
			return
		}
		for i := uint(0); i < n.NamedChildCount(); i++ {
			if c := n.NamedChild(i); c != nil {
				visit(c)
			}
		}
	}
	visit(v.root)
}

// queueTypeUses queues one type use from owner per user-defined type named
// under n. It serves the places a type is named outside the walk of
// runTypeUses: the type a using directive binds.
func (v *declVisitor) queueTypeUses(owner string, n *sitter.Node) {
	if n.Kind() == kindUserDefinedType {
		v.queue(owner, types.EdgeUsesType, refTypeUse, dottedTypeName(n, v.src), n)
		return
	}
	for i := uint(0); i < n.NamedChildCount(); i++ {
		if c := n.NamedChild(i); c != nil {
			v.queueTypeUses(owner, c)
		}
	}
}

// dottedTypeName returns the name of a user_defined_type node as written,
// segments joined by dots (`Shapes.Box`).
func dottedTypeName(n *sitter.Node, src []byte) string {
	var parts []string
	for i := uint(0); i < n.NamedChildCount(); i++ {
		if c := n.NamedChild(i); c != nil && c.Kind() == kindIdentifier {
			parts = append(parts, c.Utf8Text(src))
		}
	}
	return strings.Join(parts, qnameSep)
}

// typeUseOwner returns the id of the declaration that names the type at n:
// the nearest enclosing callable, state variable, constant, struct member,
// event or error. ok is false when the type is named outside all of them.
func (v *declVisitor) typeUseOwner(n *sitter.Node) (string, bool) {
	for cur := n.Parent(); cur != nil; cur = cur.Parent() {
		switch cur.Kind() {
		case kindFunctionDef, kindModifierDef, kindConstructorDef, kindFallbackReceiveDef:
			return v.callableIDAt(cur)
		case kindStateVariableDecl, kindConstantVariableDecl:
			return v.variableIDAt(cur)
		case kindStructMember:
			return v.structMemberIDAt(cur)
		case kindEventDefinition, kindErrorDeclaration:
			return v.variableIDAt(cur)
		case kindContractDecl, kindLibraryDecl, kindInterfaceDecl:
			return "", false
		}
	}
	return "", false
}

// structMemberIDAt returns the id of the Field node of a struct member, the
// one emitStructMembers gave it.
func (v *declVisitor) structMemberIDAt(member *sitter.Node) (string, bool) {
	nameNode := member.ChildByFieldName(fieldName)
	body := member.Parent()
	if nameNode == nil || body == nil || body.Kind() != kindStructBody {
		return "", false
	}
	decl := body.Parent()
	if decl == nil || decl.Kind() != kindStructDeclaration {
		return "", false
	}
	structName := decl.ChildByFieldName(fieldName)
	if structName == nil {
		return "", false
	}
	structQName := v.memberQName(structName, structName.Utf8Text(v.src))
	return v.declID(structQName+qnameSep+nameNode.Utf8Text(v.src), int(nameNode.StartByte())), true
}
