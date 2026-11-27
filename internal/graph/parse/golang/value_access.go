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
//	references    a variable or constant is used; dispatch_kind "write"
//	              when it is assigned (X = v, X[k] = v, X++, or an existing
//	              X on the left of :=), "read" otherwise. A function that
//	              does both has one edge of each kind. A constant used as an
//	              array length in a signature or in a type declaration is
//	              read by that function or type.
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

	fields := newFieldIndex(fsetOf(pkgs), nodes)
	locals := newLocalIndex(fsetOf(pkgs), nodes)
	acc := &accessCollector{seen: map[accessKey]int{}}
	// Every part of a declaration is read, not only code that runs: a
	// constant is also used as an array length in a signature
	// (func Sum512(data []byte) [Size]byte) or in a type declaration
	// (type Hash [HashLength]byte), 24 uses of go-stablenet.
	forEachOwnedCode(pkgs, nodes, func(c ownedCode) {
		acc.walk(c.owner, c.root, c.info(), c.pkg.Fset, byCanonical, fields, locals)
	})
	return acc.edges
}

func funcDeclQname(pkgName string, fd *ast.FuncDecl) string {
	if fd.Recv != nil && len(fd.Recv.List) > 0 {
		return pkgName + "." + recvTypeName(fd.Recv.List[0].Type) + "." + fd.Name.Name
	}
	return pkgName + "." + fd.Name.Name
}

// The dispatch kinds of a references edge.
const (
	accessRead  = "read"
	accessWrite = "write"
)

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
	writes := assignedExprs(root, info, false)

	line := func(p token.Pos) int { return fset.Position(p).Line }
	ast.Inspect(root, func(n ast.Node) bool {
		for _, u := range fieldUsesAt(n, info) {
			typ := types.EdgeReadsField
			if u.literal || writes[n.(ast.Expr)] {
				typ = types.EdgeWritesField
			}
			a.emit(src, fields.node(u.field), typ, "", line(u.pos))
		}
		switch x := n.(type) {
		case *ast.Ident:
			obj := info.Uses[x]
			if obj == nil || obj.Pkg() == nil {
				return true
			}
			if obj.Parent() != obj.Pkg().Scope() {
				// A name declared inside a body: tied to its node by the
				// position of the declaration (a parameter has no node).
				switch o := obj.(type) {
				case *gotypes.Var:
					if !o.IsField() {
						kind := accessRead
						if writes[x] {
							kind = accessWrite
						}
						a.emit(src, locals.node(o), types.EdgeReferences, kind, line(x.Pos()))
					}
				case *gotypes.Const:
					// A constant declared in a body is read like a
					// package-level one (167 reads of go-stablenet).
					a.emit(src, locals.node(o), types.EdgeReferences, accessRead, line(x.Pos()))
				}
				return true
			}
			switch o := obj.(type) {
			case *gotypes.Var:
				if o.IsField() {
					return true
				}
				kind := accessRead
				if writes[x] {
					kind = accessWrite
				}
				a.emit(src, byCanonical[goCanonicalID(o)], types.EdgeReferences, kind, line(x.Pos()))
			case *gotypes.Const:
				a.emit(src, byCanonical[goCanonicalID(o)], types.EdgeReferences, accessRead, line(x.Pos()))
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
