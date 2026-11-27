package retrieval

import (
	"context"
	"testing"

	mcp "github.com/mark3labs/mcp-go/mcp"
	server "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/pkg/graph/mcphandlers"
)

// TestDispatchProbe_CallsTheTool: a probe is answered by the registered
// MCP tool's handler with the fixture's arguments as a JSON client sends
// them, so eval-retrieval scores the tool's own default path. Names are
// scored as the tool prints them (site names kept, so two declarations
// sharing a qualified name stay two) and a traversal's seed is left out. The
// runner called the store directly, without the tool's default filter, and
// scored an in-function Variable the tool never returns.
func TestDispatchProbe_CallsTheTool(t *testing.T) {
	s := server.NewMCPServer("t", "0")
	var gotArgs map[string]any
	answer := func(nodes ...string) server.ToolHandlerFunc {
		return func(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			gotArgs = req.GetArguments()
			rows := make([]map[string]any, len(nodes))
			for i, n := range nodes {
				rows[i] = map[string]any{"qname": n}
			}
			return &mcp.CallToolResult{StructuredContent: map[string]any{"nodes": rows}}, nil
		}
	}
	s.AddTool(mcp.NewTool(mcphandlers.ToolName("search_text")), answer("p.A", "p.B@a.go:3", "p.B@b.go:9"))
	s.AddTool(mcp.NewTool(mcphandlers.ToolName("find_callers")), answer("p.Seed", "p.Caller"))

	got, err := dispatchProbe(s, Probe{Tool: "search_text", Args: map[string]any{"query": "x", "top_k": 20}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 || got[0] != "p.A" || got[1] != "p.B@a.go:3" || got[2] != "p.B@b.go:9" {
		t.Errorf("search_text got %v, want the printed names [p.A p.B@a.go:3 p.B@b.go:9]", got)
	}
	if v, ok := gotArgs["top_k"].(float64); !ok || v != 20 {
		t.Errorf("top_k sent as %T %v, want float64 20", gotArgs["top_k"], gotArgs["top_k"])
	}
	got, err = dispatchProbe(s, Probe{Tool: "find_callers", Args: map[string]any{"qname": "p.Seed"}})
	if err != nil || len(got) != 1 || got[0] != "p.Caller" {
		t.Errorf("find_callers got %v, %v; want [p.Caller]", got, err)
	}
	if _, err := dispatchProbe(s, Probe{Tool: "nope"}); err == nil {
		t.Error("an unknown tool should fail")
	}
}
