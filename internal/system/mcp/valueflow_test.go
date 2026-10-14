package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/pkg/graph/valueflow"
)

type valueAnalysisFake struct {
	ckgclient.Client
	result     valueflow.QueryResult
	ruleInput  valueflow.RuleQueryInput
	valueInput valueflow.QueryInput
}

func (f *valueAnalysisFake) RuleCandidates(_ context.Context, q valueflow.RuleQueryInput) (valueflow.QueryResult, error) {
	f.ruleInput = q
	if q.Function.ExpectedSourceSHA256 != "" {
		return valueflow.QueryResult{SchemaVersion: 1, Status: "source_mismatch", Reason: "fixture backend rejected pin"}, nil
	}
	return f.result, nil
}
func (f *valueAnalysisFake) ValueDependencies(_ context.Context, q valueflow.QueryInput) (valueflow.QueryResult, error) {
	f.valueInput = q
	out := f.result
	analysis := *out.Analysis
	analysis.Rules = nil
	out.Analysis = &analysis
	return out, nil
}
func TestComposerValueRuleFacadePreservesBackendResultAndInput(t *testing.T) {
	source := valueflow.Source{NodeID: "1111111111111111", File: "p.go", Language: "go", StartByte: 200, StartLine: 12, Commit: strings.Repeat("a", 40), SnapshotSHA256: strings.Repeat("b", 64)}
	analysis, e := valueflow.AnalyzeRules(context.Background(), source, []byte("func F(cmd string){exec.Command(cmd)}"), valueflow.Options{}, valueflow.RulePackID, valueflow.RuleOptions{})
	if e != nil {
		t.Fatal(e)
	}
	nodeID := source.NodeID
	real := &valueAnalysisFake{result: valueflow.QueryResult{SchemaVersion: 1, Status: analysis.Status, SnapshotSHA256: source.SnapshotSHA256, Analysis: &analysis}}
	f := newFixture(t, nil)
	f.deps.CKG = real
	s := mcpserver.NewMCPServer("fixture", "1", mcpserver.WithOutputSchemaValidation())
	if e = Register(s, f.deps); e != nil {
		t.Fatal(e)
	}
	direct, e := real.RuleCandidates(context.Background(), valueflow.RuleQueryInput{RulePack: valueflow.RulePackID, Function: valueflow.QueryInput{NodeID: nodeID}})
	if e != nil {
		t.Fatal(e)
	}
	for _, tc := range []struct {
		name   string
		args   map[string]any
		status string
	}{
		{ToolNameValueDependencies, map[string]any{"node_id": nodeID}, "partial"},
		{ToolNameRuleCandidates, map[string]any{"rule_pack": valueflow.RulePackID, "function": map[string]any{"node_id": nodeID}}, "partial"},
		{ToolNameRuleCandidates, map[string]any{"rule_pack": valueflow.RulePackID, "function": map[string]any{"node_id": nodeID, "expected_source_sha256": strings.Repeat("b", 64)}}, "source_mismatch"},
	} {
		req, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": tc.name, "arguments": tc.args}})
		raw, _ := json.Marshal(s.HandleMessage(context.Background(), req))
		var wire struct {
			Error  any `json:"error"`
			Result struct {
				IsError    bool                  `json:"isError"`
				Structured valueflow.QueryResult `json:"structuredContent"`
			} `json:"result"`
		}
		if e = json.Unmarshal(raw, &wire); e != nil || wire.Error != nil || wire.Result.IsError || wire.Result.Structured.Status != tc.status {
			t.Fatal(string(raw), e)
		}
		out := wire.Result.Structured
		if tc.status == "source_mismatch" {
			if out.Analysis != nil {
				t.Fatal("stale result published")
			}
			continue
		}
		if real.ruleInput.Function.NodeID != nodeID || real.ruleInput.RulePack != valueflow.RulePackID {
			t.Fatal("facade changed backend input")
		}
		if out.Analysis.Source != direct.Analysis.Source {
			t.Fatal("composer changed source pins")
		}
		if tc.name == ToolNameValueDependencies && out.Analysis.Rules != nil {
			t.Fatal("implicit pack")
		}
		if tc.name == ToolNameRuleCandidates {
			want, _ := json.Marshal(direct.Analysis.Rules)
			got, _ := json.Marshal(out.Analysis.Rules)
			if string(want) != string(got) {
				t.Fatal("facade changed engine output", string(got))
			}
		}
	}
}
func TestComposerValueRuleFacadeUnsupportedBackendAndRawInputValidation(t *testing.T) {
	f := newFixture(t, nil)
	s := mcpserver.NewMCPServer("fixture", "1")
	if e := Register(s, f.deps); e != nil {
		t.Fatal(e)
	}
	tool := s.GetTool(ToolNameRuleCandidates)
	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]any{"rule_pack": valueflow.RulePackID, "function": map[string]any{"node_id": "1111111111111111"}}
	reply, e := tool.Handler(context.Background(), req)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(reply.StructuredContent)
	var out valueflow.QueryResult
	_ = json.Unmarshal(raw, &out)
	if out.Status != "unavailable" || out.Analysis != nil {
		t.Fatal("dummy backend invented analysis", string(raw))
	}
	for _, raw := range []string{`{"function":{"node_id":"1111111111111111"}}`, `{"rule_pack":"go-local-review@1.0.0","function":{"node_id":"1111111111111111","node_id":"other"}}`, `{"rule_pack":"go-local-review@1.0.0","function":{"node_id":"1111111111111111"},"max_flow_steps":0}`} {
		req.Params.RawArguments = json.RawMessage(raw)
		if _, e = tool.Handler(context.Background(), req); e == nil {
			t.Fatal("facade bypassed strict decoder", raw)
		}
	}
}
