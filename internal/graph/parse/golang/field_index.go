package golang

import (
	"go/token"
	gotypes "go/types"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// fieldIndex finds the Field node of a struct field by where the field is
// declared.
//
// A field has no name of its own that is unique: "<importpath>.<Type>.<Field>"
// names the wrong field when a function declares a struct with the name of a
// package-level one (gencodec's `type Tuple struct` inside (Tuple).MarshalJSON),
// names no field when the use goes through a defined type over a struct
// (`type Criteria Query`; c.From is Query.From), and does not exist for an
// anonymous struct. The declaration position is unique, and go/types gives it
// for every field, so every pass that ties a use to a Field node asks here
// (the lookup itself is posIndex's).
type fieldIndex struct {
	pos posIndex
}

// newFieldIndex indexes the Field nodes. fset positions the go/types objects
// passed to node.
func newFieldIndex(fset *token.FileSet, nodes []types.Node) fieldIndex {
	return fieldIndex{pos: newPosIndex(fset, nodes, ofTypes(types.NodeField))}
}

// node returns the Field node that declares f, or nil when f is not a field
// or its declaration has no node (a struct outside the graph). When one field
// declaration lies inside another (a field of an anonymous struct that is
// itself a field's type), the innermost node holding f's position wins.
func (ix fieldIndex) node(obj gotypes.Object) *types.Node {
	f, ok := obj.(*gotypes.Var)
	if !ok || !f.IsField() {
		return nil
	}
	f = f.Origin()
	return ix.pos.at(f.Name(), f.Pos())
}
