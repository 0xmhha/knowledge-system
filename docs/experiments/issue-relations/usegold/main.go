// Command usegold lists, for every declared name of a build, the lines of
// production code that use it according to the Go type checker
// (docs/experiments/issue-relations/GREPLIKE.md). It does not read the
// knowledge graph.
//
//	usegold -src <tree> -scope build-scope.json -out uses.json
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
	"strings"

	"golang.org/x/tools/go/packages"
)

const (
	testSuffix = "_test.go"
	outMode    = 0o644
)

// Pos is one line of a file.
type Pos struct {
	File string `json:"f"`
	Line int    `json:"l"`
}

// Symbol is one declared name with its uses.
type Symbol struct {
	Kind string `json:"kind"`
	Pkg  string `json:"pkg"`
	Recv string `json:"recv,omitempty"`
	Name string `json:"name"`
	Def  Pos    `json:"def"`
	Uses []Pos  `json:"uses"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		slog.Error("usegold failed", "err", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	fs := flag.NewFlagSet("usegold", flag.ContinueOnError)
	src := fs.String("src", "", "source tree")
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
	files, err := scopeFiles(*scope)
	if err != nil {
		return err
	}
	syms, err := collect(abs, files, *tags)
	if err != nil {
		return err
	}
	b, err := json.Marshal(syms)
	if err != nil {
		return fmt.Errorf("encode: %w", err)
	}
	if err := os.WriteFile(*out, b, outMode); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	slog.Info("done", "symbols", len(syms))
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

// collect type-checks the module for linux/amd64 and indexes every use.
func collect(src string, scope map[string]bool, tags string) ([]Symbol, error) {
	fset := token.NewFileSet()
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps,
		Dir:  src, Fset: fset, BuildFlags: []string{"-tags=" + tags},
		Env: append(os.Environ(), "GOOS=linux", "GOARCH=amd64", "CGO_ENABLED=1"),
	}
	roots, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("load: %w", err)
	}
	at := func(p token.Pos) (Pos, bool) {
		q := fset.Position(p)
		r, err := filepath.Rel(src, q.Filename)
		if err != nil {
			return Pos{}, false
		}
		r = filepath.ToSlash(r)
		return Pos{File: r, Line: q.Line}, scope[r]
	}
	uses := map[types.Object]map[Pos]bool{}
	recv := map[types.Object]string{}
	for _, pkg := range roots {
		for _, f := range pkg.Syntax {
			if _, ok := at(f.Pos()); !ok {
				continue
			}
			ast.Inspect(f, func(n ast.Node) bool {
				id, ok := n.(*ast.Ident)
				if !ok {
					return true
				}
				o := origin(pkg.TypesInfo.Uses[id])
				p, in := at(id.Pos())
				if o == nil || !in {
					return true
				}
				if uses[o] == nil {
					uses[o] = map[Pos]bool{}
				}
				uses[o][p] = true
				return true
			})
		}
		fieldOwners(pkg, recv)
	}
	var out []Symbol
	for o, ps := range uses {
		def, in := at(o.Pos())
		kind := kindOf(o)
		if !in || kind == "" || o.Pkg() == nil {
			continue
		}
		s := Symbol{Kind: kind, Pkg: o.Pkg().Name(), Name: o.Name(), Def: def, Recv: recv[o]}
		if f, ok := o.(*types.Func); ok {
			s.Recv = receiver(f)
		}
		for p := range ps {
			s.Uses = append(s.Uses, p)
		}
		slices.SortFunc(s.Uses, func(a, b Pos) int {
			if c := strings.Compare(a.File, b.File); c != 0 {
				return c
			}
			return a.Line - b.Line
		})
		out = append(out, s)
	}
	slices.SortFunc(out, func(a, b Symbol) int {
		if c := strings.Compare(a.Def.File, b.Def.File); c != 0 {
			return c
		}
		if a.Def.Line != b.Def.Line {
			return a.Def.Line - b.Def.Line
		}
		return strings.Compare(a.Name, b.Name)
	})
	return out, nil
}

func origin(o types.Object) types.Object {
	switch x := o.(type) {
	case *types.Func:
		return x.Origin()
	case *types.Var:
		return x.Origin()
	case *types.Const, *types.TypeName:
		return o
	}
	return nil
}

// kindOf names the kinds that are sampled; local names get "".
func kindOf(o types.Object) string {
	pkgLevel := o.Pkg() != nil && o.Parent() == o.Pkg().Scope()
	switch x := o.(type) {
	case *types.Func:
		if x.Type().(*types.Signature).Recv() != nil {
			return "method"
		}
		return "func"
	case *types.Var:
		if x.IsField() {
			return "field"
		}
		if pkgLevel {
			return "var"
		}
	case *types.Const:
		if pkgLevel {
			return "const"
		}
	case *types.TypeName:
		if pkgLevel {
			return "type"
		}
	}
	return ""
}

func receiver(f *types.Func) string {
	r := f.Type().(*types.Signature).Recv()
	if r == nil {
		return ""
	}
	t := r.Type()
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	if n, ok := t.(*types.Named); ok {
		return n.Obj().Name()
	}
	return ""
}

func fieldOwners(pkg *packages.Package, recv map[types.Object]string) {
	if pkg.Types == nil {
		return
	}
	sc := pkg.Types.Scope()
	for _, name := range sc.Names() {
		tn, ok := sc.Lookup(name).(*types.TypeName)
		if !ok || tn.IsAlias() {
			continue
		}
		st, ok := tn.Type().Underlying().(*types.Struct)
		if !ok {
			continue
		}
		for i := range st.NumFields() {
			if f := st.Field(i); recv[f] == "" {
				recv[f] = name
			}
		}
	}
}
