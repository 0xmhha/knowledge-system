package solidity

import (
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
//
// typeName is the using-for key of the declared type (usingForTypeKey) and
// typeText the type as written (`SetLib.Set`): the using-for resolvers match
// the key, a member access resolves the text in scope.
type localDecl struct {
	declLine      int
	scopeEndLine  int
	declStartByte int
	scopeEndByte  int
	typeName      string
	typeText      string
}

// localVarTypeMap: funcID → varName → []localDecl. Each
// emitted variable_declaration appends an entry; the same name can
// have multiple entries when shadowed in nested blocks. Resolution
// at the use site uses line containment + max declLine selection.
type localVarTypeMap map[string]map[string][]localDecl

// Resolve unions the per-file results and turns their pending references
// into edges.
//
// A name is resolved in the scope of the contract that uses it (scopeIndex):
// the contract itself, its bases in linearisation order, then the file and
// the files it imports. That covers emitted events, applied modifiers, calls
// by name, reverts with a custom error, state variable and mapping reads and
// writes, and overrides. An edge inside one file is EXTRACTED and one that
// crosses files INFERRED.
//
// What Resolve needs to know about a file other than its pending references
// it reads from that file's nodes: which container declares a member (the
// qualified name), what a file imports (its Import nodes), the bases of a
// container and the parameter count of a function (the declaration header),
// the size of a type (struct member and enum value nodes). An incremental
// build parses only the changed files and hands the others over as stored
// nodes; read this way, it resolves like a full build.
//
// Kept from the earlier resolver, with their own lookups: the using-for
// bindings and the receiver shapes of `x.method()` (resolveUsingFor*), casts
// (`IFoo(addr).bar()`, `Token(addr).mint()`: invokes, AMBIGUOUS) and
// low-level calls.
func (p *Parser) Resolve(results []*parse.ParseResult) (*parse.ResolvedGraph, error) {
	out := &parse.ResolvedGraph{}

	// nodeFile maps a node to its file and knows which files a file imports,
	// so a name shared by several files resolves to the one the using file
	// can see, and cross-file resolutions are marked INFERRED.
	nodeFile := newFileIndex()
	// nodeType maps node ID -> declared type — used by inheritance
	// resolution to reclassify EdgeExtends → EdgeImplements when the
	// target is an Interface.
	nodeType := map[string]types.NodeType{}
	// byName indexes contracts, interfaces and free functions by their
	// unqualified Name. Members (events, modifiers, mappings, state
	// variables, errors) are not in it: they resolve through the scope
	// index, in the contract that uses them.
	byName := map[types.NodeType]map[string][]string{}
	add := func(nt types.NodeType, key, id string) {
		if byName[nt] == nil {
			byName[nt] = map[string][]string{}
		}
		byName[nt][key] = append(byName[nt][key], id)
	}

	// funcByQName: "Container.func" → []nodeID, for functions and
	// modifiers. Plural because two files can declare the same contract
	// name; callers pick through nodeFile.pick.
	// containerNameByID: containerID → unqualified name.
	// containerIDByFuncID: function or modifier → enclosing container.
	funcByQName := map[string][]string{}
	containerNameByID := map[string]string{}
	containerIDByFuncID := map[string]string{}

	// pending holds the references of each result, one per occurrence in the
	// source: Pass 1 hands them over folded (see packRefs).
	pending := make([][]parse.PendingRef, len(results))
	for i, r := range results {
		out.Nodes = append(out.Nodes, r.Nodes...)
		out.Edges = append(out.Edges, r.Edges...)
		pending[i] = unpackRefs(r.Pending)
	}
	for _, n := range out.Nodes {
		nodeFile.byNode[n.ID] = n.FilePath
		nodeType[n.ID] = n.Type
		switch n.Type {
		case types.NodeModifier:
			if n.QualifiedName != "" && n.QualifiedName != n.Name {
				funcByQName[n.QualifiedName] = append(funcByQName[n.QualifiedName], n.ID)
			}
		case types.NodeContract, types.NodeInterface:
			add(n.Type, n.Name, n.ID)
			containerNameByID[n.ID] = n.Name
		case types.NodeFunction:
			funcByQName[n.QualifiedName] = append(funcByQName[n.QualifiedName], n.ID)
			// Free functions (no enclosing container) are also indexed by
			// name, for `using {f as +} for T` bindings.
			if n.QualifiedName == n.Name {
				add(types.NodeFunction, n.Name, n.ID)
			}
		}
	}

	// The scope index holds membership (which container declares which
	// member). Membership used to be guessed from the qualified-name prefix
	// plus the file, once per consumer; it is read from one place now.
	scope := newScopeIndex(out.Nodes, nodeFile)
	// Imports before any other name: every later lookup asks which files a
	// file sees.
	linkImports(out, scope)
	scoped := newScopedResolver(scope, out)

	// fresh holds the files parsed in this run. On an incremental build an
	// unchanged file arrives as stored nodes without edges; its nodes are
	// already final (slot indexes include the bases' offset) and must not
	// be laid out again.
	fresh := map[string]bool{}
	for _, r := range results {
		if len(r.Edges) > 0 {
			fresh[r.Path] = true
		}
	}

	// stateVarTypes: container → variable name → declared type, for the
	// receiver of `variable.method(...)`. Own variables here; inherited ones
	// are added once the linearisation is known.
	stateVarTypes := stateVarTypeMap{}
	for id, cid := range scope.owner {
		n := scope.node[id]
		switch n.Type {
		case types.NodeFunction, types.NodeModifier:
			containerIDByFuncID[id] = cid
		case types.NodeField, types.NodeConstant:
			// The node holds the type as written; the using-for resolvers
			// look a receiver up by its key.
			key := usingForTypeKey(n.Signature)
			if key == "" {
				continue
			}
			if stateVarTypes[cid] == nil {
				stateVarTypes[cid] = map[string]string{}
			}
			stateVarTypes[cid][n.Name] = key
		}
	}

	// Pass 2a — inheritance. The bases of every container are read from
	// its declaration header, so they are known for unchanged files too.
	linkInheritance(out, scope)

	// parents: container → direct bases, in source order.
	parents := scope.inheritanceParents()

	// The linearisation of every container: the order names are looked up
	// in (itself, then its bases). Computed once and shared by the scope
	// index, receiver types, storage layout and inherited function-typed
	// fields. mroFallbackIDs are the containers whose bases have no
	// consistent linearisation.
	mroByContractID, mroFallbackIDs := computeC3LinearizationWithFallbacks(parents)
	scope.mro = mroByContractID
	scoped.parents = parents
	inheritStateVarTypes(stateVarTypes, mroByContractID)

	// Storage layout of the parsed files: slots within each container,
	// then the slots the bases occupy.
	sizes := typeSizesFromNodes(out.Nodes)
	layoutStateVariables(out.Nodes, scope.owner, fresh, sizes)
	applyInheritanceSlotOffset(out.Nodes, scope.owner, mroByContractID, fresh, sizes)

	// Pre-build the (contractID, typeName) →
	// libraryName binding map by sweeping all dispatchKindUsingForTypeBind
	// PendingRefs across results. Done before Pass 2b so the using-for-call
	// branch can consume it without ordering surprises.
	bindings := bindingMap{}
	// Pre-build (funcID, paramName) → typeName so
	// parameter-receiver method calls can resolve their receiver's
	// declared type the same way state-variable receivers do.
	paramTypes := paramTypeMap{}
	// paramTypeTexts: (funcID, paramName) → the type as written, for the
	// root of a member access (`given.high`, where given is `Lib.Limits`).
	paramTypeTexts := paramTypeMap{}
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
	for _, refs := range pending {
		for _, pr := range refs {
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
				// The record holds the type as written. A member access
				// through the parameter resolves that text in scope; the
				// using-for resolvers get its key, and no entry for a
				// mapping, which is never a using-for receiver.
				if paramTypeTexts[pr.SrcID] == nil {
					paramTypeTexts[pr.SrcID] = map[string]string{}
				}
				paramTypeTexts[pr.SrcID][paramName] = typeName
				key := usingForTypeKey(typeName)
				if key == "" {
					continue
				}
				if paramTypes[pr.SrcID] == nil {
					paramTypes[pr.SrcID] = map[string]string{}
				}
				paramTypes[pr.SrcID][paramName] = key
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
						typeName:      usingForTypeKey(typeName),
						typeText:      typeName,
					},
				)
			}
		}
	}

	scoped.paramTypeTexts, scoped.localVarTypes = paramTypeTexts, localVarTypes

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
	scoped.externalCallers = externalCallSrcs

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
	// Pass 2b — everything except inheritance (already done) and any
	// future detector-specific branches go through this loop. Overrides
	// rely on the `parents` index built between the two sub-passes.
	for _, refs := range pending {
		for _, pr := range refs {
			// Names resolved in the scope of the contract that uses them:
			// emits, modifiers, calls by name, reverts, reads and writes.
			if isScopedKind(pr.DispatchKind) {
				scoped.resolve(pr)
				continue
			}
			// Override branch — fans out one EdgeOverrides per resolved
			// parent. Bare `override` consults the inheritance index; the
			// explicit form does a direct qname lookup.
			if pr.DispatchKind == dispatchKindOverride ||
				pr.DispatchKind == dispatchKindOverrideExplicit {
				scoped.resolveOverride(pr)
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
			if argc, ok := arityOfKind(pr.DispatchKind, dispatchKindUsingForCall); ok {
				resolveUsingForCallRef(
					pr, argc, scoped, bindings, stateVarTypes, paramTypes, localVarTypes,
					containerIDByFuncID, funcByQName,
				)
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
						fn1ID = pickSameFileCandidate(ids, nodeFile.of(pr.SrcID), nodeFile)
					}
				}
				if fn1ID == "" {
					if ids := funcByQName[fn1Name]; len(ids) > 0 {
						fn1ID = pickSameFileCandidate(ids, nodeFile.of(pr.SrcID), nodeFile)
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
				fn2ID := pickSameFileCandidate(ids, nodeFile.of(pr.SrcID), nodeFile)
				if fn2ID == "" {
					continue
				}
				rt2 := funcReturnTypes[fn2ID]
				if isAddressType(rt2) {
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
						innerFuncID = pickSameFileCandidate(ids, nodeFile.of(pr.SrcID), nodeFile)
					}
				}
				if innerFuncID == "" {
					if ids := funcByQName[innerFnName]; len(ids) > 0 {
						innerFuncID = pickSameFileCandidate(ids, nodeFile.of(pr.SrcID), nodeFile)
					}
				}
				if innerFuncID == "" {
					continue
				}
				if isAddressType(funcReturnTypes[innerFuncID]) {
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
		}
	}
	out.Edges = append(out.Edges, mergeRepeatedEdges(scoped.edges)...)
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
	nodeFile.stampEdges(out.Edges)
	return out, nil
}

// applyInheritanceSlotOffset adds to the slot index of every state variable
// of a freshly parsed file the number of slots its container's bases occupy,
// so the index is the position in the storage of a contract that inherits
// them.
//
// The offset of a container is the sum of the slots its bases declare, over
// its linearisation; a base shared by two branches (a diamond) is counted
// once. The slots a container declares are where its own layout ends: the
// last slot any of its state variables occupies, plus one. GovBase ends with
// `uint256[37] __gap` in slots 13 to 49, so GovMasterMinter.fiatToken is in
// slot 50 (0x32, as the source documents), not in slot 14. Fields and
// mappings both count and both move: a mapping takes a slot like any other
// variable. owner tells which container declares a variable, so two
// contracts of the same name in two files keep separate counts.
//
// The nodes of an unchanged file (not in fresh) already include their
// offset. They are not moved, and the slots such a container declares are
// its layout end minus the offset it already carries.
func applyInheritanceSlotOffset(nodes []types.Node, owner map[string]string, mro map[string][]string,
	fresh map[string]bool, sizes typeSizes,
) {
	layoutEnd := map[string]int{}
	freshContainer := map[string]bool{}
	for i := range nodes {
		n := &nodes[i]
		cid, owned := owner[n.ID]
		if !owned || !isStateVariable(n) {
			continue
		}
		if fresh[n.FilePath] {
			freshContainer[cid] = true
		}
		if end := n.SlotIndex + sizes.slotsOccupied(n); end > layoutEnd[cid] {
			layoutEnd[cid] = end
		}
	}
	declared := map[string]int{}
	var inherited func(cid string) int
	// declaredSlots returns the slots cid itself declares.
	declaredSlots := func(cid string) int {
		if n, done := declared[cid]; done {
			return n
		}
		declared[cid] = 0 // guards against a cycle in a broken hierarchy
		n := layoutEnd[cid]
		if !freshContainer[cid] && n > 0 {
			n = max(n-inherited(cid), 0)
		}
		declared[cid] = n
		return n
	}
	inherited = func(cid string) int {
		total := 0
		if lin := mro[cid]; len(lin) > 1 {
			for _, ancestor := range lin[1:] { // skip the container itself
				total += declaredSlots(ancestor)
			}
		}
		return total
	}
	for i := range nodes {
		n := &nodes[i]
		cid, owned := owner[n.ID]
		if !owned || !isStateVariable(n) || !fresh[n.FilePath] {
			continue
		}
		n.SlotIndex += inherited(cid)
	}
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
	nodeFile *fileIndex,
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
	srcFile := nodeFile.of(pr.SrcID)
	dstID := ""
	if pathHint != "" {
		dstID = pickByPathHint(ids, pathHint, nodeFile)
	}
	if dstID == "" {
		dstID = pickSameFileCandidate(ids, srcFile, nodeFile)
	}
	conf := types.ConfExtracted
	if srcFile != "" && nodeFile.of(dstID) != "" && srcFile != nodeFile.of(dstID) {
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
func pickByPathHint(ids []string, pathHint string, nodeFile *fileIndex) string {
	// Strip a leading `./` from the hint so a recorded path of
	// `./math.sol` matches a nodeFile of `math.sol`.
	hint := strings.TrimPrefix(pathHint, "./")
	for _, id := range ids {
		if nodeFile.of(id) == hint {
			return id
		}
	}
	for _, id := range ids {
		if strings.HasSuffix(nodeFile.of(id), "/"+hint) || strings.HasSuffix(nodeFile.of(id), hint) {
			return id
		}
	}
	return ""
}

// lookupReceiver returns the declared type of the receiver of
// `name.method(...)` and whether name is a local variable or a parameter of
// the callable, as opposed to a state variable of its contract. The order is
// the language's: a local, then a parameter, then a state variable (the
// contract's own or an inherited one). The type is "" when name is none of
// them. useSiteLine and useSiteByte pick the declaration in scope when a
// local name is declared in more than one block.
func lookupReceiver(
	name, contractID, funcID string,
	useSiteLine, useSiteByte int,
	stateVarTypes stateVarTypeMap,
	paramTypes paramTypeMap,
	localVarTypes localVarTypeMap,
) (typeName string, isLocal bool) {
	if decls := localVarTypes[funcID][name]; len(decls) > 0 {
		if d, ok := selectLocalDecl(decls, useSiteLine, useSiteByte); ok && d.typeName != "" {
			return d.typeName, true
		}
	}
	if t := paramTypes[funcID][name]; t != "" {
		return t, true
	}
	return stateVarTypes[contractID][name], false
}

// lookupReceiverType returns the declared type of a receiver (see
// lookupReceiver).
func lookupReceiverType(
	name, contractID, funcID string,
	useSiteLine, useSiteByte int,
	stateVarTypes stateVarTypeMap,
	paramTypes paramTypeMap,
	localVarTypes localVarTypeMap,
) string {
	typeName, _ := lookupReceiver(name, contractID, funcID, useSiteLine, useSiteByte,
		stateVarTypes, paramTypes, localVarTypes)
	return typeName
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
//
// ok is false when no declaration is in scope at the use site. decls must
// not be empty.
func selectLocalDecl(decls []localDecl, useSiteLine, useSiteByte int) (decl localDecl, ok bool) {
	if useSiteLine == 0 {
		// Defensive fallback — first-decl-wins.
		return decls[0], true
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
			return decls[bestIdx], true
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
		return localDecl{}, false
	}
	return decls[bestIdx], true
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
	argc int,
	scoped *scopedResolver,
	bindings bindingMap,
	stateVarTypes stateVarTypeMap,
	paramTypes paramTypeMap,
	localVarTypes localVarTypeMap,
	containerIDByFuncID map[string]string,
	funcByQName map[string][]string,
) {
	// TargetQName encoding from runUsingForCalls: `receiverName|methodName`.
	sep := strings.IndexByte(pr.TargetQName, '|')
	if sep < 0 {
		return
	}
	receiverName := pr.TargetQName[:sep]
	methodName := pr.TargetQName[sep+1:]

	// A free function has no container; its receivers are parameters and
	// locals only.
	contractID := containerIDByFuncID[pr.SrcID]
	// Receiver type lookup chain: local variable, parameter, state
	// variable (inherited ones included). pr.Line and pr.ByteOffset pick the
	// declaration in scope when a local name is declared twice.
	typeName, isLocal := lookupReceiver(receiverName, contractID, pr.SrcID,
		pr.Line, pr.ByteOffset, stateVarTypes, paramTypes, localVarTypes)
	// Multi-binding aware resolution via resolveBindingLib.
	ids, ok := resolveBindingLib(bindings[contractID], typeName, methodName, funcByQName)
	if typeName != "" && ok {
		scoped.usingForCall(pr, ids, receiverName, isLocal, argc)
		return
	}
	// No using-for binding: the receiver is a variable of interface or
	// contract type, `super`, `this`, or the name of a library or a base
	// contract. A call that binds to nothing in the graph (a method of an
	// elementary type, a library outside the build) is counted as
	// unresolved.
	if !scoped.resolveMemberCall(pr, receiverName, typeName, methodName, argc) &&
		typeName != "" && !builtinMembers[methodName] {
		scoped.unresolved(pr)
	}
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
	nodeFile *fileIndex,
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
	isAddress := isAddressType(typeName)
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
	srcFile := nodeFile.of(pr.SrcID)
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
	nodeFile *fileIndex,
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
			innerFuncID = pickSameFileCandidate(ids, nodeFile.of(pr.SrcID), nodeFile)
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
	srcFile := nodeFile.of(pr.SrcID)
	dstID := pickSameFileCandidate(ids, srcFile, nodeFile)
	conf := types.ConfExtracted
	if srcFile != "" && nodeFile.of(dstID) != "" && srcFile != nodeFile.of(dstID) {
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
	nodeFile *fileIndex,
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
	srcFile := nodeFile.of(pr.SrcID)
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
	if srcFile != "" && nodeFile.of(dstID) != "" && srcFile != nodeFile.of(dstID) {
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
	nodeFile *fileIndex,
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
	srcFile := nodeFile.of(pr.SrcID)
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
	if srcFile != "" && nodeFile.of(dstID) != "" && srcFile != nodeFile.of(dstID) {
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
	nodeFile *fileIndex,
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
	srcFile := nodeFile.of(pr.SrcID)
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
	if srcFile != "" && nodeFile.of(dstID) != "" && srcFile != nodeFile.of(dstID) {
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
	nodeFile *fileIndex,
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
	srcFile := nodeFile.of(pr.SrcID)
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
	if srcFile != "" && nodeFile.of(dstID) != "" && srcFile != nodeFile.of(dstID) {
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
	nodeFile *fileIndex,
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
	srcFile := nodeFile.of(pr.SrcID)

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
	if srcFile != "" && nodeFile.of(dstID) != "" && srcFile != nodeFile.of(dstID) {
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
	nodeFile *fileIndex,
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
	srcFile := nodeFile.of(pr.SrcID)
	dstID := pickSameFileCandidate(libIDs, srcFile, nodeFile)
	conf := types.ConfExtracted
	if srcFile != "" && nodeFile.of(dstID) != "" && srcFile != nodeFile.of(dstID) {
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
	nodeFile *fileIndex,
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
	srcFile := nodeFile.of(pr.SrcID)
	dstID := pickSameFileCandidate(libIDs, srcFile, nodeFile)
	conf := types.ConfExtracted
	if srcFile != "" && nodeFile.of(dstID) != "" && srcFile != nodeFile.of(dstID) {
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
	nodeFile *fileIndex,
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
	srcFile := nodeFile.of(pr.SrcID)
	dstID := pickSameFileCandidate(libIDs, srcFile, nodeFile)
	conf := types.ConfExtracted
	if srcFile != "" && nodeFile.of(dstID) != "" && srcFile != nodeFile.of(dstID) {
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
	nodeFile *fileIndex,
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
	srcFile := nodeFile.of(pr.SrcID)
	dstID := pickSameFileCandidate(libIDs, srcFile, nodeFile)
	conf := types.ConfExtracted
	if srcFile != "" && nodeFile.of(dstID) != "" && srcFile != nodeFile.of(dstID) {
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
func pickSameFileCandidate(ids []string, srcFile string, nodeFile *fileIndex) string {
	return nodeFile.pick(ids, srcFile)
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
	nodeFile *fileIndex,
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
	dstID := nodeFile.pick(candidates, nodeFile.of(pr.SrcID))
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
	nodeFile *fileIndex,
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
	dstID := nodeFile.pick(candidates, nodeFile.of(pr.SrcID))
	return types.Edge{
		Src: pr.SrcID, Dst: dstID, Type: types.EdgeInvokes,
		Line: pr.Line, Count: 1, Confidence: types.ConfAmbiguous,
		DispatchKind: dispatchKindContractCast,
	}, true
}
