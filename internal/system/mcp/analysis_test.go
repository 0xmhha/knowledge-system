package mcp

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

// --- impact_analysis ---

func TestHandleImpactAnalysis_MissingSymbol_IsError(t *testing.T) {
	t.Parallel()
	f := newFixture(t, nil)
	res, err := handleImpactAnalysis(context.Background(), f.deps, callToolReq(nil))
	if err != nil {
		t.Fatalf("handleImpactAnalysis: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected IsError for missing symbol; got %+v", res)
	}
}

func TestHandleImpactAnalysis_HappyPath(t *testing.T) {
	t.Parallel()
	f := newFixture(t, func(f *fixture) {
		f.ckg.ImpactResult = contract.ImpactResult{
			Seed: "consensus.wbft.Finalize",
			Groups: []contract.ImpactGroup{
				{Category: contract.ImpactCallers, Hits: []contract.Citation{cit("eth/handler.go", 1, 50)}},
				{Category: contract.ImpactConcurrent, Hits: []contract.Citation{cit("miner/worker.go", 1, 100)}},
			},
		}
	})
	req := callToolReq(map[string]any{
		"symbol":    "consensus.wbft.Finalize",
		"depth":     float64(2),
		"max_total": float64(50),
	})
	res, err := handleImpactAnalysis(context.Background(), f.deps, req)
	if err != nil {
		t.Fatalf("handleImpactAnalysis: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected IsError: %s", resultText(res))
	}
	var out impactAnalysisResponse
	if err := decodeStructured(res, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Seed != "consensus.wbft.Finalize" {
		t.Errorf("Seed = %q", out.Seed)
	}
	if len(out.Result.Groups) != 2 {
		t.Errorf("Groups = %d, want 2", len(out.Result.Groups))
	}
	// Verify depth + max_total propagated.
	if len(f.ckg.Calls.ImpactOfChange) != 1 {
		t.Fatalf("ImpactOfChange calls = %d", len(f.ckg.Calls.ImpactOfChange))
	}
	got := f.ckg.Calls.ImpactOfChange[0]
	if got.Opts.Depth != 2 || got.Opts.MaxTotal != 50 {
		t.Errorf("Opts = %+v", got.Opts)
	}
}

// --- change_history ---

func TestHandleChangeHistory_RequiresAtLeastOneInput(t *testing.T) {
	t.Parallel()
	f := newFixture(t, nil)
	res, err := handleChangeHistory(context.Background(), f.deps, callToolReq(nil))
	if err != nil {
		t.Fatalf("handleChangeHistory: %v", err)
	}
	if res == nil || !res.IsError {
		t.Fatalf("expected IsError when neither intent nor symbol supplied; got %+v", res)
	}
}

func TestHandleChangeHistory_IntentOnly(t *testing.T) {
	t.Parallel()
	mergedAt := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	f := newFixture(t, func(f *fixture) {
		f.ckg.EvidenceResult = contract.ChangeHistoryResult{
			Hunks: []contract.HunkEvidence{
				{File: "consensus/wbft/finalize.go", StartLine: 100, EndLine: 120, Patch: "+ quorum check", Score: 0.9},
			},
			PRs: []contract.PRRef{
				{Number: 42, Title: "fix quorum off-by-one", MergedAt: mergedAt},
			},
		}
	})
	req := callToolReq(map[string]any{
		"intent": "quorum off by one in finalize",
		"k":      float64(5),
	})
	res, err := handleChangeHistory(context.Background(), f.deps, req)
	if err != nil {
		t.Fatalf("handleChangeHistory: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected IsError: %s", resultText(res))
	}
	var out changeHistoryResponse
	if err := decodeStructured(res, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Hunks) != 1 {
		t.Errorf("Hunks = %d, want 1", len(out.Hunks))
	}
	if len(out.PRs) != 1 || out.PRs[0].Number != 42 {
		t.Errorf("PRs = %+v", out.PRs)
	}
	if len(f.ckg.Calls.EvidenceForIntent) != 1 || f.ckg.Calls.EvidenceForIntent[0].Opts.K != 5 {
		t.Errorf("EvidenceForIntent Opts = %+v", f.ckg.Calls.EvidenceForIntent)
	}
	// Symbol-only flow should not have fired.
	if len(f.ckg.Calls.GetNodePRs) != 0 {
		t.Errorf("GetNodePRs unexpectedly called: %+v", f.ckg.Calls.GetNodePRs)
	}
}

func TestHandleChangeHistory_SymbolOnly(t *testing.T) {
	t.Parallel()
	f := newFixture(t, func(f *fixture) {
		// The seed has to resolve: change_history refuses an unresolved one
		// rather than reporting "no PRs touched this".
		f.ckg.SymbolCitations = []contract.Citation{cit("ncp/validator.go", 10, 40)}
		f.ckg.PRRefs = []contract.PRRef{
			{Number: 7, Title: "rename Validator to NCP"},
			{Number: 5, Title: "initial NCP scaffold"},
		}
	})
	req := callToolReq(map[string]any{
		"symbol":    "ncp.Validator",
		"max_count": float64(10),
	})
	res, err := handleChangeHistory(context.Background(), f.deps, req)
	if err != nil {
		t.Fatalf("handleChangeHistory: %v", err)
	}
	if res.IsError {
		t.Fatalf("unexpected IsError: %s", resultText(res))
	}
	var out changeHistoryResponse
	if err := decodeStructured(res, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.PRs) != 2 {
		t.Errorf("PRs = %d, want 2", len(out.PRs))
	}
	// Hunks should be empty (no intent given).
	if len(out.Hunks) != 0 {
		t.Errorf("Hunks should be empty, got %+v", out.Hunks)
	}
	if len(f.ckg.Calls.GetNodePRs) != 1 || f.ckg.Calls.GetNodePRs[0].Opts.MaxCount != 10 {
		t.Errorf("GetNodePRs Opts = %+v", f.ckg.Calls.GetNodePRs)
	}
}

func TestHandleChangeHistory_BothInputs_MergesResults(t *testing.T) {
	t.Parallel()
	f := newFixture(t, func(f *fixture) {
		f.ckg.EvidenceResult = contract.ChangeHistoryResult{
			Hunks: []contract.HunkEvidence{{File: "a.go", StartLine: 1, EndLine: 5, Score: 1.0}},
			PRs:   []contract.PRRef{{Number: 1, Title: "from-evidence"}},
		}
		f.ckg.PRRefs = []contract.PRRef{{Number: 2, Title: "from-getnodeprs"}}
		f.ckg.SymbolCitations = []contract.Citation{cit("pkg/foo.go", 1, 9)}
	})
	req := callToolReq(map[string]any{
		"intent": "anything",
		"symbol": "pkg.Foo",
	})
	res, err := handleChangeHistory(context.Background(), f.deps, req)
	if err != nil {
		t.Fatalf("handleChangeHistory: %v", err)
	}
	var out changeHistoryResponse
	if err := decodeStructured(res, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Hunks) != 1 {
		t.Errorf("Hunks = %d, want 1", len(out.Hunks))
	}
	if len(out.PRs) != 2 {
		t.Errorf("PRs = %d, want 2 (1 from evidence + 1 from GetNodePRs)", len(out.PRs))
	}
}

// TestHandleChangeHistory_UnresolvedSeedIsRefused covers the seed half of
// change_history. GetNodePRs is an exact-match lookup, so an unresolved symbol
// used to come back as "no PRs touched this" — the same shape as a symbol with
// a genuinely empty history, and the same shape a misspelling produces.
func TestHandleChangeHistory_UnresolvedSeedIsRefused(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		cits    []contract.Citation
		wantMsg string
	}{
		{"unknown symbol", nil, "matches no indexed symbol"},
		{
			"ambiguous symbol",
			[]contract.Citation{cit("a/one.go", 1, 4), cit("b/two.go", 7, 9)},
			"is ambiguous across 2 definitions",
		},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			f := newFixture(t, func(f *fixture) {
				f.ckg.SymbolCitations = tc.cits
				f.ckg.PRRefs = []contract.PRRef{{Number: 1, Title: "must not be reported"}}
			})
			req := callToolReq(map[string]any{"symbol": "pkg.Whatever"})
			res, err := handleChangeHistory(context.Background(), f.deps, req)
			if err != nil {
				t.Fatalf("handleChangeHistory: %v", err)
			}
			if !res.IsError {
				t.Fatalf("want a refusal, got a result: %s", resultText(res))
			}
			if got := resultText(res); !strings.Contains(got, tc.wantMsg) {
				t.Errorf("message %q does not say %q", got, tc.wantMsg)
			}
			if len(f.ckg.Calls.GetNodePRs) != 0 {
				t.Errorf("GetNodePRs ran with an unresolved seed: %+v", f.ckg.Calls.GetNodePRs)
			}
		})
	}
}
