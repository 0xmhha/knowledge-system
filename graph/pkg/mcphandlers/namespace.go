package mcphandlers

import (
	mcp "github.com/mark3labs/mcp-go/mcp"

	kmcp "github.com/0xmhha/knowledge-system/pkg/mcp"
)

// namespaceRoot is the deploy-time tool-namespace root for the graph
// engine's standalone MCP surface. The engine default is "" — the historical
// bare names (find_symbol, search_text, ...) — so existing clients are
// unaffected. A deployment injects a root via -ldflags, the
// KNOWLEDGE_MCP_NAMESPACE environment variable, or SetNamespaceRoot; the
// client-visible names then become <root>.context.<base>, matching the fused
// system server's convention.
var namespaceRoot = kmcp.Root("", "")

// SetNamespaceRoot overrides the resolved namespace root. Call before any
// Register* function; registration reads the value once per tool.
func SetNamespaceRoot(root string) { namespaceRoot = root }

// ToolName returns the client-visible name for a base tool name under the
// current namespace root. Exported so embedders (bench harness, sibling
// servers) can address tools without duplicating the naming rule.
func ToolName(base string) string {
	if namespaceRoot == "" {
		return base
	}
	return namespaceRoot + ".context." + base
}

// nsTool is mcp.NewTool with the namespace rule applied to the tool name.
// Handlers pass the bare base name (e.g. "find_symbol") for readability.
func nsTool(base string, opts ...mcp.ToolOption) mcp.Tool {
	return mcp.NewTool(ToolName(base), opts...)
}
