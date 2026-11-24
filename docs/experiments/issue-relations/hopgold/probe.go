package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/refactor/satisfy"
)

type pair struct{ iface, impl *types.Func }

type probe struct {
	fset  *token.FileSet
	src   string
	scope map[string]bool
	pkgs  []*packages.Package
	decls map[string][]Decl
	// users maps a function, method or variable to the declarations that name it.
	users map[types.Object]map[Decl]bool
	// objs maps a declaration to the objects it declares.
	objs map[Decl][]types.Object
	// conv holds interface method -> method behind it, from conversions in the build.
	conv map[*types.Func]map[*types.Func]bool
	// byType holds the same relation from method sets alone.
	byType map[*types.Func]map[*types.Func]bool
}

func newProbe(src string, scope map[string]bool) *probe {
	return &probe{
		fset: token.NewFileSet(), src: src, scope: scope,
		decls: map[string][]Decl{}, users: map[types.Object]map[Decl]bool{}, objs: map[Decl][]types.Object{},
		conv: map[*types.Func]map[*types.Func]bool{}, byType: map[*types.Func]map[*types.Func]bool{},
	}
}

// load type-checks the module for linux/amd64 and indexes the packages of the build.
func (p *probe) load(tags string) (Output, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo |
			packages.NeedTypesSizes | packages.NeedImports | packages.NeedDeps,
		Dir: p.src, Fset: p.fset, BuildFlags: []string{"-tags=" + tags},
		Env: append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=1"),
	}
	roots, err := packages.Load(cfg, "./...")
	if err != nil {
		return Output{}, fmt.Errorf("load: %w", err)
	}
	var res Output
	for _, pkg := range roots {
		if len(pkg.Errors) > 0 {
			res.Errors++
		}
		if slices.ContainsFunc(pkg.Syntax, func(f *ast.File) bool { _, _, ok := p.rel(f.Pos()); return ok }) {
			p.pkgs = append(p.pkgs, pkg)
		}
	}
	res.Loaded = len(p.pkgs)
	sort.Slice(p.pkgs, func(i, j int) bool { return p.pkgs[i].PkgPath < p.pkgs[j].PkgPath })
	finder := &satisfy.Finder{Result: map[satisfy.Constraint]bool{}}
	for _, pkg := range p.pkgs {
		p.index(pkg)
		// satisfy panics on code the checker could not type.
		if len(pkg.Errors) > 0 {
			res.Untyped = append(res.Untyped, fmt.Sprintf("%s: %v", pkg.PkgPath, pkg.Errors[0]))
			continue
		}
		finder.Find(pkg.TypesInfo, p.scopeSyntax(pkg))
	}
	for c := range finder.Result {
		link(p.conv, c.LHS, c.RHS)
	}
	p.linkByType()
	return res, nil
}

func (p *probe) scopeSyntax(pkg *packages.Package) []*ast.File {
	var out []*ast.File
	for _, f := range pkg.Syntax {
		if _, _, ok := p.rel(f.Pos()); ok {
			out = append(out, f)
		}
	}
	return out
}

func (p *probe) rel(pos token.Pos) (string, int, bool) {
	q := p.fset.Position(pos)
	r, err := filepath.Rel(p.src, q.Filename)
	if err != nil {
		return "", 0, false
	}
	r = filepath.ToSlash(r)
	return r, q.Line, p.scope[r]
}

func (p *probe) index(pkg *packages.Package) {
	for _, f := range p.scopeSyntax(pkg) {
		file, _, _ := p.rel(f.Pos())
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				p.addDecl(pkg, file, d, d.Name.Name, "func", d.Name)
			case *ast.GenDecl:
				for _, s := range d.Specs {
					switch s := s.(type) {
					case *ast.TypeSpec:
						p.addDecl(pkg, file, s, s.Name.Name, "type")
					case *ast.ValueSpec:
						p.addDecl(pkg, file, s, s.Names[0].Name, "value", s.Names...)
					}
				}
			}
		}
	}
}

func (p *probe) addDecl(pkg *packages.Package, file string, n ast.Node, name, kind string, ids ...*ast.Ident) {
	d := Decl{File: file, Start: p.fset.Position(n.Pos()).Line, End: p.fset.Position(n.End()).Line, Name: name, Kind: kind}
	p.decls[file] = append(p.decls[file], d)
	for _, id := range ids {
		if o := pkg.TypesInfo.Defs[id]; o != nil {
			p.objs[d] = append(p.objs[d], o)
		}
	}
	ast.Inspect(n, func(x ast.Node) bool {
		if id, ok := x.(*ast.Ident); ok {
			p.addUse(pkg.TypesInfo.Uses[id], d)
		}
		return true
	})
}

func (p *probe) addUse(o types.Object, d Decl) {
	switch x := o.(type) {
	case *types.Func:
		o = x.Origin()
	case *types.Var:
		o = x.Origin()
	default:
		return
	}
	if p.users[o] == nil {
		p.users[o] = map[Decl]bool{}
	}
	p.users[o][d] = true
}

func (p *probe) allDecls() []Decl {
	var out []Decl
	for _, ds := range p.decls {
		out = append(out, ds...)
	}
	sortDecls(out)
	return out
}

// link records that every method of the interface lhs may run rhs's method.
func link(m map[*types.Func]map[*types.Func]bool, lhs, rhs types.Type) {
	for _, pr := range methodPairs(lhs, rhs) {
		if m[pr.iface] == nil {
			m[pr.iface] = map[*types.Func]bool{}
		}
		m[pr.iface][pr.impl] = true
	}
}

func methodPairs(lhs, rhs types.Type) []pair {
	it, ok := lhs.Underlying().(*types.Interface)
	if !ok {
		return nil
	}
	var out []pair
	for i := range it.NumMethods() {
		im := it.Method(i)
		o, _, _ := types.LookupFieldOrMethod(rhs, true, im.Pkg(), im.Name())
		if f, ok := o.(*types.Func); ok && f.Origin() != im.Origin() {
			out = append(out, pair{im.Origin(), f.Origin()})
		}
	}
	return out
}

// linkByType pairs every package-level interface with every package-level type
// whose method set has all its methods.
func (p *probe) linkByType() {
	var ifaces, named []types.Type
	for _, pkg := range p.pkgs {
		sc := pkg.Types.Scope()
		for _, name := range sc.Names() {
			tn, ok := sc.Lookup(name).(*types.TypeName)
			if !ok || tn.IsAlias() {
				continue
			}
			n, ok := tn.Type().(*types.Named)
			if !ok || n.TypeParams().Len() > 0 {
				continue
			}
			named = append(named, n)
			if it, ok := n.Underlying().(*types.Interface); ok && it.NumMethods() > 0 {
				ifaces = append(ifaces, n)
			}
		}
	}
	for _, i := range ifaces {
		it := i.Underlying().(*types.Interface)
		for _, n := range named {
			if n != i && (types.Implements(n, it) || types.Implements(types.NewPointer(n), it)) {
				link(p.byType, i, n)
			}
		}
	}
}

func (p *probe) declAt(pos token.Pos) (Decl, bool) {
	file, line, ok := p.rel(pos)
	if !ok {
		return Decl{}, false
	}
	for _, d := range p.decls[file] {
		if d.Start <= line && line <= d.End {
			return d, true
		}
	}
	return Decl{}, false
}

func sortDecls(ds []Decl) {
	slices.SortFunc(ds, func(a, b Decl) int {
		if c := strings.Compare(a.File, b.File); c != 0 {
			return c
		}
		if a.Start != b.Start {
			return a.Start - b.Start
		}
		return a.End - b.End
	})
}
