package solidity

import (
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Solidity `using For` library extension binding detection.
//
// Spec: docs/graph/design/solidity-inheritance-and-interface-dispatch.md §4.6
// Dispatch index: docs/graph/archive/DISPATCH-WITHIN-LANG-SEMANTICS.md §2.
//
// Scope (Q9-1 (b), Q9-2 (a), Q9-3 (a)):
//
//	contract Foo { using SafeMath for uint256; }   → EdgeUsesFor (Foo → SafeMath)
//	contract Foo { using SafeMath for *; }         → EdgeUsesFor (same shape)
//	contract Foo {
//	  using SafeMath for uint256;
//	  using Address  for address;
//	}                                              → 2 EdgeUsesFor (Foo→SafeMath, Foo→Address)
//
// Initial limitations per §4.6 (several are lifted by the recovery and
// method-call walkers below):
//   - free-function form `using {Lib.f1, Lib.f2} for T;` is dropped
//     (separate AST shape).
//   - file-level using directive (Solidity 0.8.13+ global binding) is
//     out of scope; runUsingFor only recognises directives nested inside a
//     contract / library / interface body.
//   - typeName is parsed by the grammar but is not exposed on
//     the EdgeUsesFor — one edge per directive regardless of typeName
//     ("`using A for X; using A for Y;` ⇒ two A edges, not deduped").
//
// Method-call dispatch resolution (`balance.add(...)` → SafeMath.add
// EdgeCalls) is handled by runUsingForCalls on top of the receiver
// declared-type index (state-var / parameter / local-var).
// See §4.6.6 for the follow-up list.
//
// Pass 1 → Pass 2 split: same idiom as inheritance.
//   - Pass 1 emits PendingRef with DispatchKind="using_for", SrcID
//     hashed off the enclosing container's name identifier (matches
//     emitContractLikeNode's ID derivation in abstract_library.go),
//     TargetQName = libraryName.
//   - Pass 2 (resolveUsingForRef in resolve.go) resolves libraryName
//     against byName[NodeContract] (libraries are emitted as
//     NodeContract+SubKind="library") and emits one EdgeUsesFor
//     edge per match. same-file → ConfExtracted, cross-file →
//     ConfInferred, unresolved → drop.

// runUsingFor walks every `using_directive` match nested inside a
// contract / library / interface body and queues two PendingRefs per
// directive:
//
//  1. dispatchKindUsingFor — TargetQName=libraryName. Drives the
//     EdgeUsesFor (Contract → Library) emission in Pass 2.
//  2. dispatchKindUsingForTypeBind — TargetQName encodes
//     `<libraryName>|<typeName>`. Carries the bound type so Pass 2 can
//     build a per-contract binding map for method-call resolution.
//     Does not produce a graph edge — pure side-channel data.
//
// Container identifier comes from the same query capture so both
// PendingRefs' SrcID line up with the container's existing node ID.
func (v *declVisitor) runUsingFor() {
	query, qErr := sitter.NewQuery(v.lang, queryUsingFor)
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
		var containerNode *sitter.Node
		var libNode *sitter.Node
		var typeNode *sitter.Node
		for _, c := range m.Captures {
			switch names[c.Index] {
			case "container":
				n := c.Node
				containerNode = &n
			case "lib":
				n := c.Node
				libNode = &n
			case "type":
				n := c.Node
				typeNode = &n
			}
		}
		if containerNode == nil || libNode == nil {
			continue
		}
		// SrcID must align with the container node emitted by
		// runContractDecl / runLibraryDecl / runInterfaceDecl — all
		// hash on (name, "sol", name-node startByte). containerNode is
		// the same `@name` identifier these emit paths use.
		containerName := containerNode.Utf8Text(v.src)
		containerStart := int(containerNode.StartByte())
		srcID := parse.MakeID(containerName, "sol", containerStart)
		libName := libNode.Utf8Text(v.src)
		// Skip leading namespace-alias
		// identifiers. tree-sitter emits one query match per
		// type_alias child identifier, so `using L.SafeMath for ...`
		// fans out into two matches (libName="L" and libName="SafeMath").
		// The "L" match is a namespace prefix from
		// `import "./util.sol" as L` — registered in namespaceAliases
		// by runImportAliases. Without this skip Pass 2's byName
		// lookup could surface an unrelated contract named L as a
		// false-positive EdgeUsesFor.
		if v.namespaceAliases[libName] {
			continue
		}
		// Apply per-file alias map populated
		// by runImportAliases. `import {SafeMath as SM} from "..."` +
		// `using SM for ...` → SM is resolved to SafeMath here so the
		// downstream binding pipeline keys match the actual library
		// node name.
		if orig, hit := v.importAliases[libName]; hit {
			libName = orig
		}
		line := int(libNode.StartPosition().Row) + 1
		v.pending = append(v.pending, parse.PendingRef{
			SrcID:        srcID,
			EdgeType:     types.EdgeUsesFor,
			TargetQName:  libName,
			Line:         line,
			DispatchKind: dispatchKindUsingFor,
		})
		// Typebind PendingRef. Source field is either type_name
		// (specific binding) or any_source_type (`for *` wildcard); we
		// normalise both into a string token used as the bind-map key
		// (typeName "*" for wildcard, raw text otherwise — matched
		// against NodeField.Signature in Pass 2).
		typeName := normaliseUsingForType(typeNode, v.src)
		v.pending = append(v.pending, parse.PendingRef{
			SrcID:        srcID,
			EdgeType:     types.EdgeUsesFor, // unused for typebind — Resolve routes by DispatchKind
			TargetQName:  libName + "|" + typeName,
			Line:         line,
			DispatchKind: dispatchKindUsingForTypeBind,
		})
	}
}

// runFileLevelUsingFor is an ERROR-tolerant
// recovery for file-level `using LibName for T [global];` directives.
// Sol 0.8.13+ syntax is grammar-blocked in
// vendored tree-sitter-solidity v1.2.11 — the directive parses into
// a single ERROR child of source_file with the `using` keyword
// misclassified as an identifier inside a fake user_defined_type.
//
// AST shape this walks (verified against the vendored grammar):
//
//	source_file
//	  ERROR "using SafeMath for uint256 [global];"
//	    type_name
//	      user_defined_type
//	        identifier "using"          ← keyword misinterpreted
//	    identifier "<LibName>"          ← library name (recoverable)
//	    type_name
//	      primitive_type / user_defined_type / mapping  ← bound type
//	    [identifier "global"]           ← optional qualifier
//
// Recovery heuristic:
//   - Only ERROR children of source_file (not nested ERRORs).
//   - text must start with "using " (case-sensitive Sol keyword).
//   - Library name = first identifier child whose text != "using".
//   - Bound type = first type_name child (skips the fake one
//     wrapping `using`) — extractTypeNameText handles all variants.
//
// Per-container fan-out: Sol file-level using semantics ("applies to
// all contracts in this file", whether or not the `global` qualifier
// is present) mean we emit one PendingRef pair per container node in
// v.nodes (NodeContract / NodeLibrary / NodeInterface). Each emit
// produces the same (dispatchKindUsingFor + dispatchKindUsingForTypeBind)
// pair that runUsingFor produces for contract-body directives — so
// downstream binding and dispatch resolution paths reuse the existing
// infrastructure unchanged.
//
// Must run after runContractDecl / runLibraryDecl / runInterfaceDecl
// so v.nodes is populated. visit() schedules it right after
// runUsingFor.
//
// Skip silently when:
//   - text doesn't start with "using " (some other ERROR).
//   - no identifier (other than "using") found (malformed beyond
//     recovery).
//   - no type_name found (incomplete directive).
//
// Trade-offs:
//   - `global` / non-global distinction is not preserved on the edge
//     (Sol semantics make file-level binding apply to all contracts
//     either way; the qualifier only matters for cross-file scope,
//     which cross-file resolution already handles via NodeFile).
//   - This is a recovery walker, not a real parser. If the vendored
//     grammar upgrades to support file-level using natively, the
//     resulting using_directive nodes will be picked up by runUsingFor
//     (whose 3-arm query covers contract / library / interface bodies
//     but NOT source_file scope yet — a future query addition would
//     pick them up there too). At that point this walker can be
//     retired or hardened to a NOP guard.
func (v *declVisitor) runFileLevelUsingFor() {
	if v.root == nil {
		return
	}
	// Collect every container ID emitted by prior runContractDecl /
	// runLibraryDecl / runInterfaceDecl. File-level binding fans out
	// to each per Sol semantics.
	// Libraries are also NodeContract (SubKind="library" per
	// abstract_library.go) but they shouldn't bind to themselves —
	// filter them out so we don't get `Math → Math` phantom edges.
	var containerIDs []string
	for _, n := range v.nodes {
		switch n.Type {
		case types.NodeContract:
			if n.SubKind != "library" {
				containerIDs = append(containerIDs, n.ID)
			}
		case types.NodeInterface:
			containerIDs = append(containerIDs, n.ID)
		}
	}
	if len(containerIDs) == 0 {
		return
	}
	for i := uint(0); i < v.root.NamedChildCount(); i++ {
		child := v.root.NamedChild(i)
		if child == nil || child.Kind() != "ERROR" {
			continue
		}
		text := child.Utf8Text(v.src)
		if !strings.HasPrefix(text, "using ") {
			continue
		}
		var libName, typeName string
		for j := uint(0); j < child.NamedChildCount(); j++ {
			grand := child.NamedChild(j)
			if grand == nil {
				continue
			}
			switch grand.Kind() {
			case "identifier":
				t := grand.Utf8Text(v.src)
				if t != "using" && libName == "" {
					libName = t
				}
			case "type_name":
				// Skip the fake type_name wrapping the `using` keyword.
				// The recoverable bound type comes after the library
				// identifier in source order.
				if libName != "" && typeName == "" {
					typeName = extractTypeNameText(grand, v.src)
				}
			}
		}
		if libName == "" || typeName == "" {
			continue
		}
		// Apply the same alias normalisations runUsingFor does so
		// downstream binding keys line up regardless of which walker
		// recovered the directive.
		if v.namespaceAliases[libName] {
			continue
		}
		if orig, hit := v.importAliases[libName]; hit {
			libName = orig
		}
		line := int(child.StartPosition().Row) + 1
		byteOff := int(child.StartByte())
		for _, srcID := range containerIDs {
			v.pending = append(v.pending, parse.PendingRef{
				SrcID:        srcID,
				EdgeType:     types.EdgeUsesFor,
				TargetQName:  libName,
				Line:         line,
				ByteOffset:   byteOff,
				DispatchKind: dispatchKindUsingFor,
			})
			v.pending = append(v.pending, parse.PendingRef{
				SrcID:        srcID,
				EdgeType:     types.EdgeUsesFor, // routed by DispatchKind in Pass 2
				TargetQName:  libName + "|" + typeName,
				Line:         line,
				ByteOffset:   byteOff,
				DispatchKind: dispatchKindUsingForTypeBind,
			})
		}
	}
}

