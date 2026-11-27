package golang

import (
	gotypes "go/types"
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/tools/go/packages"

	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// External nodes (schema 1.36) give a node to every function, method and
// type the build uses but does not contain: the standard library, module
// dependencies, packages outside the build scope. Before them a call such as
// time.AfterFunc(d, f) had no target node, so the edge was dropped and a
// reader of the graph could not tell that the line starts a timer, or which
// package "time" is. One node per symbol is shared by all users, as a linker
// shares one symbol across object files.
//
// Only symbols the build actually references get a node: the index below
// knows every external object the type checker saw, and a node is made the
// first time an edge needs it.

// BuiltinErrorMethodID is the canonical id of the Error method of the builtin
// error interface. The method belongs to no package, so it is filed under the
// pseudo-package builtin, where the Go documentation describes error.
const BuiltinErrorMethodID = builtinPkgPath + ".error.Error"

// builtinPkgPath is the pseudo-package of the predeclared identifiers.
const builtinPkgPath = "builtin"

// externalIndex maps the canonical id of each external function, method and
// type the build's packages use to its go/types object.
type externalIndex struct {
	objs map[string]gotypes.Object
	// loaded maps import path to the loaded package, for its module.
	loaded map[string]*packages.Package
}

// newExternalIndex indexes the objects the root packages use that the build
// does not contain: those of packages that are not roots (the standard
// library, dependencies), and those of root packages declared in a file the
// build leaves out. outside says whether a file is one of those; nil means
// no file of a root package is.
//
// A build scope leaves packages of the module out (ethclient/simulated,
// internal/cmdtest: test tooling no shipped binary imports) while the code in
// scope still calls them. Left without a node because their package is a
// root, 445 calls and 71 type references of go-stablenet had no edge. They
// are code outside the build like the standard library, with the module's
// own path as their module.
func newExternalIndex(pkgs []*packages.Package, outside func(file string) bool) *externalIndex {
	x := &externalIndex{objs: map[string]gotypes.Object{}, loaded: map[string]*packages.Package{}}
	roots := map[string]bool{}
	for _, p := range pkgs {
		roots[p.PkgPath] = true
	}
	packages.Visit(pkgs, nil, func(p *packages.Package) {
		if _, ok := x.loaded[p.PkgPath]; !ok {
			x.loaded[p.PkgPath] = p
		}
	})
	for _, p := range pkgs {
		if p.TypesInfo == nil {
			continue
		}
		for _, obj := range p.TypesInfo.Uses {
			if obj == nil || obj.Pkg() == nil {
				continue
			}
			switch o := obj.(type) {
			case *gotypes.Func:
				obj = o.Origin()
			case *gotypes.TypeName:
			default:
				continue
			}
			if roots[obj.Pkg().Path()] {
				if outside == nil || p.Fset == nil || !obj.Pos().IsValid() || !outside(p.Fset.Position(obj.Pos()).Filename) {
					continue
				}
			}
			if id := goCanonicalID(obj); id != "" {
				x.objs[id] = obj
			}
		}
	}
	return x
}

// node returns the External node for canonical id cid.
func (x *externalIndex) node(cid string) (types.Node, bool) {
	if x == nil {
		return types.Node{}, false
	}
	if cid == BuiltinErrorMethodID && builtinErrorMethod != nil {
		return externalNode(cid, builtinErrorMethod, &types.ExternalPackage{Path: builtinPkgPath, Name: builtinPkgPath, Module: stdModule}), true
	}
	obj, ok := x.objs[cid]
	if !ok {
		return types.Node{}, false
	}
	return externalNode(cid, obj, x.pkgOf(obj.Pkg())), true
}

// stdModule is the module of an External node of the standard library.
const stdModule = "std"

// ExternalNodeID is the node id of the External node for canonical id cid.
func ExternalNodeID(cid string) string {
	return MakeID("external:"+cid, "go", 0)
}

// pkgOf describes the package of an external object: std for the standard
// library, else the module and version the build loaded it from.
func (x *externalIndex) pkgOf(p *gotypes.Package) *types.ExternalPackage {
	ep := &types.ExternalPackage{Path: p.Path(), Name: p.Name()}
	lp := x.loaded[p.Path()]
	switch {
	case lp != nil && lp.Module != nil:
		m := lp.Module
		ep.Module, ep.Version = m.Path, m.Version
		if m.Replace != nil && m.Replace.Version != "" {
			ep.Version = m.Replace.Version
		}
	case isStdImportPath(p.Path()):
		ep.Module = stdModule
	}
	return ep
}

// isStdImportPath: standard library import paths have no dot in their first
// element (time, net/http); module paths start with a domain.
func isStdImportPath(path string) bool {
	first, _, _ := strings.Cut(path, "/")
	return !strings.Contains(first, ".")
}

func externalNode(cid string, obj gotypes.Object, ep *types.ExternalPackage) types.Node {
	qname, kind := ep.Name+"."+obj.Name(), "type"
	if fn, ok := obj.(*gotypes.Func); ok {
		kind = "func"
		if sig, ok := fn.Type().(*gotypes.Signature); ok && sig.Recv() != nil {
			kind = "method"
			if named, _ := receiverBase(sig.Recv().Type()); named != nil {
				qname = ep.Name + "." + named.Obj().Name() + "." + fn.Name()
				if gotypes.IsInterface(named) {
					kind = "interface_method"
				}
			}
		}
	}
	qf := func(p *gotypes.Package) string { return p.Name() }
	sig := gotypes.ObjectString(obj, qf)
	if tn, ok := obj.(*gotypes.TypeName); ok {
		// A struct or interface body can run to hundreds of fields; the
		// kind is enough to say what the type is.
		switch tn.Type().Underlying().(type) {
		case *gotypes.Struct:
			sig = "type " + ep.Name + "." + tn.Name() + " struct"
		case *gotypes.Interface:
			sig = "type " + ep.Name + "." + tn.Name() + " interface"
		}
	}
	return types.Node{
		ID: ExternalNodeID(cid), Type: types.NodeExternal,
		Name: obj.Name(), QualifiedName: qname, CanonicalID: cid,
		FilePath: types.ExternalFilePath, StartLine: 1, EndLine: 1,
		Language: "go", Confidence: types.ConfExtracted,
		SubKind: kind, Signature: sig,
		External: ep,
	}
}

// ExternalTarget returns the External node for canonical id cid when cid
// names a function, method or type the build uses but does not contain.
// Every call returns an equal node; callers add it to a graph once. Without
// type information there is no External node.
func (p *Parser) ExternalTarget(cid string) (types.Node, bool) {
	if len(p.pkgs) == 0 || cid == "" {
		return types.Node{}, false
	}
	p.extOnce.Do(func() { p.ext = newExternalIndex(p.pkgs, p.outsideBuild()) })
	return p.ext.node(cid)
}

// outsideBuild returns the test "is this a source file the build leaves out"
// for the files the last Resolve was given, or nil before any Resolve: then
// no file of a root package counts as outside.
//
// Only a file under the source root can be left out. A file elsewhere that a
// root package compiles (what cgo generates into the build cache) is not
// source of the module, and its symbols stay without a node as before.
func (p *Parser) outsideBuild() func(file string) bool {
	if p.buildFiles == nil {
		return nil
	}
	files, root := p.buildFiles, p.srcRoot
	// go/packages reports absolute file names; a caller may pass the root as
	// a relative path.
	if abs, err := filepath.Abs(root); err == nil {
		root = abs
	}
	return func(file string) bool {
		rel, err := filepath.Rel(root, file)
		if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return false
		}
		for _, in := range files[filepath.Base(file)] {
			if sameFile(in, file) {
				return false
			}
		}
		return true
	}
}

