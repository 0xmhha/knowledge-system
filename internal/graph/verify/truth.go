package verify

import (
	"fmt"
	"go/ast"
	"go/token"
	gotypes "go/types"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/types/typeutil"

	"github.com/0xmhha/knowledge-system/internal/graph/detect"
)

// Decl is one package-level declaration resolved by go/types.
type Decl struct {
	// Key is CanonicalID, except for init functions: a package may declare
	// several, all with the id <importpath>.init, so their key adds the
	// position.
	Key         string `json:"key"`
	CanonicalID string `json:"canonical_id"`
	Name        string `json:"name"`
	File        string `json:"file"`
	Line        int    `json:"line"` // line of the declared name
}

// Pair is one relation between two declarations, keyed by canonical id.
type Pair struct {
	From string `json:"from"`
	To   string `json:"to"`
	File string `json:"file"`
	Line int    `json:"line"` // first site, for samples
	// InClosure is true when the first site is inside a function literal.
	InClosure bool `json:"in_closure,omitempty"`
}

// Truth is everything the compiler front end resolves for the covered files.
type Truth struct {
	Decls map[string]Decl // Decl.Key -> declaration
	// StaticCalls and InterfaceCalls are deduplicated by (From, To).
	StaticCalls    []Pair
	InterfaceCalls []Pair
	TypeRefs       []Pair
	// DynamicCallSites counts calls through a function value, method value or
	// closure: go/types gives them no callee, so there is no pair to check.
	DynamicCallSites int
	// CallsOutsideFuncs counts static calls in package-level initialisers.
	// They have no enclosing function and are not compared.
	CallsOutsideFuncs int
	// Stmts are the statement links of every function body (statements.go).
	Stmts map[StmtFact]bool
	// Load is what the go command reported while loading the truth's
	// packages; Modules compares the outside modules with the scope's.
	Load    detect.LoadProblems
	Modules detect.ModuleCheck
}

// loadPackages loads the packages under srcRoot with full type information.
// It is a variable so tests can substitute a fixed loader.
var loadPackages = func(srcRoot string, b *detect.GoBuild) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedSyntax |
			packages.NeedTypes | packages.NeedTypesInfo | packages.NeedImports | packages.NeedDeps |
			packages.NeedModule,
		Dir:        srcRoot,
		Tests:      true,
		Env:        b.Env(),
		BuildFlags: b.BuildFlags(),
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("verify: load packages in %s: %w", srcRoot, err)
	}
	return pkgs, nil
}

// collectTruth walks every loaded file that is in scope (a slash-separated
// path relative to srcRoot) and records declarations and relations.
func collectTruth(srcRoot string, scope map[string]bool, b *detect.GoBuild) (*Truth, error) {
	absRoot, err := filepath.Abs(srcRoot)
	if err != nil {
		return nil, fmt.Errorf("verify: abs %s: %w", srcRoot, err)
	}
	pkgs, err := loadPackages(absRoot, b)
	if err != nil {
		return nil, err
	}
	// The truth comes from the same go command as the graph. When it cannot
	// load a package, both lose the same references and agree, so the
	// problems are reported with the comparison.
	t := &Truth{Decls: map[string]Decl{}, Stmts: map[StmtFact]bool{}, Load: detect.SummarizeLoad(pkgs, 20), Modules: detect.CheckModules(pkgs, b.ScopeModules())}
	seenFile := map[string]bool{}
	static := pairSet{}
	iface := pairSet{}
	typeRefs := pairSet{}
	// Test variants load the same production file twice; walk each file once.
	for _, pkg := range pkgs {
		if pkg.TypesInfo == nil {
			continue
		}
		for i, f := range pkg.Syntax {
			if i >= len(pkg.CompiledGoFiles) {
				continue
			}
			rel, ok := relPath(absRoot, pkg.CompiledGoFiles[i])
			if !ok || !scope[rel] || seenFile[rel] {
				continue
			}
			seenFile[rel] = true
			w := fileWalker{t: t, info: pkg.TypesInfo, fset: pkg.Fset, rel: rel, root: absRoot, scope: scope,
				static: static, iface: iface, typeRefs: typeRefs, stmt: t.Stmts}
			w.walk(f)
		}
	}
	t.StaticCalls = static.sorted()
	t.InterfaceCalls = iface.sorted()
	t.TypeRefs = typeRefs.sorted()
	return t, nil
}

func relPath(absRoot, abs string) (string, bool) {
	rel, err := filepath.Rel(absRoot, abs)
	if err != nil || strings.HasPrefix(rel, "..") {
		return "", false
	}
	return filepath.ToSlash(rel), true
}

type pairSet map[[2]string]Pair

