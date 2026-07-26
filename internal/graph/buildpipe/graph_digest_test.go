package buildpipe

import (
	"testing"

	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

func codeNodes() []types.Node {
	return []types.Node{
		{ID: "aaaaaaaaaaaaaaaa", Type: types.NodeFunction, CanonicalID: "pkg.F", QualifiedName: "pkg.F", FilePath: "a.go", StartLine: 1, EndLine: 3},
		{ID: "bbbbbbbbbbbbbbbb", Type: types.NodeStruct, CanonicalID: "pkg.T", QualifiedName: "pkg.T", FilePath: "a.go", StartLine: 5, EndLine: 9},
	}
}

func codeEdges() []types.Edge {
	return []types.Edge{
		{Type: types.EdgeCalls, Src: "aaaaaaaaaaaaaaaa", Dst: "bbbbbbbbbbbbbbbb", Line: 2},
	}
}

// TestComputeGraphDigest_Deterministic locks the core contract: the digest is
// independent of node/edge slice order (ADR-0002 / Q1).
func TestComputeGraphDigest_Deterministic(t *testing.T) {
	n := codeNodes()
	e := codeEdges()
	d1 := ComputeGraphDigest(n, e)

	// Reversed input order must yield the identical digest.
	nRev := []types.Node{n[1], n[0]}
	d2 := ComputeGraphDigest(nRev, e)
	if d1 != d2 {
		t.Errorf("digest depends on node order: %s vs %s", d1, d2)
	}
	if d1 == "" || len(d1) != 64 {
		t.Errorf("digest not a sha256 hex: %q", d1)
	}
}

// TestComputeGraphDigest_ExcludesDerivedMetrics guards that pagerank/degrees do
// NOT affect the digest — otherwise incremental (which recomputes them) would
// diverge from cold for the same logical graph.
func TestComputeGraphDigest_ExcludesDerivedMetrics(t *testing.T) {
	base := codeNodes()
	withMetrics := codeNodes()
	withMetrics[0].PageRank = 0.42
	withMetrics[0].InDegree = 7
	withMetrics[0].OutDegree = 3
	withMetrics[0].UsageScore = 9.9
	withMetrics[1].Complexity = 12

	if ComputeGraphDigest(base, codeEdges()) != ComputeGraphDigest(withMetrics, codeEdges()) {
		t.Error("derived metrics (pagerank/degrees/usage/complexity) changed the digest")
	}
}

// TestComputeGraphDigest_ExcludesTemporal guards that Commit/Hunk nodes and
// temporal edges are ignored — a temporal-only rebuild must leave the digest
// unchanged (prevents CKV false-positive asserts).
func TestComputeGraphDigest_ExcludesTemporal(t *testing.T) {
	base := ComputeGraphDigest(codeNodes(), codeEdges())

	nodesPlusTemporal := append(codeNodes(),
		types.Node{ID: "cccccccccccccccc", Type: types.NodeCommit, QualifiedName: "commit:abc", FilePath: "", StartLine: 1, EndLine: 1},
		types.Node{ID: "dddddddddddddddd", Type: types.NodeHunk, QualifiedName: "hunk:1", FilePath: "a.go", StartLine: 1, EndLine: 1},
	)
	edgesPlusTemporal := append(codeEdges(),
		types.Edge{Type: types.EdgeChangedIn, Src: "aaaaaaaaaaaaaaaa", Dst: "cccccccccccccccc", Line: 0},
		types.Edge{Type: types.EdgeHasHunk, Src: "cccccccccccccccc", Dst: "dddddddddddddddd", Line: 0},
	)
	if got := ComputeGraphDigest(nodesPlusTemporal, edgesPlusTemporal); got != base {
		t.Errorf("temporal nodes/edges changed the digest: %s vs %s", got, base)
	}
}

// TestComputeGraphDigest_SensitiveToIdentity confirms the digest DOES change
// when a real identity field (canonical_id) changes — it is not degenerate.
func TestComputeGraphDigest_SensitiveToIdentity(t *testing.T) {
	base := ComputeGraphDigest(codeNodes(), codeEdges())
	mutated := codeNodes()
	mutated[0].CanonicalID = "pkg.F2"
	if ComputeGraphDigest(mutated, codeEdges()) == base {
		t.Error("digest did not change when canonical_id changed")
	}
}

// TestGraphDigest_EnrichmentExcluded locks the code/enrichment digest split:
// injecting policy / security-pattern rows must NOT move the coordinate pin
// (graph_digest); it must move only EnrichDigest.
func TestGraphDigest_EnrichmentExcluded(t *testing.T) {
	base := []types.Node{
		{ID: "n1", Type: types.NodeFunction, QualifiedName: "pkg.F", FilePath: "a.go", StartLine: 1, EndLine: 2},
	}
	baseEdges := []types.Edge{
		{Type: types.EdgeCalls, Src: "n1", Dst: "n1", Line: 1},
	}
	enrichedNodes := append(append([]types.Node{}, base...),
		types.Node{ID: "p1", Type: types.NodePolicy, QualifiedName: "policy.Rule", FilePath: "policy.yaml", StartLine: 1, EndLine: 1},
		types.Node{ID: "s1", Type: types.NodeSecurityPattern, QualifiedName: "sec.Pat", FilePath: "sec.yaml", StartLine: 1, EndLine: 1},
	)
	enrichedEdges := append(append([]types.Edge{}, baseEdges...),
		types.Edge{Type: types.EdgeGovernedBy, Src: "n1", Dst: "p1", Line: 1},
		types.Edge{Type: types.EdgeHasSecurityPattern, Src: "n1", Dst: "s1", Line: 1},
	)

	if got, want := ComputeGraphDigest(enrichedNodes, enrichedEdges), ComputeGraphDigest(base, baseEdges); got != want {
		t.Errorf("code digest moved with enrichment: %s != %s", got, want)
	}
	if got := ComputeEnrichDigest(base, baseEdges); got != "" {
		t.Errorf("EnrichDigest without enrichment = %q, want empty", got)
	}
	d1 := ComputeEnrichDigest(enrichedNodes, enrichedEdges)
	if d1 == "" {
		t.Fatal("EnrichDigest with enrichment is empty")
	}
	// Changing the enrichment changes the enrich digest.
	enrichedEdges[len(enrichedEdges)-1].Line = 2
	if d2 := ComputeEnrichDigest(enrichedNodes, enrichedEdges); d2 == d1 {
		t.Error("EnrichDigest did not change when enrichment changed")
	}
}
