// Package score computes per-node graph metrics in place: in/out degree,
// PageRank (damping=0.85, iterations=30), and usage_score (sum of incoming
// "calls"/"invokes" edge counts — used to size super-nodes in the viewer).
package score

import (
	"github.com/0xmhha/knowledge-system/internal/graph/graph"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Compute populates InDegree, OutDegree, PageRank, and UsageScore for each node.
//
// Meta nodes (Commit, Hunk — schema 1.4/1.8 temporal) are excluded from
// PageRank participation per hunk-graph.md §11.7 (decision 2026-05-09):
// Hunks have ~1 inbound edge each (has_hunk) and would rank near-zero,
// adding noise to the metric without contributing signal. They keep
// in/out degree counters populated (those are local, additive, and useful
// for the viewer's "how many hunks does this commit have" query) but
// their PageRank stays at the zero-initialised default.
func Compute(g *graph.Graph) {
	idx := make(map[string]int, len(g.Nodes))
	for i, n := range g.Nodes {
		g.Nodes[i].InDegree = 0
		g.Nodes[i].OutDegree = 0
		g.Nodes[i].UsageScore = 0
		idx[n.ID] = i
	}
	for _, e := range g.Edges {
		if types.IsGoroutineCode(e) {
			continue // a Goroutine's code edges repeat its function's
		}
		si, ok := idx[e.Src]
		if !ok {
			continue
		}
		di, ok := idx[e.Dst]
		if !ok {
			continue
		}
		// History edges (changed_in, blame, modifies, has_hunk, adjacent)
		// end at or start from a meta node: they record when code changed,
		// not how it depends on other code, and PageRank leaves meta nodes
		// out for that reason; degrees follow the same rule.
		if isMetaNodeType(g.Nodes[si].Type) || isMetaNodeType(g.Nodes[di].Type) {
			continue
		}
		g.Nodes[si].OutDegree += e.Count
		g.Nodes[di].InDegree += e.Count
		if e.Type == types.EdgeCalls || e.Type == types.EdgeInvokes {
			g.Nodes[di].UsageScore += float64(e.Count)
		}
	}
	pageRank(g, 0.85, 30)
}

// statementLink are the edges inside a function body (schema 1.32). They
// stay out of PageRank: site_calls repeats the function-level calls, and
// the others describe how a body is built, not how declarations depend on
// each other, so counting them would move declaration ranks for no reason.
//
// The message links (schema 1.42: handles, triggers) stay out as well: they
// start at statement sites or repeat what the channel edges say, and were
// not part of the measured ranks.
var statementLink = map[types.EdgeType]bool{
	types.EdgeSiteCalls: true, types.EdgeCondReads: true, types.EdgeCondCalls: true,
	types.EdgeBodyWrites: true, types.EdgeEncloses: true,
	types.EdgeHandles: true, types.EdgeTriggers: true, types.EdgeLiteralPassedTo: true,
}

// Type-hierarchy dispatch candidates (dispatches_to, kind type_hierarchy)
// stay out of PageRank for the same reason: they say what go/types allows,
// not a dependency the analysis found.

// isMetaNodeType mirrors buildpipe.isMetaNodeType — kept duplicated to
// avoid a buildpipe→score package cycle. Update both when a future schema
// adds another meta-node family.
func isMetaNodeType(t types.NodeType) bool {
	return t == types.NodeCommit || t == types.NodeHunk
}

// pageRank implements the standard iterative algorithm. Meta nodes
// (Commit, Hunk) are excluded from the participant set; their pr stays
// at zero-init (see Compute doc-comment).
func pageRank(g *graph.Graph, damping float64, iters int) {
	if len(g.Nodes) == 0 {
		return
	}
	// Build a compacted index over participating (non-meta) nodes only.
	// participants[k] is the index in g.Nodes of the k-th participant.
	participants := make([]int, 0, len(g.Nodes))
	idx := make(map[string]int, len(g.Nodes))
	for i, nd := range g.Nodes {
		// External nodes (schema 1.36) stay out: time.Now would collect rank
		// from every caller and pass none back, and how that changes the
		// order of repository symbols has not been measured.
		if isMetaNodeType(nd.Type) || nd.Type == types.NodeExternal {
			continue
		}
		idx[nd.ID] = len(participants)
		participants = append(participants, i)
	}
	n := len(participants)
	if n == 0 {
		return
	}
	out := make([][]int, n)
	outDeg := make([]int, n)
	for _, e := range g.Edges {
		if types.IsGoroutineCode(e) {
			continue // a Goroutine's code edges repeat its function's
		}
		si, ok := idx[e.Src]
		if !ok {
			continue
		}
		di, ok := idx[e.Dst]
		if !ok {
			continue
		}
		if statementLink[e.Type] || e.Type == types.EdgeDispatchesTo && e.DispatchKind == "type_hierarchy" ||
			e.Type == types.EdgeFlowsTo && e.DispatchKind == types.DispatchReceived {
			continue
		}
		out[si] = append(out[si], di)
		outDeg[si]++
	}
	pr := make([]float64, n)
	next := make([]float64, n)
	for i := range pr {
		pr[i] = 1.0 / float64(n)
	}
	teleport := (1 - damping) / float64(n)
	for it := 0; it < iters; it++ {
		for i := range next {
			next[i] = teleport
		}
		dangling := 0.0
		for i := 0; i < n; i++ {
			if outDeg[i] == 0 {
				dangling += pr[i]
				continue
			}
			share := damping * pr[i] / float64(outDeg[i])
			for _, j := range out[i] {
				next[j] += share
			}
		}
		// distribute dangling mass evenly
		add := damping * dangling / float64(n)
		for i := range next {
			next[i] += add
		}
		pr, next = next, pr
	}
	for k, gi := range participants {
		g.Nodes[gi].PageRank = pr[k]
	}
}
