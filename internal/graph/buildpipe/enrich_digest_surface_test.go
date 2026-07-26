package buildpipe

import (
	"os"
	"path/filepath"
	"testing"
)

// TestColdBuild_EnrichDigestSurfaced pins the enrichment-overlay contract at
// the manifest level: a cold build WITH policy enrichment must surface a
// non-empty manifest.EnrichDigest (so downstream can detect "the enrichment
// moved"), while the code graph_digest stays byte-identical to a build WITHOUT
// enrichment — operator-injected knowledge must never move the coordinate pin
// (see graph_digest.go and the digest-split ADR).
//
// Regression guard: enrichment nodes/edges are persisted to the store but kept
// out of the in-memory graph, so ComputeEnrichDigest(g...) inside
// buildManifestSkeleton sees nothing and returns "". The cold path must fill
// EnrichDigest from the injected rows instead.
func TestColdBuild_EnrichDigestSurfaced(t *testing.T) {
	const fixture = "../parse/golang/testdata/resolve"

	// Build A: no enrichment — EnrichDigest must be empty, GraphDigest is the
	// baseline coordinate pin.
	outA := t.TempDir()
	mA, err := Run(Options{
		SrcRoot:    fixture,
		OutDir:     outA,
		Languages:  []string{"auto"},
		CKGVersion: "test",
	})
	if err != nil {
		t.Fatalf("build without policy: %v", err)
	}
	if mA.EnrichDigest != "" {
		t.Errorf("no-enrichment build EnrichDigest = %q, want empty", mA.EnrichDigest)
	}
	if mA.GraphDigest == "" {
		t.Fatal("baseline GraphDigest is empty")
	}

	// Build B: same source + a policy that governs a fixture symbol.
	const policyYAML = `
policies:
  - id: "fixture.greet-policy"
    name: "Greet API contract"
    category: "fixture"
    description: "Synthetic policy for the enrich-digest surface test."
    governs:
      - "a.Greet"
`
	outB := t.TempDir()
	pPath := filepath.Join(outB, "policy.yaml")
	if err := os.WriteFile(pPath, []byte(policyYAML), 0o644); err != nil {
		t.Fatalf("write policy yaml: %v", err)
	}
	mB, err := Run(Options{
		SrcRoot:    fixture,
		OutDir:     outB,
		Languages:  []string{"auto"},
		CKGVersion: "test",
		PolicyFile: pPath,
	})
	if err != nil {
		t.Fatalf("build with policy: %v", err)
	}

	if mB.EnrichDigest == "" {
		t.Error("policy build manifest.EnrichDigest is empty — enrichment overlay not surfaced")
	}
	if mB.GraphDigest != mA.GraphDigest {
		t.Errorf("graph_digest moved by enrichment: with=%s without=%s (enrichment must not move the coordinate pin)",
			mB.GraphDigest, mA.GraphDigest)
	}
}
