// Package semanticcli exposes the source-backed semantic review surface.
package semanticcli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/0xmhha/knowledge-system/internal/system/semantic"
)

func NewCmd() *cobra.Command {
	var input, repo, salt string
	var sample int
	cmd := &cobra.Command{Use: "semantic", Short: "Inspect source-backed semantic projections"}
	review := &cobra.Command{
		Use: "review", Short: "Validate source evidence and print a deterministic claim review sample",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runReview(cmd.Context(), cmd, input, repo, salt, sample)
		},
	}
	review.Flags().StringVar(&input, "input", "", "projection JSON file")
	review.Flags().StringVar(&repo, "repo", "", "Git repository containing the cited commit and files")
	review.Flags().StringVar(&salt, "salt", "", "stable sample selection salt")
	review.Flags().IntVar(&sample, "sample", 20, "maximum proposed claims to sample")
	_ = review.MarkFlagRequired("input")
	_ = review.MarkFlagRequired("repo")
	cmd.AddCommand(review)
	return cmd
}

func runReview(ctx context.Context, cmd *cobra.Command, input, repo, salt string, sample int) error {
	if sample < 0 {
		return fmt.Errorf("sample must be non-negative")
	}
	f, err := os.Open(input)
	if err != nil {
		return err
	}
	defer f.Close()
	var p semantic.Projection
	decoder := json.NewDecoder(f)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&p); err != nil {
		return fmt.Errorf("decode projection: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err == nil {
		return fmt.Errorf("projection has trailing JSON data")
	} else if !errors.Is(err, io.EOF) {
		return fmt.Errorf("projection has trailing invalid data: %w", err)
	}
	if err := p.ValidateSources(ctx, repo); err != nil {
		return err
	}
	report, err := p.Review(sample, salt)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(cmd.OutOrStdout())
	encoder.SetIndent("", "  ")
	return encoder.Encode(report)
}
