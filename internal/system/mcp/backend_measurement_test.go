package mcp

import (
	"context"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/system/backendmeasure"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func TestBothContextHandlersSealEarlyErrorsWithCaptureID(t *testing.T) {
	for _, tool := range []string{ToolNameGetForTask, ToolNameGetForTaskV2} {
		var records []backendmeasure.Summary
		f := newFixture(t, nil)
		f.deps.BackendMeasurements = &backendmeasure.Recorder{Emit: func(_ context.Context, s backendmeasure.Summary) { records = append(records, s) }}
		req := mcpgo.CallToolRequest{}
		req.Params.Name = tool
		req.Params.Meta = &mcpgo.Meta{AdditionalFields: map[string]any{"cks.measurement_id": "0123456789abcdef0123456789abcdef"}}
		handler := handleGetForTask
		if tool == ToolNameGetForTaskV2 {
			handler = handleGetForTaskV2
		}
		result, err := handler(context.Background(), f.deps, req)
		if err != nil || result == nil || !result.IsError {
			t.Fatalf("early error changed: %+v, %v", result, err)
		}
		if len(records) != 1 || records[0].MeasurementID != "0123456789abcdef0123456789abcdef" || records[0].Outcome != "tool_error" || len(records[0].Calls) != 0 {
			t.Fatalf("lost error correlation for %s: %+v", tool, records)
		}
	}
}
