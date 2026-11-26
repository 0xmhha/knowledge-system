package solidity

import (
	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/0xmhha/knowledge-system/internal/graph/parse"
)

// langSol is the language label of every Solidity node.
const langSol = "sol"

// qnameSep joins a container's name and a member's name in a qualified name.
const qnameSep = "."

// fileQNamePrefix starts the qualified name of a File node.
const fileQNamePrefix = "file:"

// importQNamePrefix starts the qualified name of an Import node.
const importQNamePrefix = "import:"

// Tree-sitter node kinds of the declarations that can hold members.
const (
	kindContractDecl  = "contract_declaration"
	kindLibraryDecl   = "library_declaration"
	kindInterfaceDecl = "interface_declaration"
)

// Tree-sitter node kinds of the declarations whose body can make calls.
const (
	kindFunctionDef        = "function_definition"
	kindModifierDef        = "modifier_definition"
	kindConstructorDef     = "constructor_definition"
	kindFallbackReceiveDef = "fallback_receive_definition"
)

// Names of the callables that have no name of their own in the source.
const (
	nameConstructor = "constructor"
	nameFallback    = "fallback"
	nameReceive     = "receive"
)

// fieldName is the tree-sitter field that holds a declaration's identifier.
const fieldName = "name"

// declID is the node id of a node declared in the visitor's file.
//
// The file is part of the key. Without it `contract GovMinter` in v1 and in
// v2 of the system contracts, declared at the same byte offset, had one id:
// one node survived and the v1 file's defines edge pointed at the v2 node.
// Every Solidity node, the File node included, takes its id from here.
func (v *declVisitor) declID(qname string, startByte int) string {
	return parse.NodeID(langSol, v.rel, qname, startByte)
}

// nearestContractNode returns the contract, library or interface declaration
// that encloses n, or nil when n is at file level.
func nearestContractNode(n *sitter.Node) *sitter.Node {
	for cur := n; cur != nil; cur = cur.Parent() {
		switch cur.Kind() {
		case kindContractDecl, kindLibraryDecl, kindInterfaceDecl:
			return cur
		}
	}
	return nil
}

// containerOf returns the id and the name of the contract, library or
// interface that encloses n. ok is false at file level. The id is keyed by
// the start of the name identifier, the same key emitContractLikeNode uses.
func (v *declVisitor) containerOf(n *sitter.Node) (id, name string, ok bool) {
	decl := nearestContractNode(n)
	if decl == nil {
		return "", "", false
	}
	ident := decl.ChildByFieldName(fieldName)
	if ident == nil {
		return "", "", false
	}
	name = ident.Utf8Text(v.src)
	return v.declID(name, int(ident.StartByte())), name, true
}

// definerOf returns the id of the node that defines a declaration at n: the
// enclosing contract, library or interface, or the file at file level.
func (v *declVisitor) definerOf(n *sitter.Node) string {
	if id, _, ok := v.containerOf(n); ok {
		return id
	}
	return v.fileID
}

// memberQName names a declaration under its enclosing contract, library or
// interface (`Contract.member`); a file-level declaration keeps its bare
// name. Functions, modifiers, events, errors, structs, enums, state
// variables, constants and mappings all take their qualified name from here.
func (v *declVisitor) memberQName(n *sitter.Node, name string) string {
	return qualify(nearestContractName(n, v.src), name)
}

// qualify joins a container name and a member name; an empty container
// leaves the bare name.
func qualify(container, name string) string {
	if container == "" {
		return name
	}
	return container + qnameSep + name
}

// callableIDAt returns the id of the function, modifier, constructor,
// fallback or receive that encloses n. ok is false outside any of them.
func (v *declVisitor) callableIDAt(n *sitter.Node) (string, bool) {
	qname, start, ok := nearestFunctionQnameAndStart(n, v.src)
	if !ok {
		return "", false
	}
	return v.declID(qname, start), true
}

// variableIDAt returns the id of the node a declaration with a name field
// declares under its container: a state variable, a mapping, a constant, an
// event or an error. ok is false when the declaration has no name, or is a
// misparsed `using {f as +} for T;`, which looks like a state variable and
// has no node (see runStateVarDecl).
func (v *declVisitor) variableIDAt(decl *sitter.Node) (string, bool) {
	nameNode := decl.ChildByFieldName(fieldName)
	if nameNode == nil {
		return "", false
	}
	if decl.Kind() == kindStateVariableDecl {
		if _, _, misparsed := matchOperatorFormStateVar(decl, v.src); misparsed {
			return "", false
		}
	}
	return v.declID(v.memberQName(nameNode, nameNode.Utf8Text(v.src)), int(nameNode.StartByte())), true
}
