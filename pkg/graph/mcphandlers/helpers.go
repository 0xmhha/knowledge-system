package mcphandlers

import (
	"sort"
	"strings"
	"sync"

	"github.com/0xmhha/knowledge-system/pkg/graph/store"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// dispatchSeeds returns the interface methods whose dispatches_to edges
// reach the seed: value-flow links (schema 1.31, SSA + VTA) first, then
// type-hierarchy links (schema 1.34), so a method no analysed value reaches,
// for example an exported method only outside callers use, keeps its
// interface callers. Graphs without either fall back to implementsSeeds.
// source says which one answered: "dispatches_to", "type_hierarchy",
// "implements" or "" when none gave a seed.
func dispatchSeeds(reader store.Reader, seed seedRef) (seeds []string, source string) {
	if hasDispatch(reader) {
		ns, es, err := seed.neighborhood(reader, 1, true, string(types.EdgeDispatchesTo))
		if err == nil {
			own := map[string]bool{}
			for _, id := range seed.IDs {
				own[id] = true
			}
			if len(own) == 0 {
				for _, n := range ns {
					if n.QualifiedName == seed.QName {
						own[n.ID] = true
					}
				}
			}
			name := map[string]string{}
			for _, n := range ns {
				name[n.ID] = n.QualifiedName
			}
			// Value-flow links first; the type hierarchy (schema 1.34) when
			// no value of the receiver's type was seen reaching an interface.
			for _, kind := range []string{"interface_method", "type_hierarchy"} {
				seen := map[string]bool{}
				for _, e := range es {
					if e.Type != types.EdgeDispatchesTo || e.DispatchKind != kind || !own[e.Dst] || seen[name[e.Src]] || name[e.Src] == "" {
						continue
					}
					seen[name[e.Src]] = true
					seeds = append(seeds, name[e.Src])
				}
				if len(seeds) > 0 {
					sort.Strings(seeds)
					if kind == "type_hierarchy" {
						return seeds, "type_hierarchy"
					}
					return seeds, "dispatches_to"
				}
			}
		}
	}
	if seeds = implementsSeeds(reader, seed.QName); len(seeds) > 0 {
		return seeds, "implements"
	}
	return nil, ""
}

var dispatchGraphs sync.Map // store.Reader -> bool

// hasDispatch reports whether the graph holds runtime links (schema 1.31).
func hasDispatch(reader store.Reader) bool {
	if v, ok := dispatchGraphs.Load(reader); ok {
		return v.(bool)
	}
	es, err := reader.QueryEdgesByType(string(types.EdgeDispatchesTo))
	has := err == nil && len(es) > 0
	dispatchGraphs.Store(reader, has)
	return has
}

// implementsSeeds is the type-hierarchy bridge: the same-named method on
// each interface the seed's receiver type implements.
func implementsSeeds(reader store.Reader, methodQname string) []string {
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
	var ifaces []string
	for _, n := range nodes {
		if _, ok := ifaceIDs[n.ID]; ok && n.Type == types.NodeInterface {
			ifaces = append(ifaces, n.ID)
		}
	}
	if len(ifaces) == 0 {
		return nil
	}
	// Keep only interfaces that declare the method: a type that implements
	// Lifecycle does not make every one of its methods a Lifecycle method.
	members, _, err := reader.NeighborhoodByIDs(ifaces, 1, false, string(types.EdgeDefines))
	if err != nil {
		return nil
	}
	var seeds []string
	seen := map[string]struct{}{}
	for _, m := range members {
		if m.Type != types.NodeMethod || leafOf(m.QualifiedName) != methodShort {
			continue
		}
		if _, dup := seen[m.QualifiedName]; dup {
			continue
		}
		seen[m.QualifiedName] = struct{}{}
		seeds = append(seeds, m.QualifiedName)
	}
	sort.Strings(seeds) // deterministic for prompt-cache stability
	return seeds
}

func leafOf(q string) string {
	if i := strings.LastIndex(q, "."); i >= 0 {
		return q[i+1:]
	}
	return q
}

// reverseCallersUnion walks the reverse call graph from `resolved` AND from every
// interface method it satisfies (the interface-dispatch bridge). See walkCalls
// for how hops are counted.
func reverseCallersUnion(reader store.Reader, resolved seedRef, depth int) ([]types.Node, []types.Edge, error) {
	return reverseCallersUnionTraced(reader, resolved, depth, nil)
}

// reverseCallersUnionTraced is reverseCallersUnion with a trace record of the
// interface seeds and where they came from.
func reverseCallersUnionTraced(reader store.Reader, resolved seedRef, depth int, t *callTrace) ([]types.Node, []types.Edge, error) {
	seeds := []seedRef{resolved}
	ifaces, source := dispatchSeeds(reader, resolved)
	for _, q := range ifaces {
		seeds = append(seeds, seedRef{QName: q})
	}
	t.add("interface_seeds", map[string]any{"source": source, "seeds": ifaces})
	var ids []string
	for _, sd := range seeds {
		sdIDs, err := sd.nodeIDs(reader)
		if err != nil {
			return nil, nil, err
		}
		ids = append(ids, sdIDs...)
	}
	return walkCalls(reader, ids, depth, true /*reverse*/)
}

// forwardCallees walks the forward call graph from the seed.
func forwardCallees(reader store.Reader, seed seedRef, depth int) ([]types.Node, []types.Edge, error) {
	ids, err := seed.nodeIDs(reader)
	if err != nil {
		return nil, nil, err
	}
	return walkCalls(reader, ids, depth, false /*reverse*/)
}

// callEdgeTypes are the edge types find_callers / find_callees follow.
// Without this filter the BFS would also walk contains/defines and
// return the file holding a method as one of its "callers" —
// semantically wrong and noisy for LLM consumers.
//
// dispatches_to (schema 1.31) carries a dynamic call on to the code it can
// run: from an interface method to its implementations, and from a function
// that calls a function value to the functions the value can hold.
//
// has_modifier, emits_event and reverts_with are what a Solidity function
// runs besides its calls: the modifiers it applies, the events it emits and
// the errors it reverts with. Without them the tools returned none of 70
// modifier applications, 58 emits and 128 reverts of go-stablenet's
// contracts, and an event, an error or a modifier had no callers at all.
var callEdgeTypes = []string{
	string(types.EdgeCalls), string(types.EdgeInvokes), string(types.EdgeDispatchesTo),
	string(types.EdgeHasModifier), string(types.EdgeEmitsEvent), string(types.EdgeRevertsWith),
}

// seedRef is a resolved traversal seed: the qualified name reported back to
// the caller and, when the seed named exact nodes (a canonical id, a promoted
// method's declaration), their ids. Traversals start from IDs when they are
// set, so a canonical id walks only its own node even when other nodes share
// its qualified name.
type seedRef struct {
	QName string
	IDs   []string
	// Note is set when the caller named the copy of a promoted method: the
	// seed is then the method it copies and Note says so
	// (store.NameResolution.SeedNote).
	Note string
	// AlsoNamed lists the other declarations whose qualified name is the
	// input, when the input was the canonical id of the seed
	// (store.NameResolution.AlsoNamed).
	AlsoNamed []string
	// OtherCase lists names that differ from the input only in case; set
	// only when the seed did not resolve.
	OtherCase []string
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
	// Map order differs on every call; the answer must not.
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	return nodes, append(fwdE, revE...), nil
}

// nodeIDs returns the ids of the seed's own nodes.
func (s seedRef) nodeIDs(reader store.Reader) ([]string, error) {
	if len(s.IDs) > 0 {
		return s.IDs, nil
	}
	ns, err := s.nodes(reader)
	if err != nil {
		return nil, err
	}
	ids := make([]string, 0, len(ns))
	for _, n := range ns {
		ids = append(ids, n.ID)
	}
	return ids, nil
}

// nodes returns the seed's own nodes.
func (s seedRef) nodes(reader store.Reader) ([]types.Node, error) {
	if len(s.IDs) > 0 {
		return reader.NodesByIDs(s.IDs)
	}
	return reader.FindSymbol(s.QName, true, store.FindSymbolOptions{})
}

// result adds the seed to a traversal payload: seed_qname always, seed_id
// when the traversal started from one exact node, and promoted_from when the
// caller named the copy of a promoted method and got the method it copies.
func (s seedRef) result(payload map[string]any) map[string]any {
	payload["seed_qname"] = s.QName
	if len(s.IDs) == 1 {
		payload["seed_id"] = s.IDs[0]
	}
	if len(s.AlsoNamed) > 0 {
		payload[alsoNamedKey] = s.AlsoNamed
	}
	if s.Note != "" {
		payload[store.SeedKeyPromotedFrom] = s.QName
		payload[store.SeedKeyNote] = s.Note
	}
	return payload
}

// resolveSeed maps a user-supplied seed — a canonical id, a fully-qualified
// name ("a.Greet") or a bare short name ("Greet") — to the seed the
// graph-traversal tools (find_callers / find_callees / find_references /
// get_subgraph / change_history) walk from. The rule is store.ResolveName,
// the one resolver every tool that takes a symbol name uses.
//
// Return contract:
//   - ok=true, seed set             one canonical id or one distinct qname.
//   - ambiguous=true, candidates set a bare name matched more than one
//     distinct qname; the caller surfaces them so the agent can retry with a
//     full qname (ADR-0001: a multi-match is reported, never silently picked).
//   - ok=false                      nothing matched (not_found); the seed
//     carries the names that differ only in case, if any.
//
// Several nodes sharing ONE qualified_name (e.g. the same symbol across
// languages) is not ambiguous for a qname seed: every such node becomes a
// traversal root. A canonical id seed starts from its own node only.
//
// A seed that names the copy of a promoted method ("logger.dummy.Prepare"
// for state.StateDB.Prepare) is the method it copies: the copy has no edges
// but the one from its type, so walking from it answered "no callers".
func resolveSeed(reader store.Reader, seed, lang string) (resolved seedRef, candidates []string, ambiguous bool, ok bool) {
	res, err := store.ResolveName(reader, seed, lang)
	switch {
	case err != nil:
		return seedRef{}, nil, false, false
	case res.Ambiguous():
		return seedRef{}, res.Candidates, true, false
	case !res.Found():
		return seedRef{OtherCase: res.OtherCase}, nil, false, false
	}
	ref := seedRef{QName: res.QName, Note: res.SeedNote(), AlsoNamed: res.AlsoNamed}
	if res.ByID {
		for _, n := range res.Nodes {
			ref.IDs = append(ref.IDs, n.ID)
		}
	}
	return ref, nil, false, true
}

// seedMiss returns the payload a traversal tool answers with when its seed
// did not resolve to one symbol (ambiguous, or not found), and nil when it
// did. One function, so every tool reports a miss the same way.
func seedMiss(seed string, resolved seedRef, candidates []string, ambiguous, ok bool) map[string]any {
	switch {
	case ambiguous:
		return seedAmbiguousResult(seed, candidates)
	case !ok:
		out := seedNotFoundResult(seed)
		if len(resolved.OtherCase) > 0 {
			out["other_case"] = resolved.OtherCase
		}
		return out
	}
	return nil
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

// alsoNamedKey lists, in an answer for a canonical id that other
// declarations carry as their qualified name, those declarations.
const alsoNamedKey = "also_named"

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
			"qname":       store.PrintedName(reader, n),
			"file":        n.FilePath,
			"line":        n.StartLine,
			"end_line":    n.EndLine,
			"confidence":  n.Confidence,
			"signature":   n.Signature,
			"usage_score": n.UsageScore,
		}
		// A promoted copy has the declared method's file and lines; say
		// which method it is so the two are not taken for two declarations.
		if from, ok := store.PromotedFrom(n); ok {
			m["promoted_from"] = from
		}
		// sub_kind refines the type: a TypeAlias node is every type that is
		// not a struct or an interface, and sub_kind says whether it is an
		// alias (type A = B) or a defined type (type A B).
		if n.SubKind != "" {
			m["sub_kind"] = n.SubKind
		}
		// canonical_id (ADR-0001): the globally-unique, import-path-qualified
		// identity. Included only when present (empty for builtins, AST-only
		// mode, and not-yet-wired languages) so the agent can feed it back as an
		// unambiguous seed to the traversal tools. Omitted when empty.
		if n.CanonicalID != "" {
			m["canonical_id"] = n.CanonicalID
		}
		// The operations on a sync primitive at a site whose primitive the
		// graph does not name, and why (the legend explains them).
		if len(n.SyncUnknown) > 0 {
			m["sync_unknown"] = n.SyncUnknown
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
