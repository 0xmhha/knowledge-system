package golang

import (
	gotypes "go/types"

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
	byCanonical map[string]string
	byQname     map[string]string
}

// newSymbolIndex indexes the nodes of the given kinds. The first node per key
// wins, as before, so the qname fallback keeps its old behaviour.
func newSymbolIndex(nodes []types.Node, kinds ...types.NodeType) symbolIndex {
	want := make(map[types.NodeType]bool, len(kinds))
	for _, k := range kinds {
		want[k] = true
	}
	idx := symbolIndex{byCanonical: map[string]string{}, byQname: map[string]string{}}
	for _, n := range nodes {
		if !want[n.Type] {
			continue
		}
		if n.CanonicalID != "" {
			if _, dup := idx.byCanonical[n.CanonicalID]; !dup {
				idx.byCanonical[n.CanonicalID] = n.ID
			}
		}
		if _, dup := idx.byQname[n.QualifiedName]; !dup {
			idx.byQname[n.QualifiedName] = n.ID
		}
	}
	return idx
}

// lookup returns the node for obj. When obj has a canonical id only an exact
// canonical match counts; a miss means the declaration is not in the graph.
// The qname is used only when obj has no canonical id (AST-only builds pass a
// nil obj).
func (s symbolIndex) lookup(obj gotypes.Object, qname string) (string, bool) {
	if cid := goCanonicalID(obj); cid != "" {
		id, ok := s.byCanonical[cid]
		return id, ok
	}
	id, ok := s.byQname[qname]
	return id, ok
}