func (s pairSet) add(p Pair) {
	k := [2]string{p.From, p.To}
	if _, ok := s[k]; !ok {
		s[k] = p
	}
}

func (s pairSet) sorted() []Pair {
	out := make([]Pair, 0, len(s))
	for _, p := range s {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].From != out[j].From {
			return out[i].From < out[j].From
		}
		return out[i].To < out[j].To
	})
	return out
}

type fileWalker struct {
	t        *Truth
	info     *gotypes.Info
	fset     *token.FileSet
	rel      string
	root     string
	scope    map[string]bool
	static   pairSet
	iface    pairSet
	typeRefs pairSet
	stmt     map[StmtFact]bool
}

func (w *fileWalker) walk(f *ast.File) {
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			cid := w.declare(d.Name)
			if cid == "" {
				// No canonical id (a method of an alias type): its statement
				// links are keyed by file and line, so they are still checked.
				if d.Body != nil {
					w.statements(d)
				}
				continue
			}
			if d.Body == nil {
				// A declaration implemented elsewhere (assembly, linkname):
				// its signature still refers to types.
				w.typeRefsIn(cid, d.Type)
				continue
			}
			w.relations(cid, d)
			w.statements(d)
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				w.genSpec(spec)
			}
		}
	}
}

func (w *fileWalker) genSpec(spec ast.Spec) {
	switch s := spec.(type) {
	case *ast.TypeSpec:
		cid := w.declare(s.Name)
		if cid == "" {
			return
		}
		w.typeRefsIn(cid, s.Type)
		if it, ok := s.Type.(*ast.InterfaceType); ok && it.Methods != nil {
			for _, m := range it.Methods.List {
				for _, n := range m.Names {
					w.declare(n)
				}
			}
		}
	case *ast.ValueSpec:
		keys := make([]string, 0, len(s.Names))
		for _, n := range s.Names {
			keys = append(keys, w.declare(n))
		}
		for i, v := range s.Values {
			// Value i initialises name i; with one value for several names
			// (var a, b = f()) the first name owns it.
			owner := keys[0]
			if i < len(keys) {
				owner = keys[i]
			}
			if owner == "" {
				// The blank identifier has no key: its initialiser's calls
				// stay uncompared and are counted.
				ast.Inspect(v, func(n ast.Node) bool {
					if c, ok := n.(*ast.CallExpr); ok && typeutil.StaticCallee(w.info, c) != nil {
						w.t.CallsOutsideFuncs++
					}
					return true
				})
				continue
			}
			w.callsIn(owner, v)
			w.statementsIn(v)
		}
	}
}

// callsIn records the calls under root as calls by caller, closures included.
func (w *fileWalker) callsIn(caller string, root ast.Node) {
	var lits []*ast.FuncLit // function literals enclosing the current node
	ast.Inspect(root, func(n ast.Node) bool {
		if n == nil {
			return true
		}
		for len(lits) > 0 && n.Pos() >= lits[len(lits)-1].End() {
			lits = lits[:len(lits)-1]
		}
		switch n := n.(type) {
		case *ast.FuncLit:
			lits = append(lits, n)
		case *ast.CallExpr:
			w.call(caller, n, len(lits) > 0)
		}
		return true
	})
}

// declare records a package-level declaration and returns its key
// ("" for the blank identifier and objects without a package).
func (w *fileWalker) declare(name *ast.Ident) string {
	obj := w.info.Defs[name]
	cid := canonicalID(obj)
	if cid == "" {
		return ""
	}
	line := w.fset.Position(name.Pos()).Line
	key := cid
	if fn, ok := obj.(*gotypes.Func); ok && fn.Name() == "init" && fn.Type().(*gotypes.Signature).Recv() == nil {
		key = fmt.Sprintf("%s#%s:%d", cid, w.rel, line)
	}
	if _, dup := w.t.Decls[key]; !dup {
		w.t.Decls[key] = Decl{Key: key, CanonicalID: cid, Name: name.Name, File: w.rel, Line: line}
	}
	return key
}

// relations records the calls and type references inside a function body,
// closures included, attributed to the enclosing declaration.
func (w *fileWalker) relations(caller string, d *ast.FuncDecl) {
	w.typeRefsIn(caller, d.Type)
	if d.Recv != nil {
		w.typeRefsIn(caller, d.Recv)
	}
	var lits []*ast.FuncLit // function literals enclosing the current node
	ast.Inspect(d.Body, func(n ast.Node) bool {
		if n == nil {
			return true
		}
		for len(lits) > 0 && n.Pos() >= lits[len(lits)-1].End() {
			lits = lits[:len(lits)-1]
		}
		switch n := n.(type) {
		case *ast.FuncLit:
			lits = append(lits, n)
		case *ast.CallExpr:
			w.call(caller, n, len(lits) > 0)
		case *ast.Ident:
			w.typeRef(caller, n)
		}
		return true
	})
}

