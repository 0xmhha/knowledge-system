package solidity

import (
	"strings"

	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// Dispatch kinds of the invokes edges made for a call through a variable
// whose declared type is an interface or a contract (`token.mint(x)`). The
// address the variable holds decides the code that runs; the edge ends at
// the method of the declared type, which is what the source names, and the
// edge type invokes (not calls) says that the code is chosen at run time.
const (
	dispatchKindInterfaceMethod = "interface_method"
	dispatchKindContractMethod  = "contract_method"
)

// Receivers that name the current contract instead of a variable.
const (
	receiverSuper = "super"
	receiverThis  = "this"
)

// builtinCalls are the global functions a call by name can mean. A call to
// one of them has no node to point at and is not counted as unresolved.
var builtinCalls = map[string]bool{
	"require": true, "assert": true, "revert": true, "keccak256": true,
	"sha256": true, "ripemd160": true, "ecrecover": true, "addmod": true,
	"mulmod": true, "gasleft": true, "blockhash": true, "blobhash": true,
	"selfdestruct": true, "type": true,
}

// builtinMembers are the members the language gives to elementary types,
// arrays and `abi`. A call of one of them through a variable has no node to
// point at and is not counted as unresolved.
var builtinMembers = map[string]bool{
	"push": true, "pop": true, "call": true, "staticcall": true,
	"delegatecall": true, "transfer": true, "send": true, "concat": true,
	"encode": true, "encodePacked": true, "encodeWithSelector": true,
	"encodeWithSignature": true, "encodeCall": true, "decode": true,
	"wrap": true, "unwrap": true,
}

// scopedResolver turns the pending references of Pass 1 into edges by
// looking each name up in the scope of the contract that uses it.
type scopedResolver struct {
	scope *scopeIndex
	out   *parse.ResolvedGraph
	// arity maps a function to its number of parameters.
	arity map[string]int
	// parents maps a container to its direct bases, in source order.
	parents map[string][]string
	// paramTypeTexts and localVarTypes hold the declared type, as written,
	// of the parameters and locals of each callable, for the root of a
	// member access.
	paramTypeTexts paramTypeMap
	localVarTypes  localVarTypeMap
	// edges holds the edges made so far. Each reference is one site in the
	// source, so repeats are folded into a count before they join the graph.
	edges []types.Edge
	// externalCallers holds the callables found to make a low-level call to
	// an address (see external_call.go). Resolve owns the set and marks the
	// nodes.
	externalCallers map[string]bool
}

// newScopedResolver reads the number of parameters of every function from
// its declaration header.
func newScopedResolver(scope *scopeIndex, out *parse.ResolvedGraph) *scopedResolver {
	r := &scopedResolver{scope: scope, out: out, arity: map[string]int{}}
	for id, n := range scope.node {
		if n.Type != types.NodeFunction {
			continue
		}
		if count, ok := parameterCount(n); ok {
			r.arity[id] = count
		}
	}
	return r
}

// isScopedKind reports whether kind is a reference kind the scoped resolver
// owns.
func isScopedKind(kind string) bool {
	switch kind {
	case refEmit, refModifier, refRevert, refRead, refWrite, refTypeUse,
		refEncodeCall, refSelector, refImplicitOverride, refImport, refUnfollowedPointer, dispatchKindInherit:
		return true
	}
	base, _, _ := strings.Cut(kind, refSep)
	return base == refNameCall || base == refMemberRead || base == refMemberWrite || base == refAddressReceiver
}

// edge appends an edge from the reference's source to dst.
func (r *scopedResolver) edge(pr parse.PendingRef, et types.EdgeType, dst string) {
	r.edges = append(r.edges, types.Edge{
		Src: pr.SrcID, Dst: dst, Type: et,
		Line: pr.Line, Count: 1, Order: pr.Order,
		Confidence: r.scope.files.confidence(pr.SrcID, dst),
	})
}

// unresolved counts a reference whose name no declaration in scope has.
func (r *scopedResolver) unresolved(pr parse.PendingRef) {
	r.out.CountUnresolved(parse.UnresolvedNoMatch, pr.TargetQName, r.scope.files.of(pr.SrcID), pr.Line)
}

// lookup resolves a name, or `Owner.name`, as written in the callable that
// made the reference. A qualified name is looked up among the members of
// the named contract, library or interface and its bases.
func (r *scopedResolver) lookup(pr parse.PendingRef, name string, accept func(types.NodeType) bool) []string {
	file := r.scope.files.of(pr.SrcID)
	if owner, member, qualified := strings.Cut(name, qnameSep); qualified {
		if owner == receiverThis {
			return r.scope.lookupInherited(r.scope.linearisation(r.scope.owner[pr.SrcID]), member, accept)
		}
		if target, isFile := r.scope.fileAlias(file, owner); isFile {
			return r.scope.lookupFileLevel(target, member, accept)
		}
		ownerID := r.scope.container(file, owner)
		if ownerID == "" {
			return nil
		}
		return r.scope.lookupInherited(r.scope.linearisation(ownerID), member, accept)
	}
	return r.scope.lookup(r.scope.owner[pr.SrcID], file, name, accept)
}

// resolve turns one reference into edges.
func (r *scopedResolver) resolve(pr parse.PendingRef) {
	switch pr.DispatchKind {
	case refEmit:
		r.resolveTo(pr, types.EdgeEmitsEvent, isType(types.NodeEvent))
	case refRevert:
		r.resolveTo(pr, types.EdgeRevertsWith, isType(types.NodeError))
	case refModifier:
		r.resolveModifier(pr)
	case refRead, refWrite:
		r.resolveAccess(pr)
	case refImplicitOverride:
		r.resolveOverride(pr)
	case refUnfollowedPointer:
		r.out.CountUnresolved(parse.UnresolvedNoTarget, pr.TargetQName, r.scope.files.of(pr.SrcID), pr.Line)
	case refImport, dispatchKindInherit:
		// Dependency records. The imports and the bases of every file
		// are read from nodes (linkImports, linkInheritance), so that an
		// unchanged file keeps them on an incremental build.
	case refTypeUse:
		r.resolveTypeUse(pr)
	case refEncodeCall:
		r.resolveFunctionRef(pr, dispatchKindEncodeCall)
	case refSelector:
		r.resolveFunctionRef(pr, dispatchKindSelector)
	default:
		base, detail, _ := strings.Cut(pr.DispatchKind, refSep)
		switch base {
		case refNameCall:
			if argc, ok := arityOfKind(pr.DispatchKind, refNameCall); ok {
				r.resolveNameCall(pr, argc)
			}
		case refMemberRead, refMemberWrite:
			r.resolveMember(pr, detail, base == refMemberWrite)
		case refAddressReceiver:
			r.resolveAddressReceiver(pr, detail)
		}
	}
}

// isType returns a filter that accepts exactly one node type.
func isType(want types.NodeType) func(types.NodeType) bool {
	return func(t types.NodeType) bool { return t == want }
}

// anyType accepts every node type: a name resolves to its nearest
// declaration whatever that declares.
func anyType(types.NodeType) bool { return true }

// resolveTo emits one edge to the declaration the name resolves to, or
// counts the reference as unresolved.
func (r *scopedResolver) resolveTo(pr parse.PendingRef, et types.EdgeType, accept func(types.NodeType) bool) {
	ids := r.lookup(pr, pr.TargetQName, accept)
	if len(ids) == 0 {
		r.unresolved(pr)
		return
	}
	r.edge(pr, et, ids[0])
}

// resolveModifier resolves a modifier invocation. In a constructor header
// the name can be a base contract (`constructor() Base(x)`): that is a call
// of the base's constructor, when the base declares one.
func (r *scopedResolver) resolveModifier(pr parse.PendingRef) {
	if ids := r.lookup(pr, pr.TargetQName, isType(types.NodeModifier)); len(ids) > 0 {
		r.edge(pr, types.EdgeHasModifier, ids[0])
		return
	}
	if base := r.scope.container(r.scope.files.of(pr.SrcID), pr.TargetQName); base != "" {
		if ctor := r.scope.members[base][nameConstructor]; len(ctor) > 0 {
			r.edge(pr, types.EdgeCalls, ctor[0])
		}
		return
	}
	r.unresolved(pr)
}

// resolveNameCall resolves `name(args)`. A function gets a calls edge, a
// custom error a reverts_with edge (`require(ok, NotOwner())`). A struct, a
// contract, an interface or an enum is a constructor or a cast: the type is
// named, which is a uses_type edge.
func (r *scopedResolver) resolveNameCall(pr parse.PendingRef, argc int) {
	name := pr.TargetQName
	ids := r.lookup(pr, name, anyType)
	if len(ids) == 0 {
		if !builtinCalls[name] {
			r.unresolved(pr)
		}
		return
	}
	switch t := r.scope.node[ids[0]].Type; {
	case t == types.NodeError:
		r.edge(pr, types.EdgeRevertsWith, ids[0])
	case t == types.NodeFunction:
		r.callEdges(pr, types.EdgeCalls, r.byArity(r.scope.filter(ids, isType(types.NodeFunction)), argc), "")
	case isTypeDeclaration(t):
		r.typeCall(pr, ids[0])
	}
}

// typeCall emits the edges of `T(...)` where T is a type. A cast
// (`IToken(addr)`) names the type: uses_type. A struct constructor names the
// struct and initialises its members, so each member is written at the line
// of the constructor. Both forms set every member: `Item(a, b)` by position
// and `Item({owner: a, size: b})` by name, where the language demands all of
// them. A mapping member is the exception: before 0.7 the constructor of a
// struct with a mapping leaves it out, and from 0.7 such a struct cannot be
// constructed at all.
//
// Only the struct had an edge: the system contracts initialise 43 struct
// members in constructors, and `Proposal.status` showed no write where every
// proposal gets its first status.
func (r *scopedResolver) typeCall(pr parse.PendingRef, typeID string) {
	r.edge(pr, types.EdgeUsesType, typeID)
	if r.scope.node[typeID].Type != types.NodeStruct {
		return
	}
	for _, memberID := range r.scope.membersInOrder(typeID) {
		if strings.HasPrefix(r.scope.node[memberID].Signature, mappingKeyword) {
			continue
		}
		r.edge(pr, types.EdgeWritesField, memberID)
	}
}

// byArity is the overload rule of every call: by name (`f(a)`), on a library
// or a base by name (`Lib.f(a)`), through a receiver (`token.f(a)`), and
// through a using-for receiver, whose receiver is the library function's
// first parameter (the caller passes argc+1). It keeps the functions that
// take argc arguments. When none does (the count of a candidate or of the
// call is not known) every candidate is kept, and callEdges marks the edges
// AMBIGUOUS.
//
// Calls through a receiver used to take the first overload:
// `ECRecover.recover(digest, signature)` pointed at the four-parameter
// `recover`.
func (r *scopedResolver) byArity(ids []string, argc int) []string {
	var match []string
	for _, id := range ids {
		if n, known := r.arity[id]; known && n == argc {
			match = append(match, id)
		}
	}
	if len(match) > 0 {
		return match
	}
	return ids
}

// callEdges emits a call edge to each target. One target is the callee and
// gets the confidence of every resolved name (EXTRACTED inside a file,
// INFERRED across files); several are overloads the argument count could not
// tell apart, and each edge is then AMBIGUOUS. The rule is the same for calls
// and invokes: the 15 invokes edges of the system contracts each had one
// target and were all AMBIGUOUS, which said the method was not known.
func (r *scopedResolver) callEdges(pr parse.PendingRef, et types.EdgeType, targets []string, kind string) {
	for _, dst := range targets {
		conf := r.scope.files.confidence(pr.SrcID, dst)
		if len(targets) > 1 {
			conf = types.ConfAmbiguous
		}
		r.edges = append(r.edges, types.Edge{
			Src: pr.SrcID, Dst: dst, Type: et,
			Line: pr.Line, Count: 1, Confidence: conf, DispatchKind: kind,
		})
	}
}

// resolveAccess resolves a value identifier. A state variable, a constant or
// a mapping in scope is read or written. A name that is read and is a type
// (an enum, a struct, a contract, a library, an interface) is named in an
// expression: `Phase.Open` names Phase, `Lib.f(x)` and `Lib.LIMIT` name Lib,
// `abi.encodeCall(IFoo.bar, ...)` names IFoo. That is a uses_type edge, the
// one a cast `IFoo(addr)` and a type in a declaration make. Any other name
// (a function used as a value) is dropped.
//
// The type used to be dropped too: the system contracts name an enum through
// one of its values at 36 places and a library or an interface as a
// qualifier at 22, and only the value or the member had an edge.
func (r *scopedResolver) resolveAccess(pr parse.PendingRef) {
	write := pr.DispatchKind == refWrite
	if ids := r.lookup(pr, pr.TargetQName, isStorageType); len(ids) > 0 {
		r.edge(pr, accessEdgeType(r.scope.node[ids[0]].Type, write), ids[0])
		return
	}
	if write {
		return
	}
	if ids := r.lookup(pr, pr.TargetQName, isTypeDeclaration); len(ids) > 0 {
		r.edge(pr, types.EdgeUsesType, ids[0])
	}
}

// Dispatch kinds of the references edges to a function named for a
// low-level call.
const (
	dispatchKindEncodeCall = "abi_encode_call"
	dispatchKindSelector   = "selector"
)

// resolveFunctionRef emits a references edge to the function the source
// names for a low-level call: the argument of `abi.encodeCall(I.fn, ...)` or
// the function of `I.fn.selector`. The edge says what the source names, not
// that the function is called: the address the call goes to decides that.
// The 10 `abi.encodeCall` sites of the system contracts had no edge. A name
// that no function of the graph has is counted as unresolved.
func (r *scopedResolver) resolveFunctionRef(pr parse.PendingRef, kind string) {
	ids := r.lookup(pr, pr.TargetQName, isType(types.NodeFunction))
	if len(ids) == 0 {
		r.unresolved(pr)
		return
	}
	r.edges = append(r.edges, types.Edge{
		Src: pr.SrcID, Dst: ids[0], Type: types.EdgeReferences,
		Line: pr.Line, Count: 1, DispatchKind: kind,
		Confidence: r.scope.files.confidence(pr.SrcID, ids[0]),
	})
}

// accessEdgeType names the edge of a read or a write of a storage node.
func accessEdgeType(target types.NodeType, write bool) types.EdgeType {
	switch {
	case target == types.NodeMapping && write:
		return types.EdgeWritesMapping
	case target == types.NodeMapping:
		return types.EdgeReadsMapping
	case write:
		return types.EdgeWritesField
	}
	return types.EdgeReadsField
}

// resolveMemberCall resolves `receiver.method(...)` when the receiver has no
// using-for binding. receiverType is the declared type of the receiver, or ""
// when the receiver is not a variable. It reports whether it made an edge.
//
//   - a variable typed as an interface or a contract: invokes the method of
//     that type (15 such sites in the system contracts had no edge);
//   - `super`: calls the nearest base that declares the method;
//   - `this`: calls the contract's own method;
//   - a library, or a base contract, by name: calls its function (6 direct
//     library calls in the system contracts had no edge).
func (r *scopedResolver) resolveMemberCall(pr parse.PendingRef, receiver, receiverType, method string, argc int) bool {
	file := r.scope.files.of(pr.SrcID)
	self := r.scope.linearisation(r.scope.owner[pr.SrcID])
	isFunction := isType(types.NodeFunction)
	if receiverType != "" {
		typeID := r.scope.container(file, receiverType)
		if typeID == "" {
			return false
		}
		targets := r.scope.lookupInherited(r.scope.linearisation(typeID), method, isFunction)
		kind := dispatchKindContractMethod
		if r.scope.node[typeID].Type == types.NodeInterface {
			kind = dispatchKindInterfaceMethod
		}
		r.callEdges(pr, types.EdgeInvokes, r.byArity(targets, argc), kind)
		return len(targets) > 0
	}
	var targets []string
	switch receiver {
	case receiverSuper:
		if len(self) > 1 {
			targets = r.scope.lookupInherited(self[1:], method, isFunction)
		}
	case receiverThis:
		targets = r.scope.lookupInherited(self, method, isFunction)
	default:
		ownerID := r.scope.container(file, receiver)
		if ownerID == "" {
			return false
		}
		targets = r.scope.lookupInherited(r.scope.linearisation(ownerID), method, anyType)
		if len(targets) > 0 {
			switch t := r.scope.node[targets[0]].Type; {
			case t == types.NodeError:
				r.edge(pr, types.EdgeRevertsWith, targets[0])
				return true
			case isTypeDeclaration(t):
				// `Shapes.Box(...)`: a struct constructor or a cast.
				r.typeCall(pr, targets[0])
				return true
			}
		}
		targets = r.scope.filter(targets, isFunction)
	}
	r.callEdges(pr, types.EdgeCalls, r.byArity(targets, argc), "")
	return len(targets) > 0
}

// usingForCall emits the call of a library function bound by a using-for
// directive: among the functions of that name the using file sees, the
// overload that takes the receiver plus argc arguments. When the receiver is
// a state variable or a mapping (receiverIsLocal false), the library function
// takes it as a `storage` parameter and is not view or pure, the call also
// writes the receiver: `_currentBlacklist.add(account)` changes the set, and
// the graph showed it as a read only.
func (r *scopedResolver) usingForCall(pr parse.PendingRef, candidates []string, receiver string, receiverIsLocal bool, argc int) {
	if near := r.scope.files.nearest(candidates, r.scope.files.of(pr.SrcID)); len(near) > 0 {
		candidates = near
	}
	if argc != unknownArity {
		argc++ // the receiver is the first parameter
	}
	targets := r.byArity(candidates, argc)
	r.callEdges(pr, types.EdgeCalls, targets, "")
	if receiverIsLocal || len(targets) != 1 || !mutatesStorageReceiver(r.scope.node[targets[0]]) {
		return
	}
	write := pr
	write.DispatchKind, write.TargetQName = refWrite, receiver
	r.resolveAccess(write)
}

// Words of a function header that decide whether a call can change its
// storage receiver.
const (
	locationStorage = "storage"
	mutabilityView  = "view"
	mutabilityPure  = "pure"
)

// mutatesStorageReceiver reports whether a function takes its first
// parameter by storage reference and may write through it: the parameter is
// declared `storage` and the function is neither view nor pure. It reads the
// declaration header on the node.
func mutatesStorageReceiver(fn *types.Node) bool {
	open := strings.IndexByte(fn.Signature, '(')
	if open < 0 {
		return false
	}
	end := matchingParen(fn.Signature, open)
	if end < 0 {
		return false
	}
	params := splitTopLevel(fn.Signature[open+1 : end])
	if !hasWord(params[0], locationStorage) {
		return false
	}
	rest := fn.Signature[end+1:]
	if returns := strings.Index(rest, keywordReturns); returns >= 0 {
		rest = rest[:returns]
	}
	return !hasWord(rest, mutabilityView) && !hasWord(rest, mutabilityPure)
}

// keywordReturns starts the return list of a function header.
const keywordReturns = "returns"

// hasWord reports whether text holds word as a whitespace-separated word.
func hasWord(text, word string) bool {
	for _, w := range strings.Fields(text) {
		if w == word {
			return true
		}
	}
	return false
}

// linkImports resolves every Import node: it gets an imports edge to the
// file it names when that file is in the graph, and the file index learns
// that the importing file sees it. An import of a file outside the graph
// keeps its node and is counted as unresolved. The walk is over nodes, not
// over pending references, so the imports of unchanged files are known on an
// incremental build; their edges come out again and the graph keeps one.
//
// An import that names symbols (`import { A, B as C } from "./x.sol"`) also
// gets an imports edge, of kind import_symbol, to the declaration of each
// one, as the imported file sees the name: declared there or imported there
// in turn. The system contracts import 27 symbols by name, and only the file
// was linked: nothing told which files import GovBase. The edge carries the
// line of the directive, the only line an Import node keeps. A name the
// imported file does not have is counted as unresolved.
func linkImports(out *parse.ResolvedGraph, scope *scopeIndex) {
	fileNode := map[string]string{}
	known := map[string]bool{}
	for _, n := range out.Nodes {
		if n.Type == types.NodeFile {
			fileNode[n.FilePath] = n.ID
			known[n.FilePath] = true
		}
	}
	// followed maps an Import node to the file it names. The symbols are
	// resolved after every import is recorded: a name can reach the
	// imported file through that file's own imports.
	followed := map[string]string{}
	for _, n := range out.Nodes {
		if n.Type != types.NodeImport {
			continue
		}
		target := resolveImportPath(n.FilePath, n.Name, known)
		if target == "" {
			out.CountUnresolved(parse.UnresolvedOutsideGraph, n.Name, n.FilePath, n.StartLine)
			continue
		}
		followed[n.ID] = target
		scope.files.addImport(n.FilePath, target)
		for alias, name := range importAliasesOf(n.Signature) {
			scope.files.addAlias(n.FilePath, alias, target, name)
		}
		out.Edges = append(out.Edges, types.Edge{
			Src: n.ID, Dst: fileNode[target], Type: types.EdgeImports,
			Line: n.StartLine, Count: 1, Confidence: types.ConfExtracted,
		})
	}
	for _, n := range out.Nodes {
		target, ok := followed[n.ID]
		if !ok {
			continue
		}
		symbols, _ := namedImports(n.Signature)
		for _, symbol := range symbols {
			ids := scope.lookupFileLevel(target, symbol.name, anyType)
			if len(ids) == 0 {
				out.CountUnresolved(parse.UnresolvedNoMatch, symbol.name, n.FilePath, n.StartLine)
				continue
			}
			for _, dst := range ids {
				out.Edges = append(out.Edges, types.Edge{
					Src: n.ID, Dst: dst, Type: types.EdgeImports,
					Line: n.StartLine, Count: 1, DispatchKind: dispatchKindImportSymbol,
					Confidence: scope.files.confidence(n.ID, dst),
				})
			}
		}
	}
}

// dispatchKindImportSymbol marks the imports edge from an Import node to a
// declaration its directive names, as opposed to the one to the file.
const dispatchKindImportSymbol = "import_symbol"

// linkInheritance emits the extends and implements edges of every container
// from the bases its header lists, each resolved as the container's file
// sees the name. The edge is implements when a contract lists an interface
// and extends otherwise (contract to contract, interface to interface); its
// line is the line the declaration starts on. A base no file of the graph
// declares (IERC20 of an OpenZeppelin submodule that is empty) gets no node
// and is counted as unresolved.
//
// The edges come from nodes, not from the pending `is` references, because
// an incremental build first deletes every extends and implements edge of
// the graph and then hands Resolve the unchanged files without their
// references: the inheritance edges between unchanged files were lost on
// every incremental build (12 of 16 on the system contracts).
func linkInheritance(out *parse.ResolvedGraph, scope *scopeIndex) {
	for i := range out.Nodes {
		n := &out.Nodes[i]
		if !isContainerType(n.Type) {
			continue
		}
		for _, base := range basesOf(n) {
			baseID := scope.container(n.FilePath, base)
			if baseID == "" || baseID == n.ID {
				out.CountUnresolved(parse.UnresolvedNoMatch, base, n.FilePath, n.StartLine)
				continue
			}
			edgeType := types.EdgeExtends
			if n.Type == types.NodeContract && scope.node[baseID].Type == types.NodeInterface {
				edgeType = types.EdgeImplements
			}
			out.Edges = append(out.Edges, types.Edge{
				Src: n.ID, Dst: baseID, Type: edgeType,
				Line: n.StartLine, Count: 1, Confidence: scope.files.confidence(n.ID, baseID),
			})
		}
	}
}

// mergeRepeatedEdges folds edges that say the same thing at the same place
// (same type, source, destination, line and dispatch kind) into one edge
// whose Count is the number of repeats. The graph keeps one edge per such
// key, so without the count three `notBlacklisted(..)` invocations on one
// line were one use: the system contracts had 70 has_modifier edges for 83
// invocations. The first edge's order and confidence are kept.
func mergeRepeatedEdges(edges []types.Edge) []types.Edge {
	at := map[string]int{}
	out := edges[:0]
	for _, e := range edges {
		key := edgeIdentityKey(e)
		if i, seen := at[key]; seen {
			out[i].Count += e.Count
			continue
		}
		at[key] = len(out)
		out = append(out, e)
	}
	return out
}

// resolveOverride emits the overrides edges of a function or a modifier:
// from the overriding declaration to each declaration it overrides.
//
//   - `override`: for each direct base, the nearest declaration of the same
//     name in that base's linearisation. The nearest one can be a
//     grandparent's (`Minter is Mid`, `Mid is Base`, `Base` declares it);
//     only direct bases used to be searched, so that edge was missing.
//   - `override(A, B)`: the same search, starting at each named base.
//   - no keyword: Solidity lets a function implement an interface function
//     without `override`. The search is the same, and only a declaration
//     that an interface makes is kept. 20 such pairs in the system contracts
//     had no edge.
//
// A function's overloads are told apart by the number of parameters.
// The edge kind is overrides for all three: implements is the edge from a
// contract to an interface, and its destination must be an Interface.
func (r *scopedResolver) resolveOverride(pr parse.PendingRef) {
	src := r.scope.node[pr.SrcID]
	containerID := r.scope.owner[pr.SrcID]
	if src == nil || containerID == "" {
		return
	}
	accept := isType(src.Type)
	var starts []string
	if pr.DispatchKind == dispatchKindOverrideExplicit {
		base, _, _ := strings.Cut(pr.TargetQName, qnameSep)
		if id := r.scope.container(r.scope.files.of(pr.SrcID), base); id != "" {
			starts = []string{id}
		}
	} else {
		starts = r.parents[containerID]
	}
	seen := map[string]bool{}
	for _, start := range starts {
		targets := r.scope.lookupInherited(r.scope.linearisation(start), src.Name, accept)
		if arity, known := r.arity[pr.SrcID]; known {
			targets = r.byArity(targets, arity)
		}
		for _, dst := range targets {
			if seen[dst] {
				continue
			}
			if pr.DispatchKind == refImplicitOverride && r.ownerType(dst) != types.NodeInterface {
				continue
			}
			seen[dst] = true
			r.edges = append(r.edges, types.Edge{
				Src: pr.SrcID, Dst: dst, Type: types.EdgeOverrides,
				Line: pr.Line, Count: 1, Confidence: r.scope.files.confidence(pr.SrcID, dst),
			})
		}
	}
	// `override` on a function whose overridden declaration is not in the
	// graph (NativeCoinAdapter overrides six IERC20 functions and IERC20 is
	// in an empty submodule): counted, not dropped silently.
	if len(seen) == 0 && pr.DispatchKind != refImplicitOverride {
		r.unresolved(pr)
	}
}

// ownerType returns the node type of the container that defines the member.
func (r *scopedResolver) ownerType(memberID string) types.NodeType {
	if owner := r.scope.node[r.scope.owner[memberID]]; owner != nil {
		return owner.Type
	}
	return ""
}
