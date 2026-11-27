package buildpipe_test

import (
	"bytes"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/buildpipe"
	"github.com/0xmhha/knowledge-system/internal/graph/detect"
	"github.com/0xmhha/knowledge-system/internal/graph/persist"
)

// TestRun_GoBuildSelectsTargetFiles: the build configuration decides which of
// two build-tag variants of one file enters the graph, for discovery and type
// checking alike. The fixture is internal/buildscope's test module.
func TestRun_GoBuildSelectsTargetFiles(t *testing.T) {
	src, err := filepath.Abs("../../buildscope/testdata/mod")
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct{ goos, want, absent string }{
		{"linux", "lib/lib_linux.go", "lib/lib_other.go"},
		{"darwin", "lib/lib_other.go", "lib/lib_linux.go"},
	}
	for _, c := range cases {
		t.Run(c.goos, func(t *testing.T) {
			out := t.TempDir()
			runBuild(t, src, out, func(o *buildpipe.Options) {
				o.GoBuild = &detect.GoBuild{GOOS: c.goos, GOARCH: "amd64", CGOEnabled: "0"}
				o.Languages = []string{"go"}
			})
			st, err := persist.OpenReadOnly(filepath.Join(out, "graph.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = st.Close() }()
			nodes, err := st.AllNodes()
			if err != nil {
				t.Fatal(err)
			}
			files := map[string]bool{}
			var platform []string
			for _, n := range nodes {
				files[filepath.ToSlash(n.FilePath)] = true
				if n.Name == "platform" {
					platform = append(platform, n.FilePath)
				}
			}
			if !files[c.want] || files[c.absent] {
				t.Errorf("GOOS=%s: graph files have %s=%v and %s=%v, want only the first", c.goos, c.want, files[c.want], c.absent, files[c.absent])
			}
			if len(platform) != 1 {
				t.Errorf("GOOS=%s: platform declared %d times (%v), want once", c.goos, len(platform), platform)
			}
		})
	}
}

// TestRun_GoBuildChangeForcesColdBuild: rebuilding the same output directory
// for another build configuration builds cold, as every build does; the
// manifest records the configuration.
func TestRun_GoBuildChangeForcesColdBuild(t *testing.T) {
	src, err := filepath.Abs("../../buildscope/testdata/mod")
	if err != nil {
		t.Fatal(err)
	}
	out := t.TempDir()
	var logs bytes.Buffer
	build := func(goos string) persist.Manifest {
		logs.Reset()
		return runBuild(t, src, out, func(o *buildpipe.Options) {
			o.GoBuild = &detect.GoBuild{GOOS: goos, GOARCH: "amd64", CGOEnabled: "0"}
			o.Languages = []string{"go"}
			o.Logger = slog.New(slog.NewTextHandler(&logs, nil))
		})
	}
	linux := build("linux")
	if linux.GoBuild != "goos=linux;goarch=amd64;cgo=0;tags=" {
		t.Fatalf("manifest go_build = %q", linux.GoBuild)
	}
	darwin := build("darwin")
	if darwin.GoBuild != "goos=darwin;goarch=amd64;cgo=0;tags=" {
		t.Errorf("manifest go_build after the second build = %q", darwin.GoBuild)
	}
	if !strings.Contains(logs.String(), "path=cold") {
		t.Errorf("the darwin build was not cold:\n%s", logs.String())
	}
	build("darwin")
	st, err := persist.OpenReadOnly(filepath.Join(out, "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = st.Close() }()
	nodes, err := st.AllNodes()
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range nodes {
		if filepath.ToSlash(n.FilePath) == "lib/lib_linux.go" {
			t.Fatalf("node from lib/lib_linux.go survived the darwin rebuild: the linux cache was reused")
		}
	}
	m, err := st.GetManifest()
	if err != nil {
		t.Fatal(err)
	}
	if m.GoBuild != darwin.GoBuild {
		t.Errorf("stored manifest go_build = %q, want %q", m.GoBuild, darwin.GoBuild)
	}
}