// normaliseUsingForType returns the bind-map key for the source field of
// a using_directive. Handles three cases:
//
//   - any_source_type (`*` wildcard) → "*" sentinel.
//   - type_name wrapping primitive_type / user_defined_type → the
//     declared type text (matches NodeField.Signature output of
//     extractTypeNameText).
//   - nil or unknown shape → "" (binding map entry is created but won't
//     match any real receiver — Pass 2 binding lookup naturally drops).
func normaliseUsingForType(typeNode *sitter.Node, src []byte) string {
	if typeNode == nil {
		return ""
	}
	if typeNode.Kind() == "any_source_type" {
		return "*"
	}
	// type_name shape — same idiom as extractTypeNameText so the
	// stored binding key compares 1:1 against NodeField.Signature.
	return extractTypeNameText(typeNode, src)
}

// runUsingForCalls is the method-call dispatch detector.
// Scans every member_expression that fits the `<identifier>.<identifier>(...)`
// shape (identifier receiver, §4.6.6 Q9-2 (a)) and
// queues a PendingRef tagged dispatchKindUsingForCall. Pass 2 resolves
// these against the (contractID, typeName) → libraryName binding map
// built from dispatchKindUsingForTypeBind refs.
//
// Predicate: object is identifier, property is identifier, the
// member_expression's parent (or grandparent through `expression`) is a
// call_expression. The other shapes (chained calls, struct-field and
// this-prefixed receivers) are matched by the dedicated predicates below.
//
// Encoding: TargetQName=`<receiverName>|<methodName>`. Pass 2 splits on
// `|`, resolves receiverName against the state-var name table, joins
// with the binding map.
//
// Note: this runs *in addition to* the existing call-site emission that
// produces NodeCallSite via the body-walk passes. EdgeCalls is added
// only when binding resolution succeeds; mismatched receivers (no state
// var of that name, no binding for the type) silently drop, matching
// the strict-purge policy used by inheritance / override / interface dispatch.
func (v *declVisitor) runUsingForCalls() {
	const query = `(member_expression) @member`
	q, qErr := sitter.NewQuery(v.lang, query)
	if qErr != nil {
		return
	}
	defer func() { q.Close() }()
	cur := sitter.NewQueryCursor()
	defer func() { cur.Close() }()
	matches := cur.Matches(q, v.root, v.src)
	names := q.CaptureNames()
	for {
		m := matches.Next()
		if m == nil {
			break
		}
		for _, c := range m.Captures {
			if names[c.Index] != "member" {
				continue
			}
			memberNode := c.Node
			// State-variable / parameter receiver. Try first
			// because it's the most common shape.
			if receiverName, methodName, ok := matchStateVarMethodCall(&memberNode, v.src); ok {
				fnQ, fnStart, fnOK := nearestFunctionQnameAndStart(&memberNode, v.src)
				if !fnOK {
					continue
				}
				v.pending = append(v.pending, parse.PendingRef{
					SrcID:        parse.MakeID(fnQ, "sol", fnStart),
					EdgeType:     types.EdgeCalls,
					TargetQName:  receiverName + "|" + methodName,
					Line:         int(memberNode.StartPosition().Row) + 1,
					ByteOffset:   int(memberNode.StartByte()),
					DispatchKind: dispatchKindUsingForCall,
				})
				continue
			}
			// `this.<stateVar>.<method>(...)` — explicit-this
			// equivalent of the bare-name shape. Reuses its
			// dispatch kind + resolver (encoding is identical).
			if stateVarName, methodName, ok := matchThisReceiverMethodCall(&memberNode, v.src); ok {
				fnQ, fnStart, fnOK := nearestFunctionQnameAndStart(&memberNode, v.src)
				if !fnOK {
					continue
				}
				v.pending = append(v.pending, parse.PendingRef{
					SrcID:        parse.MakeID(fnQ, "sol", fnStart),
					EdgeType:     types.EdgeCalls,
					TargetQName:  stateVarName + "|" + methodName,
					Line:         int(memberNode.StartPosition().Row) + 1,
					ByteOffset:   int(memberNode.StartByte()),
					DispatchKind: dispatchKindUsingForCall,
				})
				continue
			}
			// `this.<stateVar>.<f1>.<f2>...<fN>.<method>(...)` —
			// this-prefixed nested member chain. N ≥ 1 struct-field hops
			// after `this.<stateVar>`. depth-0 (`this.<stateVar>.<method>`)
			// is matchThisReceiverMethodCall. Resolver uses callerContainerID
			// as implicit `this`
			// target — looks up stateVar in stateVarTypes only (no
			// paramTypes), then walks structFieldTypes for each hop.
			if stateVar, fields, methodName, ok := matchThisPrefixedNestedChain(&memberNode, v.src); ok {
				fnQ, fnStart, fnOK := nearestFunctionQnameAndStart(&memberNode, v.src)
				if !fnOK {
					continue
				}
				encoded := stateVar
				for _, f := range fields {
					encoded += "|" + f
				}
				encoded += "|" + methodName
				v.pending = append(v.pending, parse.PendingRef{
					SrcID:        parse.MakeID(fnQ, "sol", fnStart),
					EdgeType:     types.EdgeCalls,
					TargetQName:  encoded,
					Line:         int(memberNode.StartPosition().Row) + 1,
					ByteOffset:   int(memberNode.StartByte()),
					DispatchKind: dispatchKindUsingForThisNestedChainCall,
				})
				continue
			}
			// `<obj>.<field>.<method>(...)` — struct-field
			// receiver. obj is a state-var/parameter whose type is a
			// struct; field is one of the struct's members. Resolver
			// walks structFieldTypes to find the field's type, then
			// uses that as the binding lookup key.
			if objName, fieldName, methodName, ok := matchStructFieldReceiverMethodCall(&memberNode, v.src); ok {
				fnQ, fnStart, fnOK := nearestFunctionQnameAndStart(&memberNode, v.src)
				if !fnOK {
					continue
				}
				v.pending = append(v.pending, parse.PendingRef{
					SrcID:        parse.MakeID(fnQ, "sol", fnStart),
					EdgeType:     types.EdgeCalls,
					TargetQName:  objName + "|" + fieldName + "|" + methodName,
					Line:         int(memberNode.StartPosition().Row) + 1,
					ByteOffset:   int(memberNode.StartByte()),
					DispatchKind: dispatchKindUsingForStructFieldCall,
				})
				continue
			}
			// `<obj>.<field1>.<field2>.<method>(...)` —
			// nested struct field receiver. obj's struct has a nested
			// struct field; field2 is a member of that nested struct.
			// Resolver walks structFieldTypes twice.
			if objName, f1, f2, methodName, ok := matchNestedStructFieldReceiverMethodCall(&memberNode, v.src); ok {
				fnQ, fnStart, fnOK := nearestFunctionQnameAndStart(&memberNode, v.src)
				if !fnOK {
					continue
				}
				v.pending = append(v.pending, parse.PendingRef{
					SrcID:        parse.MakeID(fnQ, "sol", fnStart),
					EdgeType:     types.EdgeCalls,
					TargetQName:  objName + "|" + f1 + "|" + f2 + "|" + methodName,
					Line:         int(memberNode.StartPosition().Row) + 1,
					ByteOffset:   int(memberNode.StartByte()),
					DispatchKind: dispatchKindUsingForNestedStructFieldCall,
				})
				continue
			}
			// Generic member-chain walker — fallback for
			// arbitrary-depth (≥ 3) pure member access chains.
			// Depth 1/2 are caught by the hardcoded struct-field predicates.
			if objName, fields, methodName, ok := matchGenericMemberChain(&memberNode, v.src); ok {
				fnQ, fnStart, fnOK := nearestFunctionQnameAndStart(&memberNode, v.src)
				if !fnOK {
					continue
				}
				encoded := objName
				for _, f := range fields {
					encoded += "|" + f
				}
				encoded += "|" + methodName
				v.pending = append(v.pending, parse.PendingRef{
					SrcID:        parse.MakeID(fnQ, "sol", fnStart),
					EdgeType:     types.EdgeCalls,
					TargetQName:  encoded,
					Line:         int(memberNode.StartPosition().Row) + 1,
					ByteOffset:   int(memberNode.StartByte()),
					DispatchKind: dispatchKindUsingForGenericMemberChainCall,
				})
				continue
			}
			// Chained call shape `<fn>().<method>(...)`. Inner
			// expression is a plain function call (function-position
			// identifier); resolver looks up the inner function's
			// return type and treats it as the receiver type.
			if innerFnName, methodName, ok := matchChainedMethodCall(&memberNode, v.src); ok {
				fnQ, fnStart, fnOK := nearestFunctionQnameAndStart(&memberNode, v.src)
				if !fnOK {
					continue
				}
				v.pending = append(v.pending, parse.PendingRef{
					SrcID:        parse.MakeID(fnQ, "sol", fnStart),
					EdgeType:     types.EdgeCalls,
					TargetQName:  innerFnName + "|" + methodName,
					Line:         int(memberNode.StartPosition().Row) + 1,
					ByteOffset:   int(memberNode.StartByte()),
					DispatchKind: dispatchKindUsingForChainCall,
				})
				continue
			}
			// Cross-contract chained shape `<obj>.<innerFn>().<method>(...)`.
			// Inner expression is a member call on a state-var / parameter
			// receiver. Resolver follows the receiver's contract type to
			// look up the inner function's declaration, then uses that
			// function's return type as the chain receiver.
			if receiverObj, innerFnName, methodName, ok := matchCrossContractChain(&memberNode, v.src); ok {
				fnQ, fnStart, fnOK := nearestFunctionQnameAndStart(&memberNode, v.src)
				if !fnOK {
					continue
				}
				v.pending = append(v.pending, parse.PendingRef{
					SrcID:        parse.MakeID(fnQ, "sol", fnStart),
					EdgeType:     types.EdgeCalls,
					TargetQName:  receiverObj + "|" + innerFnName + "|" + methodName,
					Line:         int(memberNode.StartPosition().Row) + 1,
					ByteOffset:   int(memberNode.StartByte()),
					DispatchKind: dispatchKindUsingForCrossChainCall,
				})
				continue
			}
			// Depth-2 chained shape `<innerFn1>().<innerFn2>().<method>(...)`.
			// Each chain link is a plain function call (function-position
			// identifier at the innermost level). Resolver walks two
			// levels of funcReturnTypes to reach the binding type.
			if innerFn1, innerFn2, methodName, ok := matchDeepChainedMethodCall(&memberNode, v.src); ok {
				fnQ, fnStart, fnOK := nearestFunctionQnameAndStart(&memberNode, v.src)
				if !fnOK {
					continue
				}
				v.pending = append(v.pending, parse.PendingRef{
					SrcID:        parse.MakeID(fnQ, "sol", fnStart),
					EdgeType:     types.EdgeCalls,
					TargetQName:  innerFn1 + "|" + innerFn2 + "|" + methodName,
					Line:         int(memberNode.StartPosition().Row) + 1,
					ByteOffset:   int(memberNode.StartByte()),
					DispatchKind: dispatchKindUsingForDeepChainCall,
				})
				continue
			}
			// Deep cross-contract chained shape
			// `<obj>.<innerFn1>().<innerFn2>().<method>(...)`. Combines
			// the receiver-typed inner method shape with the depth-2
			// chain. Resolver walks: receiverObj → receiverType →
			// innerFn1 in receiverType → returnType1 → innerFn2 in
			// returnType1 → returnType2 → library binding.
			if recvObj, innerFn1, innerFn2, methodName, ok := matchDeepCrossContractChain(&memberNode, v.src); ok {
				fnQ, fnStart, fnOK := nearestFunctionQnameAndStart(&memberNode, v.src)
				if !fnOK {
					continue
				}
				v.pending = append(v.pending, parse.PendingRef{
					SrcID:        parse.MakeID(fnQ, "sol", fnStart),
					EdgeType:     types.EdgeCalls,
					TargetQName:  recvObj + "|" + innerFn1 + "|" + innerFn2 + "|" + methodName,
					Line:         int(memberNode.StartPosition().Row) + 1,
					ByteOffset:   int(memberNode.StartByte()),
					DispatchKind: dispatchKindUsingForDeepCrossChainCall,
				})
				continue
			}
			// Depth-3 same-contract chained shape
			// `<innerFn1>().<innerFn2>().<innerFn3>().<method>(...)`.
			// Three levels of funcReturnTypes walks. matchGenericChain
			// covers deeper chains.
			if innerFn1, innerFn2, innerFn3, methodName, ok := matchTripleChainedMethodCall(&memberNode, v.src); ok {
				fnQ, fnStart, fnOK := nearestFunctionQnameAndStart(&memberNode, v.src)
				if !fnOK {
					continue
				}
				v.pending = append(v.pending, parse.PendingRef{
					SrcID:        parse.MakeID(fnQ, "sol", fnStart),
					EdgeType:     types.EdgeCalls,
					TargetQName:  innerFn1 + "|" + innerFn2 + "|" + innerFn3 + "|" + methodName,
					Line:         int(memberNode.StartPosition().Row) + 1,
					ByteOffset:   int(memberNode.StartByte()),
					DispatchKind: dispatchKindUsingForTripleChainCall,
				})
				continue
			}
			// Generic iterative walker — fallback for chains
			// deeper than the hardcoded patterns. Covers depth ≥ 4
			// same-contract chains and depth ≥ 3 cross-contract chains.
			// The hardcoded predicates catch shorter shapes via earlier
			// dispatch branches; this only fires when they have rejected.
			if mode, recvObj, segs, methodName, ok := matchGenericChain(&memberNode, v.src); ok {
				fnQ, fnStart, fnOK := nearestFunctionQnameAndStart(&memberNode, v.src)
				if !fnOK {
					continue
				}
				// Encode as `mode|<recv>|<seg1>|...|<segN>|method`
				// (recv part is empty string for same-contract — kept
				// in the encoding so the resolver's SplitN positions
				// remain consistent across modes).
				encoded := mode + "|" + recvObj
				for _, s := range segs {
					encoded += "|" + s
				}
				encoded += "|" + methodName
				v.pending = append(v.pending, parse.PendingRef{
					SrcID:        parse.MakeID(fnQ, "sol", fnStart),
					EdgeType:     types.EdgeCalls,
					TargetQName:  encoded,
					Line:         int(memberNode.StartPosition().Row) + 1,
					ByteOffset:   int(memberNode.StartByte()),
					DispatchKind: dispatchKindUsingForGenericChainCall,
				})
				continue
			}
		}
	}
}

