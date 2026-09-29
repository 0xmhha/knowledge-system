package query

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/vector/types"
)

func TestResponseHitCarriesDocumentParentBounds(t *testing.T) {
	chunk := types.Chunk{
		ID: "child", File: "docs/guide.md", StartLine: 20, EndLine: 22,
		CommitHash: "abc", ParentID: "parent", ParentStartLine: 1, ParentEndLine: 100,
		HeadingPath: []string{"System", "Search"},
	}
	hit := toResponseHit(types.Hit{Chunk: chunk}, "matching passage")
	if hit.ParentCitation == nil || hit.ParentCitation.StartLine != 1 || hit.ParentCitation.EndLine != 100 || hit.ParentCitation.CommitHash != "abc" {
		t.Fatalf("parent citation lost: %+v", hit.ParentCitation)
	}
	if hit.HeadingPath != "System / Search" {
		t.Fatalf("heading path = %q", hit.HeadingPath)
	}
	chunk.ParentID = ""
	hit = toResponseHit(types.Hit{Chunk: chunk}, "matching passage")
	if hit.ParentCitation != nil || hit.HeadingPath != "" {
		t.Fatalf("unsplit chunk exposed parent: %+v", hit)
	}
}
