package mcp

import (
	"context"
	"fmt"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/internal/setup"
)

// ToolNameOpsReindex starts an asynchronous blue-green reindex. Like ops.setup
// it returns a job_id and is polled via ops.setup_status — MCP calls are
// request/response and a reindex runs for minutes, so it must not block the
// client (which would time out).
var ToolNameOpsReindex = toolName("ops.reindex")

// registerOpsReindex wires the asynchronous reindex tool, sharing the job
// registry and engine binaries with the setup surface.
func registerOpsReindex(s *mcpserver.MCPServer, d Deps) {
	tool := mcpgo.NewTool(ToolNameOpsReindex,
		mcpgo.WithDescription(
			"Start an asynchronous blue-green reindex of a dataset to a new version: build <out>/<version>, "+
				"run the alignment+quality gate, then atomically promote it to current. current is left unchanged "+
				"if the gate fails, so a bad rebuild never breaks the live dataset. Returns a job_id immediately; "+
				"poll ops.setup_status for progress. Long-running — minutes on a real repo."),
		mcpgo.WithString("src", mcpgo.Required(), mcpgo.Description("source tree to index")),
		mcpgo.WithString("out", mcpgo.Required(), mcpgo.Description("dataset root; the new version builds in <out>/<version> and current is flipped on success")),
		mcpgo.WithString("version", mcpgo.Description("version label for the new dataset directory; empty auto-generates one")),
		mcpgo.WithString("embedder", mcpgo.Description("vector embedding backend (mock, bgeonnx, ollama); empty uses the vector CLI default")),
		mcpgo.WithString("model_name", mcpgo.Description("vector embedding model name")),
		mcpgo.WithNumber("min_canonical_ratio", mcpgo.DefaultNumber(0), mcpgo.Description("gate floor for canonical_id coverage (CanonicalCount/SymbolCount); 0 disables the check")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		sc := d.Setup
		if !sc.enabled() {
			return mcpgo.NewToolResultErrorf("%s: setup surface not configured", ToolNameOpsReindex), nil
		}
		version := req.GetString("version", "")
		if version == "" {
			version = setup.NewVersion()
		}
		o := setup.Options{
			Src:       req.GetString("src", ""),
			Out:       req.GetString("out", ""),
			GraphBin:  sc.GraphBinary,
			VectorBin: sc.VectorBinary,
			OllamaURL: sc.OllamaURL,
			Embedder:  req.GetString("embedder", ""),
			ModelName: req.GetString("model_name", ""),
		}
		gopt := setup.GateOptions{MinCanonicalRatio: req.GetFloat("min_canonical_ratio", 0)}
		id := sc.Jobs.StartReindex(o, version, gopt)
		return mcpgo.NewToolResultStructured(
			map[string]string{"job_id": id, "version": version, "state": setup.JobRunning},
			fmt.Sprintf("reindex started: job_id=%s version=%s (poll %s)", id, version, ToolNameOpsSetupStatus)), nil
	})
}
