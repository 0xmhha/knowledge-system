package mcp

import (
	"context"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
)

var ToolNameConcurrencyImpact = toolName("context.concurrency_impact")

// concurrencyImpactResponse is the wire shape for concurrency_impact.
type concurrencyImpactResponse struct {
	Seed         string                      `json:"seed"`
	Result       contract.ConcurrencyResult  `json:"result"`
	Instructions []contract.DummyInstruction `json:"instructions,omitempty"`
}

// registerConcurrencyImpact wires cks.context.concurrency_impact (G7/S1).
func registerConcurrencyImpact(s *mcpserver.MCPServer, d Deps) {
	tool := mcpgo.NewTool(ToolNameConcurrencyImpact,
		mcpgo.WithDescription(
			"Concurrency blast radius of a symbol: goroutines it spawns, channels it "+
				"sends to/receives from, locks it acquires, plus modules reached over "+
				"concurrency edges. Use when a change touches shared state or ordering -- "+
				"before assuming a data race is or is not possible.",
		),
		mcpgo.WithString("symbol", mcpgo.Required(),
			mcpgo.Description("Symbol to seed the analysis: a ckg qualified_name, a bare name (only when it resolves unambiguously), or a ckg canonical_id. An ambiguous or unknown name is an error, not an empty result -- use find_symbol to get the canonical_id.")),
		mcpgo.WithNumber("depth",
			mcpgo.Description("Maximum traversal depth over concurrency edges (default: 3).")),
		mcpgo.WithNumber("max_total",
			mcpgo.Description("Cap on total modules returned (0 = no cap).")),
	)
	s.AddTool(tool, func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		return handleConcurrencyImpact(ctx, d, req)
	})
}

func handleConcurrencyImpact(ctx context.Context, d Deps, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	symbol := req.GetString("symbol", "")
	if symbol == "" {
		return mcpgo.NewToolResultError(ToolNameConcurrencyImpact + ": missing required argument \"symbol\""), nil
	}
	opts := ckgclient.ConcurrencyOpts{
		Depth:    intArg(req, "depth", 0),
		MaxTotal: intArg(req, "max_total", 0),
	}

	collector := contract.NewInstructionCollector()
	ctx = contract.WithCollector(ctx, collector)

	result, err := d.CKG.ConcurrencyImpact(ctx, symbol, opts)
	if err != nil {
		return mcpgo.NewToolResultErrorf("%s: %v", ToolNameConcurrencyImpact, err), nil
	}
	return mcpgo.NewToolResultStructured(concurrencyImpactResponse{
		Seed:         symbol,
		Result:       result,
		Instructions: collector.All(),
	}, "concurrency impact result"), nil
}
