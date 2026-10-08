package semanticcli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/0xmhha/knowledge-system/internal/system/semantic"
	"github.com/0xmhha/knowledge-system/pkg/system/contract"
	"github.com/spf13/cobra"
)

func newAnnotatePackCmd() *cobra.Command {
	var project, repo, graph, vector, storePath, input, output string
	cmd := &cobra.Command{Use: "annotate-pack", Short: "Attach reviewed semantic trace IDs to an existing EvidencePack",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			bytes, err := os.ReadFile(input)
			if err != nil {
				return err
			}
			var pack contract.EvidencePack
			if err := json.Unmarshal(bytes, &pack); err != nil {
				return err
			}
			store, err := semantic.OpenStore(storePath)
			if err != nil {
				return err
			}
			defer store.Close()
			active, err := store.CurrentAligned(cmd.Context(), project, repo, graph, vector)
			if err != nil {
				return err
			}
			if err := active.AnnotatePack(&pack); err != nil {
				return err
			}
			data, err := json.MarshalIndent(pack, "", "  ")
			if err != nil {
				return err
			}
			file, err := os.OpenFile(output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
			if err != nil {
				return fmt.Errorf("create annotated pack: %w", err)
			}
			if _, err := file.Write(append(data, '\n')); err != nil {
				file.Close()
				return err
			}
			if err := file.Close(); err != nil {
				return err
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), output)
			return err
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
		{"input", &input, "existing EvidencePack JSON"},
		{"out", &output, "new annotated pack path"},
	} {
		cmd.Flags().StringVar(flag.target, flag.name, "", flag.usage)
		_ = cmd.MarkFlagRequired(flag.name)
	}
	return cmd
}
