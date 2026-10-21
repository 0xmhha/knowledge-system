package buildpipe

import (
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/internal/testgit"
)

// A cold Run type-loads the fixture's whole dependency graph through
// go/packages: about 20 s and 3 GB under -race. Tests that only read the
// output of an identical build share one, so the package stays inside the
// default go test timeout without running builds concurrently.

type sharedBuild struct {
	out      string
	manifest persist.Manifest
	err      error
}

var (
	sharedBuildsMu   sync.Mutex
	sharedBuilds     = map[string]func() sharedBuild{}
	sharedBuildsRoot string
)

func TestMain(m *testing.M) {
	if err := testgit.IsolateFixtures(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := m.Run()
	if sharedBuildsRoot != "" {
		_ = os.RemoveAll(sharedBuildsRoot)
	}
	os.Exit(code)
}

// SharedColdBuild returns the output directory and manifest of a cold Run
// with opt, building it once per distinct opt. opt.OutDir is ignored. Callers
// must treat the output as read-only.
func SharedColdBuild(t *testing.T, opt Options) (string, persist.Manifest) {
	t.Helper()
	opt.OutDir = ""
	key := fmt.Sprintf("%#v", opt)

	sharedBuildsMu.Lock()
	if sharedBuildsRoot == "" {
		root, err := os.MkdirTemp("", "buildpipe-shared-")
		if err != nil {
			sharedBuildsMu.Unlock()
			t.Fatalf("shared build root: %v", err)
		}
		sharedBuildsRoot = root
	}
	build, ok := sharedBuilds[key]
	if !ok {
		root := sharedBuildsRoot
		build = sync.OnceValue(func() sharedBuild {
			out, err := os.MkdirTemp(root, "build-")
			if err != nil {
				return sharedBuild{err: err}
			}
			opt.OutDir = out
			m, err := Run(opt)
			return sharedBuild{out: out, manifest: m, err: err}
		})
		sharedBuilds[key] = build
	}
	sharedBuildsMu.Unlock()

	b := build()
	if b.err != nil {
		t.Fatalf("Run: %v", b.err)
	}
	return b.out, b.manifest
}
