package golang

import (
	"go/ast"
	"go/token"
	gotypes "go/types"

	"golang.org/x/tools/go/packages"

	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// uses_type.go emits `uses_type` edges from a Function /
// Method / Struct field declaration to the named Type it references.
//
// Granularity:
//   - Function arg type    : each parameter's named type → Function uses_type Type
//   - Function return type : each result's named type   → Function uses_type Type
//   - Struct field type    : every field's named type   → Struct   uses_type Type
//   - Var declaration type : (skipped in this initial pass — captured by the
//                             field/param paths above for the common cases)
//
// Why a post-Resolve pass (mirroring EmitImplementsEdges):
//   - Cross-package satisfaction needs the union of every package's Scope to
//     resolve a *gotypes.Named to its declaring package's qname.
//   - Per-file Pass 1 doesn't have the typed package handle for stdlib /
//     foreign-package types — those types resolve only against the post-load
//     packages slice.
//   - Running here lets us emit a `pending_refs` row for unresolved cross-package
//     references so the partial-cache rebuild
//     replays the same input set the cold path saw.
//
// Skip rules (avoids edge-explosion / pure noise):
//   - Builtin primitives (int, string, error, etc.) — not nodes in the graph.
//   - Anonymous types (struct{...}, interface{...}, slice/map literals) —
//     no qname → no node.
//   - Self-references (Function uses_type its own receiver Type, etc.).

// EmitUsesTypeEdges scans every loaded package's top-level declarations and
// emits `uses_type` edges from Function/Method/Struct nodes to the named
// types they reference (params, results, fields).
//
// Returns:
//   - edges: the resolved uses_type edges
//   - pending: cross-package PendingRefs for types not present in the node
//     index (q4=A — pending_refs row so the next partial build replays the
//     same input set).
//
// Idempotent — safe to call multiple times against the same nodes/pkgs.
func EmitUsesTypeEdges(pkgs []*packages.Package, nodes []types.Node) ([]types.Edge, []parse.PendingRef) {
	if len(pkgs) == 0 {
		return nil, nil
	}

	// Build qname → emitted node ID map for every type that can legitimately
	// appear as a uses_type DST. We index Struct / Interface / TypeAlias /
	// Enum nodes (the same set EmitImplementsEdges uses) so post-pass
	// resolution matches.
	typeIndex := newSymbolIndex(fsetOf(pkgs), nodes, types.NodeStruct, types.NodeInterface, types.NodeTypeAlias, types.NodeEnum)

	// Index the symbols that can be a uses_type SRC: Function, Method
	// (callers using it as param/return), Struct (caller using it as field
	// type's container).
	srcIndex := newSymbolIndex(fsetOf(pkgs), nodes, types.NodeFunction, types.NodeMethod, types.NodeStruct,
		types.NodeInterface, types.NodeTypeAlias)
	// The struct pass walks the fields of a type's underlying struct. A
	// defined type over a struct (`type snapHandler handler`) has that
	// underlying struct too but names only handler, so the pass looks its
	// sources up among Struct nodes alone.
	structIndex := newSymbolIndex(fsetOf(pkgs), nodes, types.NodeStruct)
	// Functions are found the way the other post-Resolve passes find them:
	// several init functions of a package share one canonical id.
	decls := newDeclIndex(nodes)

	// An edge's file is the file of the node it leaves, relative to the source
	// root as every node path is. The compiled file name the walkers pass as
	// hintFile is absolute and stays on pending refs only.
	fileOf := make(map[string]string, len(nodes))
	for i := range nodes {
		fileOf[nodes[i].ID] = nodes[i].FilePath
	}

	emitted := make(map[edgeDedupKey]struct{}, len(srcIndex.byQname)*2)
	var edges []types.Edge
	var pending []parse.PendingRef

	addEdge := func(srcID string, dst *gotypes.TypeName, dstQname string, line int, hintFile string) {
		if srcID == "" || dstQname == "" {
			return
		}
		if dstID, ok := typeIndex.lookup(dst, dstQname); ok {
			k := edgeDedupKey{src: srcID, dst: dstID}
			if _, dup := emitted[k]; dup {
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
		// Unresolved cross-package type → q4=A: emit pending_ref so the
		// incremental partial-cache rebuild can replay this resolution
		// against a future build's full type universe.
		pending = append(pending, parse.PendingRef{
			SrcID:             srcID,
			EdgeType:          types.EdgeUsesType,
			TargetQName:       dstQname,
			TargetCanonicalID: goCanonicalID(dst),
			HintFile:          hintFile,
			Line:              line,
		})
	}

	for _, pkg := range pkgs {
		if pkg == nil || pkg.Types == nil {
			continue
		}
		pkgName := pkg.Types.Name()
		scope := pkg.Types.Scope()
		if scope == nil {
			continue
		}
		// Functions and methods (Function/Method nodes): walk param + result
		// signatures.
		emitUsesTypeForFunctions(pkg, pkgName, decls, addEdge)
		// Struct field types.
		emitUsesTypeForStructs(scope, pkgName, structIndex, addEdge)
		// The types a type declaration names: an interface's method
		// signatures, the target of a defined type or alias.
		emitUsesTypeForTypeSpecs(pkg, pkgName, srcIndex, addEdge)
	}

	return edges, pending
}

// emitUsesTypeForFunctions walks every FuncDecl in pkg.Syntax and emits
// edges for parameters / results whose type is a *gotypes.Named.
//
// Why pkg.Syntax (not scope.Names()): scope iterates package-scope objects
// only. Methods are scope-less from the package's point of view (they hang
// off types). FuncDecls in the syntax trees give us the AST positions for
// edge.line and the same qname construction declarations.go uses.
func emitUsesTypeForFunctions(
	pkg *packages.Package, pkgName string,
	decls declIndex,
	addEdge func(srcID string, dst *gotypes.TypeName, dstQname string, line int, hintFile string),
) {
	if pkg.TypesInfo == nil || pkg.Fset == nil {
		return
	}
	for fileIdx, f := range pkg.Syntax {
		if f == nil {
			continue
		}
		var hintFile string
		if fileIdx < len(pkg.CompiledGoFiles) {
			hintFile = pkg.CompiledGoFiles[fileIdx]
		}
		for _, decl := range f.Decls {
			fd, ok := decl.(*ast.FuncDecl)
			if !ok || fd.Type == nil {
				continue
			}
			// Construct the function's qname (mirrors visitFuncDecl in
			// declarations.go). For methods, the receiver type slot uses
			// exprName which strips pointer/selector.
			var srcQname string
			if fd.Recv != nil && len(fd.Recv.List) > 0 {
				srcQname = pkgName + "." + recvTypeName(fd.Recv.List[0].Type) + "." + fd.Name.Name
			} else {
				srcQname = pkgName + "." + fd.Name.Name
			}
			src := decls.at(goCanonicalID(pkg.TypesInfo.Defs[fd.Name]), srcQname, pkg.Fset.Position(fd.Pos()))
			if src == nil {
				continue
			}
			srcID := src.ID
			line := pkg.Fset.Position(fd.Pos()).Line
			// Parameters
			if fd.Type.Params != nil {
				for _, field := range fd.Type.Params.List {
					forEachNamedTypeInExpr(pkg.TypesInfo, field.Type, func(tn *gotypes.TypeName, qname string) {
						if qname != srcQname {
							addEdge(srcID, tn, qname, line, hintFile)
						}
					})
				}
			}
			// Results
			if fd.Type.Results != nil {
				for _, field := range fd.Type.Results.List {
					forEachNamedTypeInExpr(pkg.TypesInfo, field.Type, func(tn *gotypes.TypeName, qname string) {
						if qname != srcQname {
							addEdge(srcID, tn, qname, line, hintFile)
						}
					})
				}
			}
			// The receiver type and every type the body names (a
			// conversion, a type assertion, a variable's declared type, a
			// type argument): on go-stablenet 10,754 of the 24,653 type
			// references go/types sees were outside the signatures and
			// struct fields walked above.
			named := func(tn *gotypes.TypeName, qname string, pos token.Pos) {
				addEdge(srcID, tn, qname, pkg.Fset.Position(pos).Line, hintFile)
			}
			if fd.Recv != nil {
				forEachTypeNameUnder(pkg.TypesInfo, fd.Recv, named)
			}
			forEachTypeNameUnder(pkg.TypesInfo, fd.Type, named)
			if fd.Body != nil {
				forEachTypeNameUnder(pkg.TypesInfo, fd.Body, named)
			}
		}
	}
}

// emitUsesTypeForTypeSpecs emits a uses_type edge from each package-level
// type declaration to the types its type expression names: the parameter and
// result types of an interface's methods and the interfaces it embeds, the
// target of a defined type (`type List []Item`) or of an alias, the
// parameters of a function type. Struct fields are also covered by
// emitUsesTypeForStructs; the edge is deduplicated.
func emitUsesTypeForTypeSpecs(
	pkg *packages.Package, pkgName string,
	srcIndex symbolIndex,
	addEdge func(srcID string, dst *gotypes.TypeName, dstQname string, line int, hintFile string),
) {
	if pkg.TypesInfo == nil || pkg.Fset == nil {
		return
	}
	for fileIdx, f := range pkg.Syntax {
		if f == nil {
			continue
		}
		var hintFile string
		if fileIdx < len(pkg.CompiledGoFiles) {
			hintFile = pkg.CompiledGoFiles[fileIdx]
		}
		for _, decl := range f.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, spec := range gd.Specs {
				ts, ok := spec.(*ast.TypeSpec)
				if !ok || ts.Type == nil {
					continue
				}
				self := pkg.TypesInfo.Defs[ts.Name]
				srcID, ok := srcIndex.lookup(self, pkgName+"."+ts.Name.Name)
				if !ok {
					continue
				}
				forEachTypeNameUnder(pkg.TypesInfo, ts.Type, func(tn *gotypes.TypeName, qname string, pos token.Pos) {
					if gotypes.Object(tn) != self {
						addEdge(srcID, tn, qname, pkg.Fset.Position(pos).Line, hintFile)
					}
				})
			}
		}
	}
}

// forEachTypeNameUnder yields every package-level named type that an
// identifier under root names, with the identifier's position. Type
// parameters and types declared inside a function are not package-level and
// are skipped.
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
		if !ok || tn.Pkg() == nil || tn.Parent() != tn.Pkg().Scope() {
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
		// scope.Lookup returns an obj whose Pos is the type-decl token.
		line := 0 // line is populated only for func-anchored edges
		hintFile := ""
		for i := 0; i < st.NumFields(); i++ {
			field := st.Field(i)
			forEachNamedTypeInType(field.Type(), func(ftn *gotypes.TypeName, qname string) {
				if qname != srcQname {
					addEdge(srcID, ftn, qname, line, hintFile)
				}
			})
		}
	}
}

// forEachNamedTypeInExpr walks an *ast.Expr (a parameter / result type
// expression) using types.Info to resolve identifiers / selectors to
// *gotypes.Named, then yields each distinct (pkgName + "." + typeName)
// qname encountered. Pointer / slice / map / chan layers are stripped to
// reach the underlying named element.
func forEachNamedTypeInExpr(info *gotypes.Info, e ast.Expr, yield func(tn *gotypes.TypeName, qname string)) {
	if info == nil || e == nil {
		return
	}
	t := info.TypeOf(e)
	if t == nil {
		return
	}
	forEachNamedTypeInType(t, yield)
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

// edgeDedupKey collapses (src, dst) pairs across the uses_type pass so a
// function with two `pkg.T` params produces only one edge.
type edgeDedupKey struct {
	src, dst string
}
