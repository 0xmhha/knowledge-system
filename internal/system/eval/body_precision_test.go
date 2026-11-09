package eval

import (
	"context"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"

	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

func TestRunner_Execute_BodyPrecision(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name        string
		bodies      []contract.Body
		wantAnswers int
		wantPrec    float64
		wantOther   int
	}{
		{name: "no bodies", wantPrec: 0},
		{
			name: "one answer among three",
			bodies: []contract.Body{
				{Citation: cit("noise.go", 1, 9), TokenEstimate: 100},
				{Citation: cit("login.go", 12, 20), TokenEstimate: 50},
				{Citation: cit("other.go", 3, 4), TokenEstimate: 30},
			},
			wantAnswers: 1, wantPrec: 1.0 / 3.0, wantOther: 130,
		},
		{
			name: "all answers",
			bodies: []contract.Body{
				{Citation: cit("login.go", 10, 30), TokenEstimate: 70},
			},
			wantAnswers: 1, wantPrec: 1, wantOther: 0,
		},
		{
			name: "same file outside the expected lines is not an answer",
			bodies: []contract.Body{
				{Citation: cit("login.go", 40, 50), TokenEstimate: 20},
			},
			wantAnswers: 0, wantPrec: 0, wantOther: 20,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			pack := contract.EvidencePack{
				Query:     "find login",
				Citations: []contract.Citation{cit("login.go", 10, 30)},
				Bodies:    tc.bodies,
			}
			m := &mockMCPClient{callOut: map[string]*mcpgo.CallToolResult{toolGetForTask: packResult(pack)}}
			r := &Runner{client: m}
			s := &Scenario{
				Version: 1, Name: "body", Prompt: "find login", MatchMode: MatchOverlap, Runs: 1,
				ExpectedCitations: []contract.Citation{cit("login.go", 10, 30)},
			}
			res, err := r.Execute(context.Background(), s)
			if err != nil {
				t.Fatalf("Execute: %v", err)
			}
			got := res.Metrics
			if got.AnswerBodyCount != tc.wantAnswers || !approxEq(got.BodyPrecision, tc.wantPrec) || got.OtherBodyTokens != tc.wantOther {
				t.Fatalf("answers %d precision %.4f other %d, want %d %.4f %d",
					got.AnswerBodyCount, got.BodyPrecision, got.OtherBodyTokens, tc.wantAnswers, tc.wantPrec, tc.wantOther)
			}
		})
	}
}

func TestRunner_Execute_BodyPrecision_CommitFilter(t *testing.T) {
	t.Parallel()
	// Without a commit repository a body at another commit cannot be shown
	// unchanged, so it is not counted as an answer.
	other := cit("login.go", 10, 30)
	other.CommitHash = "1111111111111111111111111111111111111111"
	pack := contract.EvidencePack{
		Query:  "find login",
		Bodies: []contract.Body{{Citation: other, TokenEstimate: 40}},
	}
	m := &mockMCPClient{callOut: map[string]*mcpgo.CallToolResult{toolGetForTask: packResult(pack)}}
	r := &Runner{client: m}
	s := &Scenario{
		Version: 1, Name: "commit", Prompt: "find login", MatchMode: MatchOverlap, Runs: 1,
		ExpectedCommit:    "2222222222222222222222222222222222222222",
		ExpectedCitations: []contract.Citation{cit("login.go", 10, 30)},
	}
	res, err := r.Execute(context.Background(), s)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if res.Metrics.AnswerBodyCount != 0 || res.Metrics.OtherBodyTokens != 40 {
		t.Fatalf("answers %d other %d, want 0 40", res.Metrics.AnswerBodyCount, res.Metrics.OtherBodyTokens)
	}
}
