// Command buildscope prints, as JSON, the files one build configuration
// compiles and the test files that belong to them (internal/buildscope).
//
// Usage:
//
//	buildscope -root ~/src/go-stablenet -config gstable-linux-amd64 \
//	  -goos linux -goarch amd64 -cgo 1 ./cmd/gstable > scope.json
//
// Exit status is 1 when the scope cannot be built; load problems the go
// command reports are listed in the "problems" field instead.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/0xmhha/knowledge-system/internal/buildscope"
)

func main() {
	root := flag.String("root", ".", "directory of the main module's go.mod")
	config := flag.String("config", "", "configuration id recorded in the output")
	goos := flag.String("goos", "", "target GOOS (host default when empty)")
	goarch := flag.String("goarch", "", "target GOARCH (host default when empty)")
	cgo := flag.String("cgo", "", "CGO_ENABLED for the build (1 or 0); the go default when empty, which is 0 for a cross build")
	tags := flag.String("tags", "", "comma-separated build tags")
	goBin := flag.String("go", "", "go command to run (go from PATH when empty)")
	flag.Usage = func() {
		_, _ = fmt.Fprintln(os.Stderr, "usage: buildscope [flags] <entry package>...")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}
	cfg := buildscope.GoConfig{
		Root: *root, ConfigID: *config, GOOS: *goos, GOARCH: *goarch, CGOEnabled: *cgo,
		EntryPoints: flag.Args(), GoBinary: *goBin,
	}
	if *tags != "" {
		cfg.Tags = strings.Split(*tags, ",")
	}
	scope, err := buildscope.ExtractGo(context.Background(), cfg)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(scope); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
