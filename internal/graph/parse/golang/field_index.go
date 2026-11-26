package golang

import (
	"go/token"
	gotypes "go/types"
	"path/filepath"

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
// for every field, so every pass that ties a use to a Field node asks here.
type fieldIndex struct {
	fset  *token.FileSet
	byKey map[fieldKey][]*types.Node
}

// fieldKey narrows the candidates to the fields of one name in files of one
// base name; the full path and the byte span decide among them.
type fieldKey struct {
	fileBase string
	name     string
}

// newFieldIndex indexes the Field nodes. fset positions the go/types objects
// passed to node.
func newFieldIndex(fset *token.FileSet, nodes []types.Node) fieldIndex {
	ix := fieldIndex{fset: fset, byKey: map[fieldKey][]*types.Node{}}
	for i := range nodes {
		n := &nodes[i]
		if n.Type != types.NodeField {
			continue
		}
		k := fieldKey{filepath.Base(n.FilePath), n.Name}
		ix.byKey[k] = append(ix.byKey[k], n)
	}
	return ix
}

// node returns the Field node that declares f, or nil when f is not a field
// or its declaration has no node (a struct outside the graph). When one field
// declaration lies inside another (a field of an anonymous struct that is
// itself a field's type), the innermost node holding f's position wins.
func (ix fieldIndex) node(obj gotypes.Object) *types.Node {
	f, ok := obj.(*gotypes.Var)
	if !ok || !f.IsField() || ix.fset == nil {
		return nil
	}
	f = f.Origin()
	if !f.Pos().IsValid() {
		return nil
	}
	pos := ix.fset.Position(f.Pos())
	var best *types.Node
	for _, n := range ix.byKey[fieldKey{filepath.Base(pos.Filename), f.Name()}] {
		if !sameFile(n.FilePath, pos.Filename) || pos.Offset < n.StartByte || pos.Offset >= n.EndByte {
			continue
		}
		if best == nil || n.EndByte-n.StartByte < best.EndByte-best.StartByte {
			best = n
		}
	}
	return best
}
