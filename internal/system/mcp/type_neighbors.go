package mcp

import (
	"context"
	"strings"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

const (
	// Only the highest-ranked Go bodies anchor a type lookup, and at most
	// two declarations join the response, so the primary ranking moves by
	// at most two places.
	maxTypeNeighborAnchors = 3
	maxTypeNeighbors       = 2
)

// withTypeNeighbors returns refs with, right after an anchoring body, the
// declaration of one indexed project type that body uses (a CKG uses_type
// edge). Short commented type declarations rank low in dense retrieval even
// when the selected function depends on them. Lookup failures leave refs
// unchanged: the extension is optional evidence, never a reason to fail.
// It returns nil when nothing was added.
func withTypeNeighbors(ctx context.Context, ckg ckgclient.Client, refs []contract.Citation) []contract.Citation {
	if ckg == nil || len(refs) == 0 {
		return nil
	}
	out := make([]contract.Citation, 0, len(refs)+maxTypeNeighbors)
	added := 0
	for i, anchor := range refs {
		out = append(out, anchor)
		if i >= maxTypeNeighborAnchors || added >= maxTypeNeighbors || !strings.HasSuffix(anchor.File, ".go") {
			continue
		}
		neighbors, err := ckg.Neighbors(ctx, anchor, ckgclient.NeighborsOpts{
			Relations: []contract.Relation{contract.RelationUsesType}, Hops: 1, ExcludeTests: true, MaxTotal: 8})
		if err != nil {
			continue
		}
		for _, n := range neighbors {
			if n.Relation != contract.RelationUsesType || !n.Target.IsValid() || overlapsCited(n.Target, refs, out) {
				continue
			}
			out = append(out, n.Target)
			added++
			break
		}
	}
	if added == 0 {
		return nil
	}
	return out
}

func overlapsCited(c contract.Citation, lists ...[]contract.Citation) bool {
	for _, list := range lists {
		for _, r := range list {
			if r.File == c.File && r.StartLine <= c.EndLine && c.StartLine <= r.EndLine {
				return true
			}
		}
	}
	return false
}
