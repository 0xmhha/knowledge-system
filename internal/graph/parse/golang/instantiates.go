package golang

import (
	"go/ast"
	gotypes "go/types"

	"golang.org/x/tools/go/packages"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// instantiates.go emits `instantiates` edges from a Function/Method, or from
// a package-level Variable for its initialiser, to a Type when that code
// materialises a value of the type via:
//
//	Type{...}        — composite literal whose type is written as a type name
//	&Type{...}       — same, addressed
//	new(Type)        — new builtin
//
// The target is the type the source names. For an alias (type G1 = bn256.G1;
// new(G1)) that is the alias, whose node stands for the name the code uses:
// resolving the expression's type instead looked through the alias, found a
// type without a node, and 22 new(T) sites of go-stablenet had no edge.
//
// `make(...)` is intentionally NOT emitted in this pass:
//   - make(map[K]V) and make([]T) target unnamed slice/map types — there is
//     no Type node to point to, and the element type is already covered by
//     uses_type.
//   - make(chan T) is already represented by Channel nodes (concurrency.go).
//
// Like uses_type, this is a post-Resolve pass — same reasons (cross-package
// types resolve only against the union of package scopes). Cross-package
// instantiations whose target type isn't in the node graph are silently
// dropped; the resolution drift they'd produce is far smaller than uses_type
// (most struct literals point to types declared in the same package) so the
// pending-ref path isn't worth the complexity for this edge.

// EmitInstantiatesEdges scans every loaded package's function bodies and
// package-level initialisers for composite literals and new() calls, emitting
// instantiates edges from the enclosing Function, Method or Variable to the
// named target type.
func EmitInstantiatesEdges(pkgs []*packages.Package, nodes []types.Node) []types.Edge {
	if len(pkgs) == 0 {
		return nil
	}
	typeIndex := newSymbolIndex(fsetOf(pkgs), nodes, types.NodeStruct, types.NodeInterface, types.NodeTypeAlias, types.NodeEnum)
	emitted := make(map[edgeDedupKey]struct{}, 128)
	var edges []types.Edge
	add := func(src *types.Node, info *gotypes.Info, typeExpr ast.Expr) {
		tn := namedTypeOf(info, typeExpr)
		if tn == nil {
			return
		}
		dstID, ok := typeIndex.lookup(tn, tn.Pkg().Name()+"."+tn.Name())
		if !ok || dstID == src.ID {
			return // a type without a node in this graph
		}
		k := edgeDedupKey{src: src.ID, dst: dstID}
		if _, dup := emitted[k]; dup {
			return
		}
		emitted[k] = struct{}{}
		edges = append(edges, types.Edge{
			Src: src.ID, Dst: dstID, Type: types.EdgeInstantiates,
			Count: 1, Confidence: types.ConfExtracted,
		})
	}
	// The code that runs: function bodies, and the initialisers of
	// package-level variables, whose Variable is the source as it is for the
	// calls in them (677 literals and new calls of go-stablenet, among them
	// var MainnetChainConfig = &ChainConfig{...}).
	forEachOwnedCode(pkgs, nodes, func(c ownedCode) {
		if !c.part.runs() {
			return
		}
		ast.Inspect(c.root, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CompositeLit:
				add(c.owner, c.info(), x.Type)
			case *ast.CallExpr:
				if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "new" && len(x.Args) == 1 {
					if _, builtin := c.info().Uses[id].(*gotypes.Builtin); builtin {
						add(c.owner, c.info(), x.Args[0])
					}
				}
			}
			return true
		})
	})
	return edges
}

// namedTypeOf returns the type name that the type expression e is: T, pkg.T,
// or an instantiated generic T[A]. nil for an unnamed type ([]T{...},
// map[K]V{...}, struct{...}{...}), a builtin, a type parameter, and for the
// elided type of an inner composite literal.
func namedTypeOf(info *gotypes.Info, e ast.Expr) *gotypes.TypeName {
	if e == nil || info == nil {
		return nil
	}
	var id *ast.Ident
	switch x := ast.Unparen(e).(type) {
	case *ast.Ident:
		id = x
	case *ast.SelectorExpr:
		id = x.Sel
	case *ast.IndexExpr:
		return namedTypeOf(info, x.X)
	case *ast.IndexListExpr:
		return namedTypeOf(info, x.X)
	default:
		return nil
	}
	tn, ok := info.Uses[id].(*gotypes.TypeName)
	if !ok || tn.Pkg() == nil {
		return nil
	}
	if _, isParam := tn.Type().(*gotypes.TypeParam); isParam {
		return nil
	}
	return tn
}
