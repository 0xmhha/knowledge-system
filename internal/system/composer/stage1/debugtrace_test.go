package stage1

import (
	"context"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/internal/system/ckvclient"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// The trace separates the user's own words from names mined out of ckv
// hits; the second kind only repeats what the semantic search ranked.
func TestExtractKeywordCandidates_TagsOrigin(t *testing.T) {
	t.Parallel()
	hits := []contract.Hit{
		{Citation: contract.Citation{File: "a/balancer.go", StartLine: 1, EndLine: 2}, Symbol: "lbBalancer.refresh"},
		{Citation: contract.Citation{File: "a/picker_test.go", StartLine: 1, EndLine: 2}},
	}
	got := map[string]string{}
	for _, c := range extractKeywordCandidates("quorum size validation", hits, contract.IntentUnknown) {
		got[c.Keyword] = c.Origin
	}
	want := map[string]string{
		"QuorumSize":         originCompound,
		"Validate":           originMorph,
		"quorum":             originPrompt,
		"lbBalancer.refresh": originHitSymbol,
		"picker":             originHitFile,
	}
	for kw, origin := range want {
		if got[kw] != origin {
			t.Errorf("origin[%q] = %q, want %q (all: %v)", kw, got[kw], origin, got)
		}
	}
}

func TestRerankScored_ReportsDropReasons(t *testing.T) {
	t.Parallel()
	ckg := &rerankFake{hitsByKW: map[string][]contract.Hit{
		"CksMCP": {hitScore(1.0)},
		"cksMCP": {hitScore(1.0)},
		"absent": {hitScore(0)},
	}}
	e := mustExtractor(t, ckg)
	kept, dropped := e.rerankScored(context.Background(), []string{"CksMCP", "cksMCP", "absent"})
	if len(kept) != 1 || kept[0].Keyword != "CksMCP" {
		t.Fatalf("kept = %+v, want only CksMCP", kept)
	}
	reasons := map[string]string{}
	for _, d := range dropped {
		reasons[d.Keyword] = d.Reason
	}
	if reasons["cksMCP"] != dropCaseFold || reasons["absent"] != dropZeroScore {
		t.Errorf("drop reasons = %v", reasons)
	}
}

// Keywords mined from ckv hits only repeat what the semantic search ranked;
// by default they stay out of the Stage 2 hand-off and only the prompt's
// own keywords remain. KeepHitKeywords restores the old hand-off.
func TestExtract_DropsHitKeywordsFromStage2HandOff(t *testing.T) {
	t.Parallel()
	hits := []contract.Hit{
		{Citation: contract.Citation{File: "a/balancer.go", StartLine: 1, EndLine: 9}, Symbol: "lbBalancer.refresh", Score: 0.9, Source: contract.HitSourceCKV},
		{Citation: contract.Citation{File: "a/picker_test.go", StartLine: 1, EndLine: 9}, Score: 0.8, Source: contract.HitSourceCKV},
	}
	run := func(keep bool) map[string]bool {
		cfg := DefaultConfig()
		cfg.MaxRounds, cfg.KnowledgeK, cfg.KeepHitKeywords = 1, 0, keep
		e, err := New(&ckvclient.Fake{SearchHits: hits}, &ckgclient.Fake{BM25Hits: []contract.Hit{hitScore(1.0)}}, WithConfig(cfg))
		if err != nil {
			t.Fatal(err)
		}
		out, err := e.Extract(context.Background(), "fix the Login handler", contract.IntentBugFix)
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]bool{}
		for _, k := range out.Keywords {
			got[k] = true
		}
		return got
	}
	def := run(false)
	if !def["Login"] || def["lbBalancer.refresh"] || def["picker"] {
		t.Errorf("default keywords = %v, want prompt keywords only", def)
	}
	kept := run(true)
	if !kept["Login"] || !kept["lbBalancer.refresh"] || !kept["picker"] {
		t.Errorf("KeepHitKeywords keywords = %v, want prompt and hit keywords", kept)
	}
}
