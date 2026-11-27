package golang

import (
	"go/token"
	gotypes "go/types"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// localIndex finds the node of a name declared inside a function body: a
// LocalVariable (x := ..., for k, v := range ..., switch v := x.(type)), a
// local `var x` or a local `const x`.
//
// A local name is not unique: a function may declare err in several blocks,
// and many functions declare the same names. The position of the declaring
// identifier is unique, and go/types gives it for every use, so a use is tied
// to its declaration by position, as a field use is (posIndex).
type localIndex struct {
	pos posIndex
}

// newLocalIndex indexes the LocalVariable nodes and the Variable and Constant
// nodes that have no canonical id (a `var` or `const` declared inside a body;
// a package-level one is found by its canonical id instead).
func newLocalIndex(fset *token.FileSet, nodes []types.Node) localIndex {
	return localIndex{pos: newPosIndex(fset, nodes, func(n *types.Node) bool {
		switch n.Type {
		case types.NodeLocalVariable:
			return true
		case types.NodeVariable, types.NodeConstant:
			return n.CanonicalID == ""
		}
		return false
	})}
}

// node returns the node that declares the local variable or constant obj, or
// nil when it has none (a parameter, a result). Every case clause of a type
// switch has its own variable object; all of them stand at the identifier in
// the switch header, where the one node is.
func (ix localIndex) node(obj gotypes.Object) *types.Node {
	if obj == nil {
		return nil
	}
	return ix.pos.declaredAt(obj.Name(), obj.Pos())
}
