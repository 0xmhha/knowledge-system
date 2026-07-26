package mcp

import kmcp "github.com/0xmhha/knowledge-system/pkg/mcp"

// namespaceRoot is the deploy-time tool-namespace root for the vector
// engine's MCP surface. The engine default is "cks" — the historical
// fused-server convention this server has always spoken (cks.context.*,
// cks.ops.*) — so existing clients are unaffected. A deployment injects a
// different root via -ldflags, KNOWLEDGE_MCP_NAMESPACE, or SetNamespaceRoot
// (see pkg/mcp for the precedence rule).
var namespaceRoot = kmcp.Root("", "cks")

// SetNamespaceRoot overrides the resolved namespace root. Call before
// NewServer; tool names are computed once at registration.
func SetNamespaceRoot(root string) { namespaceRoot = root }

// nsName returns the client-visible tool name for a base name like
// "context.semantic_search" under the current namespace root.
func nsName(base string) string { return kmcp.Name(namespaceRoot, base) }
