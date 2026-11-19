package golang

import (
	gotypes "go/types"
	"strings"

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

// externalIndex maps the canonical id of each external function, method and
// type the build's packages use to its go/types object.
type externalIndex struct {
	objs map[string]gotypes.Object
	// loaded maps import path to the loaded package, for its module.
	loaded map[string]*packages.Package
}

// newExternalIndex indexes the objects the root packages use whose package is
// not one of the roots. A root package's symbols are left out even when one
// has no node (a file outside the build scope): those stay dropped as before.
func newExternalIndex(pkgs []*packages.Package) *externalIndex {
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
			if obj == nil || obj.Pkg() == nil || roots[obj.Pkg().Path()] {
				continue
			}
			switch o := obj.(type) {
			case *gotypes.Func:
				obj = o.Origin()
			case *gotypes.TypeName:
			default:
				continue
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
	obj, ok := x.objs[cid]
	if !ok {
		return types.Node{}, false
	}
	return externalNode(cid, obj, x.pkgOf(obj.Pkg())), true
}

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
		ep.Module = "std"
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
			rt := sig.Recv().Type()
			if ptr, ok := rt.(*gotypes.Pointer); ok {
				rt = ptr.Elem()
			}
			if named, ok := rt.(*gotypes.Named); ok {
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
	p.extOnce.Do(func() { p.ext = newExternalIndex(p.pkgs) })
	return p.ext.node(cid)
}

// ExternalEdges binds pending refs whose target is outside the build to
// External nodes: the uses_type refs of EmitUsesTypeEdges, which run after
// Resolve. It returns the External nodes not yet in nodes and one edge per
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
