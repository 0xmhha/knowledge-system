package semanticcli

import (
	"encoding/json"

	"github.com/spf13/cobra"

	"github.com/0xmhha/knowledge-system/internal/system/semantic"
)

func newTraceCmd() *cobra.Command {
	var project, repo, graph, vector, storePath string
	cmd := &cobra.Command{Use: "trace", Short: "Report reviewed requirement-to-code-to-test paths in the active dataset",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := semantic.OpenStore(storePath)
			if err != nil {
				return err
			}
			defer store.Close()
			report, err := store.TraceAligned(cmd.Context(), project, repo, graph, vector)
			if err != nil {
				return err
			}
			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			return encoder.Encode(report)
		},
	}
	cmd.Flags().StringVar(&project, "project-id", "", "active project identifier")
	cmd.Flags().StringVar(&repo, "repo", "", "Git repository containing the cited commit")
	cmd.Flags().StringVar(&graph, "graph", "", "CKG data directory")
	cmd.Flags().StringVar(&vector, "vector", "", "CKV data directory")
	cmd.Flags().StringVar(&storePath, "store", "", "CKS semantic SQLite path")
	for _, flag := range []string{"project-id", "repo", "graph", "vector", "store"} {
		_ = cmd.MarkFlagRequired(flag)
	}
	return cmd
}
