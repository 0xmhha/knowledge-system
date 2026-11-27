package golang

import (
	"go/ast"
	"go/token"
	gotypes "go/types"

	"golang.org/x/tools/go/packages"

	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// uses_type.go emits `uses_type` edges from a declaration to the named types
// it refers to.
//
// The user of a type is the declaration whose text names it:
//   - a Function or Method, for its receiver, its signature and every type
//     its body names (a conversion, a type assertion, a variable's declared
//     type, a type argument, a composite literal);
//   - a package-level Variable or Constant, for its declared type and the
//     types its initialiser names (var walletCommand = &cli.Command{...});
//   - a type declaration, for the types its type expression and its type
//     parameters name (struct fields, interface method signatures, the
//     target of a defined type or of an alias).
//
// The referred type is a package-level type of the build, a type declared
// inside a function (found by position, it has no canonical id), or a type
// outside the build, which gets an External node through the pending refs
// this pass returns.
//
// Why a post-Resolve pass (mirroring EmitImplementsEdges):
//   - Cross-package references need the union of every package's nodes.
//   - Running here lets us return unresolved cross-package references as
//     pending refs, which become External nodes.
//
// Skip rules:
//   - Builtin types (int, string, error) and type parameters are not nodes.
//   - Anonymous types have no name to refer to; the names inside them are
//     references of their own.
//   - A declaration does not use itself. A recursive type (type Type struct
//     { Elem *Type }; 36 such references in go-stablenet) would be an edge
//     from the node to itself, and score.Compute counts every edge: the
//     loop would raise the node's in and out degree and, in PageRank, hand
//     the type a share of its own rank back each round, so a type would
//     rank higher for containing itself. No edge is made until the score
//     computation leaves self-loops out.
//
// One edge per (user, type); its line is the line of the first reference in
// the user's text.

// EmitUsesTypeEdges returns the uses_type edges of the loaded packages.
//
// Returns:
//   - edges: the resolved uses_type edges
//   - pending: PendingRefs for types not present in the node index (types
//     outside the build), which ExternalEdges binds to External nodes.
//
// Idempotent — safe to call multiple times against the same nodes/pkgs.
func EmitUsesTypeEdges(pkgs []*packages.Package, nodes []types.Node) ([]types.Edge, []parse.PendingRef) {
	if len(pkgs) == 0 {
		return nil, nil
	}

	// Every type that can be a uses_type DST: Struct / Interface / TypeAlias /
	// Enum nodes (the same set EmitImplementsEdges uses).
	typeIndex := newSymbolIndex(fsetOf(pkgs), nodes, types.NodeStruct, types.NodeInterface, types.NodeTypeAlias, types.NodeEnum)
	// The struct pass walks the fields of a type's underlying struct. A
	// defined type over a struct (`type snapHandler handler`) has that
	// underlying struct too but names only handler, so the pass looks its
	// sources up among Struct nodes alone.
	structIndex := newSymbolIndex(fsetOf(pkgs), nodes, types.NodeStruct)

	// An edge's file is the file of the node it leaves, relative to the source
	// root as every node path is. The compiled file name the walkers pass as
	// hintFile is absolute and stays on pending refs only.
	fileOf := make(map[string]string, len(nodes))
	for i := range nodes {
		fileOf[nodes[i].ID] = nodes[i].FilePath
	}

	emitted := map[edgeDedupKey]struct{}{}
	var edges []types.Edge
	var pending []parse.PendingRef

	addEdge := func(srcID string, dst *gotypes.TypeName, dstQname string, line int, hintFile string) {
		if srcID == "" || dstQname == "" {
			return
		}
		if dstID, ok := typeIndex.lookup(dst, dstQname); ok {
			k := edgeDedupKey{src: srcID, dst: dstID}
			if _, dup := emitted[k]; dup || srcID == dstID {
				return
			}
			emitted[k] = struct{}{}
			edges = append(edges, types.Edge{
				Src: srcID, Dst: dstID, Type: types.EdgeUsesType,
				Count: 1, Confidence: types.ConfExtracted,
				Line: line, FilePath: fileOf[srcID],
			})
			return
		}
		cid := goCanonicalID(dst)
		if cid == "" {
			return // a function-local type without a node: nothing can bind it later
		}
		// Unresolved cross-package type → q4=A: emit pending_ref so the
		// incremental partial-cache rebuild can replay this resolution
		// against a future build's full type universe.
		pending = append(pending, parse.PendingRef{
			SrcID:             srcID,
			EdgeType:          types.EdgeUsesType,
			TargetQName:       dstQname,
			TargetCanonicalID: cid,
			HintFile:          hintFile,
			Line:              line,
		})
	}

	// The references written in the source, with their lines. On go-stablenet
	// 10,754 of the 24,653 type references go/types sees are in function
	// bodies and 1,854 in package-level var and const declarations.
	forEachOwnedCode(pkgs, nodes, func(c ownedCode) {
		hintFile := c.pkg.Fset.Position(c.root.Pos()).Filename
		forEachTypeNameUnder(c.info(), c.root, func(tn *gotypes.TypeName, qname string, pos token.Pos) {
			addEdge(c.owner.ID, tn, qname, c.line(pos), hintFile)
		})
	})
	// The types of struct fields as go/types sees them. They are the written
	// ones, which the walk above has recorded with their lines, and what an
	// alias in a field type stands for.
	for _, pkg := range pkgs {
		if pkg == nil || pkg.Types == nil || pkg.Types.Scope() == nil {
			continue
		}
		emitUsesTypeForStructs(pkg.Types.Scope(), pkg.Types.Name(), structIndex, addEdge)
	}

	return edges, pending
}

// forEachTypeNameUnder yields every named type that an identifier under root
// names, with the identifier's position: package-level types and types
// declared inside a function. Builtin types and type parameters are skipped.
func forEachTypeNameUnder(info *gotypes.Info, root ast.Node, yield func(tn *gotypes.TypeName, qname string, pos token.Pos)) {
	if info == nil || root == nil {
		return
	}
	ast.Inspect(root, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		tn, ok := info.Uses[id].(*gotypes.TypeName)
		if !ok || tn.Pkg() == nil {
			return true
		}
		if _, isParam := tn.Type().(*gotypes.TypeParam); isParam {
			return true
		}
		yield(tn, tn.Pkg().Name()+"."+tn.Name(), id.Pos())
		return true
	})
}

