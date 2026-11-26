package verify

import (
	"fmt"
	"go/ast"
	"go/token"
	gotypes "go/types"
	"os"
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
	// StartByte is the offset of the declaration's first token: the keyword
	// of a function and of a type, var or const that stands alone, the
	// specification itself in a parenthesised group. The node starts there.
	StartByte int `json:"start_byte"`
	// EndByte is the offset where the declaration ends: after its last
	// token, or after the comment that follows it on its last line (for a
	// function, the comments that start on the line of its last token with
	// only blanks between). The node ends there.
	EndByte int `json:"end_byte"`
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
	// OutsideCalls and OutsideTypeRefs are the calls and type references
	// whose target is declared outside the covered files: the standard
	// library, a dependency, a file of the module the build scope leaves
	// out, the builtin error method. To is the target's canonical id; the
	// graph holds such a target as an External node with that id.
	OutsideCalls    []Pair
	OutsideTypeRefs []Pair
	// DynamicCallSites counts calls through a function value, method value or
	// closure: go/types gives them no callee, so there is no pair to check.
	DynamicCallSites int
	// CallsOutsideFuncs counts static calls in package-level initialisers.
	// They have no enclosing function and are not compared.
	CallsOutsideFuncs int
	// Stmts are the statement links of every function body (statements.go).
	Stmts map[StmtFact]bool
	// UnderLock are the (field, mutex) pairs of accessed_under_lock: a field
	// used in a function body at a point where the mutex is held
	// (underlock.go). Both are keyed "file:line:name" of their declaration.
	UnderLock map[[2]string]bool
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
	t := &Truth{Decls: map[string]Decl{}, Stmts: map[StmtFact]bool{}, UnderLock: map[[2]string]bool{}, Load: detect.SummarizeLoad(pkgs, 20), Modules: detect.CheckModules(pkgs, b.ScopeModules())}
	seenFile := map[string]bool{}
	static := pairSet{}
	iface := pairSet{}
	typeRefs := pairSet{}
	outCalls := pairSet{}
	outTypes := pairSet{}
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
				static: static, iface: iface, typeRefs: typeRefs, outCalls: outCalls, outTypes: outTypes, stmt: t.Stmts}
			w.walk(f)
		}
	}
	t.StaticCalls = static.sorted()
	t.InterfaceCalls = iface.sorted()
	t.TypeRefs = typeRefs.sorted()
	t.OutsideCalls = outCalls.sorted()
	t.OutsideTypeRefs = outTypes.sorted()
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
	outCalls pairSet
	outTypes pairSet
	stmt     map[StmtFact]bool
}

func (w *fileWalker) walk(f *ast.File) {
	// The file's bytes, for the blanks between a function and the comment
	// after it; without them a function ends at its last token.
	src, _ := os.ReadFile(filepath.Join(w.root, filepath.FromSlash(w.rel)))
	locks := &lockTruth{info: w.info, fset: w.fset, root: w.root, scope: w.scope, out: w.t.UnderLock}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			locks.function(d.Body, nil)
			cid := w.declare(d.Name, d.Pos(), w.funcEnd(d, f.Comments, src))
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
				// A specification in a group is a declaration of its own;
				// one that stands alone starts at the keyword.
				start := d.Pos()
				if d.Lparen.IsValid() {
					start = spec.Pos()
				}
				w.genSpec(spec, start)
			}
		}
	}
}

// through returns the end of a declaration whose last token ends at end and
// which the comment c follows on its last line (nil for none).
func through(end token.Pos, c *ast.CommentGroup) token.Pos {
	if c == nil || c.End() <= end {
		return end
	}
	return c.End()
}

// funcEnd returns the end of the function declaration d: after its last
// token, or after the comments that start on that token's line with only
// spaces and tabs before them. The syntax tree attaches such a comment to no
// function, so comments, the file's comment groups in source order, and src,
// its bytes, are read.
func (w *fileWalker) funcEnd(d *ast.FuncDecl, comments []*ast.CommentGroup, src []byte) token.Pos {
	end := d.End()
	tf := w.fset.File(end)
	i := sort.Search(len(comments), func(i int) bool { return comments[i].Pos() >= end })
	if tf == nil || i == len(comments) {
		return end
	}
	line := tf.Line(end)
	for _, c := range comments[i].List {
		from, to := tf.Offset(end), tf.Offset(c.Pos())
		if tf.Line(c.Pos()) != line || to > len(src) || strings.Trim(string(src[from:to]), funcCommentBlanks) != "" {
			break
		}
		end = c.End()
		line = tf.Line(end)
	}
	return end
}

// funcCommentBlanks are the bytes that may stand between a function's last
// token and the comment after it.
const funcCommentBlanks = " \t"

