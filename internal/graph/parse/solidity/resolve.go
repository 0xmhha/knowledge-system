package solidity

import (
	"maps"
	"slices"
	"strconv"
	"strings"

	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Binding-map types. The per-contract using-for
// binding info reaches Pass 2 method-call resolution through five
// intermediate maps; declaring them as package types lets helpers
// keep narrow signatures. The sixth lookup (funcID → contractID)
// reuses the existing containerIDByFuncID map built for override resolution.
//
//	bindingMap:         contractID → (typeName | "*") → libraryName
//	stateVarTypes:      contractID → varName → typeName (NodeField.Signature)
//	paramTypeMap:       funcID → paramName → typeName
//	funcReturnTypeMap:  funcID → first-return typeName
//	structFieldTypeMap: structName → fieldName → fieldType
//	                              (struct-field receiver dispatch needs
//	                              the field's declared type as the
//	                              binding lookup key).
//	localVarTypeMap:    funcID → localVarName → typeName
//	                              (function-local variable receiver
//	                              dispatch; resolver fallback after
//	                              stateVarTypes → paramTypes).
//
// bindingMap: contractID → (typeName | "*") → []libraryName.
// Multi-value to support `using A for uint256; using B for uint256;`
// where both libraries apply and each provides different methods. An
// earlier single-value map let the second directive overwrite the first;
// the list appends per directive and resolution tries each library until
// a method hit.
type bindingMap map[string]map[string][]string
type stateVarTypeMap map[string]map[string]string
type paramTypeMap map[string]map[string]string
type funcReturnTypeMap map[string]string
type structFieldTypeMap map[string]map[string]string

// localDecl carries one local-variable declaration's scope range +
// typeName. Line-range fields drive narrowest-scope-wins lookup at
// the use site. declStartByte / scopeEndByte let same-line
// shadows disambiguate via byte containment (line-only tied
// `declLine > bestDeclLine` left first-appended outer winning).
// Byte fields are zero when the parser omitted them (defensive
// fallback to line-only behavior).
type localDecl struct {
	declLine      int
	scopeEndLine  int
	declStartByte int
	scopeEndByte  int
	typeName      string
}

// localVarTypeMap: funcID → varName → []localDecl. Each
// emitted variable_declaration appends an entry; the same name can
// have multiple entries when shadowed in nested blocks. Resolution
// at the use site uses line containment + max declLine selection.
type localVarTypeMap map[string]map[string][]localDecl

// Resolve unions per-file results. Cross-file resolution is name-based:
// pending edges (emits_event, has_modifier, writes_mapping) are matched
// against any node whose Name (or QualifiedName for mappings) equals the
// pending TargetQName. Cross-file matches are tagged INFERRED; same-file
// matches stay EXTRACTED. Mirrors the TypeScript resolver.
//
// Inheritance: resolves `is`-clause
// PendingRefs (DispatchKind="inherit"). The detector (inheritance.go)
// emits these with a provisional EdgeType=EdgeExtends; this resolver
// reclassifies to EdgeImplements when the resolved parent is an
// Interface node. Contract / Interface lookups share the byName index
// keyed by NodeType.
//
// Virtual/override: resolves
// override-specifier PendingRefs (DispatchKind="override" /
// "override_explicit"). Bare overrides walk the resolved inheritance graph
// to find every parent that declares a same-name function; explicit
// `override(A, B)` looks up "Parent.method" directly. Both paths emit
// EdgeOverrides (child → parent) at ConfExtracted (same-file) or
// ConfInferred (cross-file).
//
// Interface dispatch: resolves
// `IFoo(addr).bar()` PendingRefs (DispatchKind="interface_dispatch").
// Two-step lookup: (1) TypeName must resolve to a NodeInterface; (2)
// `TypeName.MethodName` must resolve to a Function declared on that
// interface. Both hits → emit EdgeInvokes at ConfAmbiguous (§5.0 Q5 —
// confidence is constant regardless of file boundary; runtime dispatch
// makes the resolved target an over-approximation). Any miss → drop
// (strict-purge — keeps the AMBIGUOUS bucket scoped to real
// interface dispatch only).
func (p *Parser) Resolve(results []*parse.ParseResult) (*parse.ResolvedGraph, error) {
	out := &parse.ResolvedGraph{}

	// nodeFile maps node ID -> source file, so we can mark cross-file
	// resolutions as INFERRED.
	nodeFile := map[string]string{}
	// nodeType maps node ID -> declared type — used by inheritance
	// resolution to reclassify EdgeExtends → EdgeImplements when the
	// target is an Interface.
	nodeType := map[string]types.NodeType{}
	// byName indexes resolvable nodes by their unqualified Name.
	byName := map[types.NodeType]map[string][]string{}
	add := func(nt types.NodeType, key, id string) {
		if byName[nt] == nil {
			byName[nt] = map[string][]string{}
		}
		byName[nt][key] = append(byName[nt][key], id)
	}

	// Indexes — populated in the same single pass over all per-file
	// results below. The three maps work together so bare-override
	// resolution can walk Function → enclosing Container → parent
	// Containers → parent Function in O(1) per hop:
	//
	//   - funcByQName: "Container.func" → []nodeID — explicit override
	//     lookup ("Parent.foo" TargetQName resolves here). The list is
	//     plural because real-world Sol builds can contain duplicate
	//     contract names across files (e.g. test fixtures with a shared
	//     `Base` name in two unrelated subtrees); resolveOverridesRef
	//     disambiguates by file path against the source function.
	//   - containerNameByID: containerID → unqualified name. ID-keyed
	//     reverse-direction map used to label parent contract IDs from
	//     the inheritance index when constructing the "Parent.method"
	//     qname for funcByQName lookup. Sol allows three container kinds
	//     (Contract / Interface / Library) — override resolution indexes the first two
	//     (Library has no override semantics in Sol).
	//   - containerIDByFuncID: funcID → enclosing containerID. Pre-built
	//     reverse index that replaces the O(N) scan over funcByQName +
	//     reverse scan over containerNameByID that bare-override
	//     resolution used to do per PendingRef. Population is two-step
	//     because Function ↔ Container association requires both nodes
	//     loaded first (same-file + name-prefix match); see the second
	//     loop below.
	funcByQName := map[string][]string{}
	containerNameByID := map[string]string{}
	containerIDByFuncID := map[string]string{}

	// containerByNameFile is a transient lookup map for the second pass —
	// keyed by (name + file) so a Function's enclosing container can be
	// resolved without scanning every container in the build. Sol
	// functions cannot span files, so file-scoping is a complete
	// disambiguator (two `Base` contracts in different files won't
	// shadow each other).
	containerByNameFile := map[string]string{}

	for _, r := range results {
		out.Nodes = append(out.Nodes, r.Nodes...)
		out.Edges = append(out.Edges, r.Edges...)
		for _, n := range r.Nodes {
			nodeFile[n.ID] = n.FilePath
			nodeType[n.ID] = n.Type
			switch n.Type {
			case types.NodeEvent:
				add(types.NodeEvent, n.Name, n.ID)
			case types.NodeModifier:
				add(types.NodeModifier, n.Name, n.ID)
				// Also index modifiers by qname
				// (`Container.modifier`) so resolveOverridesRef can find
				// parent-modifier candidates the same way it finds parent-
				// function candidates. Modifier qname is set by runDecl's
				// NodeModifier path (declarations.go:192) and matches the
				// NodeFunction shape.
				if n.QualifiedName != "" && n.QualifiedName != n.Name {
					funcByQName[n.QualifiedName] = append(funcByQName[n.QualifiedName], n.ID)
				}
			case types.NodeMapping:
				add(types.NodeMapping, n.QualifiedName, n.ID)
			// Index Contracts and Interfaces by Name so inheritance
			// PendingRefs (which only know the parent's unqualified name)
			// can resolve cross-file.
			case types.NodeContract:
				add(types.NodeContract, n.Name, n.ID)
				containerNameByID[n.ID] = n.Name
				containerByNameFile[n.Name+"|"+n.FilePath] = n.ID
			case types.NodeInterface:
				add(types.NodeInterface, n.Name, n.ID)
				containerNameByID[n.ID] = n.Name
				containerByNameFile[n.Name+"|"+n.FilePath] = n.ID
			case types.NodeFunction:
				// Explicit override `override(A,B)` queues a
				// TargetQName of "Parent.method", so we index every Sol
				// function by its qualified name. Bare-override resolution
				// uses the same index, scoped by parent contract name.
				funcByQName[n.QualifiedName] = append(funcByQName[n.QualifiedName], n.ID)
				// Index free functions (qname ==
				// name, no enclosing container) under byName[NodeFunction]
				// so resolveUsingForRef's NodeFunction fallback can join
				// the operator-form / free-function recovery walkers'
				// PendingRefs. Restricted to file-scope functions to
				// avoid binding to a same-named contract method when the
				// developer wrote `using {mul as *} for T` expecting the
				// free function.
				if n.QualifiedName == n.Name {
					add(types.NodeFunction, n.Name, n.ID)
				}
			}
		}
	}

	// Pass 1.5 — build containerIDByFuncID and the state-variable
	// type index. Both require Function / Container / Field nodes already
	// indexed (above), so they run as a separate loop.
	//
	//   containerIDByFuncID: funcID → enclosing containerID (override
	//                        reverse index, reused by using-for for call→contract
	//                        recovery).
	//   stateVarTypes:       contractID → varName → declared typeName
	//                        (NodeField.Signature, set by runStateVarDecl
	//                        via extractTypeNameText). Drives method-call
	//                        receiver type lookup in Pass 2c.
	//
	// Both indexes derive the enclosing container from the node's
	// QualifiedName prefix (`<Container>.<member>`) — emitted by
	// runFunctionDecl and runStateVarDecl. File-level
	// helpers without a container prefix are skipped (Sol allows free
	// functions but not free state vars; either way override / using-for
	// semantics don't apply).
	stateVarTypes := stateVarTypeMap{}
	for _, r := range results {
		for _, n := range r.Nodes {
			// NodeModifier joins NodeFunction
			// in containerIDByFuncID — modifier bodies can host using-for
			// receivers (params, locals) just like function bodies. Same
			// qname-prefix idiom (Container.<name>).
			if n.Type != types.NodeFunction && n.Type != types.NodeField && n.Type != types.NodeModifier {
				continue
			}
			dot := strings.IndexByte(n.QualifiedName, '.')
			if dot < 0 {
				continue
			}
			containerName := n.QualifiedName[:dot]
			cid, ok := containerByNameFile[containerName+"|"+n.FilePath]
			if !ok {
				continue
			}
			if n.Type == types.NodeFunction || n.Type == types.NodeModifier {
				containerIDByFuncID[n.ID] = cid
				continue
			}
			// NodeField: stash typeName under the same container ID.
			if n.Signature == "" {
				continue // extraction-failed shapes
			}
			if stateVarTypes[cid] == nil {
				stateVarTypes[cid] = map[string]string{}
			}
			stateVarTypes[cid][n.Name] = n.Signature
		}
	}

	// Pass 2a — resolve inheritance edges first, before override resolution needs them.
	// Bare-override resolution walks the EdgeExtends/EdgeImplements
	// adjacency built in buildInheritanceIndex; that index must include
	// both same-file (already in out.Edges from Pass 1) and cross-file
	// (resolved here) inheritance. Splitting the pending iteration into
	// two sub-passes keeps the dependency explicit instead of relying on
	// per-result ordering.
	for _, r := range results {
		for _, pr := range r.Pending {
			if pr.DispatchKind != dispatchKindInherit {
				continue
			}
			if edge, ok := resolveInheritanceRef(pr, byName, nodeFile, nodeType); ok {
				out.Edges = append(out.Edges, edge)
			}
		}
	}

	// Inheritance edge index — contractID → []parentContractID. Built
	// from the (now fully-resolved) EdgeExtends + EdgeImplements edges
	// after Pass 2a above. Order is preserved (one entry per `is`-clause
	// parent in source order), so `override` against a multi-parent class
	// hits parents in the declared order.
	parents := buildInheritanceIndex(out.Edges)

	// Re-pack state-var SlotIndex with the
	// cross-file struct size table merged from every parsed file's
	// Pass 1 (Parser.structSizes guarded by structMu). A state var
	// typed as a foreign struct now sizes correctly; runs before
	// inheritance offset accumulation so the contract-local fix
	// applies first.
	p.structMu.Lock()
	globalStructSizes := make(map[string]int, len(p.structSizes))
	maps.Copy(globalStructSizes, p.structSizes)
	p.structMu.Unlock()
	applyCrossFileStructSizes(out.Nodes, globalStructSizes)

	// Inheritance offset on NodeField SlotIndex. Pass 1 emits
	// per-contract local indices (each contract restarts at 0); this
	// step adds the cumulative ancestor slot count so the value
	// reflects absolute EVM storage position. The ancestor walk uses
	// C3 linearization (see applyInheritanceSlotOffset and
	// docs/graph/design/solidity-storage-slot-index.md).
	applyInheritanceSlotOffset(out.Nodes, parents)

	// Pre-build the (contractID, typeName) →
	// libraryName binding map by sweeping all dispatchKindUsingForTypeBind
	// PendingRefs across results. Done before Pass 2b so the using-for-call
	// branch can consume it without ordering surprises.
	bindings := bindingMap{}
	// Pre-build (funcID, paramName) → typeName so
	// parameter-receiver method calls can resolve their receiver's
	// declared type the same way state-variable receivers do.
	paramTypes := paramTypeMap{}
	// Pre-build funcID → first-return typeName so
	// `<fn>().<method>` chained dispatch can look up the inner
	// function's return type as the receiver type.
	funcReturnTypes := funcReturnTypeMap{}
	// Pre-build (structName, fieldName) →
	// fieldType so `<obj>.<field>.<method>` dispatch can look up the
	// field's declared type and feed it to the binding map.
	structFieldTypes := structFieldTypeMap{}
	// Pre-build (funcID, localVarName) →
	// typeName so `Type x = ...; x.method(...)` local-var receiver
	// dispatch can resolve x's declared type. Fallback chain on receivers:
	// stateVarTypes → paramTypes → localVarTypes.
	localVarTypes := localVarTypeMap{}
	for _, r := range results {
		for _, pr := range r.Pending {
			switch pr.DispatchKind {
			case dispatchKindUsingForTypeBind:
				// TargetQName encoding from runUsingFor: `libraryName|typeName`.
				sep := strings.IndexByte(pr.TargetQName, '|')
				if sep < 0 {
					continue
				}
				libName := pr.TargetQName[:sep]
				typeName := pr.TargetQName[sep+1:]
				if bindings[pr.SrcID] == nil {
					bindings[pr.SrcID] = map[string][]string{}
				}
				// Append (was overwrite). Multi-binding (multiple
				// `using` directives for the same type) preserves each
				// library so resolution can try them in order.
				bindings[pr.SrcID][typeName] = append(bindings[pr.SrcID][typeName], libName)
			case dispatchKindUsingForParamType:
				// TargetQName encoding from emitParameterMetaPending:
				// `paramName|typeName`.
				sep := strings.IndexByte(pr.TargetQName, '|')
				if sep < 0 {
					continue
				}
				paramName := pr.TargetQName[:sep]
				typeName := pr.TargetQName[sep+1:]
				if paramTypes[pr.SrcID] == nil {
					paramTypes[pr.SrcID] = map[string]string{}
				}
				paramTypes[pr.SrcID][paramName] = typeName
			case dispatchKindUsingForFnReturn:
				// TargetQName encoding from emitFunctionReturnMetaPending:
				// bare `typeName` (no `|`). First return value only.
				funcReturnTypes[pr.SrcID] = pr.TargetQName
			case dispatchKindUsingForStructField:
				// TargetQName encoding from runStructFieldMeta:
				// `structName|fieldName|fieldType` (three parts).
				parts := strings.SplitN(pr.TargetQName, "|", 3)
				if len(parts) != 3 {
					continue
				}
				structName, fieldName, fieldType := parts[0], parts[1], parts[2]
				if structFieldTypes[structName] == nil {
					structFieldTypes[structName] = map[string]string{}
				}
				structFieldTypes[structName][fieldName] = fieldType
			case dispatchKindUsingForLocalVar:
				// TargetQName encoding from emitLocalVarBinding /
				// emitTryReturnsBinding —
				//   `varName|typeName|scopeEndLine|declStartByte|scopeEndByte`
				// (five parts). Byte fields default to 0 if absent
				// (defensive — Pass 2 then falls back to line-only).
				parts := strings.SplitN(pr.TargetQName, "|", 5)
				if len(parts) < 3 {
					continue
				}
				varName, typeName := parts[0], parts[1]
				scopeEnd, convErr := strconv.Atoi(parts[2])
				if convErr != nil || varName == "" || typeName == "" {
					continue
				}
				declStartByte, scopeEndByte := 0, 0
				if len(parts) >= 5 {
					if b, err := strconv.Atoi(parts[3]); err == nil {
						declStartByte = b
					}
					if b, err := strconv.Atoi(parts[4]); err == nil {
						scopeEndByte = b
					}
				}
				if localVarTypes[pr.SrcID] == nil {
					localVarTypes[pr.SrcID] = map[string][]localDecl{}
				}
				localVarTypes[pr.SrcID][varName] = append(
					localVarTypes[pr.SrcID][varName],
					localDecl{
						declLine:      pr.Line,
						scopeEndLine:  scopeEnd,
						declStartByte: declStartByte,
						scopeEndByte:  scopeEndByte,
						typeName:      typeName,
					},
				)
			}
		}
	}

	// Propagate inherited
	// bindings down the inheritance graph so a child contract picks
	// up its parents' `using` declarations. Solidity 0.8.13+
	// formalises this via `internal using`, but in practice solc
	// treats child-visible using directives this way for backwards-
	// compat; the grammar doesn't separate the `internal` keyword at
	// the using_directive level, so this pass treats every contract-scope
	// using as inherited.
	//
	// Algorithm: BFS over the inheritance graph (child → parents)
	// merging each ancestor's bindings into the descendant.
	//
	// Shadowing vs union semantics:
	//   - Child's OWN local binding for a type shadows ALL inherited
	//     bindings on that type (per Solidity scoping). We snapshot
	//     child's local binding keys BEFORE the BFS to distinguish
	//     local from inherited.
	//   - When the child has no local binding for a type, multiple
	//     ancestors contributing bindings on that type union
	//     (multi-binding semantics extended across inheritance).
	//
	// An earlier bug: the BFS used a single `if !exists` guard to
	// decide whether to copy an ancestor's binding into the child.
	// Once the first ancestor visited had populated the slot, the
	// guard then conflated "child's local binding" with "another
	// ancestor's already-merged binding," silently dropping every
	// subsequent ancestor's binding on the same type. The fix splits
	// the guard: local check uses the pre-BFS snapshot; ancestor-to-
	// ancestor merge appends de-duplicated libraries.
	//
	// EdgeImplements (contract → interface) is included because
	// interfaces can in principle carry `using` directives (rare but
	// legal); contributes no entries when the interface itself has
	// no using directives.
	//
	// Cycle defence: visited set per starting child prevents infinite
	// loops on accidental inheritance cycles (Solidity forbids them
	// but a partial parse could produce one).
	for childID := range containerNameByID {
		// Snapshot child's LOCAL binding keys before BFS so we can
		// distinguish them from bindings merged in from ancestors.
		localTypes := map[string]bool{}
		for typeName := range bindings[childID] {
			localTypes[typeName] = true
		}

		visited := map[string]bool{childID: true}
		queue := append([]string(nil), parents[childID]...)
		for len(queue) > 0 {
			ancestorID := queue[0]
			queue = queue[1:]
			if visited[ancestorID] {
				continue
			}
			visited[ancestorID] = true
			for typeName, libNames := range bindings[ancestorID] {
				// Shadowing: child's LOCAL binding wins, regardless
				// of which ancestor offers an inherited variant.
				if localTypes[typeName] {
					continue
				}
				if bindings[childID] == nil {
					bindings[childID] = map[string][]string{}
				}
				// Union: append libs that aren't already present
				// (from a previously visited ancestor on the same
				// type). De-duplication keeps the list compact and
				// preserves the multi-binding invariant.
				existing := bindings[childID][typeName]
				for _, lib := range libNames {
					if !slices.Contains(existing, lib) {
						existing = append(existing, lib)
					}
				}
				bindings[childID][typeName] = existing
			}
			queue = append(queue, parents[ancestorID]...)
		}
	}

	// Collect callable IDs that performed a
	// low-level call to an address-typed receiver so we can mark
	// HasExternalCall after Pass 2b completes. Distinct from
	// HasLowLevelCall (any low-level call) because consumers often
	// care about arbitrary-address dispatch surfaces specifically.
	externalCallSrcs := map[string]bool{}

	// Dedup EdgeUsesFor emit. Multiple
	// walkers can pattern-match the same ERROR-wrapped misparse,
	// so the resolver guards against double emit by remembering
	// (Src, Dst, Type, Line, DispatchKind) tuples seen for the
	// dispatchKindUsingFor branch.
	usingForEdgeSeen := map[string]bool{}

	// Collect callable IDs
	// that invoked a cross-contract function-pointer (receiver typed
	// as another contract whose method — or an inherited method — is
	// a function-typed state variable) so we can mark
	// HasFunctionPointerCall after Pass 2b completes.
	fnPointerCallSrcs := map[string]bool{}
	// Index function-typed NodeField rows by (contractName,
	// fieldName) for O(1) lookup during cross-contract dispatch
	// resolution.
	fnTypedFields := map[string]map[string]bool{}
	for _, n := range out.Nodes {
		if n.Type != types.NodeField || !n.IsFunctionTyped {
			continue
		}
		dot := strings.IndexByte(n.QualifiedName, '.')
		if dot <= 0 || dot == len(n.QualifiedName)-1 {
			continue
		}
		contract := n.QualifiedName[:dot]
		field := n.QualifiedName[dot+1:]
		if fnTypedFields[contract] == nil {
			fnTypedFields[contract] = map[string]bool{}
		}
		fnTypedFields[contract][field] = true
	}
	// The C3 MRO is reused here to walk
	// inherited function-typed fields. Computed once for the entire
	// resolve pass.
	//
	// The variant call also surfaces the
	// set of contracts whose hierarchy required the C3 fallback so
	// we can stamp HasInheritanceMROFallback after Pass 2b.
	mroByContractID, mroFallbackIDs := computeC3LinearizationWithFallbacks(parents)

	// Pass 2b — everything except inheritance (already done) and any
	// future detector-specific branches go through this loop. Overrides
	// rely on the `parents` index built between the two sub-passes.
	for _, r := range results {
		for _, pr := range r.Pending {
			if pr.DispatchKind == dispatchKindInherit {
				continue // already handled in Pass 2a
			}
			// Override branch — fans out one EdgeOverrides per resolved
			// parent. Bare `override` consults the inheritance index; the
			// explicit form does a direct qname lookup.
			if pr.DispatchKind == dispatchKindOverride ||
				pr.DispatchKind == dispatchKindOverrideExplicit {
				edges := resolveOverridesRef(
					pr, funcByQName, containerNameByID,
					containerIDByFuncID, parents, nodeFile,
				)
				out.Edges = append(out.Edges, edges...)
				continue
			}
			// Interface-dispatch branch — resolves IFoo(addr).bar()
			// against the Interface index. Confidence is fixed at
			// ConfAmbiguous per §5.0 Q5 regardless of cross-file boundary.
			if pr.DispatchKind == dispatchKindInterfaceDispatch {
				if edge, ok := resolveInterfaceDispatchRef(
					pr, byName, funcByQName, nodeFile,
				); ok {
					out.Edges = append(out.Edges, edge)
				}
				continue
			}
			// Contract-cast branch — resolves
			// MyContract(addr).method() against the Contract index.
			// Disjoint from interface dispatch (Interface index) so no double-emit.
			if pr.DispatchKind == dispatchKindContractCast {
				if edge, ok := resolveContractCastRef(
					pr, byName, funcByQName, nodeFile,
				); ok {
					out.Edges = append(out.Edges, edge)
				}
				continue
			}
			// Using-for branch — resolves `using LibName for ...` to
			// EdgeUsesFor (Container → Library). Library is emitted by runDecl
			// as NodeContract + SubKind="library", so we use the same
			// byName[NodeContract] index as inheritance resolution but
			// further filter to library-subkind nodes via the existing
			// containerNameByID map. Same-file → ConfExtracted, cross-file
			// → ConfInferred, unresolved → drop (strict-purge).
			if pr.DispatchKind == dispatchKindUsingFor {
				if edge, ok := resolveUsingForRef(
					pr, byName, nodeFile,
				); ok {
					// The file-level
					// using_for and file-level operator-form
					// walkers both pattern-match the same
					// ERROR-wrapped misparse in some shapes, so
					// the same logical binding can reach the
					// resolver twice. Dedup on (Src, Dst, Type,
					// Line, DispatchKind) here — the natural
					// edge identity for using-for binding emit
					// — so a single source-level directive lands
					// as a single graph edge regardless of
					// which walker caught it.
					if !usingForEdgeSeen[edgeIdentityKey(edge)] {
						usingForEdgeSeen[edgeIdentityKey(edge)] = true
						out.Edges = append(out.Edges, edge)
					}
				}
				continue
			}
			// typebind/param-type/fn-return/
			// struct-field — already consumed before this loop into
			// the bindings / paramTypes / funcReturnTypes /
			// structFieldTypes maps. Skip silently here so the default
			// switch doesn't try to emit a graph edge for them.
			if pr.DispatchKind == dispatchKindUsingForTypeBind ||
				pr.DispatchKind == dispatchKindUsingForParamType ||
				pr.DispatchKind == dispatchKindUsingForFnReturn ||
				pr.DispatchKind == dispatchKindUsingForStructField {
				continue
			}
			// Using-for method-call branch — resolves
			// `<receiver>.<method>(...)` to an EdgeCalls into the
			// library function bound for the receiver's type. Receiver
			// may be a state variable or a function parameter.
			// Drops when any link in the chain fails (no
			// receiver of that name, no binding for that type, no
			// library function with that method) — strict-purge same
			// as the other Sol resolvers.
			if pr.DispatchKind == dispatchKindUsingForCall {
				if edge, ok := resolveUsingForCallRef(
					pr, bindings, stateVarTypes, paramTypes, localVarTypes,
					containerIDByFuncID, funcByQName, nodeFile,
				); ok {
					out.Edges = append(out.Edges, edge)
				}
				continue
			}
			// Depth-2 chained shape
			// `a().b().call(data)`. Resolve a's first return type
			// to a contract, then b within that contract, then b's
			// first return type. Mark when the final return type
			// is address-like.
			if pr.DispatchKind == dispatchKindDeepChainedExternalCall {
				parts := strings.SplitN(pr.TargetQName, "|", 3)
				if len(parts) != 3 {
					continue
				}
				fn1Name, fn2Name, methodName := parts[0], parts[1], parts[2]
				if !isLowLevelMethod(methodName) {
					continue
				}
				containerName := containerNameByID[containerIDByFuncID[pr.SrcID]]
				var fn1ID string
				if containerName != "" {
					if ids := funcByQName[containerName+"."+fn1Name]; len(ids) > 0 {
						fn1ID = pickSameFileCandidate(ids, nodeFile[pr.SrcID], nodeFile)
					}
				}
				if fn1ID == "" {
					if ids := funcByQName[fn1Name]; len(ids) > 0 {
						fn1ID = pickSameFileCandidate(ids, nodeFile[pr.SrcID], nodeFile)
					}
				}
				if fn1ID == "" {
					continue
				}
				rt1 := funcReturnTypes[fn1ID]
				if rt1 == "" {
					continue
				}
				// rt1 should be a contract name. Look up
				// rt1.fn2Name to find the second function.
				ids := funcByQName[rt1+"."+fn2Name]
				if len(ids) == 0 {
					continue
				}
				fn2ID := pickSameFileCandidate(ids, nodeFile[pr.SrcID], nodeFile)
				if fn2ID == "" {
					continue
				}
				rt2 := funcReturnTypes[fn2ID]
				if rt2 == "address" || rt2 == "address payable" {
					externalCallSrcs[pr.SrcID] = true
				}
				continue
			}
			// Chained-call shape for
			// HasExternalCall. `getTarget().call(data)` resolves
			// the inner function's first return type via
			// funcReturnTypes; if that type is address-like, mark
			// the source. No edge emitted.
			if pr.DispatchKind == dispatchKindChainedExternalCall {
				sep := strings.IndexByte(pr.TargetQName, '|')
				if sep < 0 {
					continue
				}
				innerFnName := pr.TargetQName[:sep]
				containerName := containerNameByID[containerIDByFuncID[pr.SrcID]]
				var innerFuncID string
				if containerName != "" {
					if ids := funcByQName[containerName+"."+innerFnName]; len(ids) > 0 {
						innerFuncID = pickSameFileCandidate(ids, nodeFile[pr.SrcID], nodeFile)
					}
				}
				if innerFuncID == "" {
					if ids := funcByQName[innerFnName]; len(ids) > 0 {
						innerFuncID = pickSameFileCandidate(ids, nodeFile[pr.SrcID], nodeFile)
					}
				}
				if innerFuncID == "" {
					continue
				}
				if rt := funcReturnTypes[innerFuncID]; rt == "address" || rt == "address payable" {
					externalCallSrcs[pr.SrcID] = true
				}
				continue
			}
			// Cross-contract function-pointer
			// call. Resolves `r.handler(x)` to a marker on the
			// enclosing callable when r's declared type is a known
			// contract whose `handler` is a function-typed state-var.
			// No edge is emitted; the marker is the only side
			// effect.
			if pr.DispatchKind == dispatchKindCrossContractFnPointerCall {
				sep := strings.IndexByte(pr.TargetQName, '|')
				if sep < 0 {
					continue
				}
				receiverName := pr.TargetQName[:sep]
				methodName := pr.TargetQName[sep+1:]
				contractID, ok := containerIDByFuncID[pr.SrcID]
				if !ok {
					continue
				}
				typeName := lookupReceiverType(receiverName, contractID, pr.SrcID,
					pr.Line, pr.ByteOffset, stateVarTypes, paramTypes, localVarTypes)
				if typeName == "" {
					continue
				}
				// Direct contract hit.
				if fields, has := fnTypedFields[typeName]; has && fields[methodName] {
					fnPointerCallSrcs[pr.SrcID] = true
					continue
				}
				// Walk the receiver type's C3 MRO so an
				// inherited function-typed state-var also lights
				// up the marker.
				matched := false
				for _, cid := range byName[types.NodeContract][typeName] {
					for _, ancestorID := range mroByContractID[cid] {
						ancestorName := containerNameByID[ancestorID]
						if ancestorName == "" || ancestorName == typeName {
							continue
						}
						if fields, has := fnTypedFields[ancestorName]; has && fields[methodName] {
							matched = true
							break
						}
					}
					if matched {
						break
					}
				}
				if matched {
					fnPointerCallSrcs[pr.SrcID] = true
				}
				continue
			}
			// Low-level call branch — resolves
			// `target.call/delegatecall/staticcall(...)` against
			// Contract/Interface byName index. ConfAmbiguous always
			// (runtime address determines real target). Receiver
			// resolution chain mirrors using-for lookupReceiverType.
			//
			// When the receiver resolves
			// to an address-typed Sol scope variable but no
			// Contract / Interface candidate is found, mark
			// HasExternalCall on the source callable so security
			// tooling can flag arbitrary-address dispatch
			// surfaces without a concrete edge.
			if pr.DispatchKind == dispatchKindLowLevelCall {
				edge, addressTyped, ok := resolveLowLevelCallRefExt(
					pr, byName, stateVarTypes, paramTypes, localVarTypes,
					containerIDByFuncID, nodeFile,
				)
				if ok {
					out.Edges = append(out.Edges, edge)
				} else if addressTyped {
					externalCallSrcs[pr.SrcID] = true
				}
				continue
			}
			// Chained-call branch — resolves
			// `<innerFn>().<method>(...)` to an EdgeCalls into the
			// library function bound for the inner function's return
			// type. Same drop policy as the receiver method-call branch.
			if pr.DispatchKind == dispatchKindUsingForChainCall {
				if edge, ok := resolveUsingForChainCallRef(
					pr, bindings, funcReturnTypes,
					containerIDByFuncID, funcByQName, nodeFile,
				); ok {
					out.Edges = append(out.Edges, edge)
				}
				continue
			}
			// Cross-contract chained branch — resolves
			// `<obj>.<innerFn>().<method>(...)` by walking the
			// receiver's typeName to find the inner function in the
			// receiver's contract / interface, then chaining through
			// the inner function's return type to the using-for
			// binding map. Same strict-drop policy.
			if pr.DispatchKind == dispatchKindUsingForCrossChainCall {
				if edge, ok := resolveUsingForCrossChainCallRef(
					pr, bindings, stateVarTypes, paramTypes,
					funcReturnTypes, containerIDByFuncID, funcByQName,
					nodeFile,
				); ok {
					out.Edges = append(out.Edges, edge)
				}
				continue
			}
			// Depth-2 chained branch — resolves
			// `<innerFn1>().<innerFn2>().<method>(...)`. Walks two
			// levels of funcReturnTypes (innerFn1's return then
			// innerFn2's return) before reaching the using-for
			// binding map. Same strict-drop policy.
			if pr.DispatchKind == dispatchKindUsingForDeepChainCall {
				if edge, ok := resolveUsingForDeepChainCallRef(
					pr, bindings, funcReturnTypes,
					containerIDByFuncID, funcByQName, nodeFile,
				); ok {
					out.Edges = append(out.Edges, edge)
				}
				continue
			}
			// Deep cross-contract chained branch — resolves
			// `<obj>.<innerFn1>().<innerFn2>().<method>(...)` by
			// combining the cross-contract receiver-type lookup with the
			// depth-2 return-type chain. 8-step total chain.
			if pr.DispatchKind == dispatchKindUsingForDeepCrossChainCall {
				if edge, ok := resolveUsingForDeepCrossChainCallRef(
					pr, bindings, stateVarTypes, paramTypes,
					funcReturnTypes, containerIDByFuncID, funcByQName,
					nodeFile,
				); ok {
					out.Edges = append(out.Edges, edge)
				}
				continue
			}
			// Depth-3 same-contract chained branch — resolves
			// `<fn1>().<fn2>().<fn3>().<method>(...)`. Walks three
			// levels of funcReturnTypes.
			if pr.DispatchKind == dispatchKindUsingForTripleChainCall {
				if edge, ok := resolveUsingForTripleChainCallRef(
					pr, bindings, funcReturnTypes,
					containerIDByFuncID, funcByQName, nodeFile,
				); ok {
					out.Edges = append(out.Edges, edge)
				}
				continue
			}
			// Generic chain branch — handles arbitrary-depth
			// chains (depth ≥ 4 same-contract, depth ≥ 3 cross-
			// contract). Iterative walker through funcReturnTypes;
			// the hardcoded depth-1..3 predicates caught the shallow cases
			// before reaching this point.
			if pr.DispatchKind == dispatchKindUsingForGenericChainCall {
				if edge, ok := resolveUsingForGenericChainCallRef(
					pr, bindings, stateVarTypes, paramTypes,
					funcReturnTypes, containerIDByFuncID, funcByQName,
					nodeFile,
				); ok {
					out.Edges = append(out.Edges, edge)
				}
				continue
			}
			// This-prefixed nested chain — resolves
			// `this.<stateVar>.<f1>...<fN>.<method>(...)` (N ≥ 1) by
			// walking stateVar's struct type through structFieldTypes
			// for each hop, then binding lookup on the final namespace.
			// stateVar must be a state-variable on the caller's
			// container (paramTypes intentionally excluded — `this`
			// names current contract).
			if pr.DispatchKind == dispatchKindUsingForThisNestedChainCall {
				if edge, ok := resolveUsingForThisNestedChainCallRef(
					pr, bindings, stateVarTypes,
					structFieldTypes, containerIDByFuncID, funcByQName,
					nodeFile,
				); ok {
					out.Edges = append(out.Edges, edge)
				}
				continue
			}
			// Struct-field receiver — resolves
			// `<obj>.<field>.<method>(...)` by walking obj's struct
			// type to the field's declared type, then binding lookup.
			if pr.DispatchKind == dispatchKindUsingForStructFieldCall {
				if edge, ok := resolveUsingForStructFieldCallRef(
					pr, bindings, stateVarTypes, paramTypes, localVarTypes,
					structFieldTypes, containerIDByFuncID, funcByQName,
					nodeFile,
				); ok {
					out.Edges = append(out.Edges, edge)
				}
				continue
			}
			// Nested struct-field receiver — resolves
			// `<obj>.<field1>.<field2>.<method>(...)` by walking
			// structFieldTypes twice (obj's struct field1 → its
			// struct's field2).
			if pr.DispatchKind == dispatchKindUsingForNestedStructFieldCall {
				if edge, ok := resolveUsingForNestedStructFieldCallRef(
					pr, bindings, stateVarTypes, paramTypes, localVarTypes,
					structFieldTypes, containerIDByFuncID, funcByQName,
					nodeFile,
				); ok {
					out.Edges = append(out.Edges, edge)
				}
				continue
			}
			// Generic member-chain walker — arbitrary-depth
			// (≥ 3) pure member access chains. Iterative walker
			// through structFieldTypes.
			if pr.DispatchKind == dispatchKindUsingForGenericMemberChainCall {
				if edge, ok := resolveUsingForGenericMemberChainCallRef(
					pr, bindings, stateVarTypes, paramTypes, localVarTypes,
					structFieldTypes, containerIDByFuncID, funcByQName,
					nodeFile,
				); ok {
					out.Edges = append(out.Edges, edge)
				}
				continue
			}
			var targetType types.NodeType
			switch pr.EdgeType {
			case types.EdgeEmitsEvent:
				targetType = types.NodeEvent
			case types.EdgeHasModifier:
				targetType = types.NodeModifier
			case types.EdgeWritesMapping:
				targetType = types.NodeMapping
			default:
				continue
			}
			ids := byName[targetType][pr.TargetQName]
			if len(ids) == 0 {
				continue
			}
			conf := types.ConfExtracted
			if nodeFile[pr.SrcID] != "" && nodeFile[ids[0]] != "" && nodeFile[pr.SrcID] != nodeFile[ids[0]] {
				conf = types.ConfInferred
			}
			out.Edges = append(out.Edges, types.Edge{
				Src: pr.SrcID, Dst: ids[0], Type: pr.EdgeType,
				Line: pr.Line, Count: 1, Confidence: conf,
				Order: pr.Order,
			})
		}
	}
	// Apply HasExternalCall to every callable
	// node that the Pass 2b loop flagged with an address-typed low-
	// level call receiver. The marker complements HasLowLevelCall (set
	// in Pass 1) by isolating the arbitrary-address dispatch surface
	// specifically.
	if len(externalCallSrcs) > 0 {
		for i := range out.Nodes {
			if externalCallSrcs[out.Nodes[i].ID] {
				out.Nodes[i].HasExternalCall = true
			}
		}
	}
	// Apply HasFunctionPointerCall to every
	// callable that invoked a cross-contract function-pointer (a
	// member_expression whose property is a function-typed state-var
	// on the receiver's contract).
	if len(fnPointerCallSrcs) > 0 {
		for i := range out.Nodes {
			if fnPointerCallSrcs[out.Nodes[i].ID] {
				out.Nodes[i].HasFunctionPointerCall = true
			}
		}
	}
	// Stamp HasInheritanceMROFallback on
	// every NodeContract / NodeInterface whose C3 linearization
	// required the depth-first fallback. solc would reject those
	// hierarchies; downstream tooling surfaces the diagnostic.
	if len(mroFallbackIDs) > 0 {
		for i := range out.Nodes {
			if mroFallbackIDs[out.Nodes[i].ID] {
				out.Nodes[i].HasInheritanceMROFallback = true
			}
		}
	}
	return out, nil
}

// applyInheritanceSlotOffset adjusts every NodeField's SlotIndex to include the
// cumulative ancestor slot count using the C3 linearization MRO
// (Method Resolution Order) Sol's reference compiler applies to
// storage layout. Each contract's offset is the sum of slotCount
// over every ancestor in its C3 linearization — diamond inheritance
// no longer double-counts a shared grandparent.
//
// Algorithm:
//
//  1. Index contracts by Name (first occurrence wins for cross-file
//     homonyms — same disambiguation idiom as interface dispatch /
//     contract cast).
//
//  2. Count slots per contract: the maximum SlotIndex + 1 across
//     non-mapping NodeField rows whose qname prefix matches a
//     contract.
//
//  3. Compute the C3 linearization for every contract via
//     computeC3Linearization. The MRO list is (self, base1, base2,
//     …) — derived first, base last.
//
//  4. offset(c) = Σ slotCount(a) for each ancestor a in MRO[c]
//     except c itself. C3 dedupes shared ancestors so the sum
//     stays correct for diamond hierarchies.
//
//  5. Mutate every NodeField's SlotIndex by adding the offset.
//
// Limitations remaining:
//
//   - Library subkind contracts (SubKind="library") are included
//     in the index. Libraries hold no state in practice, so the
//     adjustment is a no-op for them; no explicit filter.
//   - Cross-file homonymous contracts pick the first-occurrence ID
//     (same idiom every other byName resolver uses).
//   - Inconsistent hierarchies (no C3 solution exists, which solc
//     would reject) fall back to a depth-first walk so the
//     result stays deterministic.
func applyInheritanceSlotOffset(nodes []types.Node, parents map[string][]string) {
	contractIDByName := map[string]string{}
	for _, n := range nodes {
		if n.Type != types.NodeContract && n.Type != types.NodeInterface {
			continue
		}
		if _, exists := contractIDByName[n.Name]; !exists {
			contractIDByName[n.Name] = n.ID
		}
	}
	if len(contractIDByName) == 0 {
		return
	}

	slotCount := map[string]int{}
	for _, n := range nodes {
		if n.Type != types.NodeField {
			continue
		}
		dot := strings.IndexByte(n.QualifiedName, '.')
		if dot <= 0 {
			continue
		}
		cid, ok := contractIDByName[n.QualifiedName[:dot]]
		if !ok {
			continue
		}
		if next := n.SlotIndex + 1; next > slotCount[cid] {
			slotCount[cid] = next
		}
	}

	mro := computeC3Linearization(parents)
	offset := map[string]int{}
	for _, cid := range contractIDByName {
		lin := mro[cid]
		if len(lin) == 0 {
			// Contract has no recorded parents — leaf or no
			// inheritance edges resolved. Offset is zero.
			continue
		}
		total := 0
		for _, anc := range lin[1:] { // skip self
			total += slotCount[anc]
		}
		offset[cid] = total
	}

	for i := range nodes {
		if nodes[i].Type != types.NodeField {
			continue
		}
		dot := strings.IndexByte(nodes[i].QualifiedName, '.')
		if dot <= 0 {
			continue
		}
		cid, ok := contractIDByName[nodes[i].QualifiedName[:dot]]
		if !ok {
			continue
		}
		if off := offset[cid]; off > 0 {
			nodes[i].SlotIndex += off
		}
	}
}

// buildInheritanceIndex collects every EdgeExtends / EdgeImplements edge
// into a child → []parent adjacency map. Order is preserved in append
// order, matching the source-order inheritance emit (parents listed
// left-to-right in the `is`-clause). Bare-override resolution iterates
// this list to find every parent that declares a same-name virtual
// function, so order stability matters for deterministic edge ordering.
func buildInheritanceIndex(edges []types.Edge) map[string][]string {
	out := map[string][]string{}
	for _, e := range edges {
		if e.Type != types.EdgeExtends && e.Type != types.EdgeImplements {
			continue
		}
		out[e.Src] = append(out[e.Src], e.Dst)
	}
	return out
}

// resolveOverridesRef fans one override PendingRef out into zero or more
// EdgeOverrides edges. Two dispatch kinds:
//
//   - dispatchKindOverrideExplicit: TargetQName is "Parent.method".
//     Direct lookup in funcByQName; no inheritance walk.
//   - dispatchKindOverride: TargetQName is the bare method name.
//     We use the source function's already-resolved EdgeExtends /
//     EdgeImplements parents (via the `parents` adjacency keyed by the
//     enclosing contract ID), and emit one EdgeOverrides per parent that
//     declares a same-name function. Unresolved (no parent declares it)
//     → zero edges.
//
// The child's enclosing contract is recovered via the pre-built
// containerIDByFuncID reverse index (single hop, O(1)). The earlier
// implementation scanned funcByQName to extract the qname prefix and
// then scanned containerNameByID by (name + file) to recover the ID —
// both passes are subsumed by the index.
//
// Confidence policy mirrors inheritance: same-file → ConfExtracted, cross-file →
// ConfInferred. Multiple parents in a single bare override fan out into
// multiple edges, each independently scored.
func resolveOverridesRef(
	pr parse.PendingRef,
	funcByQName map[string][]string,
	containerNameByID map[string]string,
	containerIDByFuncID map[string]string,
	parents map[string][]string,
	nodeFile map[string]string,
) []types.Edge {
	switch pr.DispatchKind {
	case dispatchKindOverrideExplicit:
		ids := funcByQName[pr.TargetQName]
		if len(ids) == 0 {
			return nil
		}
		// Disambiguate when multiple candidates share the same "P.method"
		// qname (rare but legal — homonymous contracts across files).
		// Prefer a same-file match; fall back to the first candidate so
		// genuine cross-file explicit overrides still resolve.
		srcFile := nodeFile[pr.SrcID]
		dstID := ids[0]
		for _, candidate := range ids {
			if nodeFile[candidate] == srcFile {
				dstID = candidate
				break
			}
		}
		conf := types.ConfExtracted
		if srcFile != "" && nodeFile[dstID] != "" && srcFile != nodeFile[dstID] {
			conf = types.ConfInferred
		}
		return []types.Edge{{
			Src: pr.SrcID, Dst: dstID, Type: types.EdgeOverrides,
			Line: pr.Line, Count: 1, Confidence: conf,
		}}

	case dispatchKindOverride:
		// Recover the enclosing contract of pr.SrcID via the pre-built
		// reverse index. Sol functions can't span files, so the index
		// pairs each funcID with exactly one containerID; missing entries
		// represent file-level functions (no override semantics — drop).
		contractID, ok := containerIDByFuncID[pr.SrcID]
		if !ok {
			return nil
		}
		parentIDs := parents[contractID]
		if len(parentIDs) == 0 {
			return nil
		}
		srcFile := nodeFile[pr.SrcID]
		method := pr.TargetQName
		var out []types.Edge
		for _, pid := range parentIDs {
			parentName := containerNameByID[pid]
			if parentName == "" {
				continue
			}
			ids := funcByQName[parentName+"."+method]
			if len(ids) == 0 {
				continue
			}
			// When multiple containers share a name (rare but legal across
			// files), funcByQName[parent+"."+method] may carry several IDs.
			// Pick the one whose container ID matches the actual parent
			// id we're processing — every other candidate is a homonym
			// declared elsewhere. We compare by file (the parent
			// container's file must match the function's file).
			parentFile := nodeFile[pid]
			var dstID string
			for _, fid := range ids {
				if nodeFile[fid] == parentFile {
					dstID = fid
					break
				}
			}
			if dstID == "" {
				continue
			}
			conf := types.ConfExtracted
			if srcFile != "" && parentFile != "" && srcFile != parentFile {
				conf = types.ConfInferred
			}
			out = append(out, types.Edge{
				Src: pr.SrcID, Dst: dstID, Type: types.EdgeOverrides,
				Line: pr.Line, Count: 1, Confidence: conf,
			})
		}
		return out
	}
	return nil
}

// resolveUsingForRef resolves one using-for PendingRef (`using LibName for ...`)
// to a single EdgeUsesFor edge. The library reference uses bare name
// matching against the NodeContract index (libraries are emitted by runDecl as
// NodeContract + SubKind="library").
//
// Confidence policy mirrors inheritance / override: same-file → ConfExtracted, cross-file
// → ConfInferred, unresolved → ok=false (caller drops the edge).
//
// We don't filter byName[NodeContract] hits to library-subkind only —
// rationale: Sol's `using` is grammar-permissive (the compiler enforces
// "for libraries only", but the AST has no such restriction). When a
// fixture genuinely binds against a non-library contract, the resolved
// EdgeUsesFor still lands; the graph consumer can filter by Library
// subkind downstream. Strict pre-filter would introduce a silent drop
// path that's hard to diagnose if the library declaration gets missed by
// the declaration pass (real bug surface).
//
// Multiple homonymous libraries across files: prefer same-file via
// pickSameFileCandidate (same idiom as the inheritance / explicit-override path).
func resolveUsingForRef(
	pr parse.PendingRef,
	byName map[types.NodeType]map[string][]string,
	nodeFile map[string]string,
) (types.Edge, bool) {
	// The using-for walkers may attach
	// two TargetQName hints to a binding PendingRef:
	//
	//   - "||<importPath>" — namespace / named-import alias
	//     source path for cross-file homonym disambiguation.
	//   - "\x1e<methodName>" — method name from `Lib.method`
	//     forms so the Edge.DispatchKind can surface it.
	//
	// The two separators are distinct (`||` vs `\x1e`) so the
	// decode order doesn't matter. Strip both before the lookup.
	target := pr.TargetQName
	pathHint := ""
	methodName := ""
	if idx := strings.Index(target, "\x1e"); idx >= 0 {
		methodName = target[idx+1:]
		target = target[:idx]
	}
	if idx := strings.Index(target, "||"); idx >= 0 {
		pathHint = target[idx+2:]
		target = target[:idx]
	}
	ids := byName[types.NodeContract][target]
	// Sol 0.8.13+ `using {f as +} for T;`
	// allows free-function targets (`f` resolves to a NodeFunction
	// at file scope). When the NodeContract lookup misses, fall
	// back to NodeFunction so the operator-form / free-function
	// recovery walkers can produce the binding edge they emitted.
	if len(ids) == 0 {
		ids = byName[types.NodeFunction][target]
	}
	if len(ids) == 0 {
		return types.Edge{}, false
	}
	srcFile := nodeFile[pr.SrcID]
	dstID := ""
	if pathHint != "" {
		dstID = pickByPathHint(ids, pathHint, nodeFile)
	}
	if dstID == "" {
		dstID = pickSameFileCandidate(ids, srcFile, nodeFile)
	}
	conf := types.ConfExtracted
	if srcFile != "" && nodeFile[dstID] != "" && srcFile != nodeFile[dstID] {
		conf = types.ConfInferred
	}
	// When the walker carried a method name,
	// publish it on Edge.DispatchKind as `using_for|<method>` so
	// downstream consumers can read which library member the using
	// directive targets without re-parsing source. Empty method
	// keeps the bare `using_for` value for backward compatibility.
	dispatchKind := dispatchKindUsingFor
	if methodName != "" {
		dispatchKind = dispatchKindUsingFor + "|" + methodName
	}
	return types.Edge{
		Src: pr.SrcID, Dst: dstID, Type: types.EdgeUsesFor,
		Line: pr.Line, Count: 1, Confidence: conf,
		DispatchKind: dispatchKind,
	}, true
}

// pickByPathHint returns the candidate ID whose file path matches
// the hint (exact match preferred, suffix match accepted). Used by
// resolveUsingForRef to disambiguate free-function homonyms when the using
// directive came from a namespace alias whose source path is known.
// Returns "" when no candidate matches the hint, signalling the
// caller to fall back to pickSameFileCandidate.
func pickByPathHint(ids []string, pathHint string, nodeFile map[string]string) string {
	// Strip a leading `./` from the hint so a recorded path of
	// `./math.sol` matches a nodeFile of `math.sol`.
	hint := strings.TrimPrefix(pathHint, "./")
	for _, id := range ids {
		if nodeFile[id] == hint {
			return id
		}
	}
	for _, id := range ids {
		if strings.HasSuffix(nodeFile[id], "/"+hint) || strings.HasSuffix(nodeFile[id], hint) {
			return id
		}
	}
	return ""
}

// lookupReceiverType resolves a receiver identifier to its declared type.
// Per Solidity scoping rules,
// function-scope shadows contract-scope on identifier conflict — so the
// lookup order is:
//
//  1. localVarTypes  (function-local declaration — innermost)
//  2. paramTypes     (function parameter)
//  3. stateVarTypes  (state variable — outermost)
//
// An earlier version walked state-var → param → local-var (reverse of Solidity
// semantics). The bug manifested as a false negative when a local
// shadowed a state-var with the same name: the resolver picked the
// state-var type and dropped if its type had no binding.
//
// Shared across every using-for resolver that needs to look up an
// identifier-named receiver (state-var / param / local-var receivers and
// the struct-walker obj). The `this.<state-var>...` shape intentionally
// bypasses this helper — `this` is an explicit contract reference that
// bypasses the function scope, so the named member must be a state
// variable, never a param or local.
//
// useSiteLine drives scope-aware local-var
// lookup. localVarTypes carries multiple decls per (funcID, varName)
// when shadowing happens; this helper selects the narrowest scope that
// still contains useSiteLine (max declLine where declLine <= useSiteLine
// AND useSiteLine <= scopeEndLine). useSiteLine = 0 falls back to the
// first-decl-wins behavior (used by callers that don't
// have a use-site line — e.g. binding declaration walks).
//
// useSiteByte adds byte-precision scope
// containment so same-line shadows disambiguate. When all of decl
// and use-site bytes are non-zero, selection switches to byte-based
// containment + max declStartByte tiebreak. Falls back to line-only
// when bytes are absent — defensive against parsers that don't
// populate ByteOffset.
func lookupReceiverType(
	name, contractID, funcID string,
	useSiteLine, useSiteByte int,
	stateVarTypes stateVarTypeMap,
	paramTypes paramTypeMap,
	localVarTypes localVarTypeMap,
) string {
	if localMap := localVarTypes[funcID]; localMap != nil {
		if decls := localMap[name]; len(decls) > 0 {
			if t := selectLocalDecl(decls, useSiteLine, useSiteByte); t != "" {
				return t
			}
		}
	}
	if paramMap := paramTypes[funcID]; paramMap != nil {
		if t := paramMap[name]; t != "" {
			return t
		}
	}
	if varMap := stateVarTypes[contractID]; varMap != nil {
		if t := varMap[name]; t != "" {
			return t
		}
	}
	return ""
}

// resolveBindingLib iterates the
// multi-value binding list for (typeName | "*") and returns the
// libIDs of the first library whose `<lib>.<methodName>` qname is
// indexed in funcByQName. Used by every using-for resolver path that
// otherwise had identical `bindMap[typeName] → libName → funcByQName`
// lookup chains.
//
// Wildcard fallback (`*` key) only consulted when no specific
// binding exists for typeName. Within either list, the first library
// to have the method wins — preserves Solidity's "directives apply
// in order, first method match wins" semantics for multi-binding
// while keeping single-binding behavior identical to the single-value map.
func resolveBindingLib(
	bindMap map[string][]string,
	typeName, methodName string,
	funcByQName map[string][]string,
) ([]string, bool) {
	libs := bindMap[typeName]
	if len(libs) == 0 {
		libs = bindMap["*"]
	}
	for _, lib := range libs {
		if ids := funcByQName[lib+"."+methodName]; len(ids) > 0 {
			return ids, true
		}
	}
	return nil, false
}

// selectLocalDecl picks the narrowest local declaration whose scope
// range contains the use site. When useSiteLine is 0 (caller has no
// use-site context — e.g. emit walks), falls back to the first decl
// in source order (first-decl-wins).
//
// Line-based selection: filter decls where declLine <=
// useSiteLine AND useSiteLine <= scopeEndLine, then choose the one
// with the highest declLine — narrowest enclosing scope.
//
// Byte-precision selection: when useSiteByte > 0 AND every
// candidate decl has non-zero declStartByte / scopeEndByte, filter
// by byte containment (declStartByte <= useSiteByte <= scopeEndByte)
// and tiebreak on max declStartByte. This resolves same-line
// shadow cases where the line-based `declLine > bestDeclLine` strict-`>`
// tiebreak left the first-appended outer winning. Falls back to
// line-only selection when bytes are absent.
func selectLocalDecl(decls []localDecl, useSiteLine, useSiteByte int) string {
	if useSiteLine == 0 {
		// Defensive fallback — first-decl-wins.
		return decls[0].typeName
	}
	// Byte-precision path: only when use-site AND every decl
	// carries non-zero byte ranges. Mixed populations fall back to
	// line-only selection so partial parser upgrades don't misbehave.
	if useSiteByte > 0 && allHaveBytes(decls) {
		bestIdx := -1
		for i, d := range decls {
			if d.declStartByte > useSiteByte {
				continue
			}
			if d.scopeEndByte < useSiteByte {
				continue
			}
			if bestIdx < 0 || decls[i].declStartByte > decls[bestIdx].declStartByte {
				bestIdx = i
			}
		}
		if bestIdx >= 0 {
			return decls[bestIdx].typeName
		}
		// No byte-containment match — fall through to line-based
		// (handles edge cases like use site before any decl on the
		// same line, where line filter is still permissive enough).
	}
	bestIdx := -1
	for i, d := range decls {
		if d.declLine > useSiteLine {
			continue
		}
		if d.scopeEndLine != 0 && d.scopeEndLine < useSiteLine {
			continue
		}
		if bestIdx < 0 || decls[i].declLine > decls[bestIdx].declLine {
			bestIdx = i
		}
	}
	if bestIdx < 0 {
		return ""
	}
	return decls[bestIdx].typeName
}

// allHaveBytes returns true iff every decl has both
// declStartByte and scopeEndByte populated (>0). Used to gate the
// byte-precision selection path: a mixed population (some bytes,
// some zeros) would silently mis-rank decls without bytes, so we
// only switch when the data is uniformly available.
func allHaveBytes(decls []localDecl) bool {
	for _, d := range decls {
		if d.declStartByte == 0 || d.scopeEndByte == 0 {
			return false
		}
	}
	return true
}

// resolveUsingForCallRef resolves one using-for
// method-call PendingRef (`<receiver>.<method>(...)`) to a single
// EdgeCalls edge. Four-step chain — any step's failure drops the edge
// (strict-purge):
//
//  1. funcID → enclosing containerID via containerIDByFuncID.
//  2. receiverName → typeName via three-tier fallback:
//     - (containerID, name) → stateVarTypes (state-var)
//     - (funcID, name) → paramTypes (parameter)
//     - (funcID, name) → localVarTypes (local-var)
//     stateVar / param / local share one identifier namespace per
//     function in Solidity (solc errors on shadowing within scope), so
//     the order shapes precedence but cannot mis-resolve in valid code.
//  3. (containerID, typeName) → libraryName via bindings — falls back to
//     wildcard "*" binding when no specific binding exists (Q9-3 (a)
//     specific-first decision).
//  4. `<libraryName>.<methodName>` → libraryFunctionID via funcByQName.
//
// Confidence: ConfExtracted when both endpoints are in the same file;
// ConfInferred when the chain crosses files (library declared in another
// file). Sol's library dispatch is statically determinable once the
// binding is known, so we don't downgrade to AMBIGUOUS the way
// interface dispatch does — the call resolution is concrete.
func resolveUsingForCallRef(
	pr parse.PendingRef,
	bindings bindingMap,
	stateVarTypes stateVarTypeMap,
	paramTypes paramTypeMap,
	localVarTypes localVarTypeMap,
	containerIDByFuncID map[string]string,
	funcByQName map[string][]string,
	nodeFile map[string]string,
) (types.Edge, bool) {
	// TargetQName encoding from runUsingForCalls: `receiverName|methodName`.
	sep := strings.IndexByte(pr.TargetQName, '|')
	if sep < 0 {
		return types.Edge{}, false
	}
	receiverName := pr.TargetQName[:sep]
	methodName := pr.TargetQName[sep+1:]

	contractID, ok := containerIDByFuncID[pr.SrcID]
	if !ok {
		return types.Edge{}, false
	}
	// Receiver type lookup chain: state-var → parameter →
	// local-var. Solidity scoping rules mean these three name-
	// spaces don't legally overlap within a single function, so the
	// order shapes precedence but cannot mis-resolve in valid code.
	// useSiteLine = pr.Line drives narrowest-scope-wins for
	// shadowed locals.
	// useSiteByte = pr.ByteOffset for byte-precision same-line
	// shadow disambiguation in addition to the line-precision filter.
	typeName := lookupReceiverType(receiverName, contractID, pr.SrcID,
		pr.Line, pr.ByteOffset, stateVarTypes, paramTypes, localVarTypes)
	if typeName == "" {
		return types.Edge{}, false
	}
	// Multi-binding aware resolution via resolveBindingLib.
	ids, ok := resolveBindingLib(bindings[contractID], typeName, methodName, funcByQName)
	if !ok {
		return types.Edge{}, false
	}
	srcFile := nodeFile[pr.SrcID]
	dstID := pickSameFileCandidate(ids, srcFile, nodeFile)
	conf := types.ConfExtracted
	if srcFile != "" && nodeFile[dstID] != "" && srcFile != nodeFile[dstID] {
		conf = types.ConfInferred
	}
	return types.Edge{
		Src: pr.SrcID, Dst: dstID, Type: types.EdgeCalls,
		Line: pr.Line, Count: 1, Confidence: conf,
	}, true
}

// resolveLowLevelCallRef resolves one
// low-level call PendingRef (`target.call/delegatecall/staticcall(...)`)
// to a single EdgeInvokes edge.
//
// TargetQName encoding from runLowLevelCalls: `receiverName|methodName`.
// methodName is unused for resolution (only the receiver type matters at
// the AST layer — actual selector resolution requires runtime address).
//
// Three-step chain — any failure drops:
//
//  1. funcID → receiverName via lookupReceiverType (using-for chain:
//     state-var → param → local-var). useSiteByte = pr.ByteOffset
//     drives byte-precision shadow disambiguation.
//  2. typeName → byName[NodeContract] OR byName[NodeInterface] →
//     candidate Dst IDs.
//  3. Pick same-file candidate first if available (interface-dispatch disambiguation
//     idiom).
//
// Confidence: ConfAmbiguous always — runtime address determines actual
// dispatch target. Same rule as interface dispatch (§5.0 Q5) since
// the AST evidence is the same shape (typed receiver, runtime selector).
//
// DispatchKind preserved on the emitted edge so downstream consumers
// (viewer, llmSafeStoreReader filter) can distinguish low-level calls
// from regular interface dispatch.
// resolveLowLevelCallRefExt is the extension
// of resolveLowLevelCallRef that additionally reports whether the
// receiver resolved to an address-typed Sol scope variable. The flag
// drives HasExternalCall marking on the source callable when no
// concrete Contract / Interface target was found.
func resolveLowLevelCallRefExt(
	pr parse.PendingRef,
	byName map[types.NodeType]map[string][]string,
	stateVarTypes stateVarTypeMap,
	paramTypes paramTypeMap,
	localVarTypes localVarTypeMap,
	containerIDByFuncID map[string]string,
	nodeFile map[string]string,
) (types.Edge, bool, bool) {
	sep := strings.IndexByte(pr.TargetQName, '|')
	if sep < 0 {
		return types.Edge{}, false, false
	}
	receiverName := pr.TargetQName[:sep]
	// methodName := pr.TargetQName[sep+1:] // unused — AST evidence only

	contractID, ok := containerIDByFuncID[pr.SrcID]
	if !ok {
		return types.Edge{}, false, false
	}
	typeName := lookupReceiverType(receiverName, contractID, pr.SrcID,
		pr.Line, pr.ByteOffset, stateVarTypes, paramTypes, localVarTypes)
	if typeName == "" {
		return types.Edge{}, false, false
	}
	isAddress := typeName == "address" || typeName == "address payable"
	// Try Interface first then Contract — interface-typed receivers are
	// the more common shape for low-level call patterns (proxies). Same
	// byName candidate-pick rule as interface dispatch.
	candidates := byName[types.NodeInterface][typeName]
	if len(candidates) == 0 {
		candidates = byName[types.NodeContract][typeName]
	}
	if len(candidates) == 0 {
		return types.Edge{}, isAddress, false
	}
	srcFile := nodeFile[pr.SrcID]
	dstID := pickSameFileCandidate(candidates, srcFile, nodeFile)
	return types.Edge{
		Src: pr.SrcID, Dst: dstID, Type: types.EdgeInvokes,
		Line: pr.Line, Count: 1, Confidence: types.ConfAmbiguous,
		DispatchKind: dispatchKindLowLevelCall,
	}, false, true
}

// resolveUsingForChainCallRef resolves a chained
// call PendingRef (`<innerFn>().<method>(...)`) to a single EdgeCalls
// edge by looking up the inner function's return type as the receiver
// type. Five-step chain — any failure drops the edge:
//
//  1. funcID (caller) → enclosing containerID via containerIDByFuncID.
//  2. innerFnName → innerFuncID via funcByQName. Prefers the same-
//     contract candidate (`<callerContract>.<innerFn>`) over arbitrary
//     bare-name matches. Limitation: cross-contract resolution
//     follows the first matching candidate (homonym disambiguation is
//     handled by the cross-contract chaining resolvers).
//  3. innerFuncID → returnTypeName via funcReturnTypes.
//  4. (callerContractID, returnTypeName) → libraryName via bindings;
//     wildcard `*` fallback per Q9-3 (a).
//  5. `<libraryName>.<methodName>` → libraryFunctionID via funcByQName.
//
// Confidence: ConfExtracted when caller and library are same-file;
// ConfInferred otherwise. The inner function's file doesn't downgrade
// confidence here because this resolver only fires for known callable
// returns — uncertainty about the *resolution* is captured by drop,
// not by AMBIGUOUS.
func resolveUsingForChainCallRef(
	pr parse.PendingRef,
	bindings bindingMap,
	funcReturnTypes funcReturnTypeMap,
	containerIDByFuncID map[string]string,
	funcByQName map[string][]string,
	nodeFile map[string]string,
) (types.Edge, bool) {
	// TargetQName encoding from runUsingForCalls chained branch:
	// `innerFnName|methodName`.
	sep := strings.IndexByte(pr.TargetQName, '|')
	if sep < 0 {
		return types.Edge{}, false
	}
	innerFnName := pr.TargetQName[:sep]
	methodName := pr.TargetQName[sep+1:]

	contractID, ok := containerIDByFuncID[pr.SrcID]
	if !ok {
		return types.Edge{}, false
	}
	// Resolve the inner function. Prefer same-contract qualification —
	// `Container.innerFn` keys the funcByQName index. Fall back to a
	// global single-result scan when the inner function lives outside
	// the caller's contract (e.g. file-level free function in 0.8.13+
	// — not captured yet, but the global path also covers
	// imported helpers that happen to share names).
	containerName := ""
	for qname, ids := range funcByQName {
		for _, fid := range ids {
			if fid == pr.SrcID {
				if dot := strings.IndexByte(qname, '.'); dot >= 0 {
					containerName = qname[:dot]
				}
				break
			}
		}
		if containerName != "" {
			break
		}
	}
	var innerFuncID string
	if containerName != "" {
		if ids := funcByQName[containerName+"."+innerFnName]; len(ids) > 0 {
			innerFuncID = pickSameFileCandidate(ids, nodeFile[pr.SrcID], nodeFile)
		}
	}
	if innerFuncID == "" {
		return types.Edge{}, false
	}
	returnType, ok := funcReturnTypes[innerFuncID]
	if !ok || returnType == "" {
		return types.Edge{}, false
	}
	// Multi-binding aware resolution via resolveBindingLib.
	ids, ok := resolveBindingLib(bindings[contractID], returnType, methodName, funcByQName)
	if !ok {
		return types.Edge{}, false
	}
	srcFile := nodeFile[pr.SrcID]
	dstID := pickSameFileCandidate(ids, srcFile, nodeFile)
	conf := types.ConfExtracted
	if srcFile != "" && nodeFile[dstID] != "" && srcFile != nodeFile[dstID] {
		conf = types.ConfInferred
	}
	return types.Edge{
		Src: pr.SrcID, Dst: dstID, Type: types.EdgeCalls,
		Line: pr.Line, Count: 1, Confidence: conf,
	}, true
}

// resolveUsingForCrossChainCallRef resolves a
// cross-contract chained call PendingRef
// (`<obj>.<innerFn>().<method>(...)`) to a single EdgeCalls edge.
// 7-step chain — any failure drops the edge:
//
//  1. funcID (caller) → callerContainerID via containerIDByFuncID.
//  2. receiverObjName → receiverType (stateVarTypes first, then
//     paramTypes) — same lookup order as resolveUsingForCallRef.
//  3. receiverType → receiverContainerID via byName[NodeContract /
//     NodeInterface] (primitive types like uint256 → fail because
//     they're not in the container index, dropping cleanly).
//  4. `<receiverType>.<innerFn>` → innerFuncID via funcByQName,
//     preferring the same-file candidate as the receiver contract.
//  5. innerFuncID → returnTypeName via funcReturnTypes.
//  6. (callerContainerID, returnTypeName) → libraryName via bindings,
//     wildcard `*` fallback.
//  7. `<libraryName>.<methodName>` → libraryFunctionID via funcByQName.
//
// Confidence: ConfExtracted when caller and library are same-file;
// ConfInferred when they differ. Receiver contract's file doesn't
// downgrade confidence here.
func resolveUsingForCrossChainCallRef(
	pr parse.PendingRef,
	bindings bindingMap,
	stateVarTypes stateVarTypeMap,
	paramTypes paramTypeMap,
	funcReturnTypes funcReturnTypeMap,
	containerIDByFuncID map[string]string,
	funcByQName map[string][]string,
	nodeFile map[string]string,
) (types.Edge, bool) {
	// TargetQName encoding from runUsingForCalls cross-chain branch:
	// `receiverObjName|innerFnName|methodName`.
	parts := strings.SplitN(pr.TargetQName, "|", 3)
	if len(parts) != 3 {
		return types.Edge{}, false
	}
	receiverObj := parts[0]
	innerFnName := parts[1]
	methodName := parts[2]

	callerContractID, ok := containerIDByFuncID[pr.SrcID]
	if !ok {
		return types.Edge{}, false
	}
	// Receiver type: state-var first, then parameter.
	var receiverType string
	if varMap := stateVarTypes[callerContractID]; varMap != nil {
		receiverType = varMap[receiverObj]
	}
	if receiverType == "" {
		if paramMap := paramTypes[pr.SrcID]; paramMap != nil {
			receiverType = paramMap[receiverObj]
		}
	}
	if receiverType == "" {
		return types.Edge{}, false
	}
	// Receiver type must reference a known Contract or Interface — uint256
	// and friends drop here. The inner function lookup uses the typeName
	// as a qname prefix; we don't need the receiver's container ID
	// explicitly, but the funcByQName key requires the receiver type to
	// match an existing container's name.
	innerQname := receiverType + "." + innerFnName
	ids := funcByQName[innerQname]
	if len(ids) == 0 {
		return types.Edge{}, false
	}
	srcFile := nodeFile[pr.SrcID]
	innerFuncID := pickSameFileCandidate(ids, srcFile, nodeFile)
	returnType, ok := funcReturnTypes[innerFuncID]
	if !ok || returnType == "" {
		return types.Edge{}, false
	}
	// Multi-binding aware resolution.
	libIDs, ok := resolveBindingLib(bindings[callerContractID], returnType, methodName, funcByQName)
	if !ok {
		return types.Edge{}, false
	}
	dstID := pickSameFileCandidate(libIDs, srcFile, nodeFile)
	conf := types.ConfExtracted
	if srcFile != "" && nodeFile[dstID] != "" && srcFile != nodeFile[dstID] {
		conf = types.ConfInferred
	}
	return types.Edge{
		Src: pr.SrcID, Dst: dstID, Type: types.EdgeCalls,
		Line: pr.Line, Count: 1, Confidence: conf,
	}, true
}

// resolveUsingForDeepChainCallRef resolves a
// depth-2 chained call PendingRef
// (`<innerFn1>().<innerFn2>().<method>(...)`) to a single EdgeCalls
// edge. 7-step chain — any failure drops the edge:
//
//  1. funcID (caller) → callerContainerID via containerIDByFuncID.
//  2. innerFn1 → innerFn1FuncID via funcByQName
//     (`<callerContainer>.<innerFn1>`, same as resolveUsingForChainCallRef).
//  3. innerFn1FuncID → returnType1 via funcReturnTypes.
//  4. `<returnType1>.<innerFn2>` → innerFn2FuncID via funcByQName.
//     returnType1 must be a known Container (Contract / Interface)
//     so its namespace can host innerFn2 — primitive types drop here.
//  5. innerFn2FuncID → returnType2 via funcReturnTypes.
//  6. (callerContainerID, returnType2) → libraryName via bindings,
//     wildcard `*` fallback.
//  7. `<libraryName>.<methodName>` → libraryFunctionID via funcByQName.
//
// Confidence: ConfExtracted when caller and library are same-file;
// ConfInferred otherwise. Intermediate functions' files don't downgrade.
func resolveUsingForDeepChainCallRef(
	pr parse.PendingRef,
	bindings bindingMap,
	funcReturnTypes funcReturnTypeMap,
	containerIDByFuncID map[string]string,
	funcByQName map[string][]string,
	nodeFile map[string]string,
) (types.Edge, bool) {
	// TargetQName encoding from runUsingForCalls deep-chain branch:
	// `innerFn1|innerFn2|method`.
	parts := strings.SplitN(pr.TargetQName, "|", 3)
	if len(parts) != 3 {
		return types.Edge{}, false
	}
	innerFn1 := parts[0]
	innerFn2 := parts[1]
	methodName := parts[2]

	callerContractID, ok := containerIDByFuncID[pr.SrcID]
	if !ok {
		return types.Edge{}, false
	}
	srcFile := nodeFile[pr.SrcID]
	// Step 2: locate innerFn1 in the caller's contract namespace.
	callerContainerName := ""
	for qname, ids := range funcByQName {
		for _, fid := range ids {
			if fid == pr.SrcID {
				if dot := strings.IndexByte(qname, '.'); dot >= 0 {
					callerContainerName = qname[:dot]
				}
				break
			}
		}
		if callerContainerName != "" {
			break
		}
	}
	if callerContainerName == "" {
		return types.Edge{}, false
	}
	innerFn1IDs := funcByQName[callerContainerName+"."+innerFn1]
	if len(innerFn1IDs) == 0 {
		return types.Edge{}, false
	}
	innerFn1FuncID := pickSameFileCandidate(innerFn1IDs, srcFile, nodeFile)
	// Step 3: returnType1.
	returnType1, ok := funcReturnTypes[innerFn1FuncID]
	if !ok || returnType1 == "" {
		return types.Edge{}, false
	}
	// Step 4: innerFn2 in returnType1's namespace.
	innerFn2IDs := funcByQName[returnType1+"."+innerFn2]
	if len(innerFn2IDs) == 0 {
		return types.Edge{}, false
	}
	innerFn2FuncID := pickSameFileCandidate(innerFn2IDs, srcFile, nodeFile)
	// Step 5: returnType2.
	returnType2, ok := funcReturnTypes[innerFn2FuncID]
	if !ok || returnType2 == "" {
		return types.Edge{}, false
	}
	// Step 6-7: multi-binding aware binding + library lookup.
	libIDs, ok := resolveBindingLib(bindings[callerContractID], returnType2, methodName, funcByQName)
	if !ok {
		return types.Edge{}, false
	}
	dstID := pickSameFileCandidate(libIDs, srcFile, nodeFile)
	conf := types.ConfExtracted
	if srcFile != "" && nodeFile[dstID] != "" && srcFile != nodeFile[dstID] {
		conf = types.ConfInferred
	}
	return types.Edge{
		Src: pr.SrcID, Dst: dstID, Type: types.EdgeCalls,
		Line: pr.Line, Count: 1, Confidence: conf,
	}, true
}

// resolveUsingForDeepCrossChainCallRef resolves
// a deep cross-contract chained PendingRef
// (`<obj>.<innerFn1>().<innerFn2>().<method>(...)`) to a single
// EdgeCalls edge. 8-step chain — any failure drops:
//
//  1. funcID → callerContainerID (containerIDByFuncID)
//  2. receiverObj → receiverType (stateVarTypes → paramTypes)
//  3. `<receiverType>.<innerFn1>` → innerFn1FuncID (funcByQName)
//  4. innerFn1FuncID → returnType1 (funcReturnTypes)
//  5. `<returnType1>.<innerFn2>` → innerFn2FuncID (funcByQName)
//  6. innerFn2FuncID → returnType2 (funcReturnTypes)
//  7. (callerContainerID, returnType2) → libraryName (bindings + `*`)
//  8. `<libraryName>.<methodName>` → libraryFunctionID
//
// Confidence: ConfExtracted (caller + library same-file) /
// ConfInferred (cross-file). Receiver / intermediate functions' files
// don't downgrade.
func resolveUsingForDeepCrossChainCallRef(
	pr parse.PendingRef,
	bindings bindingMap,
	stateVarTypes stateVarTypeMap,
	paramTypes paramTypeMap,
	funcReturnTypes funcReturnTypeMap,
	containerIDByFuncID map[string]string,
	funcByQName map[string][]string,
	nodeFile map[string]string,
) (types.Edge, bool) {
	// TargetQName encoding from runUsingForCalls deep cross-chain branch:
	// `receiverObj|innerFn1|innerFn2|method`.
	parts := strings.SplitN(pr.TargetQName, "|", 4)
	if len(parts) != 4 {
		return types.Edge{}, false
	}
	receiverObj := parts[0]
	innerFn1 := parts[1]
	innerFn2 := parts[2]
	methodName := parts[3]

	callerContractID, ok := containerIDByFuncID[pr.SrcID]
	if !ok {
		return types.Edge{}, false
	}
	srcFile := nodeFile[pr.SrcID]
	// Receiver type lookup (state-var, then parameter).
	var receiverType string
	if varMap := stateVarTypes[callerContractID]; varMap != nil {
		receiverType = varMap[receiverObj]
	}
	if receiverType == "" {
		if paramMap := paramTypes[pr.SrcID]; paramMap != nil {
			receiverType = paramMap[receiverObj]
		}
	}
	if receiverType == "" {
		return types.Edge{}, false
	}
	// Step 3: innerFn1 in receiverType's namespace.
	innerFn1IDs := funcByQName[receiverType+"."+innerFn1]
	if len(innerFn1IDs) == 0 {
		return types.Edge{}, false
	}
	innerFn1FuncID := pickSameFileCandidate(innerFn1IDs, srcFile, nodeFile)
	// Step 4: returnType1.
	returnType1, ok := funcReturnTypes[innerFn1FuncID]
	if !ok || returnType1 == "" {
		return types.Edge{}, false
	}
	// Step 5: innerFn2 in returnType1's namespace.
	innerFn2IDs := funcByQName[returnType1+"."+innerFn2]
	if len(innerFn2IDs) == 0 {
		return types.Edge{}, false
	}
	innerFn2FuncID := pickSameFileCandidate(innerFn2IDs, srcFile, nodeFile)
	// Step 6: returnType2.
	returnType2, ok := funcReturnTypes[innerFn2FuncID]
	if !ok || returnType2 == "" {
		return types.Edge{}, false
	}
	// Step 7-8: multi-binding aware lookup.
	libIDs, ok := resolveBindingLib(bindings[callerContractID], returnType2, methodName, funcByQName)
	if !ok {
		return types.Edge{}, false
	}
	dstID := pickSameFileCandidate(libIDs, srcFile, nodeFile)
	conf := types.ConfExtracted
	if srcFile != "" && nodeFile[dstID] != "" && srcFile != nodeFile[dstID] {
		conf = types.ConfInferred
	}
	return types.Edge{
		Src: pr.SrcID, Dst: dstID, Type: types.EdgeCalls,
		Line: pr.Line, Count: 1, Confidence: conf,
	}, true
}

// resolveUsingForTripleChainCallRef resolves a
// depth-3 same-contract chained PendingRef
// (`<fn1>().<fn2>().<fn3>().<method>(...)`) to a single EdgeCalls edge.
// 9-step chain — any failure drops:
//
//  1. funcID (caller) → callerContainerID
//  2. fn1 → fn1FuncID (`<callerContainer>.<fn1>` via funcByQName)
//  3. fn1FuncID → returnType1 (funcReturnTypes)
//  4. `<returnType1>.<fn2>` → fn2FuncID
//  5. fn2FuncID → returnType2
//  6. `<returnType2>.<fn3>` → fn3FuncID
//  7. fn3FuncID → returnType3
//  8. (callerContainerID, returnType3) → libraryName (bindings + `*`)
//  9. `<libraryName>.<methodName>` → libraryFunctionID
//
// Confidence: ConfExtracted (caller + library same-file) /
// ConfInferred (cross-file). Intermediate functions' files don't
// downgrade.
//
// Note: resolveUsingForGenericChainCallRef handles deeper chains; this
// hardcoded depth-3 resolver is a straightforward extension of
// resolveUsingForDeepChainCallRef with one more return-type hop.
func resolveUsingForTripleChainCallRef(
	pr parse.PendingRef,
	bindings bindingMap,
	funcReturnTypes funcReturnTypeMap,
	containerIDByFuncID map[string]string,
	funcByQName map[string][]string,
	nodeFile map[string]string,
) (types.Edge, bool) {
	// TargetQName encoding from runUsingForCalls triple-chain branch:
	// `fn1|fn2|fn3|method`.
	parts := strings.SplitN(pr.TargetQName, "|", 4)
	if len(parts) != 4 {
		return types.Edge{}, false
	}
	fn1, fn2, fn3, methodName := parts[0], parts[1], parts[2], parts[3]

	callerContractID, ok := containerIDByFuncID[pr.SrcID]
	if !ok {
		return types.Edge{}, false
	}
	srcFile := nodeFile[pr.SrcID]
	// Step 2: locate fn1 in the caller's contract namespace.
	callerContainerName := ""
	for qname, ids := range funcByQName {
		for _, fid := range ids {
			if fid == pr.SrcID {
				if dot := strings.IndexByte(qname, '.'); dot >= 0 {
					callerContainerName = qname[:dot]
				}
				break
			}
		}
		if callerContainerName != "" {
			break
		}
	}
	if callerContainerName == "" {
		return types.Edge{}, false
	}
	fn1IDs := funcByQName[callerContainerName+"."+fn1]
	if len(fn1IDs) == 0 {
		return types.Edge{}, false
	}
	fn1FuncID := pickSameFileCandidate(fn1IDs, srcFile, nodeFile)
	// Step 3: returnType1.
	returnType1, ok := funcReturnTypes[fn1FuncID]
	if !ok || returnType1 == "" {
		return types.Edge{}, false
	}
	// Step 4: fn2 in returnType1's namespace.
	fn2IDs := funcByQName[returnType1+"."+fn2]
	if len(fn2IDs) == 0 {
		return types.Edge{}, false
	}
	fn2FuncID := pickSameFileCandidate(fn2IDs, srcFile, nodeFile)
	// Step 5: returnType2.
	returnType2, ok := funcReturnTypes[fn2FuncID]
	if !ok || returnType2 == "" {
		return types.Edge{}, false
	}
	// Step 6: fn3 in returnType2's namespace.
	fn3IDs := funcByQName[returnType2+"."+fn3]
	if len(fn3IDs) == 0 {
		return types.Edge{}, false
	}
	fn3FuncID := pickSameFileCandidate(fn3IDs, srcFile, nodeFile)
	// Step 7: returnType3.
	returnType3, ok := funcReturnTypes[fn3FuncID]
	if !ok || returnType3 == "" {
		return types.Edge{}, false
	}
	// Step 8-9: multi-binding aware lookup.
	libIDs, ok := resolveBindingLib(bindings[callerContractID], returnType3, methodName, funcByQName)
	if !ok {
		return types.Edge{}, false
	}
	dstID := pickSameFileCandidate(libIDs, srcFile, nodeFile)
	conf := types.ConfExtracted
	if srcFile != "" && nodeFile[dstID] != "" && srcFile != nodeFile[dstID] {
		conf = types.ConfInferred
	}
	return types.Edge{
		Src: pr.SrcID, Dst: dstID, Type: types.EdgeCalls,
		Line: pr.Line, Count: 1, Confidence: conf,
	}, true
}

// resolveUsingForGenericChainCallRef is the generic
// iterative resolver for arbitrary-depth chained dispatch. Subsumes the
// hardcoded depth-1/2/3 same-contract and depth-1/2 cross-contract
// resolvers for chains the earlier predicates didn't
// already claim. Driven by `matchGenericChain`'s encoded PendingRef.
//
// TargetQName encoding (split by `|`):
//
//	"same|<recv-empty>|<fn1>|<fn2>|...|<fnN>|<method>"  (recv slot is "")
//	"cross|<obj>|<fn1>|<fn2>|...|<fnN>|<method>"
//
// Resolution algorithm:
//
//  1. funcID → callerContainerID (containerIDByFuncID).
//  2. Determine starting namespace:
//     - same-contract: starting namespace = callerContainer.
//     - cross-contract: receiverObj → receiverType
//     (stateVarTypes → paramTypes fallback). starting namespace =
//     receiverType.
//  3. For each fn_i in segs (left-to-right):
//     - `<currentNamespace>.<fn_i>` → fnFuncID (funcByQName).
//     - fnFuncID → returnType_i (funcReturnTypes).
//     - currentNamespace = returnType_i for the next iteration.
//  4. After consuming all segments: currentNamespace is the final
//     return type. (callerContainerID, currentNamespace) → libraryName
//     via bindings (with wildcard `*` fallback).
//  5. `<libraryName>.<methodName>` → libraryFunctionID.
//
// Confidence: ConfExtracted (caller + library same-file) /
// ConfInferred (cross-file). Intermediate functions' files don't
// downgrade. Same policy as the hardcoded chain resolvers.
func resolveUsingForGenericChainCallRef(
	pr parse.PendingRef,
	bindings bindingMap,
	stateVarTypes stateVarTypeMap,
	paramTypes paramTypeMap,
	funcReturnTypes funcReturnTypeMap,
	containerIDByFuncID map[string]string,
	funcByQName map[string][]string,
	nodeFile map[string]string,
) (types.Edge, bool) {
	parts := strings.Split(pr.TargetQName, "|")
	// Minimum parts: mode + recv + at least 1 segment + method = 4
	// (same-contract depth-1 hypothetical). Real generic-chain PendingRefs have
	// depth ≥ 3 cross or ≥ 4 same, but we don't enforce that here —
	// resolver works for any depth.
	if len(parts) < 4 {
		return types.Edge{}, false
	}
	mode := parts[0]
	recvObj := parts[1]
	segs := parts[2 : len(parts)-1]
	methodName := parts[len(parts)-1]
	if len(segs) == 0 || methodName == "" {
		return types.Edge{}, false
	}

	callerContractID, ok := containerIDByFuncID[pr.SrcID]
	if !ok {
		return types.Edge{}, false
	}
	srcFile := nodeFile[pr.SrcID]

	// Determine starting namespace.
	var currentNamespace string
	switch mode {
	case "same":
		// Same-contract: starting namespace is caller container name.
		for qname, ids := range funcByQName {
			for _, fid := range ids {
				if fid == pr.SrcID {
					if dot := strings.IndexByte(qname, '.'); dot >= 0 {
						currentNamespace = qname[:dot]
					}
					break
				}
			}
			if currentNamespace != "" {
				break
			}
		}
		if currentNamespace == "" {
			return types.Edge{}, false
		}
	case "cross":
		// Cross-contract: starting namespace is receiverObj's type.
		if recvObj == "" {
			return types.Edge{}, false
		}
		if varMap := stateVarTypes[callerContractID]; varMap != nil {
			currentNamespace = varMap[recvObj]
		}
		if currentNamespace == "" {
			if paramMap := paramTypes[pr.SrcID]; paramMap != nil {
				currentNamespace = paramMap[recvObj]
			}
		}
		if currentNamespace == "" {
			return types.Edge{}, false
		}
	default:
		return types.Edge{}, false
	}

	// Walk each chain segment, threading funcReturnTypes.
	for _, seg := range segs {
		fnIDs := funcByQName[currentNamespace+"."+seg]
		if len(fnIDs) == 0 {
			return types.Edge{}, false
		}
		fnFuncID := pickSameFileCandidate(fnIDs, srcFile, nodeFile)
		returnType, ok := funcReturnTypes[fnFuncID]
		if !ok || returnType == "" {
			return types.Edge{}, false
		}
		currentNamespace = returnType
	}

	// Multi-binding aware lookup on the final return type.
	libIDs, libOK := resolveBindingLib(bindings[callerContractID], currentNamespace, methodName, funcByQName)
	if !libOK {
		return types.Edge{}, false
	}
	dstID := pickSameFileCandidate(libIDs, srcFile, nodeFile)
	conf := types.ConfExtracted
	if srcFile != "" && nodeFile[dstID] != "" && srcFile != nodeFile[dstID] {
		conf = types.ConfInferred
	}
	return types.Edge{
		Src: pr.SrcID, Dst: dstID, Type: types.EdgeCalls,
		Line: pr.Line, Count: 1, Confidence: conf,
	}, true
}

// resolveUsingForStructFieldCallRef resolves a
// struct-field-receiver PendingRef (`<obj>.<field>.<method>(...)`) to a
// single EdgeCalls edge. 7-step chain — any failure drops:
//
//  1. funcID → callerContainerID (containerIDByFuncID).
//  2. objName → objType (stateVarTypes → paramTypes fallback).
//  3. objType → structFieldTypes (must be a known struct name; primitives
//     and contracts naturally miss because they aren't in the index).
//  4. structFieldTypes[objType][fieldName] → fieldType.
//  5. (callerContainerID, fieldType) → libraryName (bindings + `*`).
//  6. `<libraryName>.<methodName>` → libraryFunctionID.
//
// Confidence: ConfExtracted (caller + library same-file) /
// ConfInferred (cross-file). obj's container / struct's file don't
// downgrade.
func resolveUsingForStructFieldCallRef(
	pr parse.PendingRef,
	bindings bindingMap,
	stateVarTypes stateVarTypeMap,
	paramTypes paramTypeMap,
	localVarTypes localVarTypeMap,
	structFieldTypes structFieldTypeMap,
	containerIDByFuncID map[string]string,
	funcByQName map[string][]string,
	nodeFile map[string]string,
) (types.Edge, bool) {
	// TargetQName encoding from matchStructFieldReceiverMethodCall:
	// `objName|fieldName|methodName`.
	parts := strings.SplitN(pr.TargetQName, "|", 3)
	if len(parts) != 3 {
		return types.Edge{}, false
	}
	objName, fieldName, methodName := parts[0], parts[1], parts[2]

	callerContractID, ok := containerIDByFuncID[pr.SrcID]
	if !ok {
		return types.Edge{}, false
	}
	// State-var → param → local-var three-tier fallback.
	// useSiteLine = pr.Line drives narrowest-scope-wins for
	// shadowed locals.
	// useSiteByte = pr.ByteOffset for byte-precision same-line
	// shadow disambiguation in addition to the line-precision filter.
	objType := lookupReceiverType(objName, callerContractID, pr.SrcID,
		pr.Line, pr.ByteOffset, stateVarTypes, paramTypes, localVarTypes)
	if objType == "" {
		return types.Edge{}, false
	}
	// objType must be a known struct; structFieldTypes[objType] presence
	// is an implicit "is-struct" check.
	fieldMap := structFieldTypes[objType]
	if fieldMap == nil {
		return types.Edge{}, false
	}
	fieldType, ok := fieldMap[fieldName]
	if !ok || fieldType == "" {
		return types.Edge{}, false
	}
	// Multi-binding aware lookup on the field's type.
	libIDs, libOK := resolveBindingLib(bindings[callerContractID], fieldType, methodName, funcByQName)
	if !libOK {
		return types.Edge{}, false
	}
	srcFile := nodeFile[pr.SrcID]
	dstID := pickSameFileCandidate(libIDs, srcFile, nodeFile)
	conf := types.ConfExtracted
	if srcFile != "" && nodeFile[dstID] != "" && srcFile != nodeFile[dstID] {
		conf = types.ConfInferred
	}
	return types.Edge{
		Src: pr.SrcID, Dst: dstID, Type: types.EdgeCalls,
		Line: pr.Line, Count: 1, Confidence: conf,
	}, true
}

// resolveUsingForNestedStructFieldCallRef resolves a depth-2 nested struct-field PendingRef
// (`<obj>.<field1>.<field2>.<method>(...)`) to a single EdgeCalls
// edge. 8-step chain — any failure drops:
//
//  1. funcID → callerContainerID
//  2. objName → objType (stateVarTypes → paramTypes fallback)
//  3. structFieldTypes[objType][field1] → field1Type
//     (field1Type must itself be a struct to continue)
//  4. structFieldTypes[field1Type][field2] → field2Type
//  5. (callerContainerID, field2Type) → libraryName (bindings + `*`)
//  6. `<libraryName>.<methodName>` → libraryFunctionID
//
// Confidence: ConfExtracted (caller + library same-file) /
// ConfInferred (cross-file). Intermediate struct definitions' files
// don't downgrade.
func resolveUsingForNestedStructFieldCallRef(
	pr parse.PendingRef,
	bindings bindingMap,
	stateVarTypes stateVarTypeMap,
	paramTypes paramTypeMap,
	localVarTypes localVarTypeMap,
	structFieldTypes structFieldTypeMap,
	containerIDByFuncID map[string]string,
	funcByQName map[string][]string,
	nodeFile map[string]string,
) (types.Edge, bool) {
	// TargetQName encoding from matchNestedStructFieldReceiverMethodCall:
	// `objName|field1|field2|methodName`.
	parts := strings.SplitN(pr.TargetQName, "|", 4)
	if len(parts) != 4 {
		return types.Edge{}, false
	}
	objName, field1, field2, methodName := parts[0], parts[1], parts[2], parts[3]

	callerContractID, ok := containerIDByFuncID[pr.SrcID]
	if !ok {
		return types.Edge{}, false
	}
	// State-var → param → local-var three-tier fallback.
	// useSiteLine = pr.Line drives narrowest-scope-wins for
	// shadowed locals.
	// useSiteByte = pr.ByteOffset for byte-precision same-line
	// shadow disambiguation in addition to the line-precision filter.
	objType := lookupReceiverType(objName, callerContractID, pr.SrcID,
		pr.Line, pr.ByteOffset, stateVarTypes, paramTypes, localVarTypes)
	if objType == "" {
		return types.Edge{}, false
	}
	// Step 3: field1 in objType's struct namespace.
	field1Map := structFieldTypes[objType]
	if field1Map == nil {
		return types.Edge{}, false
	}
	field1Type, ok := field1Map[field1]
	if !ok || field1Type == "" {
		return types.Edge{}, false
	}
	// Step 4: field2 in field1Type's struct namespace.
	field2Map := structFieldTypes[field1Type]
	if field2Map == nil {
		return types.Edge{}, false
	}
	field2Type, ok := field2Map[field2]
	if !ok || field2Type == "" {
		return types.Edge{}, false
	}
	// Step 5-6: multi-binding aware lookup on field2Type.
	libIDs, libOK := resolveBindingLib(bindings[callerContractID], field2Type, methodName, funcByQName)
	if !libOK {
		return types.Edge{}, false
	}
	srcFile := nodeFile[pr.SrcID]
	dstID := pickSameFileCandidate(libIDs, srcFile, nodeFile)
	conf := types.ConfExtracted
	if srcFile != "" && nodeFile[dstID] != "" && srcFile != nodeFile[dstID] {
		conf = types.ConfInferred
	}
	return types.Edge{
		Src: pr.SrcID, Dst: dstID, Type: types.EdgeCalls,
		Line: pr.Line, Count: 1, Confidence: conf,
	}, true
}

// resolveUsingForGenericMemberChainCallRef is the generic
// iterative resolver for arbitrary-depth pure member access chains.
// Subsumes the hardcoded depth-1/2 struct-field patterns for chains
// the earlier predicates didn't already claim (depth ≥ 3).
//
// TargetQName encoding (split by `|`):
//
//	"<obj>|<f1>|<f2>|...|<fN>|<method>"  (N ≥ 3)
//
// Resolution algorithm:
//
//  1. funcID → callerContainerID.
//  2. obj → objType (stateVarTypes → paramTypes fallback).
//  3. Starting namespace = objType.
//  4. For each f_i in fields (left to right):
//     - structFieldTypes[currentNamespace][f_i] → fieldType.
//     - currentNamespace = fieldType for next iteration.
//  5. After consuming all fields: currentNamespace = final field type.
//     (callerContainerID, currentNamespace) → libraryName (bindings +
//     `*` fallback).
//  6. `<libraryName>.<methodName>` → libraryFunctionID.
//
// Confidence: ConfExtracted (caller + library same-file) /
// ConfInferred (cross-file).
func resolveUsingForGenericMemberChainCallRef(
	pr parse.PendingRef,
	bindings bindingMap,
	stateVarTypes stateVarTypeMap,
	paramTypes paramTypeMap,
	localVarTypes localVarTypeMap,
	structFieldTypes structFieldTypeMap,
	containerIDByFuncID map[string]string,
	funcByQName map[string][]string,
	nodeFile map[string]string,
) (types.Edge, bool) {
	// TargetQName encoding from matchGenericMemberChain:
	// `obj|f1|f2|...|fN|method`. Need at least obj + 3 fields + method
	// = 5 parts (depth-3 floor).
	parts := strings.Split(pr.TargetQName, "|")
	if len(parts) < 5 {
		return types.Edge{}, false
	}
	objName := parts[0]
	fields := parts[1 : len(parts)-1]
	methodName := parts[len(parts)-1]
	if objName == "" || len(fields) == 0 || methodName == "" {
		return types.Edge{}, false
	}

	callerContractID, ok := containerIDByFuncID[pr.SrcID]
	if !ok {
		return types.Edge{}, false
	}
	// State-var → param → local-var three-tier fallback.
	// useSiteLine = pr.Line drives narrowest-scope-wins for
	// shadowed locals.
	// useSiteByte = pr.ByteOffset for byte-precision same-line
	// shadow disambiguation in addition to the line-precision filter.
	objType := lookupReceiverType(objName, callerContractID, pr.SrcID,
		pr.Line, pr.ByteOffset, stateVarTypes, paramTypes, localVarTypes)
	if objType == "" {
		return types.Edge{}, false
	}
	// Walk each field, threading structFieldTypes.
	currentNamespace := objType
	for _, f := range fields {
		fieldMap := structFieldTypes[currentNamespace]
		if fieldMap == nil {
			return types.Edge{}, false
		}
		fieldType, ok := fieldMap[f]
		if !ok || fieldType == "" {
			return types.Edge{}, false
		}
		currentNamespace = fieldType
	}
	// Multi-binding aware lookup on the final field type.
	libIDs, libOK := resolveBindingLib(bindings[callerContractID], currentNamespace, methodName, funcByQName)
	if !libOK {
		return types.Edge{}, false
	}
	srcFile := nodeFile[pr.SrcID]
	dstID := pickSameFileCandidate(libIDs, srcFile, nodeFile)
	conf := types.ConfExtracted
	if srcFile != "" && nodeFile[dstID] != "" && srcFile != nodeFile[dstID] {
		conf = types.ConfInferred
	}
	return types.Edge{
		Src: pr.SrcID, Dst: dstID, Type: types.EdgeCalls,
		Line: pr.Line, Count: 1, Confidence: conf,
	}, true
}

// resolveUsingForThisNestedChainCallRef resolves a this-prefixed nested member-chain PendingRef
// (`this.<stateVar>.<f1>.<f2>....<fN>.<method>(...)`, N ≥ 1) to a
// single EdgeCalls edge.
//
// TargetQName encoding (split by `|`):
//
//	"<stateVar>|<f1>|...|<fN>|<method>"  (N ≥ 1 → ≥ 3 parts)
//
// Resolution chain — any failure drops:
//
//  1. funcID → callerContainerID (containerIDByFuncID).
//  2. stateVar → objType (stateVarTypes[callerContainerID] ONLY — `this`
//     names a contract reference, never a parameter; the implicit
//     receiver is the caller's container).
//  3. Starting namespace = objType.
//  4. For each f_i (left to right): structFieldTypes[currentNamespace][f_i]
//     → fieldType; currentNamespace = fieldType.
//  5. (callerContainerID, currentNamespace) → libraryName
//     (bindings + `*` fallback).
//  6. `<libraryName>.<methodName>` → libraryFunctionID.
//
// Cousin of the explicit-this receiver: same dispatch idiom (`this.<x>...`),
// but that one stops at stateVarType directly (no field walk). This adds
// struct-walking, with the bare-`this` constraint forcing stateVarTypes-
// only lookup (paramTypes excluded by Solidity semantics).
//
// Confidence: ConfExtracted (caller + library same-file) /
// ConfInferred (cross-file).
func resolveUsingForThisNestedChainCallRef(
	pr parse.PendingRef,
	bindings bindingMap,
	stateVarTypes stateVarTypeMap,
	structFieldTypes structFieldTypeMap,
	containerIDByFuncID map[string]string,
	funcByQName map[string][]string,
	nodeFile map[string]string,
) (types.Edge, bool) {
	// `<stateVar>|<f1>|...|<fN>|<method>` — N ≥ 1 → at least 3 parts.
	parts := strings.Split(pr.TargetQName, "|")
	if len(parts) < 3 {
		return types.Edge{}, false
	}
	stateVar := parts[0]
	fields := parts[1 : len(parts)-1]
	methodName := parts[len(parts)-1]
	if stateVar == "" || len(fields) == 0 || methodName == "" {
		return types.Edge{}, false
	}

	callerContractID, ok := containerIDByFuncID[pr.SrcID]
	if !ok {
		return types.Edge{}, false
	}
	// `this.<stateVar>` — stateVarTypes lookup only. paramTypes excluded:
	// `this` is the implicit current-contract reference, so the named
	// member must be a state variable, never a parameter.
	varMap := stateVarTypes[callerContractID]
	if varMap == nil {
		return types.Edge{}, false
	}
	objType := varMap[stateVar]
	if objType == "" {
		return types.Edge{}, false
	}
	// Walk each field, threading structFieldTypes.
	currentNamespace := objType
	for _, f := range fields {
		fieldMap := structFieldTypes[currentNamespace]
		if fieldMap == nil {
			return types.Edge{}, false
		}
		fieldType, ok := fieldMap[f]
		if !ok || fieldType == "" {
			return types.Edge{}, false
		}
		currentNamespace = fieldType
	}
	// Multi-binding aware lookup on the final field type.
	libIDs, libOK := resolveBindingLib(bindings[callerContractID], currentNamespace, methodName, funcByQName)
	if !libOK {
		return types.Edge{}, false
	}
	srcFile := nodeFile[pr.SrcID]
	dstID := pickSameFileCandidate(libIDs, srcFile, nodeFile)
	conf := types.ConfExtracted
	if srcFile != "" && nodeFile[dstID] != "" && srcFile != nodeFile[dstID] {
		conf = types.ConfInferred
	}
	return types.Edge{
		Src: pr.SrcID, Dst: dstID, Type: types.EdgeCalls,
		Line: pr.Line, Count: 1, Confidence: conf,
	}, true
}

// edgeIdentityKey returns a stable string key identifying an
// edge's logical identity (Src + Type + Dst + Line +
// DispatchKind). Used by the using-for dedup guard to discard
// duplicate EdgeUsesFor emits when two walkers pattern-match
// the same misparse.
func edgeIdentityKey(e types.Edge) string {
	return e.Src + "|" + string(e.Type) + "|" + e.Dst + "|" + e.DispatchKind + "|" + strconv.Itoa(e.Line)
}

// pickSameFileCandidate returns the candidate ID whose file matches srcFile
// when one exists, otherwise the first candidate. Used by inheritance and
// explicit-override resolution to disambiguate homonymous targets across
// files: a same-file resolution is structurally more likely correct (a
// child won't usually inherit from a parent in an unrelated file with the
// same name), and falling back to the first ID keeps cross-file resolution
// working for genuine multi-file hierarchies. ids must be non-empty.
func pickSameFileCandidate(ids []string, srcFile string, nodeFile map[string]string) string {
	if srcFile != "" {
		for _, id := range ids {
			if nodeFile[id] == srcFile {
				return id
			}
		}
	}
	return ids[0]
}

// resolveInheritanceRef resolves one inheritance PendingRef (a single `is X` parent
// reference) against the indexed Contract / Interface tables.
//
// Edge-type classification (§2.1 / §3.1) depends on *both* child and
// parent NodeType — Solidity allows three meaningful combinations:
//
//	child=Contract,  parent=Contract  → EdgeExtends    (`contract C is Base`)
//	child=Contract,  parent=Interface → EdgeImplements (`contract C is IFoo`)
//	child=Interface, parent=Interface → EdgeExtends    (`interface IB is IA`)
//
// (child=Interface, parent=Contract is syntactically invalid in Solidity
// — interfaces can only `is` other interfaces — so it's not handled
// here; solc rejects such code before our parser sees it.)
//
// Resolution order: prefer Interface first when both tables have a hit
// for the same name. Real codebases keep contract / interface namespaces
// disjoint, but solc itself uses the same lookup space — so this matches
// Solidity's own resolution semantics.
//
// Confidence policy (§2.2):
//   - same-file resolution  → ConfExtracted
//   - cross-file resolution → ConfInferred
//   - unresolved            → returns ok=false (caller drops the edge)
//
// Returns (edge, true) on success or (zero, false) when the parent name
// matches no known Contract / Interface in the build set.
func resolveInheritanceRef(
	pr parse.PendingRef,
	byName map[types.NodeType]map[string][]string,
	nodeFile map[string]string,
	nodeType map[string]types.NodeType,
) (types.Edge, bool) {
	// Locate the parent node — prefer Interface first to match solc's
	// name-resolution behaviour (interfaces and contracts share one
	// global namespace in Solidity). Within the matching type bucket,
	// prefer a same-file candidate so homonymous parents declared across
	// files don't shadow the locally-resolvable one. The explicit
	// override path does the same.
	srcFile := nodeFile[pr.SrcID]
	var dstID string
	var parentType types.NodeType
	if ids := byName[types.NodeInterface][pr.TargetQName]; len(ids) > 0 {
		dstID = pickSameFileCandidate(ids, srcFile, nodeFile)
		parentType = types.NodeInterface
	} else if ids := byName[types.NodeContract][pr.TargetQName]; len(ids) > 0 {
		dstID = pickSameFileCandidate(ids, srcFile, nodeFile)
		parentType = types.NodeContract
	} else {
		return types.Edge{}, false
	}

	// Classify based on (child, parent). The only combination that maps
	// to EdgeImplements is (Contract, Interface) — a contract realising
	// an interface. Interface-to-interface inheritance is EdgeExtends
	// (interface IB is IA → IB extends IA, not implements).
	childType := nodeType[pr.SrcID]
	edgeType := types.EdgeExtends
	if childType == types.NodeContract && parentType == types.NodeInterface {
		edgeType = types.EdgeImplements
	}

	conf := types.ConfExtracted
	if nodeFile[pr.SrcID] != "" && nodeFile[dstID] != "" && nodeFile[pr.SrcID] != nodeFile[dstID] {
		conf = types.ConfInferred
	}
	return types.Edge{
		Src: pr.SrcID, Dst: dstID, Type: edgeType,
		Line: pr.Line, Count: 1, Confidence: conf,
	}, true
}

// resolveInterfaceDispatchRef resolves one interface-dispatch PendingRef (an
// `IFoo(addr).bar(...)` invocation) into a single EdgeInvokes edge.
//
// pr.TargetQName carries `TypeName.MethodName` (set in dispatch.go).
// Two predicates must hold for emission:
//
//  1. The leading `TypeName` must be a known NodeInterface in the build —
//     filters out plain identifier casts (`address(addr).foo`,
//     `MyContract(addr).foo`) which are not interface dispatch by the
//     spec definition.
//  2. The fully-qualified `TypeName.MethodName` must resolve to a
//     Function node — i.e. the interface declares a `bar(...)` method.
//     Unknown methods on a known interface (typos, evolving APIs across
//     branch builds) drop, matching the inheritance/override strict-purge policy.
//
// Confidence is *constant* ConfAmbiguous (§5.0 Q5). This differs from
// inheritance and override resolution (file-boundary tagged) because the
// runtime address determines actual dispatch — the resolver can only
// identify the interface-method declaration, never the live target.
// The `llmSafeStoreReader` wrapper (hunk-graph §11.3) filters
// AMBIGUOUS edges from LLM-facing queries automatically.
//
// When multiple Function nodes share the same `TypeName.MethodName`
// qname (rare — would require duplicate interface declarations across
// files), pick the first candidate. Disambiguation could be sharpened
// by preferring same-file or the interface's own file, but with no
// real-world impact in the current corpus (validated against
// testdata/dispatch fixtures).
func resolveInterfaceDispatchRef(
	pr parse.PendingRef,
	byName map[types.NodeType]map[string][]string,
	funcByQName map[string][]string,
	nodeFile map[string]string,
) (types.Edge, bool) {
	// Split TargetQName on the first "." into (typeName, methodName).
	// dispatch.go always emits exactly one "." — no qualified parent
	// names (matches the inheritance resolver's known limitation: leading-identifier
	// only).
	dot := strings.IndexByte(pr.TargetQName, '.')
	if dot <= 0 || dot == len(pr.TargetQName)-1 {
		return types.Edge{}, false
	}
	typeName := pr.TargetQName[:dot]
	// Predicate 1: the leading type must be a NodeInterface. Plain
	// identifiers (variables, free functions, primitive type tokens
	// like `address`) miss the index and drop.
	if ids := byName[types.NodeInterface][typeName]; len(ids) == 0 {
		return types.Edge{}, false
	}
	// Predicate 2: the interface must declare the named method. Look
	// up by fully-qualified name (`Interface.method`).
	candidates := funcByQName[pr.TargetQName]
	if len(candidates) == 0 {
		return types.Edge{}, false
	}
	// Disambiguation: when multiple interfaces share the same name
	// across files (homonym across fixtures), prefer the candidate in
	// the source function's file, then any same-file as one of the
	// interface IDs. Fall back to candidates[0] when neither rule fires.
	srcFile := nodeFile[pr.SrcID]
	dstID := candidates[0]
	for _, fid := range candidates {
		if nodeFile[fid] == srcFile {
			dstID = fid
			break
		}
	}
	// AMBIGUOUS is the *fixed* confidence — see preamble + §5.0 Q5.
	return types.Edge{
		Src: pr.SrcID, Dst: dstID, Type: types.EdgeInvokes,
		Line: pr.Line, Count: 1, Confidence: types.ConfAmbiguous,
	}, true
}

// resolveContractCastRef resolves one
// contract-cast PendingRef (`MyContract(addr).method()`) to a single
// EdgeInvokes edge by looking up the method on the named contract.
//
// Mirrors resolveInterfaceDispatchRef step-for-step except the type
// lookup hits byName[NodeContract] and the emitted edge carries
// DispatchKind="contract_cast". Library subkinds (NodeContract with
// SubKind="library") are accepted at the byName layer — libraries
// can be cast in some patterns, and filtering would add a
// containerNameByID/subKind lookup without clear value.
//
// Confidence is fixed at ConfAmbiguous: the cast resolves a typed
// receiver but the runtime address determines the actual target,
// just like interface dispatch.
func resolveContractCastRef(
	pr parse.PendingRef,
	byName map[types.NodeType]map[string][]string,
	funcByQName map[string][]string,
	nodeFile map[string]string,
) (types.Edge, bool) {
	dot := strings.IndexByte(pr.TargetQName, '.')
	if dot <= 0 || dot == len(pr.TargetQName)-1 {
		return types.Edge{}, false
	}
	typeName := pr.TargetQName[:dot]
	if ids := byName[types.NodeContract][typeName]; len(ids) == 0 {
		return types.Edge{}, false
	}
	candidates := funcByQName[pr.TargetQName]
	if len(candidates) == 0 {
		return types.Edge{}, false
	}
	srcFile := nodeFile[pr.SrcID]
	dstID := candidates[0]
	for _, fid := range candidates {
		if nodeFile[fid] == srcFile {
			dstID = fid
			break
		}
	}
	return types.Edge{
		Src: pr.SrcID, Dst: dstID, Type: types.EdgeInvokes,
		Line: pr.Line, Count: 1, Confidence: types.ConfAmbiguous,
		DispatchKind: dispatchKindContractCast,
	}, true
}
