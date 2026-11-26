package golang

import (
	"go/ast"
	"go/token"
	gotypes "go/types"

	"golang.org/x/tools/go/packages"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// owners.go walks the code of the loaded packages for the passes that run
// after Resolve. Each of them needs the same three answers: which files to
// read (each once, although a package with tests is loaded twice), which node
// a piece of code belongs to, and which pieces a declaration has. Written per
// pass, the answers drifted: one pass walked function bodies and forgot
// package-level initialisers, another read signatures and not the types of
// package-level variables (1,854 type references of go-stablenet had no
// edge for that reason). They are answered here once.

// codePart says which part of a declaration a piece of code is.
type codePart int

const (
	// partBody is the body of a function or method.
	partBody codePart = iota
	// partSignature is the receiver or the type of a function declaration:
	// type parameters, parameters and results.
	partSignature
	// partInitialiser is a value of a package-level var or const.
	partInitialiser
	// partDeclaredType is the type written in a package-level var or const
	// declaration (`var x T`).
	partDeclaredType
	// partTypeDecl is the type parameters or the type of a package-level
	// type declaration.
	partTypeDecl
)

// runs reports whether the part holds code that runs: statements and calls.
func (p codePart) runs() bool { return p == partBody || p == partInitialiser }

// ownedCode is one piece of code and the node it belongs to: the Function or
// Method, the package-level Variable or Constant, or the type.
type ownedCode struct {
	owner *types.Node
	pkg   *packages.Package
	part  codePart
	// tok is the keyword of the declaration: FUNC, VAR, CONST or TYPE.
	tok  token.Token
	root ast.Node
}

func (c ownedCode) info() *gotypes.Info { return c.pkg.TypesInfo }

// line is the line of p in c's file.
func (c ownedCode) line(p token.Pos) int { return c.pkg.Fset.Position(p).Line }

// forEachOwnedCode calls fn for every piece of code of the package-level
// declarations of pkgs whose owner has a node, in source order. A file is
// read once: the test variant of a package holds its production files again,
// and walking both copies counted every use twice (see fileVisits).
//
// A function is found by canonical id and position (declIndex). A variable, a
// constant or a type is found by the position of its name: the blank
// identifier has a node and no canonical id, and its initialiser is code like
// any other (`var _ = register()`).
func forEachOwnedCode(pkgs []*packages.Package, nodes []types.Node, fn func(c ownedCode)) {
	decls := newDeclIndex(nodes)
	specs := newPosIndex(fsetOf(pkgs), nodes, ofTypes(types.NodeVariable, types.NodeConstant,
		types.NodeStruct, types.NodeInterface, types.NodeTypeAlias))
	walked := fileVisits{}
	for _, pkg := range pkgs {
		if pkg == nil || pkg.Types == nil || pkg.TypesInfo == nil || pkg.Fset == nil {
			continue
		}
		for _, f := range pkg.Syntax {
			if f == nil || !walked.first(pkg, f) {
				continue
			}
			for _, decl := range f.Decls {
				switch d := decl.(type) {
				case *ast.FuncDecl:
					funcCode(pkg, d, decls, fn)
				case *ast.GenDecl:
					genCode(pkg, d, specs, fn)
				}
			}
		}
	}
}

func funcCode(pkg *packages.Package, d *ast.FuncDecl, decls declIndex, fn func(c ownedCode)) {
	owner := decls.at(goCanonicalID(pkg.TypesInfo.Defs[d.Name]), funcDeclQname(pkg.Types.Name(), d), pkg.Fset.Position(d.Pos()))
	if owner == nil {
		return
	}
	yield := func(part codePart, root ast.Node) {
		fn(ownedCode{owner: owner, pkg: pkg, part: part, tok: token.FUNC, root: root})
	}
	if d.Recv != nil {
		yield(partSignature, d.Recv)
	}
	if d.Type != nil {
		yield(partSignature, d.Type)
	}
	if d.Body != nil {
		yield(partBody, d.Body)
	}
}

func genCode(pkg *packages.Package, d *ast.GenDecl, specs posIndex, fn func(c ownedCode)) {
	for _, spec := range d.Specs {
		switch s := spec.(type) {
		case *ast.TypeSpec:
			owner := specs.declaredAt(s.Name.Name, s.Name.Pos())
			if owner == nil {
				continue
			}
			if s.TypeParams != nil {
				fn(ownedCode{owner: owner, pkg: pkg, part: partTypeDecl, tok: d.Tok, root: s.TypeParams})
			}
			if s.Type != nil {
				fn(ownedCode{owner: owner, pkg: pkg, part: partTypeDecl, tok: d.Tok, root: s.Type})
			}
		case *ast.ValueSpec:
			owners := make([]*types.Node, len(s.Names))
			for i, name := range s.Names {
				owners[i] = specs.declaredAt(name.Name, name.Pos())
				if owners[i] != nil && s.Type != nil {
					fn(ownedCode{owner: owners[i], pkg: pkg, part: partDeclaredType, tok: d.Tok, root: s.Type})
				}
			}
			// `var a, b = f()` has one value for two names; the
			// initialiser belongs to the first name.
			for i, value := range s.Values {
				if owner := owners[initialiserOwner(i, len(owners))]; owner != nil {
					fn(ownedCode{owner: owner, pkg: pkg, part: partInitialiser, tok: d.Tok, root: value})
				}
			}
		}
	}
}

// isInterfaceMethod reports whether fn is declared by an interface type: a
// call of it is dispatched at run time, whether the receiver is an interface
// value or a struct that embeds one (b.Write() with HookedBatch{Batch}).
func isInterfaceMethod(fn *gotypes.Func) bool {
	if fn == nil {
		return false
	}
	sig, ok := fn.Type().(*gotypes.Signature)
	return ok && sig.Recv() != nil && gotypes.IsInterface(sig.Recv().Type())
}

// assignedExprs marks what the assignments, increments and range clauses
// under root write: x.F, pkg.V, V, m[k] into m, and, with throughPointer,
// *p into p.
//
// A short variable declaration writes the names on its left that already
// exist: in `a, err := f()` with err declared before, err is assigned, not
// declared (1,117 such uses of go-stablenet were recorded as reads). go/types
// tells the two apart: only a new name is a definition.
func assignedExprs(root ast.Node, info *gotypes.Info, throughPointer bool) map[ast.Expr]bool {
	out := map[ast.Expr]bool{}
	var mark func(e ast.Expr)
	mark = func(e ast.Expr) {
		switch x := e.(type) {
		case *ast.Ident:
			out[x] = true
		case *ast.SelectorExpr:
			out[x] = true
			out[x.Sel] = true // pkg.Var = v
		case *ast.IndexExpr:
			mark(x.X) // m[k] = v writes into m
		case *ast.ParenExpr:
			mark(x.X)
		case *ast.StarExpr:
			if throughPointer {
				mark(x.X)
			}
		}
	}
	markExisting := func(e ast.Expr) {
		if id, ok := e.(*ast.Ident); ok && info != nil && info.Defs[id] == nil {
			out[id] = true
		}
	}
	ast.Inspect(root, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range x.Lhs {
				if x.Tok == token.DEFINE {
					markExisting(lhs)
				} else {
					mark(lhs)
				}
			}
		case *ast.RangeStmt:
			if x.Tok == token.ASSIGN { // for k, v = range x
				for _, e := range []ast.Expr{x.Key, x.Value} {
					if e != nil {
						mark(e)
					}
				}
			}
		case *ast.IncDecStmt:
			mark(x.X)
		}
		return true
	})
	return out
}
