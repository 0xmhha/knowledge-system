package budget

import (
	"context"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/composer/stage2"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

func TestDocumentParentContextHasSeparateBoundedCitations(t *testing.T) {
	child := contract.Citation{File: "docs/guide.md", StartLine: 20, EndLine: 22, CommitHash: "abc"}
	parent := contract.Citation{File: child.File, StartLine: 1, EndLine: 100, CommitHash: child.CommitHash}
	seeds := []stage2.ScoredCitation{{Citation: child, ParentCitation: &parent, Score: 1, ChunkKind: "doc"}}
	candidates := mergeCandidates(seeds, nil)
	if len(candidates) != 3 {
		t.Fatalf("candidates = %d, want child and two contexts", len(candidates))
	}
	before := contract.Citation{File: child.File, StartLine: 14, EndLine: 19, CommitHash: child.CommitHash}
	after := contract.Citation{File: child.File, StartLine: 23, EndLine: 28, CommitHash: child.CommitHash}
	for _, want := range []contract.Citation{child, before, after} {
		found := false
		for _, got := range candidates {
			if got.Citation == want {
				found = true
			}
		}
		if !found {
			t.Errorf("missing citation %s", want)
		}
	}
	fetcher := &FakeFetcher{Bodies: map[string]string{
		child.Key():  "matching passage",
		before.Key(): "preceding section context",
		after.Key():  "following section context",
	}}
	allocator, err := New(fetcher, WithConfig(Config{MaxTokens: 1000, MaxCitations: 3}))
	if err != nil {
		t.Fatal(err)
	}
	out, err := allocator.Allocate(context.Background(), seeds, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Selected) != 3 {
		t.Fatalf("selected = %d, want 3: %+v", len(out.Selected), out.Selected)
	}
	if out.Selected[0].Citation != child {
		t.Fatalf("first citation = %s, want child %s", out.Selected[0].Citation, child)
	}
}

func TestDocumentParentContextRequiresSelectedChild(t *testing.T) {
	child := contract.Citation{File: "docs/guide.md", StartLine: 20, EndLine: 22}
	parent := contract.Citation{File: child.File, StartLine: 1, EndLine: 100}
	seeds := []stage2.ScoredCitation{{Citation: child, ParentCitation: &parent, Score: 1, ChunkKind: "doc"}}
	fetcher := &FakeFetcher{Bodies: map[string]string{
		"docs/guide.md:14-19": "context",
		"docs/guide.md:23-28": "context",
	}}
	allocator, err := New(fetcher, WithConfig(Config{MaxTokens: 1000, MaxCitations: 3}))
	if err != nil {
		t.Fatal(err)
	}
	out, err := allocator.Allocate(context.Background(), seeds, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Selected) != 0 || len(fetcher.Calls) != 1 {
		t.Fatalf("orphan context selected or fetched: selected=%d calls=%d", len(out.Selected), len(fetcher.Calls))
	}
}
