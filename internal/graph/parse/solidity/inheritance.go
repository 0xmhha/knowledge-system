package solidity

import (
	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Sol inheritance + interface implementation detection.
//
// Spec: docs/graph/design/solidity-inheritance-and-interface-dispatch.md §3.1, §4.1
//
// Scope: the `is`-clause on contract / interface declarations:
//
//	contract Child is Parent { ... }              → EdgeExtends    (Child → Parent)
//	contract Impl  is IERC20 { ... }              → EdgeImplements (Impl  → IERC20)
//	contract Multi is Base, IFoo, IBar { ... }    → 3 edges, one per parent
//	interface IB   is IA { ... }                  → EdgeExtends    (IB → IA)
//
// Rules:
//   - Same-file resolution → ConfExtracted; cross-file → ConfInferred.
//   - A base that no file of the graph declares gets no node and no edge;
//     it is counted as unresolved.
//   - One edge per base, in source order. The linearisation (c3_linearization.go)
//     is a separate index used for name lookup and storage layout.
//   - Edge direction: child → parent ("child inherits from parent"),
//     consistent with Go implements.go and TS heritage semantics.
//   - extends or implements is decided by the resolved base: implements when
//     a contract lists an interface, extends otherwise.
//
// The edges are emitted in Pass 2 by linkInheritance from the bases each
// container's declaration header lists, for every container of the graph.
// Pass 1 queues one pending reference per (child, base) pair as well; it
// makes no edge, it records that the child's file depends on the base's
// name, which is what makes an incremental build re-resolve the child's
// file when the base's file changes.
//
// Out of scope here (separate detectors): virtual/override,
// interface dispatch `IFoo(addr).bar()`, `using For`.

// runInheritance walks every `is`-clause match and queues the dependency
// reference of each (child, base) pair. The query (queryInheritance) matches
// both contract_declaration and interface_declaration.
func (v *declVisitor) runInheritance() {
	query, qErr := sitter.NewQuery(v.lang, queryInheritance)
	if qErr != nil {
		return
	}
	defer func() { query.Close() }()
	cur := sitter.NewQueryCursor()
	defer func() { cur.Close() }()
	matches := cur.Matches(query, v.root, v.src)
	names := query.CaptureNames()
	for {
		m := matches.Next()
		if m == nil {
			break
		}
		var childNode *sitter.Node
		var parentNode *sitter.Node
		for _, c := range m.Captures {
			switch names[c.Index] {
			case "child":
				n := c.Node
				childNode = &n
			case "parent":
				n := c.Node
				parentNode = &n
			}
		}
		if childNode == nil || parentNode == nil {
			continue
		}
		// The source is the contract or interface that declares the
		// `is` clause; containerOf gives the id its node was emitted with.
		srcID, _, ok := v.containerOf(childNode)
		if !ok {
			continue
		}
		parentName := parentNode.Utf8Text(v.src)
		v.pending = append(v.pending, parse.PendingRef{
			SrcID:        srcID,
			EdgeType:     types.EdgeExtends,
			TargetQName:  parentName,
			Line:         int(parentNode.StartPosition().Row) + 1,
			DispatchKind: dispatchKindInherit,
		})
	}
}

// dispatchKindInherit tags the dependency references of `is` clauses.
const dispatchKindInherit = "inherit"
