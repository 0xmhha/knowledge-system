package mcpcli

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/0xmhha/knowledge-system/internal/system/comparison"
	cksmcp "github.com/0xmhha/knowledge-system/internal/system/mcp"
	"github.com/spf13/cobra"
)

func newComparisonCmd() *cobra.Command {
	var profile string
	cmd := &cobra.Command{Use: "comparison", Short: "Serve explicit read-only project comparison over stdio", Args: cobra.NoArgs, RunE: func(cmd *cobra.Command, _ []string) error {
		if profile == "" {
			return fmt.Errorf("comparison: --profile is required")
		}
		ctx, cancel := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
		defer cancel()
		runtime, err := comparison.OpenComparisonRuntime(ctx, profile)
		if err != nil {
			return err
		}
		defer func() { _ = runtime.Close() }()
		return cksmcp.RunComparison(ctx, runtime, builderVersion, cmd.InOrStdin(), cmd.OutOrStdout())
	}}
	cmd.Flags().StringVar(&profile, "profile", "", "absolute path to explicit comparison profile JSON")
	return cmd
}
