package mcphandlers

import (
	"context"
	"encoding/json"

	mcp "github.com/mark3labs/mcp-go/mcp"
	server "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/valueflow"
)

// RegisterValueDependencies exposes the same graph-engine computation used by
// in-process consumers. Exact retained identity avoids same-name source mixing.
func RegisterValueDependencies(s *server.MCPServer, reader store.Reader) {
	reader = safeReader(reader)
	tool := ValueDependencyTool()
	s.AddTool(tool, func(ctx context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		raw, e := json.Marshal(req.GetRawArguments())
		if e != nil {
			return nil, e
		}
		input, e := valueflow.DecodeQueryInput(raw)
		if e != nil {
			return nil, e
		}
		result, e := valueflow.Query(ctx, reader, input)
		if e != nil {
			return nil, e
		}
		return textResult(result), nil
	})
}

// retainedValueInput keeps host-compatible object metadata. Selector exclusivity
// remains mandatory in DecodeQueryInput, including for nested rule/expansion inputs.
// Some hosts exclude tools whose root schema contains composition keywords.
func retainedValueInput(tool mcp.Tool) mcp.Tool {
	tool.InputSchema.AdditionalProperties = false
	tool.RawInputSchema, _ = json.Marshal(tool.InputSchema)
	tool.InputSchema = mcp.ToolInputSchema{}
	return tool
}

// ValueDependencyTool returns the shared public metadata for CKG and CKS.
func ValueDependencyTool() mcp.Tool {
	integer := func(cap int) map[string]any { return map[string]any{"type": "integer", "minimum": 1, "maximum": cap} }
	tool := nsTool("value_dependencies", mcp.WithReadOnlyHintAnnotation(true), mcp.WithDescription("Bounded on-demand Go CFG, lexical def-use, local value and normal-exit control dependence from retained function source. Select exactly one node_id/canonical_id from find_symbol (exclusivity is enforced at runtime; metadata is a host-compatible object); no name guessing or live source read. Returns source/blob/snapshot pins, caps, partial/unsupported/unknown and raw locations. Memory aliases, external effects, exceptions and short-circuit semantics are not proven; missing edges never establish safety."),
		mcp.WithString("node_id", mcp.MinLength(1), mcp.MaxLength(128)), mcp.WithString("canonical_id", mcp.MinLength(1), mcp.MaxLength(512)),
		mcp.WithString("expected_commit", mcp.Pattern("^([0-9a-f]{40}|[0-9a-f]{64})$")), mcp.WithString("expected_source_sha256", mcp.Pattern("^[0-9a-f]{64}$")), mcp.WithString("expected_snapshot_sha256", mcp.Pattern("^[0-9a-f]{64}$")),
		mcp.WithObject("limits", mcp.AdditionalProperties(false), mcp.Properties(map[string]any{"max_source_bytes": integer(256 * 1024), "max_ast_nodes": integer(4096), "max_blocks": integer(256), "max_nodes": integer(1024), "max_edges": integer(4096), "max_iterations": integer(128), "max_facts": integer(65536), "timeout_ms": integer(1000)})))
	tool = retainedValueInput(tool)
	tool.OutputSchema = valueFlowOutputSchema()
	return tool
}
