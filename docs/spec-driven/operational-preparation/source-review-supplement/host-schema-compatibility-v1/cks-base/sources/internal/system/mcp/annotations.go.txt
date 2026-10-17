package mcp

import (
	"context"
	"encoding/json"
	"github.com/0xmhha/knowledge-system/internal/system/ckgclient"
	"github.com/0xmhha/knowledge-system/pkg/graph/annotations"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	mcpserver "github.com/mark3labs/mcp-go/server"
)

var ToolNamePolicySecurity = toolName("context.policy_security")

func registerPolicySecurity(s *mcpserver.MCPServer, d Deps) {
	text := func(max int) map[string]any {
		return map[string]any{"type": "string", "minLength": 1, "maxLength": max}
	}
	pin := func() map[string]any { return map[string]any{"type": "string", "pattern": "^[0-9a-f]{64}$"} }
	raw, _ := json.Marshal(map[string]any{"type": "object", "additionalProperties": false, "oneOf": []any{map[string]any{"required": []string{"node_id"}, "not": map[string]any{"required": []string{"canonical_id"}}}, map[string]any{"required": []string{"canonical_id"}, "not": map[string]any{"required": []string{"node_id"}}}}, "properties": map[string]any{"node_id": text(128), "canonical_id": text(512), "expected_commit": map[string]any{"type": "string", "pattern": "^([0-9a-f]{40}|[0-9a-f]{64})$"}, "expected_source_sha256": pin(), "expected_snapshot_sha256": pin(), "max_annotations": map[string]any{"type": "integer", "minimum": 1, "maximum": 128}, "timeout_ms": map[string]any{"type": "integer", "minimum": 1, "maximum": 5000}}})
	tool := mcpgo.NewTool(ToolNamePolicySecurity, mcpgo.WithReadOnlyHintAnnotation(true), mcpgo.WithOutputSchema[annotations.Result](), mcpgo.WithDescription("Bounded retained policy/security annotations for one exact code symbol. Operator explanations and modification hints retain manual provenance; unavailable original YAML row/hash/version/reviewer/approval stay unknown. Metadata hashes are graph hashes, not original-source proof. No annotations does not prove safety. Automatic rule_candidates are a separate opt-in query and are not executed here. SQL row caps and cooperative timeout, snapshot/blob/tag/edge rechecks; no live file reads or authorization bypass."))
	tool.InputSchema = mcpgo.ToolInputSchema{}
	tool.RawInputSchema = raw
	s.AddTool(tool, func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		raw, e := json.Marshal(req.GetRawArguments())
		if e != nil {
			return nil, e
		}
		q, e := annotations.Decode(raw)
		if e != nil {
			return nil, e
		}
		backend, ok := d.CKG.(ckgclient.AnnotationClient)
		if !ok {
			return mcpgo.NewToolResultStructuredOnly(annotations.Result{SchemaVersion: 1, Status: "unavailable", Annotations: []annotations.Annotation{}, SafetyVerdict: "unknown", AutomaticRules: "separate_opt_in_rule_candidates_not_executed", Limitations: []string{"retained annotation backend unsupported"}}), nil
		}
		result, e := backend.PolicySecurity(ctx, q)
		if e != nil {
			return nil, e
		}
		return mcpgo.NewToolResultStructuredOnly(result), nil
	})
}
