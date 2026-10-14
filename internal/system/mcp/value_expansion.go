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

var ToolNameValueDependencyExpansion = toolName("context.value_dependency_expansion")

type curatedFlowSeparation struct {
	Origin            string `json:"origin"`
	Status            string `json:"status"`
	QueryTool         string `json:"query_tool"`
	SourceAssociation string `json:"source_association"`
	Reason            string `json:"reason"`
}
type valueExpansionResponse struct {
	SchemaVersion int                       `json:"schema_version"`
	Graph         valueflow.ExpansionResult `json:"graph"`
	CuratedFlow   curatedFlowSeparation     `json:"curated_flow"`
}

func registerValueDependencyExpansion(s *mcpserver.MCPServer, d Deps) {
	integer := func(cap int) map[string]any { return map[string]any{"type": "integer", "minimum": 1, "maximum": cap} }
	limits := map[string]any{"max_functions": integer(8), "max_hops": integer(3), "max_candidates": integer(128), "max_total_nodes": integer(4096), "max_total_edges": integer(16384), "max_total_facts": integer(262144), "timeout_ms": integer(5000)}
	raw, _ := json.Marshal(map[string]any{"type": "object", "additionalProperties": false, "required": []string{"function"}, "properties": map[string]any{"function": mcphandlers.ValueDependencyTool().RawInputSchema, "limits": map[string]any{"type": "object", "additionalProperties": false, "properties": limits}}})
	tool := mcpgo.NewTool(ToolNameValueDependencyExpansion, mcpgo.WithReadOnlyHintAnnotation(true), mcpgo.WithOutputSchema[valueExpansionResponse](), mcpgo.WithDescription("Bounded expansion of distinct retained Go function-local value/control analyses through outgoing calls/invokes. SQL-limited candidates, function/hop/aggregate node-edge-fact/time budgets, cycle/dedup, source/snapshot pins and stop reasons. Call hops do not prove argument/return or memory transfer. Curated operator flows remain a separate get_flow surface with an unverified dataset join, never computed dependence or a safety verdict. Exact selected identity required; no live source fallback."))
	tool.InputSchema = mcpgo.ToolInputSchema{}
	tool.RawInputSchema = raw
	s.AddTool(tool, func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		raw, e := json.Marshal(req.GetRawArguments())
		if e != nil {
			return nil, e
		}
		input, e := valueflow.DecodeExpansionInput(raw)
		if e != nil {
			return nil, e
		}
		response := valueExpansionResponse{SchemaVersion: 1, CuratedFlow: curatedFlowSeparation{Origin: "operator_curated_metadata", Status: "separate_not_joined", QueryTool: ToolNameGetFlow, SourceAssociation: "not_verified", Reason: "Curated call order/reads/writes are separate records; they are not local computed def-use/control or a verified cross-dataset join"}}
		backend, ok := d.CKG.(ckgclient.ValueExpansionClient)
		if !ok {
			response.Graph = valueflow.ExpansionResult{SchemaVersion: 1, Status: "unavailable", Functions: []valueflow.ExpandedFunction{}, Hops: []valueflow.SymbolHop{}, StopReasons: []string{"bounded_value_backend_unavailable"}, Limitations: []string{"no safe bounded expansion backend"}}
			return mcpgo.NewToolResultStructuredOnly(response), nil
		}
		response.Graph, e = backend.ValueDependencyExpansion(ctx, input)
		if e != nil {
			return nil, e
		}
		return mcpgo.NewToolResultStructuredOnly(response), nil
	})
}