// setBuildFiles records the files of the build: the ones Resolve is given,
// parsed in this build or reloaded from the store.
func (p *Parser) setBuildFiles(results []*parse.ParseResult) {
	files := make(map[string][]string, len(results))
	for _, r := range results {
		if r == nil || r.Path == "" {
			continue
		}
		base := filepath.Base(r.Path)
		files[base] = append(files[base], r.Path)
	}
	p.buildFiles = files
	p.ext, p.extOnce = nil, sync.Once{}
}

// ExternalEdges binds pending refs whose target is outside the build to
// External nodes: the uses_type refs of EmitUsesTypeEdges and the extends
// refs of EmitImplementsEdges, which run after Resolve. It returns the External nodes not yet in nodes and one edge per
// bound ref. Refs it cannot bind are left to the caller.
func (p *Parser) ExternalEdges(prs []parse.PendingRef, nodes []types.Node) ([]types.Node, []types.Edge) {
	have := map[string]bool{}
	for _, n := range nodes {
		if n.Type == types.NodeExternal {
			have[n.ID] = true
		}
	}
	var outN []types.Node
	var outE []types.Edge
	for _, pr := range prs {
		n, ok := p.ExternalTarget(pr.TargetCanonicalID)
		if !ok {
			continue
		}
		if !have[n.ID] {
			have[n.ID] = true
			outN = append(outN, n)
		}
		// No FilePath, as for the edges Resolve makes: HintFile is an
		// absolute path, and an incremental build reloads a cached file's
		// edges by its relative path, so an edge stamped with HintFile was
		// not reloaded and was inserted a second time.
		outE = append(outE, types.Edge{
			Src: pr.SrcID, Dst: n.ID, Type: pr.EdgeType,
			Line: pr.Line, Count: 1,
			Confidence: types.ConfExtracted, DispatchKind: pr.DispatchKind,
		})
	}
	return outN, outE
}
