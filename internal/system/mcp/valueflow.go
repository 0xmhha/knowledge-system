package mcp

import (
	"context"
	"encoding/json"

	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/pkg/graph/mcphandlers"
	"github.com/0xmhha/knowledge-system/pkg/graph/valueflow"
)

var ToolNameValueDependencies = toolName("context.value_dependencies")
var ToolNameRuleCandidates = toolName("context.rule_candidates")

// Share the graph engine's computation and executable schemas, with no second
// matcher, live-source fallback, transport or authorization bypass.
func registerValueAnalysis(s *mcpserver.MCPServer, d Deps) {
	for _, rules := range []bool{false, true} {
		tool := mcphandlers.ValueDependencyTool()
		name := ToolNameValueDependencies
		if rules {
			tool = mcphandlers.RuleCandidateTool()
			name = ToolNameRuleCandidates
		}
		tool.Name = name
		s.AddTool(tool, func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
			raw, e := json.Marshal(req.GetRawArguments())
			if e != nil {
				return nil, e
			}
			unavailable := valueflow.QueryResult{SchemaVersion: 1, Status: "unavailable", Reason: "backend does not support retained value/rule analysis"}
			backend, ok := d.CKG.(ckgclient.ValueAnalysisClient)
			var result valueflow.QueryResult
			if rules {
				q, err := valueflow.DecodeRuleQueryInput(raw)
				if err != nil {
					return nil, err
				}
				if !ok {
					return mcpgo.NewToolResultStructuredOnly(unavailable), nil
				}
				result, e = backend.RuleCandidates(ctx, q)
			} else {
				q, err := valueflow.DecodeQueryInput(raw)
				if err != nil {
					return nil, err
				}
				if !ok {
					return mcpgo.NewToolResultStructuredOnly(unavailable), nil
				}
				result, e = backend.ValueDependencies(ctx, q)
			}
			if e != nil {
				return nil, e
			}
			return mcpgo.NewToolResultStructuredOnly(result), nil
		})
	}
}