// matchThisPrefixedNestedChain tests
// whether a member_expression fits the this-prefixed nested chain
// shape `this.<stateVar>.<f1>.<f2>....<fN>.<method>(...)` for N ≥ 1
// struct-field hops after `this.<stateVar>`.
//
// matchThisReceiverMethodCall already catches depth-0
// (`this.<stateVar>.<method>`); this only fires for N ≥ 1.
//
// Returns (stateVar, fields, methodName, true) on match. fields[0] is
// the innermost hop (immediately after stateVar); fields[N-1] is the
// outermost hop (immediately before method).
//
// AST shape (N = 2, `this.x.f1.f2.method`):
//
//	call_expression                            ← outer .method(...)
//	  function: expression
//	    member_expression                      ← outer
//	      object: expression
//	        member_expression                  ← this.x.f1.f2
//	          object: expression
//	            member_expression              ← this.x.f1
//	              object: expression
//	                member_expression          ← this.x
//	                  object: identifier "this"
//	                  property: identifier (stateVar)
//	              property: identifier (f1)
//	          property: identifier (f2)
//	      property: identifier (methodName)
//
// Disambiguation:
//   - matchThisReceiverMethodCall catches depth-0 (it fires first).
//   - The struct-field predicates explicitly reject `this` as innermost
//     object — this is the dedicated cousin for the this-prefixed shape.
//
// Caller dispatch order: state-var → this-receiver → this-prefixed
// nested chain → struct-field → nested struct-field → generic member
// chain → call-chain predicates.
func matchThisPrefixedNestedChain(member *sitter.Node, src []byte) (string, []string, string, bool) {
	if member == nil {
		return "", nil, "", false
	}
	property := member.ChildByFieldName("property")
	if property == nil || property.Kind() != "identifier" {
		return "", nil, "", false
	}
	parent := member.Parent()
	if parent != nil && parent.Kind() == "expression" {
		parent = parent.Parent()
	}
	if parent == nil || parent.Kind() != "call_expression" {
		return "", nil, "", false
	}
	methodName := property.Utf8Text(src)
	// Walk back through the member-expression chain, collecting properties
	// outer-first. Bail when we hit a non-member, non-`this` innermost.
	var revFields []string
	cur := unwrapExpression(member.ChildByFieldName("object"))
	for cur != nil && cur.Kind() == "member_expression" {
		curProp := cur.ChildByFieldName("property")
		if curProp == nil || curProp.Kind() != "identifier" {
			return "", nil, "", false
		}
		revFields = append(revFields, curProp.Utf8Text(src))
		innerObj := unwrapExpression(cur.ChildByFieldName("object"))
		if innerObj == nil {
			return "", nil, "", false
		}
		if innerObj.Kind() == "identifier" {
			if innerObj.Utf8Text(src) != "this" {
				return "", nil, "", false
			}
			// revFields collected outer→inner. Floor is N ≥ 1
			// struct hop: revFields length ≥ 2 (1 stateVar + ≥ 1 field).
			// length == 1 → matchThisReceiverMethodCall territory; reject.
			if len(revFields) < 2 {
				return "", nil, "", false
			}
			rev := reverseStrSlice(revFields)
			return rev[0], rev[1:], methodName, true
		}
		if innerObj.Kind() == "member_expression" {
			cur = innerObj
			continue
		}
		return "", nil, "", false
	}
	return "", nil, "", false
}

