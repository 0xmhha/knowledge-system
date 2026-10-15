package mcp

import (
	"context"
	"encoding/json"

	"github.com/0xmhha/knowledge-system/internal/system/maintenance"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

var ToolNameDatasetInspect = toolName("dataset.inspect_profile")
var ToolNameDatasetBuild = toolName("dataset.build")
var ToolNameDatasetStatus = toolName("dataset.status")
var ToolNameDatasetCancel = toolName("dataset.cancel")

func registerMaintenance(server *mcpserver.MCPServer, service *maintenance.Service) {
	addMaintenanceTool(server, ToolNameDatasetInspect, "Inspect an operator-registered project's exact source/config/model/base pins without building. Existing rendered inputs only. Returns process-lifetime job capability; serving adoption is separate.", true, service.Inspect)
	addMaintenanceTool(server, ToolNameDatasetBuild, "Start a scoped immutable version build with exact inspected pins. Returns job status, not a success assertion. Poll dataset.status; held candidates do not change current. Jobs/replay are process-lifetime in this stage.", false, service.Build)
	addMaintenanceTool(server, ToolNameDatasetStatus, "Read one job using project_id, profile_id and job_id. Separates queued/running, validated held candidate and actual activation receipt. Does not claim readers adopted a new version.", true,
		func(_ context.Context, r maintenance.JobReference) (maintenance.JobStatus, error) {
			return service.Status(r)
		})
	addMaintenanceTool(server, ToolNameDatasetCancel, "Request cancellation of one scoped job. accepted is not canceled completion; poll status. Publication already committed returns too_late and never rolls back current.", false,
		func(_ context.Context, r maintenance.JobReference) (maintenance.CancelResult, error) {
			return service.Cancel(r)
		})
}

func addMaintenanceTool[Request, Result any](server *mcpserver.MCPServer, name, description string, readOnly bool, call func(context.Context, Request) (Result, error)) {
	server.AddTool(mcpgo.NewTool(name, mcpgo.WithDescription(description), mcpgo.WithReadOnlyHintAnnotation(readOnly),
		mcpgo.WithInputSchema[Request](), mcpgo.WithOutputSchema[Result]()), func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		var request Request
		raw, err := json.Marshal(req.GetRawArguments())
		if err == nil {
			err = maintenance.Decode(raw, &request)
		}
		if err != nil {
			return maintenanceToolError(&maintenance.Error{Code: "invalid_request", Phase: "accepted"}), nil
		}
		value, err := call(ctx, request)
		if err != nil {
			return maintenanceToolError(err), nil
		}
		return mcpgo.NewToolResultStructured(value, "scoped dataset maintenance result; inspect structured status"), nil
	})
}

func maintenanceToolError(err error) *mcpgo.CallToolResult {
	safe := maintenance.ErrorResult(err)
	result := mcpgo.NewToolResultError("dataset request refused: " + safe.Code)
	result.StructuredContent = safe
	return result
}
