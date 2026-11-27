package main

import (
	"fmt"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/execflow"
)

// newFlowCmd is `ckg flow <name>`: the execution_flow tool on the command
// line, with the same answer text.
func newFlowCmd() *cobra.Command {
	var graph, direction, follow, focus, tests, via, from string
	var depth, budget, branches int
	cmd := &cobra.Command{
		Use:   "flow <name>",
		Short: "What happens from a function in execution order, or what leads to it (the execution_flow tool)",
		Long: `Print what happens, in execution order, from a function, method or goroutine
(--direction forward) or what leads to it (--direction backward), across
calls, goroutine starts and channel messages, with the locks held, as a
numbered narrative of source lines. It is the execution_flow tool of
ckg mcp and prints the same text.

Examples:

  ckg flow --graph /tmp/ckg-self eth.Peer.broadcastTransactions
  ckg flow --graph /tmp/ckg-self --direction backward p2p.protoRW.WriteMsg
  ckg flow --graph /tmp/ckg-self --focus total pool.Run`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			links, err := execflow.ParseLinks(follow)
			if err != nil {
				return err
			}
			reader, err := persist.OpenReadOnly(filepath.Join(graph, "graph.db"))
			if err != nil {
				return fmt.Errorf("open graph: %w", err)
			}
			defer func() { _ = reader.Close() }()
			if cmd.Flags().Changed("depth") && depth == 0 {
				depth = execflow.OwnStepsOnly
			}
			text, err := execflow.Trace(reader, execflow.Query{Seed: args[0], Direction: execflow.Direction(direction),
				Follow: links, Focus: focus, Via: via, From: from, Depth: depth, Budget: budget, Branches: branches, Tests: execflow.Tests(tests)})
			if err != nil {
				return fmt.Errorf("flow: %w", err)
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), text)
			return err
		},
	}
	cmd.Flags().StringVar(&graph, "graph", "", "graph directory (required)")
	cmd.Flags().StringVar(&direction, "direction", string(execflow.Forward), "forward (what happens from here) or backward (what leads here)")
	cmd.Flags().StringVar(&follow, "follow", "", "links to follow, comma-separated: calls, messages, goroutines (default all)")
	cmd.Flags().StringVar(&focus, "focus", "", "a field or variable: keep only the steps that read, write or guard it (forward)")
	cmd.Flags().StringVar(&via, "via", "", "keep only the paths through this node")
	cmd.Flags().StringVar(&from, "from", "", "backward: the paths from this node to the seed, shortest first")
	cmd.Flags().IntVar(&depth, "depth", 0, fmt.Sprintf("hops opened beyond the seed's own steps (default %d, %d with --from; 0: the seed's own steps only)", execflow.DefaultDepth, execflow.MaxDepth))
	cmd.Flags().IntVar(&budget, "budget", execflow.DefaultBudget, "steps shown, the nearest first")
	cmd.Flags().IntVar(&branches, "branches", execflow.DefaultBranches, "alternatives shown per branch point")
	cmd.Flags().StringVar(&tests, "tests", string(execflow.TestsExclude), "exclude or include test code")
	_ = cmd.MarkFlagRequired("graph")
	return cmd
}
