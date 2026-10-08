package stage1

import (
	"context"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/internal/system/ckvclient"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

func TestExtract_KnowledgePassIssuesKindScopedSearch(t *testing.T) {
	t.Parallel()
	// KnowledgeK>0 adds exactly one extra ckv search after the recall
	// rounds, scoped to knowledge chunk kinds. The pass must not affect
	// round accounting and its failure must not fail Extract.
	ckv := &ckvclient.Fake{
		SearchHits: []contract.Hit{hit("handlers.go", 1, 0.9, contract.HitSourceCKV)},
	}
	ckg := &ckgclient.Fake{
		BM25Hits: []contract.Hit{hit("handlers.go", 1, 10.0, contract.HitSourceCKG)},
	}
	e, _ := New(ckv, ckg, WithConfig(Config{
		MaxRounds:     1,
		InitialK:      DefaultInitialK,
		RerankPerKW:   DefaultRerankPerKW,
		MinConfidence: DefaultMinConfidence,
		MaxKeywords:   DefaultMaxKeywords,
		AugmentTopN:   DefaultAugmentTopN,
		KnowledgeK:    2,
	}))

	out, err := e.Extract(context.Background(), "fix the Login handler", contract.IntentBugFix)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if out.Rounds != 1 {
		t.Errorf("Rounds = %d, want 1 (knowledge pass is not a round)", out.Rounds)
	}
	calls := ckv.Calls.SemanticSearch
	if len(calls) != 2 {
		t.Fatalf("SemanticSearch calls = %d, want 2 (1 round + 1 knowledge pass)", len(calls))
	}
	kp := calls[len(calls)-1]
	if kp.Opts.K != 2 {
		t.Errorf("knowledge pass K = %d, want 2", kp.Opts.K)
	}
	kinds := kp.Opts.Filter.ChunkKinds
	if len(kinds) != 2 || kinds[0] != "invariant" || kinds[1] != "convention" {
		t.Errorf("knowledge pass ChunkKinds = %v, want [invariant convention]", kinds)
	}
	// Fake returns the same citation for both calls; mergeHits dedupes.
	if len(out.Hits) != 1 {
		t.Errorf("Hits = %d, want 1 (deduped)", len(out.Hits))
	}
	if out.KnowledgeHits != 1 {
		t.Errorf("KnowledgeHits = %d, want 1", out.KnowledgeHits)
	}
}

func TestExtract_KnowledgePassDisabledByZeroK(t *testing.T) {
	t.Parallel()
	ckv := &ckvclient.Fake{
		SearchHits: []contract.Hit{hit("handlers.go", 1, 0.9, contract.HitSourceCKV)},
	}
	ckg := &ckgclient.Fake{
		BM25Hits: []contract.Hit{hit("handlers.go", 1, 10.0, contract.HitSourceCKG)},
	}
	e, _ := New(ckv, ckg, WithConfig(Config{
		MaxRounds:     1,
		InitialK:      DefaultInitialK,
		RerankPerKW:   DefaultRerankPerKW,
		MinConfidence: DefaultMinConfidence,
		MaxKeywords:   DefaultMaxKeywords,
		AugmentTopN:   DefaultAugmentTopN,
	}))
	if _, err := e.Extract(context.Background(), "fix the Login handler", contract.IntentBugFix); err != nil {
		t.Fatalf("Extract: %v", err)
	}
	if n := len(ckv.Calls.SemanticSearch); n != 1 {
		t.Errorf("SemanticSearch calls = %d, want 1 (KnowledgeK=0 disables the pass)", n)
	}
}

func TestDefaultConfig_KnowledgeK(t *testing.T) {
	t.Parallel()
	if DefaultConfig().KnowledgeK != 6 {
		t.Errorf("DefaultConfig.KnowledgeK = %d, want 6", DefaultConfig().KnowledgeK)
	}
}

// A broad CKV recall may return knowledge chunks before the separate kind-
// scoped pass. Those hits remain evidence, but their pseudo paths and policy
// titles are not code symbols to send to CKG's raw FTS or FindSymbol pipeline.
func TestExtract_KnowledgeRecallDoesNotGenerateCodeKeywords(t *testing.T) {
	t.Parallel()
	ckv := &ckvclient.Fake{SearchHits: []contract.Hit{
		{Citation: contract.Citation{File: "main.go", StartLine: 3, EndLine: 3, CommitHash: "abc"}, Symbol: "Alpha", ChunkKind: "symbol", Source: contract.HitSourceCKV},
		{Citation: contract.Citation{File: "/<convention>", StartLine: 1, EndLine: 1, CommitHash: "abc"}, ChunkKind: "convention", Source: contract.HitSourceCKV},
		{Citation: contract.Citation{File: "/<invariant>", StartLine: 1, EndLine: 1, CommitHash: "abc"}, Symbol: "PolicyAnchor", ChunkKind: "invariant", Source: contract.HitSourceCKV},
	}}
	ckg := &ckgclient.Fake{}
	cfg := DefaultConfig()
	cfg.MaxRounds = 2
	e, err := New(ckv, ckg, WithConfig(cfg))
	if err != nil {
		t.Fatal(err)
	}
	out, err := e.Extract(context.Background(), "Alpha implementation", contract.IntentBugFix)
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range ckg.Calls.BM25Search {
		switch call.Query {
		case "<convention>", "<invariant>", "PolicyAnchor":
			t.Errorf("knowledge hit became a code keyword: %q", call.Query)
		}
	}
	if len(out.Hits) != 3 {
		t.Fatalf("knowledge evidence lost: got %d hits", len(out.Hits))
	}
	if len(ckv.Calls.SemanticSearch) != 3 || out.Rounds != 2 {
		t.Fatalf("recall rounds or separate knowledge pass changed: rounds=%d calls=%d", out.Rounds, len(ckv.Calls.SemanticSearch))
	}
	for _, call := range ckv.Calls.SemanticSearch[:2] {
		if call.Opts.K != DefaultInitialK || len(call.Opts.Filter.ChunkKinds) != 0 {
			t.Fatalf("raw recall K/filter changed: %+v", call.Opts)
		}
	}
	if ckv.Calls.SemanticSearch[2].Opts.K != cfg.KnowledgeK {
		t.Fatal("knowledge pass K changed")
	}
}
