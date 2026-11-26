package golang

import (
	"go/ast"
	"go/token"
	gotypes "go/types"
)

// fieldUse is one use of a struct field that a syntax node makes itself.
type fieldUse struct {
	field *gotypes.Var
	pos   token.Pos // the field's name in the use, or the element of a literal
	// literal says the use is an element of a composite literal, which sets
	// the field (T{F: v} or T{v}).
	literal bool
}

// fieldUsesAt returns the struct-field uses that n makes itself, not those
// of the nodes under it: x.F (also on the left of an assignment, and a
// promoted field, which is the embedded struct's field), and every element
// of a struct composite literal (T{F: v} names F, T{v} sets the field at its
// place). It is the one notion of "a use of a field" that reads_field and
// writes_field (value_access.go) and accessed_under_lock (lock_held.go)
// share. A method value (x.M) selects no field.
//
// accessed_under_lock read only selectors, so the fields a literal set while
// a mutex was held had no edge (180 pairs of go-stablenet, among them
// accounts.WalletEvent.Wallet in KeyStore.refreshWallets under KeyStore.mu).
func fieldUsesAt(n ast.Node, info *gotypes.Info) []fieldUse {
	switch x := n.(type) {
	case *ast.SelectorExpr:
		if sel := info.Selections[x]; sel != nil && sel.Kind() == gotypes.FieldVal {
			if v, ok := sel.Obj().(*gotypes.Var); ok {
				return []fieldUse{{field: v, pos: x.Sel.Pos()}}
			}
		}
	case *ast.CompositeLit:
		_, st := literalStruct(info.TypeOf(x))
		if st == nil {
			return nil
		}
		var out []fieldUse
		for i, elt := range x.Elts {
			var field *gotypes.Var
			pos := elt.Pos()
			if kv, ok := elt.(*ast.KeyValueExpr); ok {
				if key, ok := kv.Key.(*ast.Ident); ok {
					field, _ = info.Uses[key].(*gotypes.Var)
					pos = key.Pos()
				}
			} else if i < st.NumFields() {
				field = st.Field(i)
			}
			if field != nil && field.IsField() {
				out = append(out, fieldUse{field: field, pos: pos, literal: true})
			}
		}
		return out
	}
	return nil
}
