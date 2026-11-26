package golang

import (
	"go/token"
	gotypes "go/types"
	"path/filepath"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// localIndex finds the node of a name declared inside a function body: a
// LocalVariable (x := ..., for k, v := range ...) or a local `var x`.
//
// A local name is not unique: a function may declare err in several blocks,
// and many functions declare the same names. The position of the declaring
// identifier is unique, and go/types gives it for every use, so a use is tied
// to its declaration by position, as a field use is (fieldIndex).
type localIndex struct {
	fset  *token.FileSet
	byKey map[localKey][]*types.Node
}

// localKey narrows the candidates to the names starting at one byte offset in
// files of one base name; the full path decides among them.
type localKey struct {
	fileBase string
	name     string
	offset   int
}

// newLocalIndex indexes the LocalVariable nodes and the Variable nodes that
// have no canonical id (a `var` declared inside a body; a package-level
// variable is found by its canonical id instead).
func newLocalIndex(fset *token.FileSet, nodes []types.Node) localIndex {
	ix := localIndex{fset: fset, byKey: map[localKey][]*types.Node{}}
	for i := range nodes {
		n := &nodes[i]
		if n.Type != types.NodeLocalVariable && (n.Type != types.NodeVariable || n.CanonicalID != "") {
			continue
		}
		k := localKey{filepath.Base(n.FilePath), n.Name, n.StartByte}
		ix.byKey[k] = append(ix.byKey[k], n)
	}
	return ix
}

// node returns the node that declares the local variable v, or nil when v has
// none (a parameter, a result, the variable of a type switch).
func (ix localIndex) node(v *gotypes.Var) *types.Node {
	if v == nil || ix.fset == nil || !v.Pos().IsValid() {
		return nil
	}
	pos := ix.fset.Position(v.Pos())
	for _, n := range ix.byKey[localKey{filepath.Base(pos.Filename), v.Name(), pos.Offset}] {
		if sameFile(n.FilePath, pos.Filename) {
			return n
		}
	}
	return nil
}
