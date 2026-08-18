package mcp

import (
	"context"
	"fmt"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/internal/setup"
)

// Tool names for the asynchronous knowledge-setup surface. MCP tools are
// request/response, and an index build runs for minutes — so the surface is
// split into a start call that returns a job ID immediately and a status
// call that polls the progress-event tail. The job registry lives in
// internal/setup and is shared with the CLI's execution machinery.
var (
	ToolNameOpsSetup       = toolName("ops.setup")
	ToolNameOpsSetupStatus = toolName("ops.setup_status")
)

// jobStartedResponse is the wire shape both async starters return: the job
// ID to poll with ops.setup_status, and the state it starts in. It replaces a
// bare map so the tools can declare an output schema — a map generates none,
// which is how a caller ends up guessing at the field names.
//
// Version is set by ops.reindex, which builds into a named version directory;
// ops.setup omits it.
type jobStartedResponse struct {
	JobID   string `json:"job_id"`
	State   string `json:"state"`
	Version string `json:"version,omitempty"`
}

// SetupConfig carries what the setup tools need from server configuration.
type SetupConfig struct {
	// GraphBinary / VectorBinary are the engine CLIs (empty → PATH lookup).
	// Reuses the same binaries the index tool is configured with.
	GraphBinary  string
	VectorBinary string
	// OllamaURL is exported to the vector build when set.
	OllamaURL string
	// Jobs is the shared async registry. Nil disables the setup tools.
	Jobs *setup.Jobs
}

func (c SetupConfig) enabled() bool { return c.Jobs != nil }

// registerOpsSetup wires the asynchronous dataset-build tool.
func registerOpsSetup(s *mcpserver.MCPServer, d Deps) {
	tool := mcpgo.NewTool(ToolNameOpsSetup,
		mcpgo.WithOutputSchema[jobStartedResponse](),
		mcpgo.WithDescription(
			"Start an asynchronous knowledge-dataset build for a source tree: graph index, "+
				"vector index aligned to it, and an alignment verification gate. Returns a job_id "+
				"immediately; poll ops.setup_status for progress. Long-running — minutes on a real repo."),
		mcpgo.WithString("src", mcpgo.Required(), mcpgo.Description("source tree to index")),
		mcpgo.WithString("out", mcpgo.Required(), mcpgo.Description("dataset root; graph index in <out>/graph, vector index in <out>/vector")),
		mcpgo.WithString("embedder", mcpgo.Description("vector embedding backend (mock, bgeonnx, ollama); empty uses the vector CLI default")),
		mcpgo.WithString("model_name", mcpgo.Description("vector embedding model name")),
		mcpgo.WithBoolean("skip_vector", mcpgo.DefaultBool(false), mcpgo.Description("build only the graph index")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		sc := d.Setup
		if !sc.enabled() {
			return mcpgo.NewToolResultErrorf("%s: setup surface not configured", ToolNameOpsSetup), nil
		}
		src := req.GetString("src", "")
		out := req.GetString("out", "")
		plan, err := setup.BuildPlan(setup.Options{
			Src:        src,
			Out:        out,
			GraphBin:   sc.GraphBinary,
			VectorBin:  sc.VectorBinary,
			OllamaURL:  sc.OllamaURL,
			Embedder:   req.GetString("embedder", ""),
			ModelName:  req.GetString("model_name", ""),
			SkipVector: req.GetBool("skip_vector", false),
		})
		if err != nil {
			return mcpgo.NewToolResultErrorf("%s: %v", ToolNameOpsSetup, err), nil
		}
		id := sc.Jobs.Start(plan)
		return mcpgo.NewToolResultStructured(jobStartedResponse{JobID: id, State: setup.JobRunning},
			fmt.Sprintf("setup started: job_id=%s (poll %s)", id, ToolNameOpsSetupStatus)), nil
	})
}

// registerOpsSetupStatus wires the poll side of the async pair.
func registerOpsSetupStatus(s *mcpserver.MCPServer, d Deps) {
	tool := mcpgo.NewTool(ToolNameOpsSetupStatus,
		mcpgo.WithOutputSchema[setup.JobSnapshot](),
		mcpgo.WithDescription("Progress of an asynchronous ops job (ops.setup or ops.reindex): state (running|done|failed), error if any, and the tail of the progress-event stream."),
		mcpgo.WithString("job_id", mcpgo.Required()),
		mcpgo.WithNumber("tail", mcpgo.DefaultNumber(20), mcpgo.Description("max trailing events to return")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		sc := d.Setup
		if !sc.enabled() {
			return mcpgo.NewToolResultErrorf("%s: setup surface not configured", ToolNameOpsSetupStatus), nil
		}
		id := req.GetString("job_id", "")
		snap, ok := sc.Jobs.Get(id, req.GetInt("tail", 20))
		if !ok {
			return mcpgo.NewToolResultErrorf("%s: unknown job_id %q", ToolNameOpsSetupStatus, id), nil
		}
		text := fmt.Sprintf("job %s: %s", snap.ID, snap.State)
		if snap.Error != "" {
			text += " — " + snap.Error
		}
		return mcpgo.NewToolResultStructured(snap, text), nil
	})
}
