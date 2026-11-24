// Command goldprobe lists, for synthetic change requests, every declaration the
// Go type checker says the change touches (docs/experiments/impact-scope/DESIGN.md).
// It does not read the knowledge graph.
//
//	goldprobe -src <tree> -scope build-scope.json -out gold.json
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// Kind names what a change request changes.
type Kind string

const (
	// KindField changes the meaning of a struct field: all readers and writers.
	KindField Kind = "field"
	// KindFunc changes a package-level function: all functions that use it.
	KindFunc Kind = "func"
	// KindOmit adds a required field: literals that do not set it.
	KindOmit Kind = "omit"
)

const (
	testSuffix = "_test.go"
	outMode    = 0o644
)

// Decl is one top-level declaration range.
type Decl struct {
	File  string `json:"file"`
	Start int    `json:"start_line"`
	End   int    `json:"end_line"`
	Name  string `json:"name"`
}

// Target is one change request with its gold declarations.
type Target struct {
	Kind  Kind   `json:"kind"`
	Pkg   string `json:"pkg"`
	QName string `json:"qname"`
	Name  string `json:"name"`
	File  string `json:"file"`
	Line  int    `json:"line"`
	Gold  []Decl `json:"gold"`
}

// Output is the probe's result file.
type Output struct {
	Decls   []Decl   `json:"decls"`
	Targets []Target `json:"targets"`
	Errors  int      `json:"package_errors"`
	Loaded  int      `json:"packages"`
}

type probe struct {
	fset  *token.FileSet
	src   string
	scope map[string]bool
	pkgs  []*packages.Package
	decls map[string][]Decl
	users map[types.Object]map[Decl]bool
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("goldprobe failed", "err", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("goldprobe", flag.ContinueOnError)
	src := fs.String("src", "", "source tree of the base commit")
	scope := fs.String("scope", "", "build-scope.json of the graph")
	out := fs.String("out", "", "result file")
	tags := fs.String("tags", "urfave_cli_no_docs,ckzg", "build tags")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *src == "" || *scope == "" || *out == "" {
		return errors.New("-src, -scope and -out are required")
	}
	abs, err := filepath.EvalSymlinks(*src)
	if err != nil {
		return fmt.Errorf("resolve -src: %w", err)
	}
	p := &probe{src: abs, decls: map[string][]Decl{}, users: map[types.Object]map[Decl]bool{}}
	if p.scope, err = scopeFiles(*scope); err != nil {
		return err
	}
	res, err := p.load(*tags)
	if err != nil {
		return err
	}
	res.Targets = p.targets()
	for _, ds := range p.decls {
		res.Decls = append(res.Decls, ds...)
	}
	sortDecls(res.Decls)
	b, err := json.Marshal(res)
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	if err := os.WriteFile(*out, b, outMode); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	slog.Info("done", "packages", res.Loaded, "errors", res.Errors, "targets", len(res.Targets), "decls", len(res.Decls))
	return nil
}

func scopeFiles(path string) (map[string]bool, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read scope: %w", err)
	}
	var doc struct {
		Files []struct {
			Path string `json:"path"`
		} `json:"files"`
	}
	if err := json.Unmarshal(b, &doc); err != nil {
		return nil, fmt.Errorf("parse scope: %w", err)
	}
	out := map[string]bool{}
	for _, f := range doc.Files {
		if strings.HasSuffix(f.Path, ".go") && !strings.HasSuffix(f.Path, testSuffix) {
			out[f.Path] = true
		}
	}
	return out, nil
}

// load type-checks the module for linux/amd64 and indexes declarations and uses.
func (p *probe) load(tags string) (Output, error) {
	p.fset = token.NewFileSet()
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps,
		Dir:  p.src, Fset: p.fset, BuildFlags: []string{"-tags=" + tags},
		Env: append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=1"),
	}
	roots, err := packages.Load(cfg, "./...")
	if err != nil {
		return Output{}, fmt.Errorf("load: %w", err)
	}
	var res Output
	seen := map[string]bool{}
	for _, pkg := range roots {
		if len(pkg.Errors) > 0 {
			res.Errors++
		}
		if seen[pkg.PkgPath] {
			continue
		}
		seen[pkg.PkgPath] = true
		p.pkgs = append(p.pkgs, pkg)
		res.Loaded++
	}
	sort.Slice(p.pkgs, func(i, j int) bool { return p.pkgs[i].PkgPath < p.pkgs[j].PkgPath })
	for _, pkg := range p.pkgs {
		p.index(pkg)
	}
	return res, nil
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

