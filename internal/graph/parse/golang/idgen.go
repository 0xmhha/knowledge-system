package golang

import "github.com/0xmhha/knowledge-system/internal/graph/parse"

// MakeID delegates to the shared parse.MakeID so all language parsers compute
// identical IDs for the same (qname, lang, startByte) tuple.
func MakeID(qname, lang string, startByte int) string {
	return parse.MakeID(qname, lang, startByte)
}

// declID is the node id of a node declared in the visitor's file.
func (v *declVisitor) declID(qname string, startByte int) string {
	return parse.NodeID("go", v.relPath, qname, startByte)
}
