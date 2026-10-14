package mcphandlers

import mcp "github.com/mark3labs/mcp-go/mcp"

func valueFlowOutputSchema() mcp.ToolOutputSchema {
	text := map[string]any{"type": "string"}
	integer := map[string]any{"type": "integer", "minimum": 0}
	object := func(properties map[string]any, required ...string) map[string]any {
		result := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
		if len(required) > 0 {
			result["required"] = required
		}
		return result
	}
	array := func(item any) map[string]any { return map[string]any{"type": "array", "items": item} }
	location := object(map[string]any{"start_byte": integer, "end_byte": integer, "start_line": integer, "end_line": integer}, "start_byte", "end_byte", "start_line", "end_line")
	source := object(map[string]any{"node_id": text, "canonical_id": text, "file": text, "commit": text, "sha256": text, "language": text, "start_line": integer, "start_byte": integer, "bytes": integer, "snapshot_sha256": text, "function_name": text}, "node_id", "file", "sha256", "language", "start_line", "start_byte", "bytes")
	options := map[string]any{}
	for _, key := range []string{"max_source_bytes", "max_ast_nodes", "max_blocks", "max_nodes", "max_edges", "max_iterations", "max_facts", "timeout_ms"} {
		options[key] = integer
	}
	node := object(map[string]any{"id": text, "block": map[string]any{"type": "integer", "minimum": -1}, "kind": text, "location": location, "text": text, "synthetic": map[string]any{"type": "boolean"}}, "id", "block", "kind", "location", "text", "synthetic")
	block := object(map[string]any{"id": integer, "kind": text, "reachable": map[string]any{"type": "boolean"}, "nodes": array(text)}, "id", "kind", "reachable", "nodes")
	edge := object(map[string]any{"from": text, "to": text, "kind": text, "condition": text, "certainty": text, "variable": text}, "from", "to", "kind", "certainty")
	variable := object(map[string]any{"id": text, "name": text, "kind": text, "declaration": location}, "id", "name", "kind", "declaration")
	occurrence := object(map[string]any{"id": text, "node": text, "variable": text, "role": text, "location": location, "reachable": map[string]any{"type": "boolean"}}, "id", "node", "variable", "role", "location", "reachable")
	defuse := object(map[string]any{"definition": text, "use": text, "variable": text, "certainty": text}, "definition", "use", "variable", "certainty")
	diagnostic := object(map[string]any{"code": text, "message": text, "location": location}, "code", "message")
	analysis := object(map[string]any{"schema_version": map[string]any{"type": "integer", "const": 1}, "source": source, "limits": object(options), "status": text, "capability": map[string]any{"type": "object", "additionalProperties": text}, "blocks": array(block), "nodes": array(node), "edges": array(edge), "diagnostics": array(diagnostic), "truncated": map[string]any{"type": "boolean"}, "elapsed_micros": integer, "variables": array(variable), "occurrences": array(occurrence), "def_use": array(defuse), "iterations": integer, "facts_peak": integer}, "schema_version", "source", "limits", "status", "capability", "blocks", "nodes", "edges", "diagnostics", "truncated", "elapsed_micros", "variables", "occurrences", "def_use", "iterations", "facts_peak")
	return mcp.ToolOutputSchema{Type: "object", AdditionalProperties: false, Required: []string{"schema_version", "status", "analysis", "elapsed_micros"}, Properties: map[string]any{"schema_version": map[string]any{"type": "integer", "const": 1}, "status": map[string]any{"type": "string", "enum": []string{"analyzed_bounded", "partial", "budget_exhausted", "unsupported", "source_mismatch", "snapshot_changed", "unavailable", "not_found", "ambiguous"}}, "reason": text, "snapshot_sha256": text, "elapsed_micros": integer, "analysis": map[string]any{"anyOf": []any{map[string]any{"type": "null"}, analysis}}}}
}
