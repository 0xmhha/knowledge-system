package mcphandlers

import (
	"context"
	"strings"
	"testing"

	mcp "github.com/mark3labs/mcp-go/mcp"
	server "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
)

// callFlow calls execution_flow on store and returns its text and whether
// it is an error result.
func callFlow(t *testing.T, store persist.StoreReader, args map[string]any) (string, bool) {
	t.Helper()
	s := server.NewMCPServer("test", "0")
	RegisterAll(s, store)
	tool := s.GetTool(executionFlowTool)
	if tool == nil {
		t.Fatal("execution_flow not registered")
	}
	req := mcp.CallToolRequest{}
	req.Params.Name = executionFlowTool
	req.Params.Arguments = args
	res, err := tool.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("execution_flow: %v", err)
	}
	var text strings.Builder
	for _, c := range res.Content {
		if tc, ok := c.(mcp.TextContent); ok {
			text.WriteString(tc.Text)
		}
	}
	return text.String(), res.IsError
}

// TestExecutionFlowTool: the tool answers with the narrative text, a send
// crossing to the code its receive runs; a query it cannot run is an error
// result, not a failed call.
func TestExecutionFlowTool(t *testing.T) {
	s := messagesFixture(t)
	out, isErr := callFlow(t, s, map[string]any{"qname": "mutex_fixture.RunServe"})
	if isErr {
		t.Fatalf("error result: %s", out)
	}
	for _, want := range []string{"execution_flow forward from mutex_fixture.RunServe", "starts goroutine",
		"sends on", "received at", "calls mutex_fixture.record"} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}
	back, _ := callFlow(t, s, map[string]any{"qname": "mutex_fixture.record", "direction": "backward"})
	if !strings.Contains(back, "runs when a message arrives at") {
		t.Errorf("backward from record does not name the receive whose message runs it:\n%s", back)
	}
	if msg, isErr := callFlow(t, s, map[string]any{"qname": "mutex_fixture.RunServe", "direction": "sideways"}); !isErr {
		t.Errorf("a bad direction is not an error result: %s", msg)
	}
}
