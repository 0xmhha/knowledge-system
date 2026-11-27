// Package mcp — bench.go exposes the registered tool handlers for
// in-process latency measurement. Production stdio serving still
// goes through Run; this entrypoint exists so cmd/ckg/bench-mcp can
// invoke each handler directly (no subprocess spawn, no JSON-RPC
// framing) and surface the graph layer's contribution to MCP tool
// latency separately from the stdio transport's.
//
// The §11.3 retrieval boundary is preserved: mcphandlers.RegisterAll
// wraps the store before threading. AMBIGUOUS Hunk/Commit nodes never
// reach the bench callers.
package mcp

import (
	"maps"
	"slices"

	server "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/mcphandlers"
)

// NewBenchHandlers builds a fresh MCPServer with every tool registered
// through mcphandlers.RegisterAll, then returns the handler map keyed by
// the client-visible tool name. The list of tools is the registry's, so a
// tool added to RegisterAll is benched without a change here. The
// MCPServer is returned alongside so the caller holds the reference for
// the run.
func NewBenchHandlers(store persist.StoreReader) (*server.MCPServer, map[string]server.ToolHandlerFunc) {
	s := server.NewMCPServer("ckg-bench", "0.0.0")
	mcphandlers.RegisterAll(s, store)
	tools := s.ListTools()
	handlers := make(map[string]server.ToolHandlerFunc, len(tools))
	for name, t := range tools {
		if t == nil || t.Handler == nil {
			continue
		}
		handlers[name] = t.Handler
	}
	return s, handlers
}

// SortedToolNames returns the tool names of handlers in sorted order, the
// order bench output uses so runs compare row by row.
func SortedToolNames(handlers map[string]server.ToolHandlerFunc) []string {
	return slices.Sorted(maps.Keys(handlers))
}
