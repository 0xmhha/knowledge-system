package golang

import (
	"go/token"
	gotypes "go/types"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// implements.go emits implements / extends edges by asking go/types whether
// each concrete type of the loaded packages satisfies each interface.
//
// Why a separate post-pass and not inline with declarations.go:
//   - Per-file Pass 1 only sees one file's AST. Cross-package satisfaction
//     (the common case — sqliteStore implements StoreReader where the
//     interface lives in another file or package) requires the *union* of
//     types across the whole module.
//   - go/types' Implements only works on resolved *types.Named values, which
//     are only available after packages.Load completes.
//
// Which types take part: every named type a loaded file declares, at package
// level or inside a function (a local `type reader interface{...}` is
// satisfied by package-level types, and a local struct that embeds an
// interface satisfies it), including the types of test files.
//
// Test files are the reason for the two-universe rule below. A package with
// tests is type-checked twice: as itself, and as its test variant, which
// holds the production files again plus the _test.go files. The two copies
// have separate type objects: *rlp.Stream of the variant is not *rlp.Stream
// of the package. A type declared in a test file exists only in the variant,
// so go/types does not see it satisfy an interface taken from the package
// itself, whose methods name the package's own Stream. Compared that way, 22
// pairs of go-stablenet had no edge (rlp's testDecoder and rlp.Decoder).
// A pair from two universes is therefore compared by the text of its method
// signatures, with types written by import path, which is the same in both.

// namedDecl is one named type declaration taking part in the comparison.
type namedDecl struct {
	id    string // node id
	named *gotypes.Named
	iface *gotypes.Interface // non-nil for an interface
	// variant is set for a type that only a test variant declares (a type of
	// a _test.go file).
	variant bool
	// methods caches methodTexts(named) for the two-universe comparison.
	methods map[string]string
}

// EmitImplementsEdges returns the implements edges (concrete type →
// interface) for every satisfaction pair and the extends edges (interface →
// interface it embeds) of the loaded packages. An embedded interface from
// outside the build (io.Reader) has no node yet: it is returned as a pending
// ref, which ExternalEdges binds to an External node, as for uses_type.
//
// Receiver shape: satisfaction is checked on BOTH the named type T and the
// pointer type *T so both value-receiver and pointer-receiver method sets
// count (the standard Go semantics).
//
// The empty interface is left out: every type satisfies it. An uninstantiated
// generic type satisfies nothing in go/types and is skipped likewise.
//
// Returns empty slices (never nil edges) when pkgs is empty — callers can
// append unconditionally.
func EmitImplementsEdges(pkgs []*packages.Package, nodes []types.Node) ([]types.Edge, []parse.PendingRef) {
	if len(pkgs) == 0 {
		return []types.Edge{}, nil
	}
	// Only kinds that can legitimately appear as src/dst of
	// implements/extends are indexed:
	//   - Struct, TypeAlias, Enum: valid implements src
	//   - Interface: valid implements dst AND extends src/dst
	index := newSymbolIndex(fsetOf(pkgs), nodes, types.NodeStruct, types.NodeInterface, types.NodeTypeAlias, types.NodeEnum)
	concretes, ifaces := namedDecls(pkgs, index)

	out := make([]types.Edge, 0, len(concretes))
	for _, iface := range ifaces {
		// Skip the empty interface — every type satisfies it; emitting would
		// produce O(N) low-signal edges per concrete type.
		if iface.iface.Empty() {
			continue
		}
		for _, impl := range concretes {
			if satisfies(impl, iface) {
				out = append(out, types.Edge{
					Src: impl.id, Dst: iface.id,
					Type:       types.EdgeImplements,
					Count:      1,
					Confidence: types.ConfExtracted,
				})
			}
		}
	}

	// extends: the literal embedding (NumEmbeddeds), not the transitive
	// "method set includes", which interface satisfaction would also give.
	var pending []parse.PendingRef
	for _, a := range ifaces {
		for i := 0; i < a.iface.NumEmbeddeds(); i++ {
			embNamed, ok := gotypes.Unalias(a.iface.EmbeddedType(i)).(*gotypes.Named)
			if !ok {
				continue // a union or an interface literal: no name to point to
			}
			embObj := embNamed.Obj()
			if embObj == nil || embObj.Pkg() == nil {
				continue // builtin (error, comparable) — no node to point to
			}
			embQname := embObj.Pkg().Name() + "." + embObj.Name()
			if dstID, ok := index.lookup(embObj, embQname); ok {
				if dstID != a.id {
					out = append(out, types.Edge{
						Src: a.id, Dst: dstID,
						Type:       types.EdgeExtends,
						Count:      1,
						Confidence: types.ConfExtracted,
					})
				}
				continue
			}
			// Outside the build: 21 interfaces of go-stablenet embed
			// io.Reader, io.Closer and the like.
			if cid := goCanonicalID(embObj); cid != "" {
				pending = append(pending, parse.PendingRef{
					SrcID: a.id, EdgeType: types.EdgeExtends,
					TargetQName: embQname, TargetCanonicalID: cid,
				})
			}
		}
	}
	return out, pending
}

// namedDecls collects the named types the loaded files declare that have a
// node, split into concrete types and interfaces, in a fixed order.
//
// A declaration is taken once, from the package itself when it has it and
// from the test variant only when no primary package does (the order in which
// packages.Load lists them is not fixed, and a type taken from the variant
// would be compared across universes for nothing).
func namedDecls(pkgs []*packages.Package, index symbolIndex) (concretes, ifaces []*namedDecl) {
	if fsetOf(pkgs) == nil {
		return nil, nil
	}
	// Keyed by file and offset, not by token.Pos: the variant's copy of a
	// file need not share the positions of the package's.
	fset := fsetOf(pkgs)
	seen := map[token.Position]bool{}
	at := func(tn *gotypes.TypeName) token.Position {
		p := fset.Position(tn.Pos())
		return token.Position{Filename: p.Filename, Offset: p.Offset}
	}
	collect := func(wantVariant bool) {
		for _, pkg := range pkgs {
			if pkg == nil || pkg.Types == nil || pkg.TypesInfo == nil || isTestVariantPkg(pkg) != wantVariant {
				continue
			}
			var names []*gotypes.TypeName
			for _, obj := range pkg.TypesInfo.Defs {
				if tn, ok := obj.(*gotypes.TypeName); ok && !tn.IsAlias() && tn.Pos().IsValid() && !seen[at(tn)] {
					names = append(names, tn)
				}
			}
			sort.Slice(names, func(i, j int) bool { return names[i].Pos() < names[j].Pos() })
			for _, tn := range names {
				seen[at(tn)] = true
				named, ok := tn.Type().(*gotypes.Named)
				if !ok {
					continue // a type parameter
				}
				id, ok := index.lookup(tn, pkg.Types.Name()+"."+tn.Name())
				if !ok {
					continue
				}
				d := &namedDecl{id: id, named: named, variant: wantVariant}
				if iface, ok := named.Underlying().(*gotypes.Interface); ok {
					d.iface = iface
					ifaces = append(ifaces, d)
				} else {
					concretes = append(concretes, d)
				}
			}
		}
	}
	collect(false)
	collect(true)
	return concretes, ifaces
}

// satisfies reports whether the concrete type impl implements iface.
func satisfies(impl, iface *namedDecl) bool {
	if impl.id == iface.id {
		return false
	}
	if gotypes.Implements(impl.named, iface.iface) ||
		gotypes.Implements(gotypes.NewPointer(impl.named), iface.iface) {
		return true
	}
	// Both from primary packages: one universe, and go/types has answered.
	// A generic type or a type-set interface has no method-text answer.
	if !impl.variant && !iface.variant || impl.named.TypeParams().Len() > 0 || !iface.iface.IsMethodSet() {
		return false
	}
	if impl.methods == nil {
		impl.methods = methodTexts(gotypes.NewMethodSet(gotypes.NewPointer(impl.named)))
	}
	if iface.methods == nil {
		iface.methods = methodTexts(gotypes.NewMethodSet(iface.named))
	}
	for name, sig := range iface.methods {
		if impl.methods[name] != sig {
			return false
		}
	}
	return true
}

// methodTexts maps each method of the method set to the text of its
// signature. A method is keyed by its name, and an unexported one by its
// package path as well, as go/types identifies methods.
func methodTexts(mset *gotypes.MethodSet) map[string]string {
	out := make(map[string]string, mset.Len())
	for i := 0; i < mset.Len(); i++ {
		fn, ok := mset.At(i).Obj().(*gotypes.Func)
		if !ok {
			continue
		}
		key := fn.Name()
		if !fn.Exported() && fn.Pkg() != nil {
			key = fn.Pkg().Path() + "." + key
		}
		out[key] = signatureText(fn.Type().(*gotypes.Signature))
	}
	return out
}

// signatureText writes the parameter and result types of sig with every type
// qualified by its import path and without parameter names, so that the same
// signature reads the same in a package and in its test variant.
func signatureText(sig *gotypes.Signature) string {
	byPath := func(p *gotypes.Package) string { return p.Path() }
	var b strings.Builder
	tuple := func(t *gotypes.Tuple, variadic bool) {
		b.WriteByte('(')
		for i := 0; i < t.Len(); i++ {
			if i > 0 {
				b.WriteByte(',')
			}
			if variadic && i == t.Len()-1 {
				b.WriteString("...")
			}
			b.WriteString(gotypes.TypeString(t.At(i).Type(), byPath))
		}
		b.WriteByte(')')
	}
	tuple(sig.Params(), sig.Variadic())
	tuple(sig.Results(), false)
	return b.String()
}