// matchGenericMemberChain is the generic
// iterative walker for arbitrary-depth pure member access chain
// `<obj>.<field1>.<field2>....<fieldN>.<method>(...)`. No call
// expressions in between — pure struct field traversal.
//
// Used as fallback after the hardcoded struct-field predicates have
// rejected (depth ≥ 3 member chain).
//
// Returns (objName, fields, methodName, true) on match. fields[0] is
// the innermost field (closest to obj), fields[N-1] is the outermost.
//
// Disambiguation: matchStructFieldReceiverMethodCall catches depth-1,
// matchNestedStructFieldReceiverMethodCall catches depth-2. This only
// fires when depth ≥ 3. this-prefixed nested chain drops (consistent
// with the struct-field predicates — "this" is handled by the
// this-receiver predicates).
func matchGenericMemberChain(member *sitter.Node, src []byte) (string, []string, string, bool) {
	if member == nil {
		return "", nil, "", false
	}
	property := member.ChildByFieldName("property")
	if property == nil || property.Kind() != "identifier" {
		return "", nil, "", false
	}
	parent := member.Parent()
	if parent != nil && parent.Kind() == "expression" {
		parent = parent.Parent()
	}
	if parent == nil || parent.Kind() != "call_expression" {
		return "", nil, "", false
	}
	methodName := property.Utf8Text(src)
	var revFields []string
	cur := unwrapExpression(member.ChildByFieldName("object"))
	for cur != nil && cur.Kind() == "member_expression" {
		curProp := cur.ChildByFieldName("property")
		if curProp == nil || curProp.Kind() != "identifier" {
			return "", nil, "", false
		}
		revFields = append(revFields, curProp.Utf8Text(src))
		innerObj := unwrapExpression(cur.ChildByFieldName("object"))
		if innerObj == nil {
			return "", nil, "", false
		}
		if innerObj.Kind() == "identifier" {
			objText := innerObj.Utf8Text(src)
			if objText == "this" {
				return "", nil, "", false
			}
			if len(revFields) < 3 {
				return "", nil, "", false
			}
			return objText, reverseStrSlice(revFields), methodName, true
		}
		if innerObj.Kind() == "member_expression" {
			cur = innerObj
			continue
		}
		return "", nil, "", false
	}
	return "", nil, "", false
}

// matchNestedStructFieldReceiverMethodCall tests whether a
// member_expression fits the depth-2 nested struct
// field receiver shape `<obj>.<field1>.<field2>.<method>(...)`. obj is
// state-var/parameter whose declared type is a struct; field1 is a
// nested-struct member of that type; field2 is a member of field1's
// struct type; method is the using-for dispatch target.
//
// Returns (objName, field1Name, field2Name, methodName, true) on match.
//
// AST shape (4 levels of member_expression nesting, no call_expressions
// in between — pure member access chain):
//
//	call_expression                              ← outer .method(...)
//	  function: expression
//	    member_expression                        ← outer
//	      object: expression
//	        member_expression                    ← mid <obj>.<field1>.<field2>
//	          object: expression
//	            member_expression                ← inner <obj>.<field1>
//	              object: identifier (objName)
//	              property: identifier (field1Name)
//	          property: identifier (field2Name)
//	      property: identifier (methodName)
//
// Disambiguation:
//   - matchStructFieldReceiverMethodCall (`<obj>.<field>.<method>`): one less member layer.
//   - matchCrossContractChain (`<obj>.<fn>().<method>`): middle layer is call_expression,
//     not member_expression.
//   - matchThisReceiverMethodCall (`this.<state-var>.<method>`): innermost is `this`, not
//     state-var/parameter identifier.
//
// Caller dispatch order: state-var → this-receiver → struct-field →
// nested struct-field → call-chain predicates.
func matchNestedStructFieldReceiverMethodCall(member *sitter.Node, src []byte) (string, string, string, string, bool) {
	if member == nil {
		return "", "", "", "", false
	}
	property := member.ChildByFieldName("property")
	if property == nil || property.Kind() != "identifier" {
		return "", "", "", "", false
	}
	object := member.ChildByFieldName("object")
	midMember := unwrapExpression(object)
	if midMember == nil || midMember.Kind() != "member_expression" {
		return "", "", "", "", false
	}
	midProperty := midMember.ChildByFieldName("property")
	if midProperty == nil || midProperty.Kind() != "identifier" {
		return "", "", "", "", false
	}
	midObject := midMember.ChildByFieldName("object")
	innerMember := unwrapExpression(midObject)
	if innerMember == nil || innerMember.Kind() != "member_expression" {
		return "", "", "", "", false
	}
	innerObj := innerMember.ChildByFieldName("object")
	innerObjIdent := unwrapExpression(innerObj)
	if innerObjIdent == nil || innerObjIdent.Kind() != "identifier" {
		return "", "", "", "", false
	}
	objText := innerObjIdent.Utf8Text(src)
	if objText == "this" {
		// `this.<field1>.<field2>.<method>` is a cousin of
		// matchThisReceiverMethodCall. This predicate only handles
		// non-this receivers to keep scope tight; the this-prefixed
		// form is matched by matchThisPrefixedNestedChain.
		return "", "", "", "", false
	}
	innerProperty := innerMember.ChildByFieldName("property")
	if innerProperty == nil || innerProperty.Kind() != "identifier" {
		return "", "", "", "", false
	}
	// Outer must be invoked.
	parent := member.Parent()
	if parent != nil && parent.Kind() == "expression" {
		parent = parent.Parent()
	}
	if parent == nil || parent.Kind() != "call_expression" {
		return "", "", "", "", false
	}
	return objText,
		innerProperty.Utf8Text(src),
		midProperty.Utf8Text(src),
		property.Utf8Text(src),
		true
}

