package buildpipe_test

import (
	"path/filepath"
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