func (w *fileWalker) call(caller string, c *ast.CallExpr, inClosure bool) {
	line := w.fset.Position(c.Lparen).Line
	if fn := typeutil.StaticCallee(w.info, c); fn != nil {
		if fn.Origin() != nil {
			fn = fn.Origin()
		}
		if w.inScope(fn) {
			w.static.add(Pair{From: caller, To: canonicalID(fn), File: w.rel, Line: line, InClosure: inClosure})
		}
		return
	}
	if sel, ok := ast.Unparen(c.Fun).(*ast.SelectorExpr); ok {
		// An interface method call: the receiver is an interface, or a struct
		// that embeds one (c.Read() with Conn{Reader}). In both cases the
		// selected method is declared by an interface; StaticCallee is nil.
		if s := w.info.Selections[sel]; s != nil && s.Kind() == gotypes.MethodVal {
			if fn, ok := s.Obj().(*gotypes.Func); ok && isInterfaceMethod(fn) {
				if w.inScope(fn) {
					w.iface.add(Pair{From: caller, To: canonicalID(fn), File: w.rel, Line: line, InClosure: inClosure})
				}
				return
			}
		}
	}
	if tv, ok := w.info.Types[c.Fun]; ok && (tv.IsType() || tv.IsBuiltin()) {
		return // conversion or builtin, not a call to code
	}
	w.t.DynamicCallSites++
}

// isInterfaceMethod reports whether fn is declared by an interface type.
func isInterfaceMethod(fn *gotypes.Func) bool {
	sig, ok := fn.Type().(*gotypes.Signature)
	return ok && sig.Recv() != nil && gotypes.IsInterface(sig.Recv().Type())
}

func (w *fileWalker) typeRefsIn(from string, n ast.Node) {
	if n == nil {
		return
	}
	ast.Inspect(n, func(x ast.Node) bool {
		if id, ok := x.(*ast.Ident); ok {
			w.typeRef(from, id)
		}
		return true
	})
}

// typeRef records a use of a package-level named type declared in scope.
func (w *fileWalker) typeRef(from string, id *ast.Ident) {
	tn, ok := w.info.Uses[id].(*gotypes.TypeName)
	if !ok || tn.Pkg() == nil || tn.Parent() != tn.Pkg().Scope() || !w.inScope(tn) {
		return
	}
	to := canonicalID(tn)
	if to == "" || to == from {
		return
	}
	w.typeRefs.add(Pair{From: from, To: to, File: w.rel, Line: w.fset.Position(id.Pos()).Line})
}

// inScope reports whether obj is declared in a file the graph covers.
func (w *fileWalker) inScope(obj gotypes.Object) bool {
	if obj == nil || !obj.Pos().IsValid() {
		return false
	}
	rel, ok := relPath(w.root, w.fset.Position(obj.Pos()).Filename)
	return ok && w.scope[rel]
}

// canonicalID formats obj the way the graph documents its canonical ids:
//
//	function:         <importpath>.<Func>
//	method:           <importpath>.(<*?RecvType>).<Method>
//	interface method: <importpath>.<Interface>.<Method>
//	type/const/var:   <importpath>.<Name>
//
// Written independently of the parser so the two can disagree.
func canonicalID(obj gotypes.Object) string {
	if obj == nil || obj.Pkg() == nil || obj.Name() == "_" {
		return ""
	}
	pkgPath := obj.Pkg().Path()
	fn, ok := obj.(*gotypes.Func)
	if !ok {
		if obj.Parent() != obj.Pkg().Scope() {
			return "" // not package level (local declaration, field, parameter)
		}
		return pkgPath + "." + obj.Name()
	}
	sig, ok := fn.Type().(*gotypes.Signature)
	if !ok || sig.Recv() == nil {
		if fn.Parent() != fn.Pkg().Scope() {
			return ""
		}
		return pkgPath + "." + fn.Name()
	}
	rt := sig.Recv().Type()
	star := ""
	if p, ok := rt.(*gotypes.Pointer); ok {
		rt, star = p.Elem(), "*"
	}
	named, ok := rt.(*gotypes.Named)
	if !ok || named.Obj() == nil {
		return ""
	}
	if gotypes.IsInterface(named) {
		return pkgPath + "." + named.Obj().Name() + "." + fn.Name()
	}
	return pkgPath + ".(" + star + named.Obj().Name() + ")." + fn.Name()
}
