package golang

import (
	"go/ast"

	"golang.org/x/tools/go/packages"
)

// fileVisits records the source files a pass has walked.
//
// A package with in-package tests is loaded twice: as itself and as its test
// variant, which holds the same production files again. A pass that counts
// what it sees must walk each file once, or every use in such a package is
// counted twice. Every counting pass asks first before it walks a file.
type fileVisits map[string]bool

// first reports whether f, a syntax file of pkg, has not been walked yet, and
// records it as walked.
func (v fileVisits) first(pkg *packages.Package, f *ast.File) bool {
	name := pkg.Fset.Position(f.Pos()).Filename
	if v[name] {
		return false
	}
	v[name] = true
	return true
}
