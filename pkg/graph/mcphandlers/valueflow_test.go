package mcphandlers

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	mcp "github.com/mark3labs/mcp-go/mcp"
	server "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
	"github.com/0xmhha/knowledge-system/pkg/graph/valueflow"
)

func valueStore(t *testing.T) persist.Store {
	t.Helper()
	s, e := persist.Open(filepath.Join(t.TempDir(), "graph.db"))
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { _ = s.Close() })
	if e = s.Migrate(); e != nil {
		t.Fatal(e)
	}
	source := []byte("func F(a,b int,flag bool)int{x:=a;if flag{x=b};return x}")
	node := types.Node{ID: "1111111111111111", Type: types.NodeFunction, Name: "F", QualifiedName: "p.F", CanonicalID: "example.invalid/p.F", FilePath: "p.go", Language: "go", StartLine: 1, EndLine: 1, StartByte: 10, EndByte: 10 + len(source), Confidence: types.ConfExtracted}
	if e = s.InsertNodes([]types.Node{node}); e != nil {
		t.Fatal(e)
	}
	if e = s.InsertBlobs(map[string][]byte{node.ID: source}); e != nil {
		t.Fatal(e)
	}
	if e = s.SetManifest(persist.Manifest{SchemaVersion: "1.9", SrcCommit: strings.Repeat("a", 40), BuildTimestamp: "fixture"}); e != nil {
		t.Fatal(e)
	}
	return s
}
func TestValueDependenciesRegisteredWithRuntimeContractAndReadonlyAnnotation(t *testing.T) {
	s := server.NewMCPServer("test", "0")
	RegisterAll(s, valueStore(t))
	tool := s.GetTool(ToolName("value_dependencies"))
	if tool == nil || tool.Handler == nil {
		t.Fatal("public registration missing")
	}
	raw, e := json.Marshal(tool.Tool)
	if e != nil {
		t.Fatal(e)
	}
	var schema struct {
		Input       map[string]any `json:"inputSchema"`
		Annotations map[string]any `json:"annotations"`
	}
	if e = json.Unmarshal(raw, &schema); e != nil {
		t.Fatal(e)
	}
	if schema.Annotations["readOnlyHint"] != true || schema.Input["additionalProperties"] != false || len(schema.Input["oneOf"].([]any)) != 2 {
		t.Fatal("input/readonly schema mismatch", string(raw))
	}
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"node_id": "1111111111111111"}
	reply, e := tool.Handler(context.Background(), req)
	if e != nil || reply == nil || reply.IsError {
		t.Fatal(reply, e)
	}
	payload, e := json.Marshal(reply.StructuredContent)
	if e != nil {
		t.Fatal(e)
	}
	var out valueflow.QueryResult
	if e = json.Unmarshal(payload, &out); e != nil {
		t.Fatal(e)
	}
	if out.Analysis == nil || len(out.Analysis.DefUse) == 0 || out.Analysis.Capability["control_dependence"] == "not_computed" {
		t.Fatal("public result lacks computed relations", out)
	}
}
func TestValueDependenciesActualRawDuplicateNullFractionalAndUnknownInputRejected(t *testing.T) {
	s := server.NewMCPServer("test", "0")
	RegisterValueDependencies(s, valueStore(t))
	tool := s.GetTool(ToolName("value_dependencies"))
	for _, raw := range []string{`{"node_id":"1111111111111111","node_id":"1111111111111111"}`, `{"node_id":"1111111111111111","limits":{"max_nodes":1.1}}`, `{"node_id":"1111111111111111","limits":null}`, `{"node_id":"1111111111111111","unknown":true}`} {
		req := mcp.CallToolRequest{}
		req.Params.RawArguments = json.RawMessage(raw)
		if _, e := tool.Handler(context.Background(), req); e == nil {
			t.Fatal("runtime schema bypass", raw)
		}
	}
}
func TestSameNameDifferentFileQueriesSelectExactRetainedIdentity(t *testing.T) {
	store := valueStore(t)
	source := []byte("func F()int{return 2}")
	node := types.Node{ID: "2222222222222222", Type: types.NodeFunction, Name: "F", QualifiedName: "p.F", CanonicalID: "example.invalid/other.F", FilePath: "other.go", Language: "go", StartLine: 4, EndLine: 4, StartByte: 20, EndByte: 20 + len(source), Confidence: types.ConfExtracted}
	if e := store.InsertNodes([]types.Node{node}); e != nil {
		t.Fatal(e)
	}
	if e := store.InsertBlobs(map[string][]byte{node.ID: source}); e != nil {
		t.Fatal(e)
	}
	s := server.NewMCPServer("test", "0")
	RegisterValueDependencies(s, store)
	tool := s.GetTool(ToolName("value_dependencies"))
	req := mcp.CallToolRequest{}
	req.Params.Arguments = map[string]any{"canonical_id": "example.invalid/other.F"}
	reply, e := tool.Handler(context.Background(), req)
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := json.Marshal(reply.StructuredContent)
	var out valueflow.QueryResult
	if e = json.Unmarshal(raw, &out); e != nil {
		t.Fatal(e)
	}
	if out.Analysis == nil || out.Analysis.Source.NodeID != node.ID || out.Analysis.Source.File != "other.go" {
		t.Fatal("same-name source mixed", out)
	}
	req.Params.Arguments = map[string]any{"qname": "p.F"}
	if _, e = tool.Handler(context.Background(), req); e == nil {
		t.Fatal("ambiguous qname silently guessed")
	}
}

