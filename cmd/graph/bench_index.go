package main

import (
	"github.com/spf13/cobra"
)

// newBenchIndexCmd wires `ckg bench-index`, which compared a full build
// with incremental rebuilds. It refuses to run: the incremental build is
// disabled (errIncrementalDisabled); time `ckg build` for the build time.
func newBenchIndexCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "bench-index",
		Short: "Disabled: measured full vs incremental build time; time ckg build instead",
		Long: `Disabled. ckg bench-index compared a full build with incremental rebuilds.
The incremental build was removed (it did not reproduce the cold graph and
was slower than a cold build); every ckg build is a full build, so time ckg
build for the build time.`,
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true,
		RunE: func(*cobra.Command, []string) error {
			return errIncrementalDisabled
		},
	}
}
