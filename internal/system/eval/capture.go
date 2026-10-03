package eval

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/0xmhha/knowledge-system/internal/system/envelope"
	"strings"
	"time"

	sysmcp "github.com/0xmhha/knowledge-system/internal/system/mcp"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// CaptureRequest has no gold/expected-answer fields. Capture is a read-only
// transport recorder; quality and human claim review happen separately.
type CaptureRequest struct {
	ID        string         `json:"id"`
	Tool      string         `json:"tool"`
	Arguments map[string]any `json:"arguments"`
}

func (r CaptureRequest) Validate() error {
	if strings.TrimSpace(r.ID) == "" {
		return fmt.Errorf("capture request ID is required")
	}
	isV2 := r.Tool == sysmcp.ToolNameGetForTaskV2
	if !isV2 && r.Tool != toolGetForTask {
		return fmt.Errorf("capture only permits v1/v2 get_for_task")
	}
	prompt, ok := r.Arguments["prompt"].(string)
	if !ok || strings.TrimSpace(prompt) == "" {
		return fmt.Errorf("capture prompt is required")
	}
	for key, value := range r.Arguments {
		switch key {
		case "prompt":
		case "intent":
			intent, ok := value.(string)
			if !ok {
				return fmt.Errorf("capture intent must be a string")
			}
			if _, valid := contract.ParseIntent(intent); !valid {
				return fmt.Errorf("capture intent is invalid")
			}
		case "include_knowledge":
			if _, ok := value.(bool); !isV2 || !ok {
				return fmt.Errorf("include_knowledge requires v2 and a boolean")
			}
		case "knowledge_as_of", "knowledge_subsystem":
			if _, ok := value.(string); !isV2 || !ok {
				return fmt.Errorf("knowledge scope requires v2 and strings")
			}
		default:
			return fmt.Errorf("unsupported capture argument %q", key)
		}
	}
	// Scope completeness is checked by the server. Intentionally malformed
	// scope is a useful error fixture and its full response must be retained.
	return nil
}

// Capture returns every decoded tool result, including IsError=true. Transport
// errors and serialization failures are distinct; it computes no quality score.
func (r *Runner) Capture(ctx context.Context, request CaptureRequest) (RawToolCall, error) {
	if err := request.Validate(); err != nil {
		return RawToolCall{}, err
	}
	arguments, err := json.Marshal(request.Arguments)
	if err != nil {
		return RawToolCall{}, err
	}
	req := mcpgo.CallToolRequest{}
	req.Params.Name, req.Params.Arguments = request.Tool, request.Arguments
	measurementID := envelope.NewTraceID()
	req.Params.Meta = &mcpgo.Meta{AdditionalFields: map[string]any{"cks.measurement_id": measurementID}}
	start := time.Now()
	result, callErr := r.client.CallTool(ctx, req)
	completed := time.Now()
	entry := RawToolCall{MeasurementID: measurementID, Tool: request.Tool, Arguments: arguments, ElapsedNS: completed.Sub(start).Nanoseconds(), CompletedAt: &completed}
	if result != nil {
		entry.Response, err = json.Marshal(result)
		if err != nil {
			return entry, fmt.Errorf("capture response: %w", err)
		}
	}
	if callErr != nil {
		entry.TransportError = callErr.Error()
	}
	return entry, callErr
}
