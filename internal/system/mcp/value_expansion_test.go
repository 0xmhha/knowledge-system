package mcp

import (
	"context"
	"encoding/json"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/pkg/graph/valueflow"
)

type expansionFake struct {
	ckgclient.Client
	input valueflow.ExpansionInput
}

func (f *expansionFake) ValueDependencyExpansion(_ context.Context, q valueflow.ExpansionInput) (valueflow.ExpansionResult, error) {
	f.input = q
	return valueflow.ExpansionResult{SchemaVersion: 1, Status: "expanded_bounded", Functions: []valueflow.ExpandedFunction{}, Hops: []valueflow.SymbolHop{}, StopReasons: []string{}, Limitations: []string{"fixture local analyses only"}}, nil
}
func TestValueExpansionPublicSchemaBackendDelegationAndCuratedSeparation(t *testing.T) {
	fixture := newFixture(t, nil)
	backend := &expansionFake{Client: fixture.deps.CKG}
	fixture.deps.CKG = backend
	s := mcpserver.NewMCPServer("fixture", "1", mcpserver.WithOutputSchemaValidation())
	if e := Register(s, fixture.deps); e != nil {
		t.Fatal(e)
	}
	request, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": ToolNameValueDependencyExpansion, "arguments": map[string]any{"function": map[string]any{"node_id": "exact-source"}, "limits": map[string]any{"max_functions": 2}}}})
	wire, _ := json.Marshal(s.HandleMessage(context.Background(), request))
	var response struct {
		Error  any `json:"error"`
		Result struct {
			IsError    bool                   `json:"isError"`
			Structured valueExpansionResponse `json:"structuredContent"`
		} `json:"result"`
	}
	if e := json.Unmarshal(wire, &response); e != nil || response.Error != nil || response.Result.IsError {
		t.Fatal(string(wire), e)
	}
	if backend.input.Function.NodeID != "exact-source" || backend.input.Limits.MaxFunctions != 2 || response.Result.Structured.CuratedFlow.Status != "separate_not_joined" || response.Result.Structured.CuratedFlow.SourceAssociation != "not_verified" {
		t.Fatal("delegation/provenance changed", response)
	}
}
func TestValueExpansionActualDuplicateNullFractionalUnknownAndLegacyBackend(t *testing.T) {
	fixture := newFixture(t, nil)
	s := mcpserver.NewMCPServer("fixture", "1")
	if e := Register(s, fixture.deps); e != nil {
		t.Fatal(e)
	}
	tool := s.GetTool(ToolNameValueDependencyExpansion)
	for _, raw := range []string{`{"function":{"node_id":"a","node_id":"b"}}`, `{"function":null}`, `{"function":{"node_id":"a"},"limits":{"max_functions":1.5}}`, `{"function":{"node_id":"a"},"unknown":true}`} {
		req := mcpgo.CallToolRequest{}
		req.Params.RawArguments = json.RawMessage(raw)
		if _, e := tool.Handler(context.Background(), req); e == nil {
			t.Fatal("runtime input bypass", raw)
		}
	}
	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]any{"function": map[string]any{"node_id": "a"}}
	reply, e := tool.Handler(context.Background(), req)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(reply.StructuredContent)
	var out valueExpansionResponse
	_ = json.Unmarshal(raw, &out)
	if out.Graph.Status != "unavailable" {
		t.Fatal("legacy invented value expansion", string(raw))
	}
}
