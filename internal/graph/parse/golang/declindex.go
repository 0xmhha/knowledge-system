package golang

import (
	"go/token"
	"path/filepath"
	"strings"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// declIndex finds the node of a function or method declaration for the
// post-Resolve passes. Canonical ids are unique except for init functions:
// every file of a package may declare one, and all share <importpath>.init
// (lineQualifyDuplicateCanonicalIDs only separates duplicates inside one
// file). With several candidates the lookup takes the one declared in the
// file, and on the line, of the declaration; it returns nil rather than
// another file's node.
type declIndex struct {
	byCanonical map[string][]*types.Node
	byQname     map[string][]*types.Node
}

func newDeclIndex(nodes []types.Node) declIndex {
	ix := declIndex{byCanonical: map[string][]*types.Node{}, byQname: map[string][]*types.Node{}}
	for i := range nodes {
		n := &nodes[i]
		if n.Type != types.NodeFunction && n.Type != types.NodeMethod {
			continue
		}
		if n.CanonicalID != "" {
			ix.byCanonical[n.CanonicalID] = append(ix.byCanonical[n.CanonicalID], n)
		}
		ix.byQname[n.QualifiedName] = append(ix.byQname[n.QualifiedName], n)
	}
	return ix
}

// at returns the node of the declaration with canonical id cid (or, when no
// node has that id, qualified name qname) declared at pos.
func (ix declIndex) at(cid, qname string, pos token.Position) *types.Node {
	cands := ix.byCanonical[cid]
	if cid == "" || len(cands) == 0 {
		cands = ix.byQname[qname]
	}
	return pickDecl(cands, pos)
}

func pickDecl(cands []*types.Node, pos token.Position) *types.Node {
	if len(cands) == 1 {
		return cands[0]
	}
	var inFile *types.Node
	for _, c := range cands {
		if !sameFile(c.FilePath, pos.Filename) {
			continue
		}
		if c.StartLine <= pos.Line && pos.Line <= c.EndLine {
			return c
		}
		if inFile == nil {
			inFile = c
		}
	}
	return inFile
}

// sameFile compares a node's file path (relative to the build root, or
// absolute) with a position's file name (absolute).
func sameFile(nodePath, posFile string) bool {
	a, b := filepath.ToSlash(nodePath), filepath.ToSlash(posFile)
	return a == b || strings.HasSuffix(b, "/"+a)
}
