// Command buildscope prints, as JSON, the files one build configuration
// compiles and the test files that belong to them (internal/buildscope).
//
// Usage:
//
//	buildscope -root ~/src/go-stablenet -profile projects/stablenet/filelist.yaml \
//	  -tags urfave_cli_no_docs,ckzg -binary build/bin/gstable -out scope.json
//	buildscope -root ~/src/go-stablenet -goos linux -goarch amd64 -cgo 1 ./cmd/gstable
//
// It is the same extraction as `ckg scope`. Exit status is 1 when the scope
// cannot be built, when the go command reports load errors (unless
// -allow-problems) and when a -binary was built with other options.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/0xmhha/knowledge-system/internal/buildscope"
)

func main() {
	root := flag.String("root", ".", "directory of the main module's go.mod")
	config := flag.String("config", "", "configuration id recorded in the output")
	profile := flag.String("profile", "", "build profile YAML (projects/<pack>/filelist.yaml); the flags below override it")
	goos := flag.String("goos", "", "target GOOS (the profile's, else the host's)")
	goarch := flag.String("goarch", "", "target GOARCH (the profile's, else the host's)")
	cgo := flag.String("cgo", "", "CGO_ENABLED for the build (1 or 0); the go default when unset, which is 0 for a cross build")
	tags := flag.String("tags", "", "comma-separated build tags; given, even empty, it replaces the profile's tags")
	include := flag.String("include", "", "comma-separated globs of non-Go files to add")
	binaries := flag.String("binary", "", "comma-separated built binaries to check the scope against")
	allowProblems := flag.Bool("allow-problems", false, "write the scope even when the go command reports load errors")
	out := flag.String("out", "-", `file to write ("-" for standard output)`)
	goBin := flag.String("go", "", "go command to run (go from PATH when empty)")
	flag.Usage = func() {
		_, _ = fmt.Fprintln(os.Stderr, "usage: buildscope [flags] [entry package]...")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 0 && *profile == "" {
		flag.Usage()
		os.Exit(2)
	}
	opts := buildscope.Options{GOOS: *goos, GOARCH: *goarch, CGOEnabled: *cgo, EntryPoints: flag.Args(), ExtraGlobs: split(*include)}
	flag.Visit(func(f *flag.Flag) {
		if f.Name == "tags" {
			opts.Tags, opts.TagsSet = split(*tags), true
		}
	})
	scope, err := buildscope.ExtractProject(context.Background(), buildscope.ProjectConfig{
		Root: *root, ConfigID: *config, ProfilePath: *profile, Options: opts,
		Binaries: split(*binaries), AllowProblems: *allowProblems, GoBinary: *goBin,
	})
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := buildscope.Write(*out, scope); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func split(s string) []string {
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
