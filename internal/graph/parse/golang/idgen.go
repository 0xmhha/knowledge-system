package golang

import "github.com/0xmhha/knowledge-system/internal/graph/parse"

// MakeID delegates to the shared parse.MakeID so all language parsers compute
// identical IDs for the same (qname, lang, startByte) tuple.
func MakeID(qname, lang string, startByte int) string {
	return parse.MakeID(qname, lang, startByte)
}

// nodeLanguage is the language of the nodes this package makes.
const nodeLanguage = "go"

// declID is the node id of a node declared in the visitor's file. byteOff is
// the offset of the token that names the declaration: its identifier for a
// type, var, const, local name, field or interface method, its `func`
// keyword for a function. A use finds a declaration that has no canonical id
// by recomputing this id from the identifier's position (posIndex.declaredAt),
// so the offset must stay the identifier's even where the node's span starts
// earlier (spanOf).
func (v *declVisitor) declID(qname string, byteOff int) string {
	return parse.NodeID(nodeLanguage, v.relPath, qname, byteOff)
}
