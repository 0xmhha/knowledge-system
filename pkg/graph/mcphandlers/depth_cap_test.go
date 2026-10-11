package mcphandlers

import (
	"testing"

	"github.com/mark3labs/mcp-go/mcp"

	"github.com/0xmhha/knowledge-system/pkg/graph/impact"
)

func TestTraversalDepthIsCapped(t *testing.T) {
	for in, want := range map[float64]int{1e9: impact.DepthCap, 6: impact.DepthCap, 3: 3, 0: 0, -4: 0} {
		req := mcp.CallToolRequest{}
		req.Params.Arguments = map[string]any{"depth": in}
		if got := traversalDepth(req); got != want {
			t.Errorf("depth %v: got %d, want %d", in, got, want)
		}
	}
	if got := traversalDepth(mcp.CallToolRequest{}); got != 2 {
		t.Errorf("default depth %d, want 2", got)
	}
}
