package buildpipe_test

import (
	"bytes"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/buildpipe"
	"github.com/0xmhha/knowledge-system/internal/graph/detect"
)

// TestRun_ReportsGoLoadErrors: a package the go command cannot load or
// type-check still gives a graph, so the build must say so: a WARN in the
// load area with samples, and the count in the manifest. A clean module
// reports none.
func TestRun_ReportsGoLoadErrors(t *testing.T) {
	dir := t.TempDir()
	mustWrite(t, filepath.Join(dir, "go.mod"), "module example.com/loaderr\n\ngo 1.21\n")
	mustWrite(t, filepath.Join(dir, "ok.go"), "package loaderr\n\nfunc OK() int { return 1 }\n")
	// A type error and an import of a package that does not exist.
	if err := os.MkdirAll(filepath.Join(dir, "bad"), 0o755); err != nil {
		t.Fatal(err)
	}
	mustWrite(t, filepath.Join(dir, "bad", "bad.go"), `package bad

import "example.com/loaderr/nothere"

func Bad() int { return "text" }

var _ = nothere.X
`)
	var logs bytes.Buffer
	out := t.TempDir()
	m := runBuild(t, dir, out, func(o *buildpipe.Options) {
		o.Languages = []string{"go"}
		o.NoCache = true
		o.Logger = slog.New(slog.NewTextHandler(&logs, nil))
	})
	if m.GoLoadErrorsCount == 0 {
		t.Errorf("manifest go_load_errors_count = 0, want the load errors counted\n%s", logs.String())
	}
	if !strings.Contains(logs.String(), "go packages loaded with problems") || !strings.Contains(logs.String(), "example.com/loaderr/bad") {
		t.Errorf("no load WARN naming the package:\n%s", logs.String())
	}

	clean := makeMiniGoModule(t)
	logs.Reset()
	m = runBuild(t, clean, t.TempDir(), func(o *buildpipe.Options) {
		o.Languages = []string{"go"}
		o.Logger = slog.New(slog.NewTextHandler(&logs, nil))
	})
	if m.GoLoadErrorsCount != 0 || !strings.Contains(logs.String(), "go packages loaded") || strings.Contains(logs.String(), "with problems") {
		t.Errorf("clean module: count %d\n%s", m.GoLoadErrorsCount, logs.String())
	}
}

// TestRun_ComparesScopeModules: the outside modules a scope records are
// compared with the ones the build loads; one read from another directory and
// one the build does not reach are counted.
func TestRun_ComparesScopeModules(t *testing.T) {
	src, err := filepath.Abs("../../buildscope/testdata/mod")
	if err != nil {
		t.Fatal(err)
	}
	var logs bytes.Buffer
	runBuild(t, src, t.TempDir(), func(o *buildpipe.Options) {
		o.Languages = []string{"go"}
		o.NoCache = true
		o.GoBuild = &detect.GoBuild{GOOS: "linux", GOARCH: "amd64", CGOEnabled: "0",
			Modules: []detect.ModuleUse{
				{Path: "example.com/ext", Version: "v0.0.0", Dir: "/elsewhere/ext"},
				{Path: "example.com/not/in/this/build", Version: "v1.0.0"},
			}}
		o.Logger = slog.New(slog.NewTextHandler(&logs, nil))
	})
	for _, want := range []string{"scope_modules=2", "modules_other_dir=1", "modules_not_loaded=1", "modules_matched=0"} {
		if !strings.Contains(logs.String(), want) {
			t.Errorf("missing %s in the load record:\n%s", want, logs.String())
		}
	}
}
