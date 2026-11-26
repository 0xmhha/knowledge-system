package golang

import (
	"go/ast"
	"go/token"
	gotypes "go/types"

	"golang.org/x/tools/go/packages"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// value_access.go records who uses a value, so an agent can follow a field,
// a package-level variable or a constant to the code that reads or sets it.
// Calls alone do not reach that code: the fix for a stale value is usually
// where the value is written, and the place that misbehaves is where it is
// read.
//
// Edges, from the Function/Method (or the package-level var/const whose
// initializer uses the value) to the value node:
//
//	writes_field  a struct field is assigned: x.F = v, x.F op= v, x.F++,
//	              or set in a composite literal T{F: v} / T{v}
//	reads_field   any other use of a struct field
//	references    a package-level variable or constant is used;
//	              dispatch_kind "write" when it is assigned (X = v, X[k] = v,
//	              X++), "read" otherwise. A function that does both has
//	              one edge of each kind.
//
// One edge per (source, value, kind, line), as for calls: every line on
// which the source uses the value has its own edge, so a reader sees all the
// places a function touches a value. Count is the number of uses on that
// line, FilePath the source's file. Closures count for the function that
// contains them.
//
// Values resolve through go/types to the node's canonical id, not to the
// package-leaf qualified name, so same-named packages (two "core" packages)
// do not mix. A promoted field (o.F where F belongs to an embedded struct)
// resolves to the embedded struct's field. Local variables, function
// parameters, fields of anonymous structs and values outside the graph have
// no node and are skipped.
func EmitValueAccessEdges(pkgs []*packages.Package, nodes []types.Node) []types.Edge {
	if len(pkgs) == 0 {
		return nil
	}
	byCanonical := make(map[string]*types.Node, len(nodes))
	for i := range nodes {
		n := &nodes[i]
		switch n.Type {
		case types.NodeField, types.NodeVariable, types.NodeConstant, types.NodeFunction, types.NodeMethod:
		default:
			continue
		}
		if n.CanonicalID != "" {
			if _, ok := byCanonical[n.CanonicalID]; !ok {
				byCanonical[n.CanonicalID] = n
			}
		}
	}

	decls := newDeclIndex(nodes)
	fields := newFieldIndex(fsetOf(pkgs), nodes)
	locals := newLocalIndex(fsetOf(pkgs), nodes)
	acc := &accessCollector{seen: map[accessKey]int{}}
	// Each file is walked once (see fileVisits): walking the test variant's
	// copy as well counted every use twice (count 2 on 9,654 writes_field and
	// 22,631 reads_field edges of go-stablenet).
	walked := fileVisits{}
	for _, pkg := range pkgs {
		if pkg == nil || pkg.Types == nil || pkg.TypesInfo == nil {
			continue
		}
		info := pkg.TypesInfo
		for _, f := range pkg.Syntax {
			if f == nil || !walked.first(pkg, f) {
				continue
			}
			for _, decl := range f.Decls {
				switch d := decl.(type) {
				case *ast.FuncDecl:
					if d.Body == nil {
						continue
					}
					src := decls.at(goCanonicalID(info.Defs[d.Name]), funcDeclQname(pkg.Types.Name(), d), pkg.Fset.Position(d.Pos()))
					if src != nil {
						acc.walk(src, d.Body, info, pkg.Fset, byCanonical, fields, locals)
					}
				case *ast.GenDecl:
					if d.Tok != token.VAR && d.Tok != token.CONST {
						continue
					}
					for _, spec := range d.Specs {
						vs, ok := spec.(*ast.ValueSpec)
						if !ok || len(vs.Values) == 0 || len(vs.Names) == 0 {
							continue
						}
						// `var a, b = f()` has one value for two names; the
						// initializer is attributed to the first name.
						for i, v := range vs.Values {
							name := vs.Names[initialiserOwner(i, len(vs.Names))]
							if src := byCanonical[goCanonicalID(info.Defs[name])]; src != nil {
								acc.walk(src, v, info, pkg.Fset, byCanonical, fields, locals)
							}
						}
					}
				}
			}
		}
	}
	return acc.edges
}

func funcDeclQname(pkgName string, fd *ast.FuncDecl) string {
	if fd.Recv != nil && len(fd.Recv.List) > 0 {
		return pkgName + "." + recvTypeName(fd.Recv.List[0].Type) + "." + fd.Name.Name
	}
	return pkgName + "." + fd.Name.Name
}

type accessKey struct {
	src, dst string
	typ      types.EdgeType
	kind     string
	line     int
}

type accessCollector struct {
	seen  map[accessKey]int // index into edges
	edges []types.Edge
}

func (a *accessCollector) emit(src, dst *types.Node, typ types.EdgeType, kind string, line int) {
	if dst == nil || src.ID == dst.ID {
		return
	}
	k := accessKey{src.ID, dst.ID, typ, kind, line}
	if i, ok := a.seen[k]; ok {
		a.edges[i].Count++
		return
	}
	a.seen[k] = len(a.edges)
	a.edges = append(a.edges, types.Edge{
		Src: src.ID, Dst: dst.ID, Type: typ, DispatchKind: kind,
		FilePath: src.FilePath, Line: line, Count: 1, Confidence: types.ConfExtracted,
	})
}

// walk records every value use under root as an access by src.
func (a *accessCollector) walk(src *types.Node, root ast.Node, info *gotypes.Info, fset *token.FileSet, byCanonical map[string]*types.Node, fields fieldIndex, locals localIndex) {
	writes := map[ast.Expr]bool{}
	var markWrite func(e ast.Expr)
	markWrite = func(e ast.Expr) {
		switch x := e.(type) {
		case *ast.Ident:
			writes[x] = true
		case *ast.SelectorExpr:
			writes[x] = true
			writes[x.Sel] = true // pkg.Var = v
		case *ast.IndexExpr:
			markWrite(x.X) // m[k] = v writes into m
		case *ast.ParenExpr:
			markWrite(x.X)
		}
	}
	ast.Inspect(root, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			if x.Tok != token.DEFINE {
				for _, lhs := range x.Lhs {
					markWrite(lhs)
				}
			}
		case *ast.IncDecStmt:
			markWrite(x.X)
		}
		return true
	})

	line := func(p token.Pos) int { return fset.Position(p).Line }
	ast.Inspect(root, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			if sel := info.Selections[x]; sel != nil && sel.Kind() == gotypes.FieldVal {
				typ := types.EdgeReadsField
				if writes[x] {
					typ = types.EdgeWritesField
				}
				a.emit(src, fields.node(sel.Obj()), typ, "", line(x.Sel.Pos()))
			}
		case *ast.CompositeLit:
			_, st := literalStruct(info.TypeOf(x))
			if st == nil {
				return true
			}
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
					a.emit(src, fields.node(field), types.EdgeWritesField, "", line(pos))
				}
			}
		case *ast.Ident:
			obj := info.Uses[x]
			if obj == nil || obj.Pkg() == nil {
				return true
			}
			if obj.Parent() != obj.Pkg().Scope() {
				// A name declared inside a body: tied to its node by the
				// position of the declaration (a parameter has no node).
				if o, ok := obj.(*gotypes.Var); ok && !o.IsField() {
					kind := "read"
					if writes[x] {
						kind = "write"
					}
					a.emit(src, locals.node(o), types.EdgeReferences, kind, line(x.Pos()))
				}
				return true
			}
			switch o := obj.(type) {
			case *gotypes.Var:
				if o.IsField() {
					return true
				}
				kind := "read"
				if writes[x] {
					kind = "write"
				}
				a.emit(src, byCanonical[goCanonicalID(o)], types.EdgeReferences, kind, line(x.Pos()))
			case *gotypes.Const:
				a.emit(src, byCanonical[goCanonicalID(o)], types.EdgeReferences, "read", line(x.Pos()))
			}
		}
		return true
	})
}

// literalStruct returns the named type (nil if anonymous) and the struct
// type that t, or what t points to, denotes.
func literalStruct(t gotypes.Type) (*gotypes.Named, *gotypes.Struct) {
	if t == nil {
		return nil, nil
	}
	t = gotypes.Unalias(t)
	if p, ok := t.(*gotypes.Pointer); ok {
		t = gotypes.Unalias(p.Elem())
	}
	named, _ := t.(*gotypes.Named)
	st, _ := t.Underlying().(*gotypes.Struct)
	return named, st
}
