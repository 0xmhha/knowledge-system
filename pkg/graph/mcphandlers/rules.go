package mcphandlers

import (
	"context"
	"encoding/json"

	mcp "github.com/mark3labs/mcp-go/mcp"
	server "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/valueflow"
)

// RegisterRuleCandidates exposes an explicit opt-in pack; never writes manual tags.
func RegisterRuleCandidates(s *server.MCPServer, reader store.Reader) {
	reader = safeReader(reader)
	tool := RuleCandidateTool()
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		raw, e := json.Marshal(req.GetRawArguments())
		if e != nil {
			return nil, e
		}
		input, e := valueflow.DecodeRuleQueryInput(raw)
		if e != nil {
			return nil, e
		}
		out, e := valueflow.QueryRules(ctx, reader, input)
		if e != nil {
			return nil, e
		}
		return textResult(out), nil
	})
}

// RuleCandidateTool returns shared opt-in input/output metadata for facades.
func RuleCandidateTool() mcp.Tool {
	function := ValueDependencyTool().RawInputSchema
	integer := func(cap int) map[string]any { return map[string]any{"type": "integer", "minimum": 1, "maximum": cap} }
	raw, _ := json.Marshal(map[string]any{"type": "object", "additionalProperties": false, "required": []string{"rule_pack", "function"}, "properties": map[string]any{"rule_pack": map[string]any{"type": "string", "const": valueflow.RulePackID}, "function": function, "max_candidates": integer(64), "max_flow_steps": integer(4096)}})
	tool := nsTool("rule_candidates", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDescription("Explicit opt-in go-local-review@1.0.0 retained Go rule candidates using one shared lexical value/control analysis. Parameter to command-name and discarded conversion-error to return patterns. Candidates are automatic review inputs, never verified vulnerabilities or manual policy/security annotations. Syntax-only targets, unknown sanitizers, memory/alias and feasibility limitations remain explicit. Empty results never prove safety."))
	tool.InputSchema = mcp.ToolInputSchema{}
	tool.RawInputSchema = raw
	tool.OutputSchema = valueFlowOutputSchema()
	return tool
}
