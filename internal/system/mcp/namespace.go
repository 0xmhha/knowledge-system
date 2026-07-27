package mcp

import kmcp "github.com/0xmhha/knowledge-system/pkg/mcp"

// namespaceRoot is the deploy-time tool-namespace root for the fused system
// server. The engine default is "cks" — the historical convention — so
// existing clients are unaffected. A deployment injects a different root via
// -ldflags or KNOWLEDGE_MCP_NAMESPACE (see pkg/mcp for the precedence rule).
var namespaceRoot = kmcp.Root("", "cks")

// toolName returns the client-visible tool name for a base name like
// "context.get_for_task" under the current namespace root.
func toolName(base string) string { return kmcp.Name(namespaceRoot, base) }

// DefaultInstanceName is the fallback instance identity when a config leaves
// name empty. It resolves to the deployment namespace root (the same value the
// tool names use), so a branded deployment reports a consistent identity
// instead of the literal "cks".
func DefaultInstanceName() string { return namespaceRoot }
