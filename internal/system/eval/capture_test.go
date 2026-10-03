package eval

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	sysmcp "github.com/0xmhha/knowledge-system/internal/system/mcp"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func TestCaptureRejectsGoldAndWriteToolsBeforeCall(t *testing.T) {
	for _, request := range []CaptureRequest{
		{ID: "bad", Tool: "cks.knowledge.review", Arguments: map[string]any{"prompt": "Alpha"}},
		{ID: "bad", Tool: toolGetForTask, Arguments: map[string]any{"prompt": "Alpha", "expected_answer": "secret gold"}},
		{ID: "bad", Tool: toolGetForTask, Arguments: map[string]any{"prompt": "Alpha", "include_knowledge": true}},
		{ID: "bad", Tool: sysmcp.ToolNameGetForTaskV2, Arguments: map[string]any{"prompt": "Alpha", "include_knowledge": "true"}},
		{ID: "bad", Tool: toolGetForTask, Arguments: map[string]any{"prompt": "Alpha", "intent": "made-up"}},
	} {
		client := &mockMCPClient{}
		runner := &Runner{client: client}
		if _, err := runner.Capture(context.Background(), request); err == nil {
			t.Fatalf("accepted unsafe request: %+v", request)
		}
		if len(client.calls) != 0 {
			t.Fatal("invalid input reached MCP")
		}
	}
}

func TestCapturePreservesV2CoordinatesBodiesAndStructuredErrors(t *testing.T) {
	result := mcpgo.NewToolResultStructured(map[string]any{
		"format_version": 2,
		"coordinates":    map[string]any{"project_id": "fixture", "dataset_id": "dataset", "snapshot_id": "snapshot"},
		"bodies":         []map[string]any{{"text": "retained policy body", "file": "README.md"}},
		"semantic":       map[string]any{"knowledge_context": map[string]any{"state": "conflict"}},
	}, "full v2 pack")
	client := &mockMCPClient{callOut: map[string]*mcpgo.CallToolResult{sysmcp.ToolNameGetForTaskV2: result}, callErr: map[string]error{}}
	runner := &Runner{client: client}
	request := CaptureRequest{ID: "v2", Tool: sysmcp.ToolNameGetForTaskV2, Arguments: map[string]any{"prompt": "Alpha", "include_knowledge": true}}
	call, err := runner.Capture(context.Background(), request)
	if err != nil {
		t.Fatal(err)
	}
	var raw mcpgo.CallToolResult
	if err := json.Unmarshal(call.Response, &raw); err != nil {
		t.Fatal(err)
	}
	var pack map[string]any
	encoded, _ := json.Marshal(raw.StructuredContent)
	if err := json.Unmarshal(encoded, &pack); err != nil {
		t.Fatal(err)
	}
	if pack["coordinates"].(map[string]any)["dataset_id"] != "dataset" || pack["bodies"].([]any)[0].(map[string]any)["text"] != "retained policy body" {
		t.Fatalf("lost raw coordinates/body: %s", call.Response)
	}
	if len(client.calls) != 1 || client.calls[0].args["include_knowledge"] != true || len(client.calls[0].args) != 2 {
		t.Fatal("request changed")
	}
	result.IsError = true
	result.StructuredContent = map[string]any{"code": "invalid_knowledge_scope", "message": "scope required"}
	failed, err := runner.Capture(context.Background(), request)
	if err != nil {
		t.Fatal("tool error was incorrectly treated as transport failure", err)
	}
	if err := json.Unmarshal(failed.Response, &raw); err != nil || !raw.IsError {
		t.Fatal("tool failure lost", err)
	}
	if string(call.Response) == string(failed.Response) {
		t.Fatal("responses were not frozen independently")
	}
	client.callErr[sysmcp.ToolNameGetForTaskV2] = errors.New("connection closed")
	transport, err := runner.Capture(context.Background(), request)
	if err == nil || transport.TransportError != "connection closed" || len(transport.Response) != 0 {
		t.Fatalf("transport error lost: %+v %v", transport, err)
	}
}
