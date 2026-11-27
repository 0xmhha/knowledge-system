package mcphandlers

import (
	"encoding/json"

	mcp "github.com/mark3labs/mcp-go/mcp"
)

// params.go gives every argument a description: a client shows a model the
// parameter descriptions with the tool, and an argument without one left it
// to guess the form (a qualified name or a canonical id, the default, the
// limit). A tool's own description of an argument wins; these fill the
// arguments a registration declares without one.

// commonParams describe the arguments several tools share, by name.
var commonParams = map[string]string{
	"qname":         "the symbol: a canonical id, a qualified name (\"pkg.Type.Method\") or a bare name; an ambiguous name returns candidates, each a name that opens one node",
	"seed_qname":    "the seed symbol: a canonical id, a qualified name or a bare name, as qname in the other tools",
	"name":          "the symbol's name: with exact=true a qualified name or a canonical id (a bare name that no qualified name equals is resolved as in every tool); with exact=false a suffix of qualified names",
	"depth":         "hops to walk (default 2): one hop past the direct neighbours finds a caller behind a helper; each further hop multiplies the answer by the fan-out for little more of what is asked",
	"include_blobs": "true adds each node's source text (default false)",
	"language":      "only nodes of this language: go, ts, sol or proto (default all)",
	"limit":         "the most rows to return (default 100); total and truncated say when there were more",
}

// toolParams describe the arguments of one tool, by tool and argument.
var toolParams = map[string]map[string]string{
	"find_symbol": {"exact": "true (default): name is a qualified name or a canonical id; false: name is a suffix of qualified names, in the case written"},
	"find_references": {
		"limit": "the most users to return (default 100); total says how many there are",
	},
	"get_subgraph":         {"depth": "hops to walk in both directions (default 2), over every edge type"},
	"impact_of_change":     {"seed_file": "a file path as the graph has it (relative to the source root): every declaration of the file is a seed; seed_qname wins when both are given", "depth": "hops of reverse dependency to follow (default 2)"},
	"concurrency_impact":   {"depth": "hops of calls to follow from the seed (default 2)", "max_total": "the most modules (the per-package groups of the answer) to return (default 0: no limit)"},
	"change_history":       {"k": "the most PRs to return (default 20)", "cutoff": "an RFC3339 time: only PRs merged strictly before it"},
	"search_text":          {"query": "words of the names to find, or names joined by |; any other regular expression is run by search_code", "top_k": "the most nodes to return (default 10)", "mode": "or (default): any word; and: every word in each hit", "node_kinds": "node types to search (default: declarations only, not statements, imports, local names or External stubs)"},
	"investigate_open":     {"open": "what to open: \"card:<name>\" (the card of a declaration), \"rest\" (the blocks an investigate result left out, in order) or a handle it listed"},
	"get_context_for_task": {"task_description": "the task in words; matched against names, signatures and doc comments", "budget_tokens": "the most tokens of the answer (default 8000)", "hop_depth": "hops to expand from the matches (default 1, at most 3)", "impact_depth": "hops of reverse dependency with include_impact (default 1)", "include_blobs": "true (default) adds the source of the top bodies", "include_impact": "true adds what depends on the first match (default false)", "include_recent_prs": "true adds the merged PRs that changed the bodies (default false)", "max_bodies": "how many matches come with their source (default 5)", "prs_per_node": "the most PRs per body with include_recent_prs (default 3)"},
	"rule_candidates":      {"function": "the function to review: node_id or canonical_id (from find_symbol), with optional expected_* pins and limits", "rule_pack": "the rule pack: go-local-review@1.0.0", "max_candidates": "the most candidates to return (1 to 64)", "max_flow_steps": "the most value-flow steps a candidate may take (1 to 4096)"},
	"value_dependencies":   {"node_id": "the function's node id (from find_symbol); give it or canonical_id, not both", "canonical_id": "the function's canonical id; give it or node_id, not both", "expected_commit": "fail unless the graph was built from this commit", "expected_snapshot_sha256": "fail unless the graph snapshot has this hash (snapshot_sha256 of an earlier answer)", "expected_source_sha256": "fail unless the function's source has this hash", "limits": "caps of the analysis (AST nodes, blocks, edges, facts, iterations, nodes, source bytes, timeout_ms); an answer that reaches one says partial"},
}

// describeParams fills the descriptions the arguments of tool lack.
func describeParams(tool mcp.Tool) mcp.Tool {
	describe := func(props map[string]any) {
		for name, v := range props {
			p, ok := v.(map[string]any)
			if !ok {
				continue
			}
			if d, _ := p["description"].(string); d != "" {
				continue
			}
			if text, ok := toolParams[baseToolName(tool.Name)][name]; ok {
				p["description"] = text
			} else if text, ok := commonParams[name]; ok {
				p["description"] = text
			}
		}
	}
	if len(tool.RawInputSchema) > 0 {
		var schema map[string]any
		if json.Unmarshal(tool.RawInputSchema, &schema) == nil {
			if props, ok := schema["properties"].(map[string]any); ok {
				describe(props)
				if b, err := json.Marshal(schema); err == nil {
					tool.RawInputSchema = b
				}
			}
		}
		return tool
	}
	describe(tool.InputSchema.Properties)
	return tool
}
