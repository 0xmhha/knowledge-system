package main

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

// ckv identity resolves the exact embedding space before a coordinated
// build. Setup can calculate dataset_id from this input before writing either
// engine; CKV then reports the same identity in its completed manifest.
func newIdentityCmd() *cobra.Command {
	return &cobra.Command{Use: "identity", Short: "Print the current embedding-space identity as JSON",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			emb, cleanup, err := resolveEmbedder(globalFlags.embedder, globalFlags.modelDir)
			if err != nil {
				return err
			}
			defer cleanup()
			id := emb.Identity()
			var value any = id
			if id.Version < 2 {
				value = struct {
					Model    string `json:"model"`
					Dim      int    `json:"dim"`
					Checksum string `json:"checksum"`
				}{emb.Name(), emb.Dimension(), id.Checksum()}
			}
			if id.Provider == "ollama" && id.ModelDigest == "" {
				return fmt.Errorf("Ollama model digest missing")
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{"embedding_identity": value})
		},
	}
}