// matchStructFieldReceiverMethodCall tests
// whether a member_expression fits the struct-field-receiver shape
// `<obj>.<field>.<method>(...)` where:
//   - `<obj>` is an identifier (state-var or parameter receiver).
//   - `<obj>` is NOT "this" — that's matchThisReceiverMethodCall.
//   - `<field>` is a struct member name (resolver verifies against the
//     struct field type index).
//   - `<method>` is the using-for dispatch target.
//
// Returns (objName, fieldName, methodName, true) on match.
//
// AST shape (same as matchThisReceiverMethodCall except inner.object is any non-"this"
// identifier):
//
//	call_expression                            ← outer .method(...)
//	  function: expression
//	    member_expression                      ← outer
//	      object: expression
//	        member_expression                  ← inner <obj>.<field>
//	          object: identifier (objName, != "this")
//	          property: identifier (fieldName)
//	      property: identifier (method)
//
// Disambiguation:
//   - matchThisReceiverMethodCall (`this.<field>.<method>`): caller
//     dispatch tries it first; it requires inner.object text =
//     "this", so this only fires for non-this receivers.
//   - matchCrossContractChain (`<obj>.<fn>().<method>`): its middle is a call, not a
//     member access. Shape-disjoint.
//
// Caller dispatch order: state-var → this-receiver → struct-field →
// call-chain predicates.
func matchStructFieldReceiverMethodCall(member *sitter.Node, src []byte) (string, string, string, bool) {
	if member == nil {
		return "", "", "", false
	}
	property := member.ChildByFieldName("property")
	if property == nil || property.Kind() != "identifier" {
		return "", "", "", false
	}
	object := member.ChildByFieldName("object")
	innerMember := unwrapExpression(object)
	if innerMember == nil || innerMember.Kind() != "member_expression" {
		return "", "", "", false
	}
	innerObj := innerMember.ChildByFieldName("object")
	innerObjIdent := unwrapExpression(innerObj)
	if innerObjIdent == nil || innerObjIdent.Kind() != "identifier" {
		return "", "", "", false
	}
	objText := innerObjIdent.Utf8Text(src)
	if objText == "this" {
		return "", "", "", false // matchThisReceiverMethodCall handles this case
	}
	innerProperty := innerMember.ChildByFieldName("property")
	if innerProperty == nil || innerProperty.Kind() != "identifier" {
		return "", "", "", false
	}
	// Outer must be invoked.
	parent := member.Parent()
	if parent != nil && parent.Kind() == "expression" {
		parent = parent.Parent()
	}
	if parent == nil || parent.Kind() != "call_expression" {
		return "", "", "", false
	}
	return objText, innerProperty.Utf8Text(src), property.Utf8Text(src), true
}

// matchThisReceiverMethodCall tests whether
// a member_expression fits the `this.<stateVar>.<method>(...)` shape.
// `this` is implicit current-contract reference; the resolver treats
// `<stateVar>` as a state variable on the caller's container — same
// dispatch as the bare-name shape `<stateVar>.<method>(...)`.
//
// Returns (stateVarName, methodName, true) on match.
//
// AST shape:
//
//	call_expression                         ← outer .method(...)
//	  function: expression
//	    member_expression                   ← outer
//	      object: expression
//	        member_expression               ← inner this.<stateVar>
//	          object: identifier "this"
//	          property: identifier (stateVarName)
//	      property: identifier (method)
//
// Reuses matchStateVarMethodCall's dispatch kind + resolver: PendingRef
// encoding `<stateVarName>|<method>` is identical, so the resolver
// path is single-source. This only adds a new predicate; no new
// resolver helper required.
//
// Caller dispatch order ensures the simpler bare `<x>.<method>` shape
// claims the call when shapes don't include `this`. This sits after
// the hardcoded chain predicates and before the generic chain walker —
// `this.x.method` would also match the cross-contract chain pattern
// structurally (innerObj=identifier "this"), but that path would look up
// "this" in stateVarTypes / paramTypes and miss; this predicate short-circuits
// that wasted lookup by recognising the `this` keyword directly.
func matchThisReceiverMethodCall(member *sitter.Node, src []byte) (string, string, bool) {
	if member == nil {
		return "", "", false
	}
	property := member.ChildByFieldName("property")
	if property == nil || property.Kind() != "identifier" {
		return "", "", false
	}
	object := member.ChildByFieldName("object")
	innerMember := unwrapExpression(object)
	if innerMember == nil || innerMember.Kind() != "member_expression" {
		return "", "", false
	}
	innerObj := innerMember.ChildByFieldName("object")
	innerObjIdent := unwrapExpression(innerObj)
	if innerObjIdent == nil || innerObjIdent.Kind() != "identifier" {
		return "", "", false
	}
	if innerObjIdent.Utf8Text(src) != "this" {
		return "", "", false
	}
	innerProperty := innerMember.ChildByFieldName("property")
	if innerProperty == nil || innerProperty.Kind() != "identifier" {
		return "", "", false
	}
	// Outer must be invoked.
	parent := member.Parent()
	if parent != nil && parent.Kind() == "expression" {
		parent = parent.Parent()
	}
	if parent == nil || parent.Kind() != "call_expression" {
		return "", "", false
	}
	return innerProperty.Utf8Text(src), property.Utf8Text(src), true
}

// matchStateVarMethodCall tests whether a member_expression fits the
// state-variable method-call shape `<identifier>.<identifier>` AND its
// parent context is a call_expression (i.e. it's actually being called,
// not just member-accessed for a property read).
//
// Returns (receiverName, methodName, true) on match. Rejects chained
// shapes (`foo().bar`, `IFoo(x).bar`), member receivers (`a.b.c`), and
// pure property reads (`obj.field` outside a call) — those are
// out of scope here.
func matchStateVarMethodCall(member *sitter.Node, src []byte) (string, string, bool) {
	if member == nil {
		return "", "", false
	}
	property := member.ChildByFieldName("property")
	if property == nil || property.Kind() != "identifier" {
		return "", "", false
	}
	object := member.ChildByFieldName("object")
	innerObj := unwrapExpression(object)
	if innerObj == nil || innerObj.Kind() != "identifier" {
		return "", "", false
	}
	// Must be the function-position of a call_expression. The member
	// node is wrapped in an expression node, which is itself a child of
	// the call_expression's `function` field.
	parent := member.Parent()
	if parent != nil && parent.Kind() == "expression" {
		parent = parent.Parent()
	}
	if parent == nil || parent.Kind() != "call_expression" {
		return "", "", false
	}
	return innerObj.Utf8Text(src), property.Utf8Text(src), true
}

