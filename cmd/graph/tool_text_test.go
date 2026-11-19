package main

import (
	"context"
	"testing"

	mcp "github.com/mark3labs/mcp-go/mcp"
	server "github.com/mark3labs/mcp-go/server"
)

// TestApplyToolText: tools take the text's description and schema and keep
// their own handler; tools the text does not list are removed.
func TestApplyToolText(t *testing.T) {
	s := server.NewMCPServer("t", "0")
	called := false
	s.AddTool(mcp.NewTool("keep", mcp.WithDescription("new text")), func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		called = true
		return mcp.NewToolResultText("ok"), nil
	})
	s.AddTool(mcp.NewTool("extra", mcp.WithDescription("only in the new build")), nil)
	old := mcp.NewTool("keep", mcp.WithDescription("old text"), mcp.WithString("tests"))
	applyToolText(s, &toolText{Instructions: "old guide", Tools: map[string]mcp.Tool{"keep": old}})

	if s.GetTool("extra") != nil {
		t.Error("a tool the text does not list is still served")
	}
	k := s.GetTool("keep")
	if k == nil || k.Tool.Description != "old text" {
		t.Fatalf("keep = %+v, want the old description", k)
	}
	if _, ok := k.Tool.InputSchema.Properties["tests"]; !ok {
		t.Error("keep lost the old schema's tests argument")
	}
	if _, err := k.Handler(context.Background(), mcp.CallToolRequest{}); err != nil || !called {
		t.Error("keep no longer runs its own handler")
	}
}
