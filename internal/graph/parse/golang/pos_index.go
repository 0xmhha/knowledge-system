package golang

import (
	"go/token"
	"path/filepath"

	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// posIndex finds the node of a declaration by where the declaration is
// written.
//
// A name is not an address: a function may declare err in several blocks,
// "<importpath>.<Type>.<Field>" names the wrong field when a function declares
// a struct called like a package-level one, and a type, an interface method or
// a mutex declared inside a function has no canonical id at all. The position
// of the declaring identifier is unique, and go/types gives it for every use,
// in whichever package the use is type-checked (a package with tests is
// checked twice, with two sets of objects but one set of positions). Every
// pass that ties a use to a node without a canonical id asks here:
// fieldIndex, localIndex, mutexIndex, and the lookups of local types and of
// the methods of local and unnamed interfaces.
type posIndex struct {
	fset  *token.FileSet
	byKey map[posKey][]*types.Node
}

// posKey narrows the candidates to the nodes of one name in files of one
// base name; the full path and the byte span decide among them.
type posKey struct {
	fileBase string
	name     string
}

// newPosIndex indexes the nodes keep accepts. fset positions the token.Pos
// values passed to at and declaredAt; nil disables the index.
func newPosIndex(fset *token.FileSet, nodes []types.Node, keep func(n *types.Node) bool) posIndex {
	ix := posIndex{fset: fset, byKey: map[posKey][]*types.Node{}}
	for i := range nodes {
		n := &nodes[i]
		if !keep(n) {
			continue
		}
		k := posKey{filepath.Base(n.FilePath), n.Name}
		ix.byKey[k] = append(ix.byKey[k], n)
	}
	return ix
}

// ofTypes is a keep function for newPosIndex: the nodes of the given types.
func ofTypes(kinds ...types.NodeType) func(n *types.Node) bool {
	want := make(map[types.NodeType]bool, len(kinds))
	for _, k := range kinds {
		want[k] = true
	}
	return func(n *types.Node) bool { return want[n.Type] }
}

// at returns the indexed node called name whose span holds pos, or nil. When
// one declaration lies inside another of the same name (a field of an
// anonymous struct that is itself a field's type), the innermost one wins.
func (ix posIndex) at(name string, pos token.Pos) *types.Node {
	if ix.fset == nil || !pos.IsValid() {
		return nil
	}
	p := ix.fset.Position(pos)
	var best *types.Node
	for _, n := range ix.byKey[posKey{filepath.Base(p.Filename), name}] {
		if !sameFile(n.FilePath, p.Filename) || p.Offset < n.StartByte || p.Offset >= n.EndByte {
			continue
		}
		if best == nil || n.EndByte-n.StartByte < best.EndByte-best.StartByte {
			best = n
		}
	}
	return best
}

// declaredAt returns the indexed node that the identifier called name at pos
// declares, or nil.
//
// A node's span says where the declaration's text is, not where its name
// stands: `var x = f(func(x int) int { return x })` spans the parameter x as
// well, so the innermost span around a position can belong to another
// declaration of that name. The node id says it exactly: it is the hash of
// the file, the qualified name and the offset of the declaring identifier
// (declID), so the node whose id is the id of a declaration at pos is the one.
func (ix posIndex) declaredAt(name string, pos token.Pos) *types.Node {
	if ix.fset == nil || !pos.IsValid() {
		return nil
	}
	p := ix.fset.Position(pos)
	for _, n := range ix.byKey[posKey{filepath.Base(p.Filename), name}] {
		// The span check comes first because it is cheap: the identifier
		// lies inside the declaration it belongs to.
		if p.Offset < n.StartByte || p.Offset >= n.EndByte || !sameFile(n.FilePath, p.Filename) {
			continue
		}
		if n.ID == parse.NodeID(nodeLanguage, n.FilePath, n.QualifiedName, p.Offset) {
			return n
		}
	}
	return nil
}
