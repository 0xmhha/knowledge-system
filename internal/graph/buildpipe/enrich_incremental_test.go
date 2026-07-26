package buildpipe_test

import (
	"path/filepath"
	"testing"

	"github.com/0xmhha/knowledge-system/internal/graph/buildpipe"
	"github.com/0xmhha/knowledge-system/internal/graph/persist"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// TestIncremental_EnrichmentSurvives guards against enrichment erosion across
// incremental rebuilds. A cold build with a policy governing cachetest.Add
// (in a.go) emits a governed_by edge + a non-empty enrich_digest. Touching
// a.go and rebuilding must not silently drop that overlay: the governed_by
// edge must still resolve and enrich_digest must remain populated.
func TestIncremental_EnrichmentSurvives(t *testing.T) {
	src := makeMiniGoModule(t)
	out := t.TempDir()

	policyYAML := `
policies:
  - id: "cachetest.add-contract"
    name: "Add is a public API"
    category: "api"
    description: "Add's signature is load-bearing."
    governs:
      - "cachetest.Add"
`
	pPath := filepath.Join(t.TempDir(), "policy.yaml")
	mustWrite(t, pPath, policyYAML)
	withPolicy := func(o *buildpipe.Options) { o.PolicyFile = pPath }

	// Cold build with policy.
	cold := runBuild(t, src, out, withPolicy)
	if cold.EnrichDigest == "" {
		t.Fatal("cold build: enrich_digest empty (setup precondition failed)")
	}
	govCold := countGovernedBy(t, out)
	if govCold == 0 {
		t.Fatalf("cold build: expected >=1 governed_by edge to cachetest.Add, got 0")
	}
	t.Logf("cold: governed_by=%d enrich_digest=%.12s…", govCold, cold.EnrichDigest)

	// Dirty the file that holds the governed symbol, then rebuild (incremental).
	mustWrite(t, filepath.Join(src, "a.go"), `package cachetest

// Add returns a + b.
func Add(a, b int) int { return a + b }

// touched to force an incremental rebuild of a.go
var _ = 1
`)
	inc := runBuild(t, src, out, withPolicy)
	govInc := countGovernedBy(t, out)
	t.Logf("incremental: governed_by=%d enrich_digest=%.12s…", govInc, inc.EnrichDigest)

	if govInc == 0 {
		t.Errorf("incremental rebuild DROPPED the governed_by edge (cold=%d → inc=0): enrichment eroded", govCold)
	}
	if inc.EnrichDigest == "" {
		t.Errorf("incremental rebuild RESET enrich_digest to empty (cold=%.12s…): overlay pin lost", cold.EnrichDigest)
	}
}

func countGovernedBy(t *testing.T, out string) int {
	t.Helper()
	st, err := persist.OpenReadOnly(filepath.Join(out, "graph.db"))
	if err != nil {
		t.Fatalf("open graph.db: %v", err)
	}
	defer func() { _ = st.Close() }()
	edges, err := st.QueryEdgesByType(string(types.EdgeGovernedBy))
	if err != nil {
		t.Fatalf("QueryEdgesByType: %v", err)
	}
	return len(edges)
}
