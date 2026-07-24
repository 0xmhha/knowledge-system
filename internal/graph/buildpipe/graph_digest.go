package buildpipe

import (
	"crypto/sha256"
	"encoding/hex"
	"sort"
	"strconv"
	"strings"

	"github.com/0xmhha/knowledge-system/graph/pkg/types"
)

// ComputeGraphDigest returns a deterministic, build-invariant hex hash of the
// CODE graph — the coordinate pin anchor published in manifest.GraphDigest and
// consumed by CKV/CKS (docs/coordination-reindex-migration-2026-07-10.md § Q1).
//
// Definition (agreed cross-repo, 2026-07-10):
//
//	digest = sha256( <nodes-block> "\n--edges--\n" <edges-block> )
//	  node line = id \t canonical_id \t type \t qualified_name \t file_path \t
//	              start_line \t end_line          — sorted by id ascending
//	  edge line = type \t src \t dst \t line       — the canonical (Type,Src,Dst,
//	              Line) edge identity; exact duplicates collapsed; sorted
//
// Excluded, by design:
//   - Derived metrics (pagerank / in_degree / out_degree / usage_score) — they
//     are recomputed on any incremental dirt, so including them would make the
//     incremental digest differ from a cold one for the same logical graph.
//   - Index-derived / content columns (search_tokens, simple_name, attrs,
//     signature, doc_comment) and start_byte (already folded into id).
//   - Temporal nodes/edges (Commit/Hunk + changed_in/blame/has_hunk/adjacent/
//     modifies) — they depend on --temporal-depth + git state, orthogonal to the
//     code graph CKV/CKS pin. A temporal-only rebuild leaves this digest
//     unchanged, which is the point.
//   - Enrichment nodes/edges (Policy/SecurityPattern + governed_by/
//     has_security_pattern) — knowledge injected from operator-supplied YAML,
//     not derived from the source. The vector engine aligns on code symbol
//     identity, so enrichment must not invalidate the coordinate pin; it is
//     tracked separately in manifest.EnrichDigest (ComputeEnrichDigest).
//     NOTE: graphs built WITH enrichment before this split carry a digest
//     that included those rows — their next rebuild changes graph_digest
//     once and triggers one vector realignment.
//
// Because every retained field is deterministic for a pinned source under
// ADR-0002 and the lines are sorted, the digest is identical across cold and
// incremental builds and across machines.
func ComputeGraphDigest(nodes []types.Node, edges []types.Edge) string {
	nodeLines := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if isMetaNodeType(n.Type) {
			continue // temporal (Commit/Hunk) excluded
		}
		if isEnrichmentNodeType(n.Type) {
			continue // operator-injected enrichment excluded (see EnrichDigest)
		}
		nodeLines = append(nodeLines, strings.Join([]string{
			n.ID,
			n.CanonicalID,
			string(n.Type),
			n.QualifiedName,
			n.FilePath,
			strconv.Itoa(n.StartLine),
			strconv.Itoa(n.EndLine),
		}, "\t"))
	}
	sort.Strings(nodeLines)

	edgeSet := make(map[string]struct{}, len(edges))
	for _, e := range edges {
		if isTemporalEdgeType(e.Type) {
			continue // temporal edges excluded
		}
		if isEnrichmentEdgeType(e.Type) {
			continue // enrichment edges excluded (see EnrichDigest)
		}
		edgeSet[strings.Join([]string{
			string(e.Type),
			e.Src,
			e.Dst,
			strconv.Itoa(e.Line),
		}, "\t")] = struct{}{}
	}
	edgeLines := make([]string, 0, len(edgeSet))
	for l := range edgeSet {
		edgeLines = append(edgeLines, l)
	}
	sort.Strings(edgeLines)

	h := sha256.New()
	h.Write([]byte(strings.Join(nodeLines, "\n")))
	h.Write([]byte("\n--edges--\n"))
	h.Write([]byte(strings.Join(edgeLines, "\n")))
	return hex.EncodeToString(h.Sum(nil))
}

// isTemporalEdgeType reports whether e belongs to the temporal (git-history)
// family excluded from the code-graph digest. Mirrors isMetaNodeType for edges.
func isTemporalEdgeType(t types.EdgeType) bool {
	switch t {
	case types.EdgeChangedIn, types.EdgeBlame, types.EdgeHasHunk,
		types.EdgeAdjacent, types.EdgeModifies:
		return true
	default:
		return false
	}
}

// isEnrichmentNodeType reports whether t belongs to the operator-injected
// enrichment family (policy / security-pattern YAML), excluded from the code
// digest and hashed into EnrichDigest instead.
func isEnrichmentNodeType(t types.NodeType) bool {
	switch t {
	case types.NodePolicy, types.NodeSecurityPattern:
		return true
	default:
		return false
	}
}

// isEnrichmentEdgeType mirrors isEnrichmentNodeType for edges.
func isEnrichmentEdgeType(t types.EdgeType) bool {
	switch t {
	case types.EdgeGovernedBy, types.EdgeHasSecurityPattern:
		return true
	default:
		return false
	}
}

// ComputeEnrichDigest returns a deterministic hex hash over ONLY the
// enrichment nodes/edges (same line format as ComputeGraphDigest), or ""
// when the graph carries no enrichment. It changes when the injected
// policy / security knowledge changes and is deliberately NOT part of the
// coordinate pin: consumers use it to detect "the enrichment overlay moved"
// without forcing a vector realignment.
func ComputeEnrichDigest(nodes []types.Node, edges []types.Edge) string {
	nodeLines := make([]string, 0, 8)
	for _, n := range nodes {
		if !isEnrichmentNodeType(n.Type) {
			continue
		}
		nodeLines = append(nodeLines, strings.Join([]string{
			n.ID,
			n.CanonicalID,
			string(n.Type),
			n.QualifiedName,
			n.FilePath,
			strconv.Itoa(n.StartLine),
			strconv.Itoa(n.EndLine),
		}, "\t"))
	}
	edgeSet := make(map[string]struct{}, 8)
	for _, e := range edges {
		if !isEnrichmentEdgeType(e.Type) {
			continue
		}
		edgeSet[strings.Join([]string{
			string(e.Type),
			e.Src,
			e.Dst,
			strconv.Itoa(e.Line),
		}, "\t")] = struct{}{}
	}
	if len(nodeLines) == 0 && len(edgeSet) == 0 {
		return ""
	}
	sort.Strings(nodeLines)
	edgeLines := make([]string, 0, len(edgeSet))
	for l := range edgeSet {
		edgeLines = append(edgeLines, l)
	}
	sort.Strings(edgeLines)

	h := sha256.New()
	h.Write([]byte(strings.Join(nodeLines, "\n")))
	h.Write([]byte("\n--edges--\n"))
	h.Write([]byte(strings.Join(edgeLines, "\n")))
	return hex.EncodeToString(h.Sum(nil))
}
