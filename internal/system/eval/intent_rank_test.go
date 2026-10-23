package eval

import (
	"context"
	"strings"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

func TestFirstHitRank(t *testing.T) {
	t.Parallel()
	want := []contract.Citation{cit("b.go", 10, 20)}
	tests := []struct {
		name     string
		expected []contract.Citation
		actual   []contract.Citation
		rank     int
	}{
		{"first", want, []contract.Citation{cit("b.go", 15, 16), cit("a.go", 1, 2)}, 1},
		{"third", want, []contract.Citation{cit("a.go", 1, 2), cit("c.go", 1, 2), cit("b.go", 20, 30)}, 3},
		{"duplicates do not shift rank", want, []contract.Citation{cit("a.go", 1, 2), cit("a.go", 1, 2), cit("b.go", 10, 10)}, 2},
		{"miss", want, []contract.Citation{cit("a.go", 1, 2)}, 0},
		{"empty actual", want, nil, 0},
		{"nothing expected", nil, []contract.Citation{cit("a.go", 1, 2)}, 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := firstHitRank(tc.expected, tc.actual, MatchOverlap); got != tc.rank {
				t.Errorf("firstHitRank = %d, want %d", got, tc.rank)
			}
		})
	}
}

func TestMedianRank(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ranks []int
		want  int
	}{
		{nil, 0},
		{[]int{2}, 2},
		{[]int{0, 1, 3}, 3},
		{[]int{0, 0, 1}, 0},
		{[]int{4, 1}, 1},
	}
	for _, tc := range tests {
		if got := medianRank(tc.ranks); got != tc.want {
			t.Errorf("medianRank(%v) = %d, want %d", tc.ranks, got, tc.want)
		}
	}
}

func TestParseScenario_ExpectedIntent(t *testing.T) {
	t.Parallel()
	base := "version: 1\nname: n\nprompt: p\n"
	tests := []struct {
		name    string
		extra   string
		wantErr string
	}{
		{"expected only", "expected_intent: bug_fix\n", ""},
		{"unknown value", "expected_intent: nope\n", "expected_intent"},
		{"both set", "intent: bug_fix\nexpected_intent: bug_fix\n", "mutually exclusive"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s, err := ParseScenario([]byte(base + tc.extra))
			if tc.wantErr == "" {
				if err != nil || s.ExpectedIntent != contract.IntentBugFix {
					t.Fatalf("ParseScenario = %+v, %v", s, err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

// expected_intent must reach the report but never the request, or the run
// scores a caller-chosen intent instead of the classifier.
func TestRunner_ExpectedIntentIsScoredNotSent(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		classified contract.Intent
		match      bool
	}{
		{"match", contract.IntentBugFix, true},
		{"mismatch", contract.IntentRefactor, false},
		{"unknown", contract.IntentUnknown, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			pack := contract.EvidencePack{Intent: tc.classified, Query: "q",
				Citations: []contract.Citation{cit("x.go", 1, 2), cit("y.go", 5, 9)},
				Metadata:  contract.PackMetadata{UsedTokens: 321}}
			m := &mockMCPClient{callOut: map[string]*mcpgo.CallToolResult{toolGetForTask: packResult(pack)}}
			r := &Runner{client: m}
			s := &Scenario{Version: 1, Name: "s", Prompt: "q", MatchMode: MatchOverlap, Runs: 2,
				ExpectedIntent: contract.IntentBugFix, ExpectedCitations: []contract.Citation{cit("y.go", 6, 7)}}
			res, err := r.Execute(context.Background(), s)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			for _, c := range m.calls {
				if _, sent := c.args["intent"]; sent || len(c.args) != 1 {
					t.Fatalf("request args = %v, want prompt only", c.args)
				}
			}
			if res.IntentMatch == nil || *res.IntentMatch != tc.match || len(res.ClassifiedIntents) != 2 {
				t.Errorf("intent result = %v %v", res.IntentMatch, res.ClassifiedIntents)
			}
			if res.Metrics.FirstHitRank != 2 || res.Metrics.UsedTokens != 321 {
				t.Errorf("rank/tokens = %d/%d, want 2/321", res.Metrics.FirstHitRank, res.Metrics.UsedTokens)
			}
		})
	}
}

func TestRunner_ExpectedIntentErrorIsNotAMatch(t *testing.T) {
	t.Parallel()
	m := &mockMCPClient{callOut: map[string]*mcpgo.CallToolResult{toolGetForTask: errorResult("boom")}}
	r := &Runner{client: m}
	s := &Scenario{Version: 1, Name: "s", Prompt: "q", MatchMode: MatchOverlap, Runs: 1, ExpectedIntent: contract.IntentBugFix}
	res, err := r.Execute(context.Background(), s)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.IntentMatch == nil || *res.IntentMatch {
		t.Errorf("IntentMatch = %v, want false for an errored run", res.IntentMatch)
	}
}

func TestSummarizeRanks(t *testing.T) {
	t.Parallel()
	results := []ScenarioResult{
		{RetrievalState: "pass", Metrics: Metrics{FirstHitRank: 1}},
		{RetrievalState: "miss", Metrics: Metrics{FirstHitRank: 3}},
		{RetrievalState: "miss", Metrics: Metrics{FirstHitRank: 0}},
		{RetrievalState: "error", Error: "run 1: boom", Metrics: Metrics{FirstHitRank: 1}},
		{RetrievalState: "not_evaluated"},
	}
	got := SummarizeRanks(results)
	if got == nil || got.Scenarios != 4 {
		t.Fatalf("summary = %+v, want 4 scenarios (errors stay in the denominator)", got)
	}
	if !approxEq(got.HitAt1, 0.25) || !approxEq(got.HitAt3, 0.5) || !approxEq(got.MRR, (1+1.0/3)/4) {
		t.Errorf("summary = %+v", got)
	}
	if SummarizeRanks([]ScenarioResult{{RetrievalState: "not_evaluated"}}) != nil {
		t.Error("summary without expected citations should be nil")
	}
}

func TestSummarizeIntents(t *testing.T) {
	t.Parallel()
	yes, no := true, false
	results := []ScenarioResult{
		{ExpectedIntent: "bug_fix", ClassifiedIntents: []string{"bug_fix"}, IntentMatch: &yes},
		{ExpectedIntent: "bug_fix", ClassifiedIntents: []string{""}, IntentMatch: &no},
		{ExpectedIntent: "refactor", IntentMatch: &no},
		{Name: "no expectation"},
	}
	got := SummarizeIntents(results)
	if got == nil || got.Scenarios != 3 || got.Correct != 1 || !approxEq(got.Accuracy, 1.0/3) {
		t.Fatalf("summary = %+v", got)
	}
	if got.Confusion["bug_fix"]["bug_fix"] != 1 || got.Confusion["bug_fix"][classifiedUnknown] != 1 || got.Confusion["refactor"][classifiedError] != 1 {
		t.Errorf("confusion = %v", got.Confusion)
	}
}
