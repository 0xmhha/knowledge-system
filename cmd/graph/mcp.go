package main

import (
	"fmt"
	"os"
	"path/filepath"

	server "github.com/mark3labs/mcp-go/server"
	"github.com/spf13/cobra"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/mcphandlers"
	kmcp "github.com/0xmhha/knowledge-system/pkg/mcp"
)

const mcpServerVersion = "0.1.0"

// newMCPCmd serves the graph tool set over MCP stdio. The client-visible
// tool names follow the shared namespace rule (see pkg/mcp): bare names by
// default, <root>.context.<name> when a namespace root is injected via
// --namespace, KNOWLEDGE_MCP_NAMESPACE, or -ldflags.
func newMCPCmd() *cobra.Command {
	var graph, name, namespace string
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run the MCP stdio server",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, cleanup, err := newLogger(rootVerbose, rootLogFile)
			if err != nil {
				return fmt.Errorf("init logger: %w", err)
			}
			defer cleanup()

			db := filepath.Join(graph, "graph.db")
			store, err := persist.OpenReadOnly(db)
			if err != nil {
				return fmt.Errorf("open graph: %w", err)
			}
			defer func() { _ = store.Close() }()

			mcphandlers.SetNamespaceRoot(kmcp.Root(namespace, ""))
			s := server.NewMCPServer(name, mcpServerVersion)
			mcphandlers.RegisterAll(s, store)

			_, _ = fmt.Fprintf(os.Stderr, "ckg mcp: stdio server bound to %s\n", db)
			if err := server.ServeStdio(s); err != nil {
				return fmt.Errorf("serve stdio: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&graph, "graph", "", "graph directory (required)")
	cmd.Flags().StringVar(&name, "name", "knowledge-graph", "MCP server name reported in the handshake")
	cmd.Flags().StringVar(&namespace, "namespace", "", "tool-namespace root override (default: env/build-time, else bare names)")
	_ = cmd.MarkFlagRequired("graph")
	return cmd
}
