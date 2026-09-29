package semanticcli

import (
	"encoding/json"

	"github.com/0xmhha/knowledge-system/internal/system/semantic"
	"github.com/spf13/cobra"
)

func newExportTextCmd() *cobra.Command {
	var project, repo, graph, vector, storePath, output string
	cmd := &cobra.Command{Use: "export-text", Short: "Render verified semantic records as a versioned CKV Markdown corpus",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			store, err := semantic.OpenStore(storePath)
			if err != nil {
				return err
			}
			defer store.Close()
			active, err := store.CurrentAligned(cmd.Context(), project, repo, graph, vector)
			if err != nil {
				return err
			}
			manifest, err := active.ExportTextCorpus(output, repo)
			if err != nil {
				return err
			}
			encoder := json.NewEncoder(cmd.OutOrStdout())
			encoder.SetIndent("", "  ")
			return encoder.Encode(manifest)
		},
	}
	for _, flag := range []struct {
		name   string
		target *string
		usage  string
	}{
		{"project-id", &project, "active project identifier"},
		{"repo", &repo, "Git repository containing the cited commit"},
		{"graph", &graph, "CKG data directory"},
		{"vector", &vector, "CKV data directory"},
		{"store", &storePath, "CKS semantic SQLite path"},
		{"out", &output, "new directory for verified Markdown and manifest"},
	} {
		cmd.Flags().StringVar(flag.target, flag.name, "", flag.usage)
		_ = cmd.MarkFlagRequired(flag.name)
	}
	return cmd
}
