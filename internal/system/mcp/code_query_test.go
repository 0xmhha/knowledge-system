package mcp

import (
	"context"
	"testing"

	mcpgo "github.com/mark3labs/mcp-go/mcp"

	"github.com/0xmhha/knowledge-system/internal/system/composer/stage1"
)

func TestWithCodeQueryArg(t *testing.T) {
	t.Parallel()
	req := mcpgo.CallToolRequest{}
	req.Params.Arguments = map[string]any{"prompt": "p", "code_query": " `http2Client.NewStream` "}
	if got := stage1.CodeQueryFrom(withCodeQueryArg(context.Background(), req)); got != "`http2Client.NewStream`" {
		t.Fatalf("code query = %q", got)
	}
	req.Params.Arguments = map[string]any{"prompt": "p"}
	if got := stage1.CodeQueryFrom(withCodeQueryArg(context.Background(), req)); got != "" {
		t.Fatalf("absent code_query leaked %q", got)
	}
}
