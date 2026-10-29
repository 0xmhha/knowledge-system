package stage1

import (
	"context"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/internal/system/ckvclient"
	"github.com/0xmhha/knowledge-system/internal/system/vocab"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// A caller-supplied code query replaces the prompt as the first semantic
// search: the prompt is often in another language or vocabulary than the
// code, and the rewrite is what lifts the owning function (2026-10-29 dev
// split, grpc-go: core in vector top 12 rose from 16 to 23 of 35 for bge-m3).
func TestExtract_CodeQueryIsFirstSearch(t *testing.T) {
	t.Parallel()
	ckv := &ckvclient.Fake{SearchHits: []contract.Hit{hit("a.go", 1, 0.9, contract.HitSourceCKV)}}
	e, err := New(ckv, &ckgclient.Fake{})
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithCodeQuery(context.Background(), "  `http2Client.NewStream` stats handler nil check  ")
	if _, err := e.Extract(ctx, "통계 핸들러가 없을 때 처리하지 않게", contract.IntentBugFix); err != nil {
		t.Fatal(err)
	}
	if len(ckv.Calls.SemanticSearch) == 0 {
		t.Fatal("expected a SemanticSearch call")
	}
	if got := ckv.Calls.SemanticSearch[0].Query; got != "`http2Client.NewStream` stats handler nil check" {
		t.Fatalf("first query = %q, want the trimmed code query", got)
	}
}

// Later rounds augment the code query, not the prompt, so the code
// vocabulary survives keyword augmentation.
func TestExtract_CodeQueryIsAugmentationBase(t *testing.T) {
	r, err := vocab.New(vocab.Glossary{Version: 1, Entries: []vocab.Entry{{Aliases: []string{"쿼럼"}, Canonical: "quorum", CodeKeywords: []string{"QuorumSize"}}}})
	if err != nil {
		t.Fatal(err)
	}
	ckv := &ckvclient.Fake{}
	e, err := New(ckv, &ckgclient.Fake{}, WithVocab(r))
	if err != nil {
		t.Fatal(err)
	}
	ctx := WithCodeQuery(context.Background(), "core.checkQuorum boundary")
	if _, err := e.Extract(ctx, "쿼럼 처리", contract.IntentBugFix); err != nil {
		t.Fatal(err)
	}
	if len(ckv.Calls.SemanticSearch) < 2 {
		t.Fatalf("expected an augmented round, got %d searches", len(ckv.Calls.SemanticSearch))
	}
	if q := ckv.Calls.SemanticSearch[1].Query; !strings.HasPrefix(q, "core.checkQuorum boundary") {
		t.Fatalf("round 2 query = %q, want it built on the code query", q)
	}
}

// Without a code query the prompt path is unchanged.
func TestExtract_EmptyCodeQueryKeepsPrompt(t *testing.T) {
	t.Parallel()
	ckv := &ckvclient.Fake{SearchHits: []contract.Hit{hit("a.go", 1, 0.9, contract.HitSourceCKV)}}
	e, err := New(ckv, &ckgclient.Fake{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.Extract(WithCodeQuery(context.Background(), "   "), "find the quorum check", contract.IntentBugFix); err != nil {
		t.Fatal(err)
	}
	if got := ckv.Calls.SemanticSearch[0].Query; got != "find the quorum check" {
		t.Fatalf("first query = %q, want the prompt", got)
	}
}
