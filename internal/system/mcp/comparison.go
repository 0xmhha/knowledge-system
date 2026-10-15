package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/0xmhha/knowledge-system/internal/system/comparison"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

var ToolNameComparisonDescribe = toolName("comparison.describe")
var ToolNameComparisonExtract = toolName("comparison.extract")

// RegisterComparisonTools is an explicit read-only surface. It does not
// register the default composer, operations, maintenance or write tools.
func RegisterComparisonTools(s *mcpserver.MCPServer, runtime *comparison.ComparisonRuntime) {
	s.AddTool(mcpgo.NewTool(ToolNameComparisonDescribe, mcpgo.WithReadOnlyHintAnnotation(true), mcpgo.WithDestructiveHintAnnotation(false), mcpgo.WithIdempotentHintAnnotation(true), mcpgo.WithOpenWorldHintAnnotation(false), mcpgo.WithDescription("Describe explicitly configured comparison scopes, common definitions/facets and fixed profile/catalog pins. configured does not mean source/read readiness. No local paths or writes."), mcpgo.WithInputSchema[struct{}](), mcpgo.WithOutputSchema[comparison.ComparisonDescription]()), func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		data, err := json.Marshal(req.GetRawArguments())
		if err != nil {
			return comparisonToolError(comparison.ErrComparisonRequest), nil
		}
		out, err := runtime.DescribeRequest(ctx, data)
		if err != nil {
			return comparisonToolError(err), nil
		}
		return mcpgo.NewToolResultStructured(out, "configured comparison scope; source readiness not checked"), nil
	})
	s.AddTool(mcpgo.NewTool(ToolNameComparisonExtract, mcpgo.WithReadOnlyHintAnnotation(true), mcpgo.WithDestructiveHintAnnotation(false), mcpgo.WithIdempotentHintAnnotation(true), mcpgo.WithOpenWorldHintAnnotation(false), mcpgo.WithDescription("Extract cited source evidence from explicit project/version/condition scopes and a shared concept/facets. Pin profile/catalog from describe. comparison only, no patch target or writes. include_proposed explicitly offers tagged candidates; final tradeoffs belong to the LLM/user."), mcpgo.WithInputSchema[comparison.ComparisonRequest](), mcpgo.WithOutputSchema[comparison.ComparisonBundle]()), func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		data, err := json.Marshal(req.GetRawArguments())
		if err != nil || len(data) > 64<<10 {
			return comparisonToolError(comparison.ErrComparisonRequest), nil
		}
		out, err := runtime.Extract(ctx, data)
		if err != nil {
			return comparisonToolError(err), nil
		}
		return mcpgo.NewToolResultStructured(out, "project/facet source evidence; no automatic superiority conclusion"), nil
	})
}
func comparisonToolError(err error) *mcpgo.CallToolResult {
	code := "operation_failed"
	for name, sentinel := range map[string]error{"profile_invalid": comparison.ErrComparisonProfile, "request_invalid": comparison.ErrComparisonRequest, "stale_configuration": comparison.ErrComparisonStale, "closed": comparison.ErrComparisonClosed, "response_budget_exceeded": comparison.ErrComparisonBudget, "canceled": context.Canceled, "deadline": context.DeadlineExceeded} {
		if errors.Is(err, sentinel) {
			code = name
			break
		}
	}
	result := mcpgo.NewToolResultError("comparison request refused: " + code)
	result.StructuredContent = map[string]string{"code": code, "message": "The selected comparison could not be verified or completed."}
	return result
}
func RunComparison(ctx context.Context, runtime *comparison.ComparisonRuntime, version string, in io.Reader, out io.Writer) error {
	s := mcpserver.NewMCPServer(runtime.Name(), version)
	RegisterComparisonTools(s, runtime)
	return runStdio(ctx, s, in, out)
}
