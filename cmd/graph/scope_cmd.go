package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/0xmhha/knowledge-system/internal/buildscope"
)

// newScopeCmd is `ckg scope`: extract the build scope a graph is built from.
// Run it before `ckg build --scope` with the options the build uses; a
// graph built without a scope reads every file of the tree.
//
//	ckg scope --src ~/src/go-stablenet --profile projects/stablenet/filelist.yaml \
//	  --tags urfave_cli_no_docs,ckzg --binary build/bin/gstable --out scope.json
//	ckg build --src ~/src/go-stablenet --scope scope.json --out graph/
func newScopeCmd() *cobra.Command {
	var src, profile, configID, goos, goarch, cgo, tags, out, goBin string
	var entries, includes, binaries []string
	var allowProblems bool
	cmd := &cobra.Command{
		Use:   "scope",
		Short: "Extract the build scope (the files one build compiles) for ckg build --scope",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if cgo != "" && cgo != "0" && cgo != "1" {
				return fmt.Errorf("--cgo must be 0 or 1")
			}
			if profile == "" && len(entries) == 0 {
				return fmt.Errorf("give --profile or --entry")
			}
			opts := buildscope.Options{GOOS: goos, GOARCH: goarch, CGOEnabled: cgo, EntryPoints: entries, ExtraGlobs: includes}
			if cmd.Flags().Changed("tags") {
				opts.Tags, opts.TagsSet = splitList(tags), true
			}
			s, err := buildscope.ExtractProject(cmd.Context(), buildscope.ProjectConfig{
				Root: src, ConfigID: configID, ProfilePath: profile, Options: opts,
				Binaries: binaries, AllowProblems: allowProblems, GoBinary: goBin,
			})
			if err != nil {
				return err
			}
			if err := buildscope.Write(out, s); err != nil {
				return fmt.Errorf("write scope: %w", err)
			}
			_, _ = fmt.Fprintf(os.Stderr, "ckg: scope %s: %d files for %s/%s cgo=%s tags=%v, %d binaries checked\n",
				out, len(s.Files), s.Target.GOOS, s.Target.GOARCH, s.Target.CGOEnabled, s.Target.Tags, len(s.CheckedBinaries))
			return nil
		},
	}
	cmd.Flags().StringVar(&src, "src", ".", "project root (the directory of the main go.mod)")
	cmd.Flags().StringVar(&profile, "profile", "", "build profile YAML (projects/<pack>/filelist.yaml); the flags below override it")
	cmd.Flags().StringVar(&configID, "config-id", "", "configuration id recorded in the scope")
	cmd.Flags().StringVar(&goos, "goos", "", "target GOOS of the build")
	cmd.Flags().StringVar(&goarch, "goarch", "", "target GOARCH of the build")
	cmd.Flags().StringVar(&cgo, "cgo", "", "CGO_ENABLED of the build (0 or 1)")
	cmd.Flags().StringVar(&tags, "tags", "", "comma-separated build tags of the build; given, even empty, it replaces the profile's tags")
	cmd.Flags().StringSliceVar(&entries, "entry", nil, "entry packages the build builds (e.g. ./cmd/gstable); replaces the profile's build_roots")
	cmd.Flags().StringSliceVar(&includes, "include", nil, "glob of non-Go files to add (e.g. contracts/**/*.sol); added to the profile's extra_globs")
	cmd.Flags().StringSliceVar(&binaries, "binary", nil, "built binary to check the scope against (go version -m); a mismatch fails")
	cmd.Flags().BoolVar(&allowProblems, "allow-problems", false, "write the scope even when the go command reports load errors")
	cmd.Flags().StringVar(&goBin, "go", "", "go command to run (go from PATH when empty)")
	cmd.Flags().StringVar(&out, "out", "", `scope JSON to write ("-" for standard output) (required)`)
	_ = cmd.MarkFlagRequired("out")
	return cmd
}

func splitList(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
