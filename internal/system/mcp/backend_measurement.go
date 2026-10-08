package mcp

import (
	"context"
	"encoding/hex"

	"github.com/0xmhha/knowledge-system/internal/system/backendmeasure"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func beginBackendMeasurement(ctx context.Context, d Deps, req mcpgo.CallToolRequest) (context.Context, func(*mcpgo.CallToolResult, error)) {
	if d.BackendMeasurements == nil {
		return ctx, func(*mcpgo.CallToolResult, error) {}
	}
	id := ""
	if req.Params.Meta != nil {
		candidate, _ := req.Params.Meta.AdditionalFields["cks.measurement_id"].(string)
		if bytes, err := hex.DecodeString(candidate); err == nil && len(bytes) == 16 {
			id = candidate
		}
	}
	var scope *backendmeasure.Scope
	ctx, scope = d.BackendMeasurements.Begin(ctx, id, req.Params.Name)
	return ctx, func(result *mcpgo.CallToolResult, err error) {
		outcome := "returned"
		if err != nil {
			outcome = "transport_error"
		} else if result == nil {
			outcome = "missing_result"
		} else if result.IsError {
			outcome = "tool_error"
		}
		scope.Finish(outcome)
	}
}