func (w *fileWalker) genSpec(spec ast.Spec, start token.Pos) {
	switch s := spec.(type) {
	case *ast.TypeSpec:
		cid := w.declare(s.Name, start, through(s.End(), s.Comment))
		if cid == "" {
			return
		}
		if s.TypeParams != nil {
			// The constraint of a type parameter is named by the declaration
			// (type Cache[K comparable, V blobType] ...).
			w.typeRefsIn(cid, s.TypeParams)
		}
		w.typeRefsIn(cid, s.Type)
		if it, ok := s.Type.(*ast.InterfaceType); ok && it.Methods != nil {
			for _, m := range it.Methods.List {
				for _, n := range m.Names {
					w.declare(n, m.Pos(), through(m.End(), m.Comment))
				}
			}
		}
	case *ast.ValueSpec:
		keys := make([]string, 0, len(s.Names))
		for _, n := range s.Names {
			key := w.declare(n, start, through(s.End(), s.Comment))
			keys = append(keys, key)
			// The type written in the declaration is used by every name it
			// declares (var a, b T).
			if key != "" {
				w.typeRefsIn(key, s.Type)
			}
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
			w.typeRefsIn(owner, v)
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

// declare records a package-level declaration that spans [start, end) and
// returns its key ("" for the blank identifier and objects without a
// package).
func (w *fileWalker) declare(name *ast.Ident, start, end token.Pos) string {
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
		w.t.Decls[key] = Decl{Key: key, CanonicalID: cid, Name: name.Name, File: w.rel, Line: line,
			StartByte: w.fset.Position(start).Offset, EndByte: w.fset.Position(end).Offset}
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
		w.addCall(w.static, Pair{From: caller, File: w.rel, Line: line, InClosure: inClosure}, fn)
		return
	}
	if sel, ok := ast.Unparen(c.Fun).(*ast.SelectorExpr); ok {
		// An interface method call: the receiver is an interface, or a struct
		// that embeds one (c.Read() with Conn{Reader}). In both cases the
		// selected method is declared by an interface; StaticCallee is nil.
		if s := w.info.Selections[sel]; s != nil && s.Kind() == gotypes.MethodVal {
			if fn, ok := s.Obj().(*gotypes.Func); ok && isInterfaceMethod(fn) {
				w.addCall(w.iface, Pair{From: caller, File: w.rel, Line: line, InClosure: inClosure}, fn)
				return
			}
		}
	}
	if tv, ok := w.info.Types[c.Fun]; ok && (tv.IsType() || tv.IsBuiltin()) {
		return // conversion or builtin, not a call to code
	}
	w.t.DynamicCallSites++
}

// addCall records the call p of fn: in set when fn is declared in a covered
// file, among the outside calls otherwise. A callee without a canonical id (a
// method of an interface declared in a function or written without a name)
// has no key to compare by and is left out.
func (w *fileWalker) addCall(set pairSet, p Pair, fn *gotypes.Func) {
	if p.To = canonicalID(fn); p.To == "" {
		return
	}
	if w.inScope(fn) {
		set.add(p)
	} else {
		w.outCalls.add(p)
	}
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

// typeRef records a use of a package-level named type: among the type
// references when it is declared in scope, among the outside ones otherwise.
func (w *fileWalker) typeRef(from string, id *ast.Ident) {
	tn, ok := w.info.Uses[id].(*gotypes.TypeName)
	if !ok || tn.Pkg() == nil || tn.Parent() != tn.Pkg().Scope() {
		return
	}
	to := canonicalID(tn)
	if to == "" || to == from {
		return
	}
	p := Pair{From: from, To: to, File: w.rel, Line: w.fset.Position(id.Pos()).Line}
	if w.inScope(tn) {
		w.typeRefs.add(p)
	} else {
		w.outTypes.add(p)
	}
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
// A method declared on an alias (type E = fe12; func (e *E) One()) has the id
// of the aliased type's method. The Error method of the builtin error
// interface, which has no package, has builtinErrorID. A method of an
// interface declared inside a function has none.
//
// Written independently of the parser so the two can disagree.
func canonicalID(obj gotypes.Object) string {
	if obj == nil || obj.Name() == "_" {
		return ""
	}
	if obj.Pkg() == nil {
		if fn, ok := obj.(*gotypes.Func); ok && fn.FullName() == builtinErrorFullName {
			return builtinErrorID
		}
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
	rt := gotypes.Unalias(sig.Recv().Type())
	star := ""
	if p, ok := rt.(*gotypes.Pointer); ok {
		rt, star = gotypes.Unalias(p.Elem()), "*"
	}
	named, ok := rt.(*gotypes.Named)
	if !ok || named.Obj() == nil || named.Obj().Parent() != pkgScope(named.Obj()) {
		return ""
	}
	if gotypes.IsInterface(named) {
		return pkgPath + "." + named.Obj().Name() + "." + fn.Name()
	}
	return pkgPath + ".(" + star + named.Obj().Name() + ")." + fn.Name()
}

// builtinErrorID is the canonical id of the Error method of the builtin error
// interface, and builtinErrorFullName the name go/types gives that method.
const (
	builtinErrorID       = "builtin.error.Error"
	builtinErrorFullName = "(error).Error"
)

// pkgScope is the scope of obj's package, nil for an object without one.
func pkgScope(obj gotypes.Object) *gotypes.Scope {
	if obj.Pkg() == nil {
		return nil
	}
	return obj.Pkg().Scope()
}