// emitUsesTypeForStructs walks every Struct type at package scope and
// emits a uses_type edge for every field's named type.
func emitUsesTypeForStructs(
	scope *gotypes.Scope, pkgName string,
	srcIndex symbolIndex,
	addEdge func(srcID string, dst *gotypes.TypeName, dstQname string, line int, hintFile string),
) {
	for _, name := range scope.Names() {
		obj := scope.Lookup(name)
		tn, ok := obj.(*gotypes.TypeName)
		if !ok {
			continue
		}
		named, ok := tn.Type().(*gotypes.Named)
		if !ok {
			continue
		}
		st, ok := named.Underlying().(*gotypes.Struct)
		if !ok {
			continue
		}
		srcQname := pkgName + "." + tn.Name()
		srcID, ok := srcIndex.lookup(tn, srcQname)
		if !ok {
			continue
		}
		// A field's type has no position of its own here: line 0.
		for i := 0; i < st.NumFields(); i++ {
			forEachNamedTypeInType(st.Field(i).Type(), func(ftn *gotypes.TypeName, qname string) {
				addEdge(srcID, ftn, qname, 0, "")
			})
		}
	}
}

// forEachNamedTypeInType walks a *gotypes.Type to find every named-type
// reference and yields its package-prefixed qname.
//
// Stripping layers:
//
//	*T          → recurse into T
//	[]T, [N]T   → recurse into element
//	map[K]V     → recurse into K and V
//	chan T      → recurse into element
//	struct{...} → recurse into each field
//	(no special case for interface — interface methods aren't "uses_type"
//	 in this pass's scope; they're already covered by `implements`).
//
// Yield is called once per distinct top-level *types.Named encountered.
// Builtin / unnamed types yield nothing.
func forEachNamedTypeInType(t gotypes.Type, yield func(tn *gotypes.TypeName, qname string)) {
	if t == nil {
		return
	}
	switch tt := t.(type) {
	case *gotypes.Pointer:
		forEachNamedTypeInType(tt.Elem(), yield)
	case *gotypes.Slice:
		forEachNamedTypeInType(tt.Elem(), yield)
	case *gotypes.Array:
		forEachNamedTypeInType(tt.Elem(), yield)
	case *gotypes.Map:
		forEachNamedTypeInType(tt.Key(), yield)
		forEachNamedTypeInType(tt.Elem(), yield)
	case *gotypes.Chan:
		forEachNamedTypeInType(tt.Elem(), yield)
	case *gotypes.Named:
		obj := tt.Obj()
		if obj == nil || obj.Pkg() == nil {
			return // builtin (error / comparable) — no node
		}
		yield(obj, obj.Pkg().Name()+"."+obj.Name())
	case *gotypes.Struct:
		for i := 0; i < tt.NumFields(); i++ {
			forEachNamedTypeInType(tt.Field(i).Type(), yield)
		}
	}
}

// edgeDedupKey collapses (src, dst) pairs across a pass so that a function
// with two `pkg.T` params produces only one edge.
type edgeDedupKey struct {
	src, dst string
}