// index records the top-level declarations of a package and which of them use
// which object.
func (p *probe) index(pkg *packages.Package) {
	for _, f := range pkg.Syntax {
		file, _, ok := p.rel(f.Pos())
		if !ok {
			continue
		}
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				p.addDecl(file, d, d.Name.Name)
				p.uses(pkg, d, p.lastDecl(file))
			case *ast.GenDecl:
				for _, s := range d.Specs {
					name := ""
					switch s := s.(type) {
					case *ast.TypeSpec:
						name = s.Name.Name
					case *ast.ValueSpec:
						name = s.Names[0].Name
					default:
						continue
					}
					p.addDecl(file, s, name)
					p.uses(pkg, s, p.lastDecl(file))
				}
			}
		}
	}
}

func (p *probe) addDecl(file string, n ast.Node, name string) {
	s, e := p.fset.Position(n.Pos()).Line, p.fset.Position(n.End()).Line
	p.decls[file] = append(p.decls[file], Decl{File: file, Start: s, End: e, Name: name})
}

func (p *probe) lastDecl(file string) Decl { return p.decls[file][len(p.decls[file])-1] }

func (p *probe) uses(pkg *packages.Package, root ast.Node, d Decl) {
	ast.Inspect(root, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		if o := pkg.TypesInfo.Uses[id]; o != nil {
			p.addUse(o, d)
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
	}
	if p.users[o] == nil {
		p.users[o] = map[Decl]bool{}
	}
	p.users[o][d] = true
}

// targets builds every request of every kind.
func (p *probe) targets() []Target {
	var out []Target
	for _, pkg := range p.pkgs {
		scope := pkg.Types.Scope()
		for _, name := range scope.Names() {
			o := scope.Lookup(name)
			file, line, ok := p.rel(o.Pos())
			if !ok {
				continue
			}
			base := Target{Pkg: pkg.PkgPath, File: file, Line: line}
			switch o := o.(type) {
			case *types.Func:
				t := base
				t.Kind, t.Name, t.QName = KindFunc, name, pkg.Name+"."+name
				t.Gold = p.goldOf(o)
				out = append(out, t)
			case *types.TypeName:
				out = append(out, p.typeTargets(base, pkg, o)...)
			}
		}
	}
	return out
}

func (p *probe) goldOf(objs ...types.Object) []Decl {
	m := map[Decl]bool{}
	for _, o := range objs {
		for d := range p.users[o] {
			m[d] = true
		}
	}
	ds := make([]Decl, 0, len(m))
	for d := range m {
		ds = append(ds, d)
	}
	sortDecls(ds)
	return ds
}

func (p *probe) typeTargets(base Target, pkg *packages.Package, o *types.TypeName) []Target {
	var out []Target
	n, ok := o.Type().(*types.Named)
	if !ok || o.IsAlias() {
		return nil
	}
	if u, ok := n.Underlying().(*types.Struct); ok {
		own, hasDecl := p.declAt(o.Pos())
		for i := range u.NumFields() {
			f := u.Field(i)
			if f.Embedded() || f.Name() == "_" {
				continue
			}
			file, line, inScope := p.rel(f.Pos())
			// A derived type (type A B) shares the fields of B; they belong to B.
			if !inScope || !hasDecl || file != own.File || line < own.Start || line > own.End {
				continue
			}
			t := base
			t.File, t.Line, t.Name, t.QName = file, line, f.Name(), pkg.Name+"."+o.Name()+"."+f.Name()
			fieldT := t
			fieldT.Kind, fieldT.Gold = KindField, p.goldOf(f)
			out = append(out, fieldT)
			omitT := t
			omitT.Kind, omitT.Gold = KindOmit, p.literalsWithout(n, f)
			out = append(out, omitT)
		}
	}
	return out
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

// literalsWithout lists declarations with a keyed or empty literal of the type
// that does not set the field.
func (p *probe) literalsWithout(n *types.Named, f *types.Var) []Decl {
	found := map[Decl]bool{}
	for _, pkg := range p.pkgs {
		for _, file := range pkg.Syntax {
			ast.Inspect(file, func(x ast.Node) bool {
				lit, ok := x.(*ast.CompositeLit)
				if !ok || pkg.TypesInfo.Types[lit].Type == nil {
					return true
				}
				ln, ok := types.Unalias(pkg.TypesInfo.Types[lit].Type).(*types.Named)
				if !ok || ln.Obj() != n.Obj() || setsField(lit, f) {
					return true
				}
				if d, ok := p.declAt(lit.Pos()); ok {
					found[d] = true
				}
				return true
			})
		}
	}
	ds := make([]Decl, 0, len(found))
	for d := range found {
		ds = append(ds, d)
	}
	sortDecls(ds)
	return ds
}

// setsField reports whether a literal sets the field by key or lists values by
// position.
func setsField(lit *ast.CompositeLit, f *types.Var) bool {
	for _, e := range lit.Elts {
		kv, ok := e.(*ast.KeyValueExpr)
		if !ok {
			return true
		}
		if id, ok := kv.Key.(*ast.Ident); ok && id.Name == f.Name() {
			return true
		}
	}
	return false
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
