package mcphandlers

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	mcp "github.com/mark3labs/mcp-go/mcp"
	server "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
	"github.com/0xmhha/knowledge-system/pkg/graph/valueflow"
)

func TestRuleCandidatesPublicSchemaAndManualAnnotationCoexistence(t *testing.T) {
	store := valueStore(t)
	blob := []byte("func F(cmd string){exec.Command(cmd)}")
	nodes, e := store.NodesByIDs([]string{"1111111111111111"})
	if e != nil {
		t.Fatal(e)
	}
	node := nodes[0]
	node.EndByte = node.StartByte + len(blob)
	if e = store.InsertNodes([]types.Node{node}); e != nil {
		t.Fatal(e)
	}
	if e = store.InsertBlobs(map[string][]byte{node.ID: blob}); e != nil {
		t.Fatal(e)
	}
	manual := types.Node{ID: "security:manual-review", Type: types.NodeSecurityPattern, Name: "Manual review", QualifiedName: "manual-review", Language: "security", FilePath: "review.yaml", StartLine: 1, Confidence: types.ConfExtracted}
	if e = store.InsertNodes([]types.Node{manual}); e != nil {
		t.Fatal(e)
	}
	if e = store.InsertEdges([]types.Edge{{Src: node.ID, Dst: manual.ID, Type: types.EdgeHasSecurityPattern, FilePath: "review.yaml", Confidence: types.ConfExtracted}}); e != nil {
		t.Fatal(e)
	}
	s := server.NewMCPServer("test", "1", server.WithOutputSchemaValidation())
	RegisterAll(s, store)
	tool := s.GetTool(ToolName("rule_candidates"))
	if tool == nil {
		t.Fatal("rule tool not registered")
	}
	schema, _ := json.Marshal(tool.Tool)
	var document map[string]any
	_ = json.Unmarshal(schema, &document)
	if document["annotations"].(map[string]any)["readOnlyHint"] != true {
		t.Fatal("candidate tool not read-only")
	}
	args := map[string]any{"rule_pack": valueflow.RulePackID, "function": map[string]any{"node_id": node.ID}}
	request, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": ToolName("rule_candidates"), "arguments": args}})
	response, _ := json.Marshal(s.HandleMessage(context.Background(), request))
	var wire struct {
		Error  any `json:"error"`
		Result struct {
			IsError    bool                  `json:"isError"`
			Structured valueflow.QueryResult `json:"structuredContent"`
		} `json:"result"`
	}
	if e = json.Unmarshal(response, &wire); e != nil || wire.Error != nil || wire.Result.IsError || wire.Result.Structured.Analysis == nil {
		t.Fatal(string(response), e)
	}
	rules := wire.Result.Structured.Analysis.Rules
	if rules == nil || len(rules.Candidates) != 1 || rules.Candidates[0].Origin != "automatic" {
		t.Fatal(rules)
	}
	current, e := store.NodesByIDs([]string{manual.ID})
	if e != nil || len(current) != 1 || !reflect.DeepEqual(current[0], manual) {
		t.Fatal("manual annotation overwritten", current, e)
	}
	plain := s.GetTool(ToolName("value_dependencies"))
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"node_id": node.ID}
	reply, e := plain.Handler(context.Background(), req)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(reply.StructuredContent)
	var base valueflow.QueryResult
	_ = json.Unmarshal(raw, &base)
	if base.Analysis == nil || base.Analysis.Rules != nil {
		t.Fatal("default enabled rules", string(raw))
	}
}
func TestRuleCandidatesRawContractRejectsImplicitPackAndInvalidBounds(t *testing.T) {
	s := server.NewMCPServer("test", "1")
	RegisterRuleCandidates(s, valueStore(t))
	tool := s.GetTool(ToolName("rule_candidates"))
	for _, raw := range []string{`{"function":{"node_id":"1111111111111111"}}`, `{"rule_pack":"latest","function":{"node_id":"1111111111111111"}}`, `{"rule_pack":"go-local-review@1.0.0","function":{"node_id":"1111111111111111","node_id":"x"}}`, `{"rule_pack":"go-local-review@1.0.0","function":{"node_id":"1111111111111111"},"max_candidates":65}`, `{"rule_pack":"go-local-review@1.0.0","function":{"node_id":"1111111111111111"},"max_flow_steps":null}`} {
		req := mcp.CallToolRequest{}
		req.Params.RawArguments = json.RawMessage(raw)
		if _, e := tool.Handler(context.Background(), req); e == nil {
			t.Fatal("invalid raw input accepted", raw)
		}
	}
}
func TestRuleOutputValidatorRejectsFalseCandidateType(t *testing.T) {
	s := server.NewMCPServer("test", "1", server.WithOutputSchemaValidation())
	RegisterRuleCandidates(s, valueStore(t))
	tool := s.GetTool(ToolName("rule_candidates"))
	s.AddTool(tool.Tool, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{"rule_pack": valueflow.RulePackID, "function": map[string]any{"node_id": "1111111111111111"}}
		reply, e := tool.Handler(context.Background(), req)
		if e != nil {
			return nil, e
		}
		raw, _ := json.Marshal(reply.StructuredContent)
		var value map[string]any
		_ = json.Unmarshal(raw, &value)
		value["analysis"].(map[string]any)["rules"].(map[string]any)["candidates"] = 42
		return mcp.NewToolResultStructuredOnly(value), nil
	})
	req, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": ToolName("rule_candidates"), "arguments": map[string]any{"rule_pack": valueflow.RulePackID, "function": map[string]any{"node_id": "1111111111111111"}}}})
	raw, _ := json.Marshal(s.HandleMessage(context.Background(), req))
	var wire struct {
		Result struct {
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	_ = json.Unmarshal(raw, &wire)
	if !wire.Result.IsError {
		t.Fatal("schema validation skipped", string(raw))
	}
}
