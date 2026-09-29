// Package semanticcli exposes the source-backed semantic review surface.
package semanticcli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func NewCmd() *cobra.Command {
	var input, repo, salt string
	var sample int
	cmd := &cobra.Command{Use: "semantic", Short: "Inspect source-backed semantic projections"}
	review := &cobra.Command{
		Use: "review", Short: "Validate sources and sample proposed claims, relations, and concepts",
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
	cmd.AddCommand(newBuildCmd())
	cmd.AddCommand(newPromoteCmd())
	cmd.AddCommand(newTraceCmd())
	return cmd
}

func runReview(ctx context.Context, cmd *cobra.Command, input, repo, salt string, sample int) error {
	if sample < 0 {
		return fmt.Errorf("sample must be non-negative")
	}
	p, err := readProjection(input)
	if err != nil {
		return err
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
