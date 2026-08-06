// Command cks is the system engine's CLI: everything that composes the
// graph and vector engines lives under one command tree.
//
//	cks mcp                    serve the fused MCP server (foreground)
//	cks mcp up|down|...        manage supervised instances
//	cks mcp gen-config         write a runtime config from flags
//	cks mcp client-config      print an MCP client registration entry
//	cks domain <tool>          domain-knowledge toolchain
//	cks agent                  compose LLM-ready context for a task
//	cks eval                   scenario evaluation harness
//
// See docs/design/cli-consolidation.md for the full command map.
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/0xmhha/knowledge-system/cmd/cks/agentcli"
	"github.com/0xmhha/knowledge-system/cmd/cks/domaincli"
	"github.com/0xmhha/knowledge-system/cmd/cks/evalcli"
	"github.com/0xmhha/knowledge-system/cmd/cks/evalgatecli"
	"github.com/0xmhha/knowledge-system/cmd/cks/filelistcli"
	"github.com/0xmhha/knowledge-system/cmd/cks/mcpcli"
	"github.com/0xmhha/knowledge-system/cmd/cks/setupcli"
)

// builderVersion is stamped into the MCP server handshake and
// cks.ops.health responses. Override at build time:
//
//	go build -ldflags "-X main.builderVersion=cks-mcp/0.1.0-$(git rev-parse --short HEAD)"
var builderVersion = "cks-mcp/0.0.1-dev"

func main() {
	root := &cobra.Command{
		Use:           "cks",
		Short:         "System engine: fused graph+vector retrieval and its toolchain",
		Version:       builderVersion,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(mcpcli.NewCmd(builderVersion))
	root.AddCommand(domaincli.NewCmd())
	root.AddCommand(agentcli.NewCmd())
	root.AddCommand(evalcli.NewCmd())
	root.AddCommand(setupcli.NewCmd())
	root.AddCommand(filelistcli.NewCmd())
	root.AddCommand(evalgatecli.NewCmd())

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "cks: %v\n", err)
		os.Exit(1)
	}
}
