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
	// A declaration's leading doc comment is part of its evidence; the CKG
	// node span starts at the declaration line.
	maxDocCommentLines = 30
)

// withTypeNeighbors returns refs with, right after an anchoring body, the
// declaration of one indexed project type that body uses (a CKG uses_type
// edge). Short commented type declarations rank low in dense retrieval even
// when the selected function depends on them. Lookup failures leave refs
// unchanged: the extension is optional evidence, never a reason to fail.
// It returns nil when nothing was added.
//
// source, when non-nil, returns the archived bytes of a file so a declaration
// can include the contiguous // comment block directly above it.
func withTypeNeighbors(ctx context.Context, ckg ckgclient.Client, refs []contract.Citation, source func(string) ([]byte, error)) []contract.Citation {
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
			if n.Relation != contract.RelationUsesType || !n.Target.IsValid() {
				continue
			}
			declaration := withDocComment(n.Target, source)
			if overlapsCited(declaration, refs, out) {
				continue
			}
			out = append(out, declaration)
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

// withDocComment extends c upward over the contiguous line comments that end
// right above it. Unreadable sources leave c unchanged.
func withDocComment(c contract.Citation, source func(string) ([]byte, error)) contract.Citation {
	if source == nil || c.StartLine < 2 {
		return c
	}
	buf, err := source(c.File)
	if err != nil {
		return c
	}
	lines := strings.Split(string(buf), "\n")
	if c.StartLine > len(lines) {
		return c
	}
	first := c.StartLine
	for first > 1 && c.StartLine-(first-1) <= maxDocCommentLines &&
		strings.HasPrefix(strings.TrimSpace(lines[first-2]), "//") {
		first--
	}
	c.StartLine = first
	return c
}
