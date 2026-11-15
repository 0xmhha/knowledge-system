package golang

import (
	"go/token"
	gotypes "go/types"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// symbolIndex finds the graph node of a go/types object for the post-Resolve
// passes (uses_type, instantiates, implements).
//
// It prefers the canonical id. The short qualified name ("pkg.Name") is not
// unique: two packages can share a name (crypto/bn256 and
// crypto/bn256/cloudflare are both "bn256"), and a type declared inside a
// function body has the same qualified name as a package-level one. Indexing
// by that name let the first node seen win, so references to core.Genesis
// resolved to the gencodec type declared inside (Genesis).MarshalJSON.
type symbolIndex struct {
	// byCanonical keeps every node per canonical id: several declarations can
	// share one (the init functions of a package, a function declared in two
	// files behind different build tags), and the first one is not the answer.
	byCanonical map[string][]types.Node
	byQname     map[string]string
	fset        *token.FileSet
}

// newSymbolIndex indexes the nodes of the given kinds. fset positions the
// go/types objects passed to lookup; nil disables position matching.
func newSymbolIndex(fset *token.FileSet, nodes []types.Node, kinds ...types.NodeType) symbolIndex {
	want := make(map[types.NodeType]bool, len(kinds))
	for _, k := range kinds {
		want[k] = true
	}
	idx := symbolIndex{byCanonical: map[string][]types.Node{}, byQname: map[string]string{}, fset: fset}
	for _, n := range nodes {
		if !want[n.Type] {
			continue
		}
		if n.CanonicalID != "" {
			idx.byCanonical[n.CanonicalID] = append(idx.byCanonical[n.CanonicalID], n)
		}
		if _, dup := idx.byQname[n.QualifiedName]; !dup {
			idx.byQname[n.QualifiedName] = n.ID
		}
	}
	return idx
}

// lookup returns the node for obj. When obj has a canonical id only an exact
// canonical match counts; a miss means the declaration is not in the graph.
// When several nodes share the canonical id, the one whose file and lines
// contain obj's position wins. An object without a canonical id that go/types
// knows (a function-local type) has no package-level node, so it does not
// fall back to the short qualified name, which would bind it to a
// package-level declaration of the same name. The qname is used only when
// there is no object (AST-only builds).
func (s symbolIndex) lookup(obj gotypes.Object, qname string) (string, bool) {
	if cid := goCanonicalID(obj); cid != "" {
		cands := s.byCanonical[cid]
		switch len(cands) {
		case 0:
			return "", false
		case 1:
			return cands[0].ID, true
		}
		if s.fset != nil && obj.Pos().IsValid() {
			pos := s.fset.Position(obj.Pos())
			file := filepath.ToSlash(pos.Filename)
			for _, n := range cands {
				fp := filepath.ToSlash(n.FilePath)
				if (file == fp || strings.HasSuffix(file, "/"+fp)) && n.StartLine <= pos.Line && pos.Line <= n.EndLine {
					return n.ID, true
				}
			}
		}
		return cands[0].ID, true
	}
	if obj != nil && obj.Pkg() != nil {
		return "", false
	}
	id, ok := s.byQname[qname]
	return id, ok
}

// fsetOf returns the file set of the loaded packages (go/packages shares one).
func fsetOf(pkgs []*packages.Package) *token.FileSet {
	for _, p := range pkgs {
		if p.Fset != nil {
			return p.Fset
		}
	}
	return nil
}
