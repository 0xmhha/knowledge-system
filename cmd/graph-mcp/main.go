// graph-mcp is the standalone MCP stdio server for the graph engine: it
// serves the graph tool set (find_symbol, find_callers, search_text, ...)
// from a built graph directory, without the vector engine or the fused
// system pipeline. Use it when an agent only needs code-graph retrieval.
//
// The client-visible tool names follow the shared namespace rule (see
// pkg/mcp): bare names by default, <root>.context.<name> when a namespace
// root is injected via --namespace, KNOWLEDGE_MCP_NAMESPACE, or -ldflags.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	server "github.com/mark3labs/mcp-go/server"

	"github.com/0xmhha/knowledge-system/graph/pkg/mcphandlers"
	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	kmcp "github.com/0xmhha/knowledge-system/pkg/mcp"
)

const serverVersion = "0.1.0"

func main() {
	graphDir := flag.String("graph", "", "graph data directory holding graph.db (required)")
	name := flag.String("name", "knowledge-graph", "MCP server name reported in the handshake")
	namespace := flag.String("namespace", "", "tool-namespace root override (default: env/build-time, else bare names)")
	flag.Parse()

	if err := run(*graphDir, *name, *namespace); err != nil {
		fmt.Fprintf(os.Stderr, "graph-mcp: %v\n", err)
		os.Exit(1)
	}
}

func run(graphDir, name, namespace string) error {
	if graphDir == "" {
		return fmt.Errorf("--graph is required")
	}
	db := filepath.Join(graphDir, "graph.db")
	store, err := persist.OpenReadOnly(db)
	if err != nil {
		return fmt.Errorf("open graph: %w", err)
	}
	defer func() { _ = store.Close() }()

	mcphandlers.SetNamespaceRoot(kmcp.Root(namespace, ""))
	s := server.NewMCPServer(name, serverVersion)
	mcphandlers.RegisterAll(s, store)

	fmt.Fprintf(os.Stderr, "graph-mcp: stdio server bound to %s\n", db)
	if err := server.ServeStdio(s); err != nil {
		return fmt.Errorf("serve stdio: %w", err)
	}
	return nil
}