// matchDeepChainedMethodCall tests whether
// a member_expression fits the depth-2 chained shape
// `<innerFn1>().<innerFn2>().<method>(...)`. Each link in the chain is
// a plain function call whose function-position is an identifier; the
// chain unwinds left-to-right by resolving each function's return type.
//
// Returns (innerFn1, innerFn2, method, true) on match.
//
// AST shape:
//
//	call_expression                                  ← outer .method(...)
//	  function: expression
//	    member_expression                            ← outer
//	      object: expression
//	        call_expression                          ← middle .innerFn2(...)
//	          function: expression
//	            member_expression                    ← middle
//	              object: expression
//	                call_expression                  ← inner .innerFn1(...)
//	                  function: expression
//	                    identifier (innerFn1)        ← matchDeepChainedMethodCall captures this
//	              property: identifier (innerFn2)
//	      property: identifier (method)
//
// Rejects:
//   - `obj.foo().bar().baz()` — innermost identifier is preceded by
//     `obj.` (member_expression). That's matchDeepCrossContractChain.
//   - depth >= 3 (`f().g().h().i()`) — outer's chain has one more
//     layer wrapping than this handles (matchTripleChainedMethodCall).
//
// Disambiguation from matchCrossContractChain (`<obj>.<innerFn>().<method>`):
// that one has inner member_expression.object = identifier (state-var / param); this
// has inner member_expression.object = call_expression (further chain).
// Caller dispatches matchCrossContractChain first, so its predicate
// claims the call when shapes overlap on simpler chains.
func matchDeepChainedMethodCall(member *sitter.Node, src []byte) (string, string, string, bool) {
	if member == nil {
		return "", "", "", false
	}
	property := member.ChildByFieldName("property")
	if property == nil || property.Kind() != "identifier" {
		return "", "", "", false
	}
	object := member.ChildByFieldName("object")
	middleCall := unwrapExpression(object)
	if middleCall == nil || middleCall.Kind() != "call_expression" {
		return "", "", "", false
	}
	middleFn := middleCall.ChildByFieldName("function")
	middleMember := unwrapExpression(middleFn)
	if middleMember == nil || middleMember.Kind() != "member_expression" {
		return "", "", "", false
	}
	middleProperty := middleMember.ChildByFieldName("property")
	if middleProperty == nil || middleProperty.Kind() != "identifier" {
		return "", "", "", false
	}
	middleObject := middleMember.ChildByFieldName("object")
	innerCall := unwrapExpression(middleObject)
	if innerCall == nil || innerCall.Kind() != "call_expression" {
		return "", "", "", false
	}
	innerFn := innerCall.ChildByFieldName("function")
	innerIdent := unwrapExpression(innerFn)
	if innerIdent == nil || innerIdent.Kind() != "identifier" {
		return "", "", "", false
	}
	// Outer must itself be the function-position of a call_expression.
	parent := member.Parent()
	if parent != nil && parent.Kind() == "expression" {
		parent = parent.Parent()
	}
	if parent == nil || parent.Kind() != "call_expression" {
		return "", "", "", false
	}
	return innerIdent.Utf8Text(src), middleProperty.Utf8Text(src), property.Utf8Text(src), true
}

// matchGenericChain is the generic iterative
// walker for arbitrary-depth chained dispatch. Used as the final
// fallback after the hardcoded chain predicates have rejected.
//
// Two chain shapes recognised:
//
//   - same-contract: `<fn1>().<fn2>()....<fnN>().<method>(...)`
//     The innermost function-position is a plain identifier (local
//     function reference). Returns mode="same", recvObj="".
//
//   - cross-contract: `<obj>.<fn1>().<fn2>()....<fnN>().<method>(...)`
//     The innermost function-position is a member_expression whose
//     object is a plain identifier (receiver var / parameter).
//     Returns mode="cross", recvObj=identifier text.
//
// Segments returned in source order: segs[0] is the innermost chain
// link (called first in the runtime chain), segs[N-1] is the outermost
// chain link (called last before method).
//
// Returns (mode, recvObj, segs, method, true) on match. Drops shapes:
//   - empty chain (`<obj>.<method>(...)` — covered by matchStateVarMethodCall).
//   - non-call-expression inner shapes (`obj.field.method()`).
//   - outermost wrapper not a call_expression (member-read, not call).
//
// Caller dispatch order ensures this only fires after the hardcoded
// predicates reject: matchChainedMethodCall catches depth-1 same-contract,
// matchDeepChainedMethodCall depth-2, matchTripleChainedMethodCall depth-3,
// matchCrossContractChain depth-1 cross-contract,
// matchDeepCrossContractChain depth-2 cross-contract. This
// handles depth ≥ 4 same-contract and depth ≥ 3 cross-contract.
func matchGenericChain(member *sitter.Node, src []byte) (string, string, []string, string, bool) {
	if member == nil {
		return "", "", nil, "", false
	}
	property := member.ChildByFieldName("property")
	if property == nil || property.Kind() != "identifier" {
		return "", "", nil, "", false
	}
	// Outer must be invoked (it's a method call, not just a read).
	parent := member.Parent()
	if parent != nil && parent.Kind() == "expression" {
		parent = parent.Parent()
	}
	if parent == nil || parent.Kind() != "call_expression" {
		return "", "", nil, "", false
	}
	method := property.Utf8Text(src)
	// Walk down: outer.object → call → fn → ...
	// segs collected in REVERSE order (outermost first), reversed at end.
	var revSegs []string
	cur := unwrapExpression(member.ChildByFieldName("object"))
	for cur != nil && cur.Kind() == "call_expression" {
		fn := unwrapExpression(cur.ChildByFieldName("function"))
		if fn == nil {
			return "", "", nil, "", false
		}
		switch fn.Kind() {
		case "identifier":
			// Innermost — same-contract chain. Done.
			fnName := fn.Utf8Text(src)
			revSegs = append(revSegs, fnName)
			return "same", "", reverseStrSlice(revSegs), method, true
		case "member_expression":
			seg := fn.ChildByFieldName("property")
			if seg == nil || seg.Kind() != "identifier" {
				return "", "", nil, "", false
			}
			revSegs = append(revSegs, seg.Utf8Text(src))
			innerObj := unwrapExpression(fn.ChildByFieldName("object"))
			if innerObj == nil {
				return "", "", nil, "", false
			}
			if innerObj.Kind() == "identifier" {
				// Innermost member.object is identifier — cross-contract.
				return "cross", innerObj.Utf8Text(src), reverseStrSlice(revSegs), method, true
			}
			if innerObj.Kind() == "call_expression" {
				cur = innerObj
				continue
			}
			// Unknown inner shape (e.g. member-of-member `a.b.c`) — not handled.
			return "", "", nil, "", false
		default:
			return "", "", nil, "", false
		}
	}
	return "", "", nil, "", false
}

