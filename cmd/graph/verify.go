package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/internal/graph/verify"
)

func newVerifyCmd() *cobra.Command {
	var src, graph, format string
	var samples int
	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Compare a Go graph's declarations, calls and type references with go/types",
		Long: "Loads the source under --src with go/packages and go/types and compares the declarations, static\n" +
			"calls, interface calls and type references it resolves with the nodes and edges of the graph in\n" +
			"--graph. Only Go files recorded in the graph are checked. The report is a measurement; the command\n" +
			"exits 0 unless it cannot run.",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			store, err := persist.OpenReadOnly(filepath.Join(graph, "graph.db"))
			if err != nil {
				return fmt.Errorf("open graph: %w", err)
			}
			defer func() { _ = store.Close() }()
			report, err := verify.RunGo(src, store, verify.Options{Samples: samples})
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			switch format {
			case "json":
				return report.WriteJSON(out)
			case "text":
				return report.WriteText(out)
			default:
				_, _ = fmt.Fprintf(os.Stderr, "ckg: verify: unknown --format %q\n", format)
				return fmt.Errorf("unknown format %q", format)
			}
		},
	}
	cmd.Flags().StringVar(&src, "src", ".", "source root the graph was built from")
	cmd.Flags().StringVar(&graph, "graph", ".ckg", "graph directory holding graph.db")
	cmd.Flags().StringVar(&format, "format", "text", "text or json")
	cmd.Flags().IntVar(&samples, "samples", 20, "examples listed per category")
	return cmd
}
