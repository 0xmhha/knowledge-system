package mcphandlers

// newFixtureStore is shared across test files that need a real
// persist.Store built from the resolve fixture. The graph is built once
// per test binary (each build also walks this repository's git history,
// about 15 s) and every test opens its own read-only handle, closed via
// t.Cleanup; read-only immutable handles can share the file safely.
//
// The file also carries the test-only shims (computeImpact /
// impactDepthCap) that the historical internal/mcp test suite uses.
// New tests should call pkg/impact.Compute / impact.DepthCap directly;
// these wrappers exist so the impact_test.go body (moved from
// internal/mcp during T-14b) keeps working without churn.

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/buildpipe"
	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/internal/testgit"
	"github.com/0xmhha/knowledge-system/pkg/graph/impact"
)

var (
	fixtureOnce sync.Once
	fixtureDir  string
	fixtureErr  error
)

// TestMain removes the shared fixture graph after the package's tests.
func TestMain(m *testing.M) {
	if err := testgit.IsolateFixtures(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := m.Run()
	if fixtureDir != "" {
		_ = os.RemoveAll(fixtureDir)
	}
	os.Exit(code)
}

func newFixtureStore(t *testing.T) persist.Store {
	t.Helper()
	fixtureOnce.Do(func() {
		if fixtureDir, fixtureErr = os.MkdirTemp("", "mcphandlers-fixture-"); fixtureErr != nil {
			return
		}
		_, fixtureErr = buildpipe.Run(buildpipe.Options{
			SrcRoot:    "../../../internal/graph/parse/golang/testdata/resolve",
			OutDir:     fixtureDir,
			Languages:  []string{"auto"},
			CKGVersion: "test",
		})
	})
	if fixtureErr != nil {
		t.Fatal(fixtureErr)
	}
	store, err := persist.OpenReadOnly(filepath.Join(fixtureDir, "graph.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// computeImpact is a thin pass-through retained so the migrated
// impact_test.go body keeps working with its historical 5-argument
// signature instead of having to switch to the impact.Options form
// in every call site.
func computeImpact(store persist.StoreReader, seedQname, seedFile string, depth int, includeBlobs bool) (map[string]any, error) {
	return impact.Compute(store, seedQname, seedFile, impact.Options{
		Depth:        depth,
		IncludeBlobs: includeBlobs,
	})
}

// impactDepthCap re-exports the shared cap so the existing
// TestImpact_DepthCap assertion (which compares against this constant)
// continues to work without referencing pkg/impact directly.
const impactDepthCap = impact.DepthCap
