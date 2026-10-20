package setupcli

import (
	"github.com/0xmhha/knowledge-system/internal/setup"
	"github.com/spf13/cobra"
)

// NewDatasetStepCmd is a private parent-liveness supervisor, not a public
// maintenance operation. It requires an inherited Unix owner channel.
func NewDatasetStepCmd() *cobra.Command {
	return &cobra.Command{Use: "__dataset-step", Hidden: true, Args: cobra.ExactArgs(1), RunE: func(_ *cobra.Command, args []string) error { return setup.RunGuardedDatasetStep(args[0]) }}
}
