package main

import (
	"errors"

	"github.com/spf13/cobra"
)

// errIncrementalDisabled is what `ckg watch` and `ckg bench-index` answer:
// both depended on the incremental build, which was removed.
var errIncrementalDisabled = errors.New("the incremental build is disabled: it did not reproduce the cold graph " +
	"(11 of 13 measured edits gave a different graph) and was slower than a cold build; " +
	"run `ckg build` again after a change, which always builds the whole graph")

// newWatchCmd wires `ckg watch`, which rebuilt graph.db incrementally on
// every change. It refuses to run: the incremental build is disabled
// (errIncrementalDisabled); rebuild with `ckg build` instead.
func newWatchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "watch",
		Short: "Disabled: rebuilt graph.db incrementally on change; use ckg build",
		Long: `Disabled. ckg watch rebuilt graph.db with the incremental build on every
file change. The incremental build did not reproduce the graph a cold build
gives and was slower than a cold build, so it was removed. Run ckg build
after a change: every build parses every file.`,
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true,
		RunE: func(*cobra.Command, []string) error {
			return errIncrementalDisabled
		},
	}
}
