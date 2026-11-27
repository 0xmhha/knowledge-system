package main

import (
	"errors"
	"slices"

	"github.com/spf13/cobra"
)

// errIncrementalDisabled is what `ckg watch` and `ckg bench-index` answer:
// both depended on the incremental build, which was removed.
var errIncrementalDisabled = errors.New("the incremental build is disabled: it did not reproduce the cold graph " +
	"(11 of 13 measured edits gave a different graph) and was slower than a cold build; " +
	"run `ckg build` again after a change, which always builds the whole graph")

// helpArgs are the arguments that ask a command for its help.
var helpArgs = []string{"-h", "--help", "help"}

// disabledCommand is a command that no longer runs: its help (-h, --help)
// prints and exits 0, saying it is disabled and why; any other call fails
// with err. Flags are not parsed, so old invocations reach err instead of
// a flag error.
func disabledCommand(use, short, long string, err error) *cobra.Command {
	return &cobra.Command{
		Use:                use,
		Short:              short,
		Long:               long,
		Args:               cobra.ArbitraryArgs,
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			for _, a := range args {
				if slices.Contains(helpArgs, a) {
					return cmd.Help()
				}
			}
			return err
		},
	}
}

// newWatchCmd wires `ckg watch`, which rebuilt graph.db incrementally on
// every change. It refuses to run: the incremental build is disabled
// (errIncrementalDisabled); rebuild with `ckg build` instead.
func newWatchCmd() *cobra.Command {
	return disabledCommand("watch",
		"Disabled: rebuilt graph.db incrementally on change; use ckg build",
		`Disabled. ckg watch rebuilt graph.db with the incremental build on every
file change. The incremental build did not reproduce the graph a cold build
gives and was slower than a cold build, so it was removed. Run ckg build
after a change: every build parses every file.`, errIncrementalDisabled)
}