func TestDeclaredValueOutputSchemaValidatesActualWireResults(t *testing.T) {
	s := server.NewMCPServer("test", "0", server.WithOutputSchemaValidation())
	RegisterValueDependencies(s, valueStore(t))
	for _, args := range []map[string]any{{"node_id": "1111111111111111"}, {"node_id": "2222222222222222"}, {"node_id": "1111111111111111", "limits": map[string]any{"max_iterations": 1}}} {
		request, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": ToolName("value_dependencies"), "arguments": args}})
		reply := s.HandleMessage(context.Background(), request)
		raw, e := json.Marshal(reply)
		if e != nil {
			t.Fatal(e)
		}
		var response struct {
			Error  any `json:"error"`
			Result struct {
				IsError    bool `json:"isError"`
				Structured any  `json:"structuredContent"`
			} `json:"result"`
		}
		if e = json.Unmarshal(raw, &response); e != nil {
			t.Fatal(e)
		}
		if response.Error != nil || response.Result.IsError || response.Result.Structured == nil {
			t.Fatal("declared output rejected factual wire result", string(raw))
		}
	}
}

func TestOutputValidationIsActiveAndRejectsFalseSchemaTypes(t *testing.T) {
	s := server.NewMCPServer("test", "0", server.WithOutputSchemaValidation())
	RegisterValueDependencies(s, valueStore(t))
	tool := s.GetTool(ToolName("value_dependencies"))
	s.AddTool(tool.Tool, func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return mcp.NewToolResultStructuredOnly(map[string]any{"schema_version": 1, "status": 42, "analysis": nil, "elapsed_micros": 0}), nil
	})
	raw, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": ToolName("value_dependencies"), "arguments": map[string]any{"node_id": "1111111111111111"}}})
	reply := s.HandleMessage(context.Background(), raw)
	wire, _ := json.Marshal(reply)
	var response struct {
		Result struct {
			IsError bool `json:"isError"`
		} `json:"result"`
	}
	if e := json.Unmarshal(wire, &response); e != nil {
		t.Fatal(e)
	}
	if !response.Result.IsError {
		t.Fatal("output validator skipped/accepted wrong declared type", string(wire))
	}
}

func TestBoundedEdgeCapabilityPreservesAmbiguousHistoryBoundary(t *testing.T) {
	s := valueStore(t)
	ambiguous := types.Node{ID: "ambiguous-source", Type: types.NodeHunk, Name: "Ambiguous", Confidence: types.ConfAmbiguous}
	if e := s.InsertNodes([]types.Node{ambiguous}); e != nil {
		t.Fatal(e)
	}
	if e := s.InsertEdges([]types.Edge{{Src: ambiguous.ID, Dst: "1111111111111111", Type: "calls", Confidence: types.ConfExtracted}}); e != nil {
		t.Fatal(e)
	}
	reader := NewLLMSafeReader(s)
	bounded, ok := reader.(store.BoundedEdgeReader)
	if !ok {
		t.Fatal("optional bounded capability hidden")
	}
	edges, _, e := bounded.IncidentEdges(context.Background(), store.EdgePageInput{NodeID: "1111111111111111", Direction: "incoming", Types: []string{"calls"}, Limit: 4})
	if e != nil || len(edges) != 0 {
		t.Fatal("ambiguous history edge leaked", edges, e)
	}
}