// reverseStrSlice reverses a string slice in place and returns it.
// Helper for matchGenericChain — segments collected outermost-first
// during the walk, reversed at the end so callers see source order.
func reverseStrSlice(s []string) []string {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

// matchTripleChainedMethodCall tests
// whether a member_expression fits the depth-3 chained shape
// `<innerFn1>().<innerFn2>().<innerFn3>().<method>(...)`. Same-contract
// chain only (the cross-contract variant and deeper chains are handled
// by matchGenericChain).
//
// Returns (innerFn1, innerFn2, innerFn3, method, true) on match.
//
// AST shape (4 levels of call_expression / member_expression nesting):
//
//	call_expression                           ← outer .method(...)
//	  function: expression
//	    member_expression                     ← outer
//	      object: expression
//	        call_expression                   ← L3 .innerFn3(...)
//	          function: expression
//	            member_expression             ← L3
//	              object: expression
//	                call_expression           ← L2 .innerFn2(...)
//	                  function: expression
//	                    member_expression     ← L2
//	                      object: expression
//	                        call_expression   ← L1 .innerFn1(...)
//	                          function: expression
//	                            identifier (innerFn1)
//	                      property: identifier (innerFn2)
//	              property: identifier (innerFn3)
//	      property: identifier (method)
//
// Disambiguation:
//   - matchDeepChainedMethodCall (`a().b().c()`): one less call_expression nesting layer.
//   - matchDeepCrossContractChain (`obj.a().b().c()`): same nesting depth but innermost
//     function-position is member_expression with identifier receiver,
//     not bare identifier.
//   - matchGenericChain: even deeper chains, generic walker.
//
// Caller dispatch order (state-var → depth-1 → depth-1 cross-contract →
// depth-2 → depth-2 cross-contract → depth-3) ensures simpler predicates
// reject before this fires.
func matchTripleChainedMethodCall(member *sitter.Node, src []byte) (string, string, string, string, bool) {
	if member == nil {
		return "", "", "", "", false
	}
	property := member.ChildByFieldName("property")
	if property == nil || property.Kind() != "identifier" {
		return "", "", "", "", false
	}
	object := member.ChildByFieldName("object")
	l3Call := unwrapExpression(object)
	if l3Call == nil || l3Call.Kind() != "call_expression" {
		return "", "", "", "", false
	}
	l3Fn := l3Call.ChildByFieldName("function")
	l3Member := unwrapExpression(l3Fn)
	if l3Member == nil || l3Member.Kind() != "member_expression" {
		return "", "", "", "", false
	}
	l3Property := l3Member.ChildByFieldName("property")
	if l3Property == nil || l3Property.Kind() != "identifier" {
		return "", "", "", "", false
	}
	l3Object := l3Member.ChildByFieldName("object")
	l2Call := unwrapExpression(l3Object)
	if l2Call == nil || l2Call.Kind() != "call_expression" {
		return "", "", "", "", false
	}
	l2Fn := l2Call.ChildByFieldName("function")
	l2Member := unwrapExpression(l2Fn)
	if l2Member == nil || l2Member.Kind() != "member_expression" {
		return "", "", "", "", false
	}
	l2Property := l2Member.ChildByFieldName("property")
	if l2Property == nil || l2Property.Kind() != "identifier" {
		return "", "", "", "", false
	}
	l2Object := l2Member.ChildByFieldName("object")
	l1Call := unwrapExpression(l2Object)
	if l1Call == nil || l1Call.Kind() != "call_expression" {
		return "", "", "", "", false
	}
	l1Fn := l1Call.ChildByFieldName("function")
	l1Ident := unwrapExpression(l1Fn)
	if l1Ident == nil || l1Ident.Kind() != "identifier" {
		return "", "", "", "", false
	}
	parent := member.Parent()
	if parent != nil && parent.Kind() == "expression" {
		parent = parent.Parent()
	}
	if parent == nil || parent.Kind() != "call_expression" {
		return "", "", "", "", false
	}
	return l1Ident.Utf8Text(src),
		l2Property.Utf8Text(src),
		l3Property.Utf8Text(src),
		property.Utf8Text(src),
		true
}

// matchDeepCrossContractChain tests
// whether a member_expression fits the deep cross-contract chained
// shape `<obj>.<innerFn1>().<innerFn2>().<method>(...)`. Two-link chain
// originating from a state-var / parameter receiver.
//
// Returns (receiverObj, innerFn1, innerFn2, method, true) on match.
//
// AST shape:
//
//	call_expression                                  ← outer .method(...)
//	  function: expression
//	    member_expression                            ← outer
//	      object: expression
//	        call_expression                          ← middle .innerFn2(...)
//	          function: expression
//	            member_expression                    ← middle
//	              object: expression
//	                call_expression                  ← inner .innerFn1(...)
//	                  function: expression
//	                    member_expression            ← inner
//	                      object: expression
//	                        identifier (obj)         ← matchDeepCrossContractChain captures
//	                      property: identifier (innerFn1)
//	              property: identifier (innerFn2)
//	      property: identifier (method)
//
// Disambiguation:
//   - matchCrossContractChain (`obj.foo().bar()`): outer's chain stops one link shorter —
//     middle.object is identifier, not call_expression.
//   - matchDeepChainedMethodCall (`foo().bar().baz()`): innermost function-position is
//     identifier, not member_expression on an identifier.
//   - deeper chains (`obj.foo().bar().baz().qux()`): even one link deeper —
//     outer's object is yet another call_expression wrapping this
//     pattern.
//
// Caller dispatch order (state-var → depth-1 → depth-1 cross-contract →
// depth-2 → depth-2 cross-contract) ensures this only fires on shapes
// the simpler predicates rejected.
func matchDeepCrossContractChain(member *sitter.Node, src []byte) (string, string, string, string, bool) {
	if member == nil {
		return "", "", "", "", false
	}
	property := member.ChildByFieldName("property")
	if property == nil || property.Kind() != "identifier" {
		return "", "", "", "", false
	}
	object := member.ChildByFieldName("object")
	middleCall := unwrapExpression(object)
	if middleCall == nil || middleCall.Kind() != "call_expression" {
		return "", "", "", "", false
	}
	middleFn := middleCall.ChildByFieldName("function")
	middleMember := unwrapExpression(middleFn)
	if middleMember == nil || middleMember.Kind() != "member_expression" {
		return "", "", "", "", false
	}
	middleProperty := middleMember.ChildByFieldName("property")
	if middleProperty == nil || middleProperty.Kind() != "identifier" {
		return "", "", "", "", false
	}
	middleObject := middleMember.ChildByFieldName("object")
	innerCall := unwrapExpression(middleObject)
	if innerCall == nil || innerCall.Kind() != "call_expression" {
		return "", "", "", "", false
	}
	innerFn := innerCall.ChildByFieldName("function")
	innerMember := unwrapExpression(innerFn)
	if innerMember == nil || innerMember.Kind() != "member_expression" {
		return "", "", "", "", false
	}
	innerObj := innerMember.ChildByFieldName("object")
	innerObjIdent := unwrapExpression(innerObj)
	if innerObjIdent == nil || innerObjIdent.Kind() != "identifier" {
		return "", "", "", "", false
	}
	innerProperty := innerMember.ChildByFieldName("property")
	if innerProperty == nil || innerProperty.Kind() != "identifier" {
		return "", "", "", "", false
	}
	parent := member.Parent()
	if parent != nil && parent.Kind() == "expression" {
		parent = parent.Parent()
	}
	if parent == nil || parent.Kind() != "call_expression" {
		return "", "", "", "", false
	}
	return innerObjIdent.Utf8Text(src),
		innerProperty.Utf8Text(src),
		middleProperty.Utf8Text(src),
		property.Utf8Text(src),
		true
}

// matchCrossContractChain tests whether a
// member_expression fits the cross-contract chained shape
// `<obj>.<innerFn>().<method>` where the outer expression is invoking
// `<method>` on the return value of a method call on a state-var /
// parameter receiver.
//
// Returns (receiverObjName, innerFnName, methodName, true) on match.
//
// AST shape (verified via probe):
//
//	call_expression                                ← outer .method(...)
//	  function: expression
//	    member_expression                          ← outer
//	      object: expression
//	        call_expression                        ← inner .innerFn(...)
//	          function: expression
//	            member_expression                  ← inner
//	              object: expression
//	                identifier (receiverObjName)   ← state var / param
//	              property: identifier (innerFnName)
//	      property: identifier (methodName)
//
// Rejects:
//   - `factory().bar()` — inner function-position is identifier, not
//     member_expression (handled by matchChainedMethodCall).
//   - `obj.field.bar()` — inner is just member access without call
//     (handled by the struct-field predicates).
//   - Deeper chains like `obj.foo().baz().bar()` — outer's object is
//     itself a call_expression whose function is a member_expression on
//     another call_expression (handled by the deeper chain predicates).
//
// Edge case: the receiverObjName might also match an interface variable
// (`IFoo iface; iface.fn()`) — the resolver re-checks against
// stateVarTypes / paramTypes, where the recorded typeName
// would be "IFoo". The resolver step 3 maps that to the interface's
// declared methods.
func matchCrossContractChain(member *sitter.Node, src []byte) (string, string, string, bool) {
	if member == nil {
		return "", "", "", false
	}
	property := member.ChildByFieldName("property")
	if property == nil || property.Kind() != "identifier" {
		return "", "", "", false
	}
	object := member.ChildByFieldName("object")
	innerCall := unwrapExpression(object)
	if innerCall == nil || innerCall.Kind() != "call_expression" {
		return "", "", "", false
	}
	innerFn := innerCall.ChildByFieldName("function")
	innerMember := unwrapExpression(innerFn)
	if innerMember == nil || innerMember.Kind() != "member_expression" {
		return "", "", "", false
	}
	innerObj := innerMember.ChildByFieldName("object")
	innerObjIdent := unwrapExpression(innerObj)
	if innerObjIdent == nil || innerObjIdent.Kind() != "identifier" {
		return "", "", "", false
	}
	innerProperty := innerMember.ChildByFieldName("property")
	if innerProperty == nil || innerProperty.Kind() != "identifier" {
		return "", "", "", false
	}
	// Outer must be a call_expression (the chained `.method()` is itself
	// invoked).
	parent := member.Parent()
	if parent != nil && parent.Kind() == "expression" {
		parent = parent.Parent()
	}
	if parent == nil || parent.Kind() != "call_expression" {
		return "", "", "", false
	}
	return innerObjIdent.Utf8Text(src), innerProperty.Utf8Text(src), property.Utf8Text(src), true
}

// matchChainedMethodCall tests whether a
// member_expression fits the chained-call shape `<fn>(...).<method>`
// where the inner expression is a *plain function call* (bare
// identifier in the function position), not a type cast.
//
// Returns (innerFnName, methodName, true) on match.
//
// Shape (verified via AST dump):
//
//	call_expression                       ← outer .method(...)
//	  function: expression
//	    member_expression
//	      object: expression
//	        call_expression               ← inner fn() call
//	          function: expression
//	            identifier (innerFnName)  ← what matchChainedMethodCall captures
//	      property: identifier (methodName)
//
// Rejects shapes already covered or out of scope for matchChainedMethodCall:
//   - `IFoo(addr).bar()` (interface dispatch — inner identifier is
//     a TYPE name; matchInterfaceDispatch handles this).
//   - `obj.foo().bar()` (member-receiver chain — inner function-position
//     is itself a member_expression, not a bare identifier). See matchCrossContractChain.
//   - `obj.field.bar()` (pure property access chain). See the struct-field predicates.
//
// Disambiguation from interface dispatch: the runUsingForCalls walker calls this
// AFTER matchStateVarMethodCall has rejected. To avoid emitting both an
// interface-dispatch EdgeInvokes and a chained EdgeCalls for the same site, callers should
// also verify the resolved inner identifier maps to a *function* (not
// an interface) — that's Pass 2's job via funcByQName, so the parser
// emits the chained PendingRef unconditionally and the resolver drops
// when no funcID matches.
func matchChainedMethodCall(member *sitter.Node, src []byte) (string, string, bool) {
	if member == nil {
		return "", "", false
	}
	property := member.ChildByFieldName("property")
	if property == nil || property.Kind() != "identifier" {
		return "", "", false
	}
	object := member.ChildByFieldName("object")
	innerCall := unwrapExpression(object)
	if innerCall == nil || innerCall.Kind() != "call_expression" {
		return "", "", false
	}
	innerFn := innerCall.ChildByFieldName("function")
	innerIdent := unwrapExpression(innerFn)
	if innerIdent == nil || innerIdent.Kind() != "identifier" {
		return "", "", false
	}
	// Outer must be a call_expression (the chained `.method()` is itself
	// invoked, not just read as a property).
	parent := member.Parent()
	if parent != nil && parent.Kind() == "expression" {
		parent = parent.Parent()
	}
	if parent == nil || parent.Kind() != "call_expression" {
		return "", "", false
	}
	return innerIdent.Utf8Text(src), property.Utf8Text(src), true
}

// dispatchKindUsingFor tags PendingRefs originating from using-for
// binding detection. String literal matches the existing idiom (inheritance
// "inherit", override "override"/"override_explicit", interface dispatch "interface_dispatch").
const dispatchKindUsingFor = "using_for"

// dispatchKindUsingForTypeBind carries the bound-type information
// for binding-map construction. Does not produce a graph edge — Pass 2
// reads these to populate (contractID, typeName) → libraryName map.
const dispatchKindUsingForTypeBind = "using_for_typebind"

// dispatchKindUsingForCall tags PendingRefs that resolve to
// EdgeCalls (caller function → library function) once the binding map
// has been built. TargetQName encodes `<receiverName>|<methodName>`.
const dispatchKindUsingForCall = "using_for_call"

// dispatchKindUsingForChainCall (matchChainedMethodCall) tags PendingRefs for chained
// call dispatch: `<fn>().<method>(...)`. TargetQName encodes
// `<innerFnName>|<methodName>` — same shape as using_for_call but the
// receiver lookup goes through funcReturnTypes instead of stateVarTypes
// / paramTypes (Pass 2 splits the resolver paths by DispatchKind).
const dispatchKindUsingForChainCall = "using_for_chain_call"

// dispatchKindUsingForCrossChainCall (matchCrossContractChain) tags PendingRefs for
// cross-contract chained dispatch: `<obj>.<innerFn>().<method>(...)`.
// TargetQName encodes `<receiverObjName>|<innerFnName>|<methodName>`
// (three parts, '|'-separated). Pass 2 splits the chain across
// stateVarTypes / paramTypes (obj→type) + byName (type→contract) +
// funcByQName (contract.innerFn→funcID) + funcReturnTypes (funcID→
// returnType) + bindings (callerContractID, returnType→library).
const dispatchKindUsingForCrossChainCall = "using_for_cross_chain_call"

// dispatchKindUsingForDeepChainCall (matchDeepChainedMethodCall) tags PendingRefs for depth-2
// chained dispatch: `<innerFn1>().<innerFn2>().<method>(...)`.
// TargetQName encodes `<innerFn1>|<innerFn2>|<method>` (three parts).
// Resolver walks two levels of funcReturnTypes (innerFn1 → returnType1
// → resolve innerFn2 in returnType1's namespace → returnType2 → binding
// lookup on returnType2) before reaching the library function.
const dispatchKindUsingForDeepChainCall = "using_for_deep_chain_call"

// dispatchKindUsingForDeepCrossChainCall (matchDeepCrossContractChain) tags PendingRefs for
// the deep cross-contract chained dispatch
// `<obj>.<innerFn1>().<innerFn2>().<method>(...)`. TargetQName encodes
// `<obj>|<innerFn1>|<innerFn2>|<method>` (four parts). Resolver
// combines the cross-contract receiver type lookup with the depth-2
// return chain — 8-step total chain.
const dispatchKindUsingForDeepCrossChainCall = "using_for_deep_cross_chain_call"

// dispatchKindUsingForTripleChainCall (matchTripleChainedMethodCall) tags PendingRefs for
// depth-3 same-contract chained dispatch:
// `<innerFn1>().<innerFn2>().<innerFn3>().<method>(...)`. TargetQName
// encodes `<innerFn1>|<innerFn2>|<innerFn3>|<method>` (four parts).
// Resolver walks three levels of funcReturnTypes — matchDeepChainedMethodCall's pattern
// with one more link.
const dispatchKindUsingForTripleChainCall = "using_for_triple_chain_call"

// dispatchKindUsingForStructFieldCall (matchStructFieldReceiverMethodCall) tags PendingRefs for
// struct-field-receiver dispatch `<obj>.<field>.<method>(...)`.
// TargetQName encodes `<objName>|<fieldName>|<methodName>`.
// Resolver chain: obj → typeName (stateVarTypes / paramTypes) →
// (typeName, fieldName) → fieldType (structFieldTypes) →
// (callerContractID, fieldType) → libraryName (bindings) → method.
const dispatchKindUsingForStructFieldCall = "using_for_struct_field_call"

// dispatchKindUsingForNestedStructFieldCall (matchNestedStructFieldReceiverMethodCall) tags PendingRefs
// for depth-2 nested struct field dispatch
// `<obj>.<field1>.<field2>.<method>(...)`. TargetQName encodes
// `<objName>|<field1>|<field2>|<methodName>` (4 parts). Resolver walks
// structFieldTypes twice — obj's struct field1 → its struct's field2 →
// binding lookup on field2's type.
const dispatchKindUsingForNestedStructFieldCall = "using_for_nested_struct_field_call"

// dispatchKindUsingForThisNestedChainCall (matchThisPrefixedNestedChain) — this-prefixed
// nested member chain. PendingRef target encoding:
// `<stateVar>|<f1>|...|<fN>|<method>` (N ≥ 1). Resolver uses
// callerContainerID as implicit `this` target, looks up stateVar in
// stateVarTypes only (no paramTypes), then walks structFieldTypes.
const dispatchKindUsingForThisNestedChainCall = "using_for_this_nested_chain_call"

// dispatchKindUsingForGenericMemberChainCall (matchGenericMemberChain) — generic
// iterative member-chain dispatch (depth ≥ 3). TargetQName encodes
// `<obj>|<f1>|<f2>|...|<fN>|<method>` (variable parts). Resolver walks
// structFieldTypes N times.
const dispatchKindUsingForGenericMemberChainCall = "using_for_generic_member_chain_call"

// dispatchKindUsingForGenericChainCall (matchGenericChain) tags PendingRefs from
// the generic iterative chain walker. Covers arbitrary-depth chains
// after the hardcoded chain predicates reject — depth ≥ 4
// same-contract chains and depth ≥ 3 cross-contract chains.
//
// TargetQName encodes:
//
//	"same|<fn1>|<fn2>|...|<fnN>|<method>"  (same-contract, N ≥ 4)
//	"cross|<obj>|<fn1>|<fn2>|...|<fnN>|<method>"  (cross-contract, N ≥ 3)
//
// Resolver splits on `|`, dispatches on the leading mode token,
// iterates funcReturnTypes for each segment, and finishes with the
// using-for binding lookup on the last segment's return type.
const dispatchKindUsingForGenericChainCall = "using_for_generic_chain_call"
