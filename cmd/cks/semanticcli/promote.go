package semanticcli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/0xmhha/knowledge-system/internal/system/semantic"
)

func newPromoteCmd() *cobra.Command {
	var input, repo, graph, vector, storePath, versionDir string
	var minimumCoverage float64
	var activate bool
	cmd := &cobra.Command{
		Use: "promote", Short: "Validate and store a reviewed semantic projection against CKV and CKG",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := semantic.ValidateCanonicalCoverage(vector, minimumCoverage); err != nil {
				return err
			}
			p, err := readProjection(input)
			if err != nil {
				return err
			}
			root, err := filepath.Abs(repo)
			if err != nil {
				return err
			}
			if err := semantic.ValidateDatasetAlignment(p, root, graph, vector); err != nil {
				return err
			}
			if err := semantic.ValidateCodeAnchors(p, graph); err != nil {
				return err
			}
			validateSource := func() error { return p.ValidateSources(cmd.Context(), root) }
			if versionDir != "" {
				validateSource = func() error { return p.ValidateRetainedSources(versionDir) }
			}
			if err := validateSource(); err != nil {
				return err
			}
			store, err := semantic.OpenStore(storePath)
			if err != nil {
				return err
			}
			defer store.Close()
			put := func() error { return store.PutAligned(cmd.Context(), p, root, graph, vector) }
			if versionDir != "" {
				put = func() error { return store.PutAlignedRetained(cmd.Context(), p, root, graph, vector, versionDir) }
			}
			if err := put(); err != nil {
				return err
			}
			if activate {
				activateCandidate := func() error {
					return store.ActivateAligned(cmd.Context(), p.Snapshot.ProjectID, p.Snapshot.DatasetID, root, graph, vector)
				}
				if versionDir != "" {
					activateCandidate = func() error {
						return store.ActivateAlignedRetained(cmd.Context(), p.Snapshot.ProjectID, p.Snapshot.DatasetID, root, graph, vector, versionDir)
					}
				}
				if err := activateCandidate(); err != nil {
					return err
				}
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(map[string]any{
				"project_id": p.Snapshot.ProjectID, "dataset_id": p.Snapshot.DatasetID,
				"claims": len(p.Claims), "assertions": len(p.Assertions),
				"concepts": len(p.Concepts), "activated": activate,
			})
		},
	}
	cmd.Flags().StringVar(&input, "input", "", "reviewed projection JSON")
	cmd.Flags().StringVar(&repo, "repo", "", "source Git repository")
	cmd.Flags().StringVar(&graph, "graph", "", "CKG data directory")
	cmd.Flags().StringVar(&vector, "vector", "", "CKV data directory")
	cmd.Flags().StringVar(&storePath, "store", "", "CKS semantic SQLite path")
	cmd.Flags().StringVar(&versionDir, "version-dir", "", "pinned retained source candidate for mutable or held projection review")
	cmd.Flags().Float64Var(&minimumCoverage, "min-canonical-ratio", 0, "measured project minimum for CKV-to-CKG symbol alignment (0 disables)")
	cmd.Flags().BoolVar(&activate, "activate", false, "activate this dataset after validation")
	for _, flag := range []string{"input", "repo", "graph", "vector", "store"} {
		_ = cmd.MarkFlagRequired(flag)
	}
	return cmd
}

func readProjection(path string) (semantic.Projection, error) {
	f, err := os.Open(path)
	if err != nil {
		return semantic.Projection{}, err
	}
	defer f.Close()
	var p semantic.Projection
	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&p); err != nil {
		return semantic.Projection{}, fmt.Errorf("decode projection: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return semantic.Projection{}, fmt.Errorf("projection has trailing JSON data")
	} else if !errors.Is(err, io.EOF) {
		return semantic.Projection{}, fmt.Errorf("projection has trailing invalid data: %w", err)
	}
	return p, nil
}
