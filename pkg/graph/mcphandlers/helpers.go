package mcphandlers

import (
	"sort"
	"strings"

	mcp "github.com/mark3labs/mcp-go/mcp"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// interfaceMethodSeeds returns extra find_callers seeds for the interface-dispatch
// bridge. When the resolved seed is a concrete method `pkg.T.M`, callers that
// reach it through an interface are recorded as `invokes` edges to the INTERFACE
// method node (`pkg.I.M`), not to the concrete method — so a reverse walk from the
// concrete method alone misses every interface-dispatched caller. This returns the
// same-named method on each interface that T implements, using the `implements`
// edges already in the graph (no reindex needed). Generalized: no language/domain
// specifics; a top-level function (no receiver segment) or a type implementing
// nothing yields no extra seeds.
func interfaceMethodSeeds(reader store.Reader, methodQname string) []string {
	dot := strings.LastIndex(methodQname, ".")
	if dot <= 0 || dot == len(methodQname)-1 {
		return nil // no `Type.Method` shape → nothing to bridge
	}
	typeQname, methodShort := methodQname[:dot], methodQname[dot+1:]

	// Forward `implements` edges from the receiver type → interfaces it satisfies.
	nodes, edges, err := reader.NeighborhoodByQname(typeQname, 1, false, string(types.EdgeImplements))
	if err != nil {
		return nil
	}
	ifaceIDs := map[string]struct{}{}
	for _, e := range edges {
		if e.Type == types.EdgeImplements {
			ifaceIDs[e.Dst] = struct{}{}
		}
	}
	var seeds []string
	seen := map[string]struct{}{}
	for _, n := range nodes {
		if _, ok := ifaceIDs[n.ID]; !ok || n.Type != types.NodeInterface {
			continue
		}
		mq := n.QualifiedName + "." + methodShort
		if _, dup := seen[mq]; dup {
			continue
		}
		seen[mq] = struct{}{}
		seeds = append(seeds, mq)
	}
	sort.Strings(seeds) // deterministic for prompt-cache stability
	return seeds
}

// reverseCallersUnion walks the reverse call graph from `resolved` AND from every
// interface method it satisfies (the interface-dispatch bridge), unioning the
// nodes/edges. Dedups nodes by ID and edges by (src,dst,type).
func reverseCallersUnion(reader store.Reader, resolved seedRef, depth int) ([]types.Node, []types.Edge, error) {
	seeds := []seedRef{resolved}
	for _, q := range interfaceMethodSeeds(reader, resolved.QName) {
		seeds = append(seeds, seedRef{QName: q})
	}
	nodeByID := map[string]types.Node{}
	edgeSeen := map[string]struct{}{}
	var edges []types.Edge
	for _, sd := range seeds {
		ns, es, err := sd.neighborhood(reader, depth, true /*reverse*/, callEdgeTypes...)
		if err != nil {
			return nil, nil, err
		}
		for _, n := range ns {
			nodeByID[n.ID] = n
		}
		for _, e := range es {
			k := e.Src + "|" + e.Dst + "|" + string(e.Type)
			if _, ok := edgeSeen[k]; ok {
				continue
			}
			edgeSeen[k] = struct{}{}
			edges = append(edges, e)
		}
	}
	nodes := make([]types.Node, 0, len(nodeByID))
	for _, n := range nodeByID {
		nodes = append(nodes, n)
	}
	return nodes, edges, nil
}

// textResult wraps a payload in the mcp-go structured-result envelope.
// All handlers route their JSON payload through here so the response
// shape stays uniform across the tool set.
func textResult(payload any) *mcp.CallToolResult {
	return mcp.NewToolResultStructured(payload, "")
}

// callEdgeTypes are the edge types find_callers / find_callees follow.
// Without this filter the BFS would also walk contains/defines and
// return the file holding a method as one of its "callers" —
// semantically wrong and noisy for LLM consumers.
var callEdgeTypes = []string{"calls", "invokes"}

// seedRef is a resolved traversal seed: the qualified name reported back to
// the caller and, when the seed named one exact node (a canonical id), that
// node's id. Traversals start from IDs when they are set, so a canonical id
// walks only its own node even when other nodes share its qualified name.
type seedRef struct {
	QName string
	IDs   []string
}

// neighborhood walks from the seed: from its node ids when the seed is
// exact, otherwise from every node carrying its qualified name.
func (s seedRef) neighborhood(reader store.Reader, depth int, reverse bool, edgeTypes ...string) ([]types.Node, []types.Edge, error) {
	if len(s.IDs) > 0 {
		return reader.NeighborhoodByIDs(s.IDs, depth, reverse, edgeTypes...)
	}
	return reader.NeighborhoodByQname(s.QName, depth, reverse, edgeTypes...)
}

// subgraph walks from the seed in both directions over every edge type.
func (s seedRef) subgraph(reader store.Reader, depth int) ([]types.Node, []types.Edge, error) {
	if len(s.IDs) == 0 {
		return reader.SubgraphByQname(s.QName, depth)
	}
	fwdN, fwdE, err := reader.NeighborhoodByIDs(s.IDs, depth, false)
	if err != nil {
		return nil, nil, err
	}
	revN, revE, err := reader.NeighborhoodByIDs(s.IDs, depth, true)
	if err != nil {
		return nil, nil, err
	}
	merged := map[string]types.Node{}
	for _, n := range append(fwdN, revN...) {
		merged[n.ID] = n
	}
	nodes := make([]types.Node, 0, len(merged))
	for _, n := range merged {
		nodes = append(nodes, n)
	}
	return nodes, append(fwdE, revE...), nil
}

// nodes returns the seed's own nodes.
func (s seedRef) nodes(reader store.Reader) ([]types.Node, error) {
	if len(s.IDs) > 0 {
		return reader.NodesByIDs(s.IDs)
	}
	return reader.FindSymbol(s.QName, true, store.FindSymbolOptions{})
}

// result adds the seed to a traversal payload: seed_qname always, seed_id
// when the traversal started from one exact node.
func (s seedRef) result(payload map[string]any) map[string]any {
	payload["seed_qname"] = s.QName
	if len(s.IDs) == 1 {
		payload["seed_id"] = s.IDs[0]
	}
	return payload
}

// resolveSeed maps a user-supplied seed — a canonical id, a fully-qualified
// name ("a.Greet") or a bare short name ("Greet") — to the seed the
// graph-traversal tools (find_callers / find_callees / get_subgraph /
// change_history) walk from. It mirrors find_symbol's exact-or-suffix logic
// so an agent that located a symbol via find_symbol(exact=false) can feed
// the same bare name to the traversal tools instead of silently getting an
// empty result.
//
// Resolution order:
//  0. exact match on canonical_id — globally unique (ADR-0001), so a hit is
//     unambiguous by construction; the seed is that one node.
//  1. exact match on qualified_name — the common, unambiguous case.
//  2. suffix match (bare name) when exact found nothing.
//
// Return contract:
//   - ok=true, seed set             exactly one node (step 0) or one distinct
//     qname matched.
//   - ambiguous=true, candidates set the suffix matched >1 distinct qname;
//     the caller should surface them so the agent can retry with a full
//     qname (ADR-0001: a multi-match is reported, never silently picked).
//   - all-zero (ok=false)           nothing matched (not_found).
//
// Several nodes sharing ONE qualified_name (e.g. the same symbol across
// languages) is not ambiguous for a qname seed: every such node becomes a
// traversal root. A canonical id seed starts from its own node only.
func resolveSeed(reader store.Reader, seed, lang string) (resolved seedRef, candidates []string, ambiguous bool, ok bool) {
	if seed == "" {
		return seedRef{}, nil, false, false
	}
	opts := store.FindSymbolOptions{Language: lang}

	// 0. exact canonical_id — unambiguous; the seed is that node.
	if n, found, err := reader.FindByCanonicalID(seed); err == nil && found {
		return seedRef{QName: n.QualifiedName, IDs: []string{n.ID}}, nil, false, true
	}

	// 1. exact qualified_name.
	if nodes, err := reader.FindSymbol(seed, true, opts); err == nil && len(nodes) > 0 {
		return seedRef{QName: seed}, nil, false, true
	}

	// 2. suffix fallback for bare names.
	nodes, err := reader.FindSymbol(seed, false, opts)
	if err != nil || len(nodes) == 0 {
		return seedRef{}, nil, false, false
	}
	seen := map[string]struct{}{}
	distinct := make([]string, 0, len(nodes))
	for _, n := range nodes {
		if _, dup := seen[n.QualifiedName]; dup {
			continue
		}
		seen[n.QualifiedName] = struct{}{}
		distinct = append(distinct, n.QualifiedName)
	}
	if len(distinct) == 1 {
		return seedRef{QName: distinct[0]}, nil, false, true
	}
	sort.Strings(distinct) // deterministic order keeps cks's prompt cache stable
	return seedRef{}, distinct, true, false
}

// seedNotFoundResult is the uniform payload the traversal tools return
// when a seed resolves to nothing. nodes/edges are empty (not omitted) so
// the consumer's shape stays stable, and not_found distinguishes "the
// symbol has no neighbours" from "the symbol name did not resolve".
func seedNotFoundResult(seed string) map[string]any {
	return map[string]any{
		"seed_qname": seed,
		"not_found":  true,
		"nodes":      []map[string]any{},
		"edges":      []types.Edge{},
	}
}

// seedAmbiguousResult is returned when a bare seed name matches more than
// one distinct qualified_name. The agent is expected to retry with one of
// the candidates rather than receive a silently-picked (and possibly
// wrong) neighbourhood.
func seedAmbiguousResult(seed string, candidates []string) map[string]any {
	return map[string]any{
		"seed_qname": seed,
		"ambiguous":  true,
		"candidates": candidates,
		"nodes":      []map[string]any{},
		"edges":      []types.Edge{},
	}
}

// attachBlobs converts a node slice into the JSON-friendly maps the
// MCP tools return, optionally inlining the source blob from the
// blobs table when include is true. GetBlob errors are silently
// swallowed: nodes like Package have no blob (sql.ErrNoRows is
// expected and harmless for them).
func attachBlobs(reader store.Reader, nodes []types.Node, include bool) []map[string]any {
	out := make([]map[string]any, 0, len(nodes))
	for _, n := range nodes {
		m := map[string]any{
			"id":          n.ID,
			"type":        n.Type,
			"name":        n.Name,
			"qname":       n.QualifiedName,
			"file":        n.FilePath,
			"line":        n.StartLine,
			"confidence":  n.Confidence,
			"signature":   n.Signature,
			"usage_score": n.UsageScore,
		}
		// canonical_id (ADR-0001): the globally-unique, import-path-qualified
		// identity. Included only when present (empty for builtins, AST-only
		// mode, and not-yet-wired languages) so the agent can feed it back as an
		// unambiguous seed to the traversal tools. Omitted when empty.
		if n.CanonicalID != "" {
			m["canonical_id"] = n.CanonicalID
		}
		if include {
			if b, err := reader.GetBlob(n.ID); err == nil {
				m["source"] = string(b)
			}
		}
		out = append(out, m)
	}
	return out
}
