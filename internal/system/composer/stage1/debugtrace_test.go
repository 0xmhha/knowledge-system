package stage1

import (
	"context"
	"testing"

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
