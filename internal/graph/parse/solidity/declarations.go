package solidity

import (
	"strings"

	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/0xmhha/knowledge-system/internal/graph/parse"
	"github.com/0xmhha/knowledge-system/pkg/graph/types"
)

// declVisitor walks tree-sitter query matches and emits Pass 1 nodes/edges.
// Mirrors the TypeScript declVisitor structure for consistency.
type declVisitor struct {
	rel     string
	src     []byte
	lang    *sitter.Language
	root    *sitter.Node
	fileID  string
	nodes   []types.Node
	edges   []types.Edge
	pending []parse.PendingRef
	abi     map[string][]ABISig
	// Per-file alias → original-name map
	// from `import {Original as Alias} from "..."` directives. Used
	// during runUsingFor to resolve the library identifier through the
	// alias before pushing onto the binding pipeline. Populated by
	// runImportAliases prior to runUsingFor.
	importAliases map[string]string
	// Per-file set of whole-file namespace
	// aliases from `import "./util.sol" as L` directives — `L` is a
	// namespace prefix, not a library name. runUsingFor consults this
	// set when iterating the type_alias identifier sequence of
	// `using L.SafeMath for ...` so the leading namespace identifier
	// is skipped (otherwise Pass 2 byName[NodeContract] would emit a
	// false-positive EdgeUsesFor against any unrelated contract named L).
	namespaceAliases map[string]bool
	// Namespace alias -> source path text
	// captured from `import "./path.sol" as Alias;`. Used during
	// using-for resolution to disambiguate free-function homonyms:
	// when `using {M.mul as +}` reaches Pass 2 with multiple `mul`
	// candidates across files, the resolver prefers a candidate
	// whose file path matches M's recorded source path.
	namespacePaths map[string]string
	// Named-import alias -> source path
	// captured from `import {orig as alias} from "./path.sol";`.
	// Same lookup role as namespacePaths but for the named-import
	// form. resolveUsingForRef consults both maps when resolving a
	// using-for binding's leading identifier.
	importPaths map[string]string
}

// newDeclVisitor allocates a per-file visitor with a local abi map. The
// caller (ParseFile) merges v.abi into the shared Parser.abi under lock
// after visit() returns — this keeps collectABI race-free under the
// concurrent ParseFile dispatch buildpipe now uses.
func newDeclVisitor(rel string, src []byte, lang *sitter.Language, root *sitter.Node) *declVisitor {
	v := &declVisitor{
		rel:              rel,
		src:              src,
		lang:             lang,
		root:             root,
		abi:              map[string][]ABISig{},
		importAliases:    map[string]string{},
		namespaceAliases: map[string]bool{},
		namespacePaths:   map[string]string{},
		importPaths:      map[string]string{},
	}
	fileQ := fileQNamePrefix + rel
	v.fileID = v.declID(fileQ, 0)
	v.nodes = append(v.nodes, types.Node{
		ID: v.fileID, Type: types.NodeFile, Name: rel, QualifiedName: fileQ,
		FilePath: rel, StartLine: 1, EndLine: 1,
		Language: langSol, Confidence: types.ConfExtracted,
	})
	return v
}

func (v *declVisitor) visit() {
	// Contract & library decls use SubKind-aware emit paths so the
	// graph distinguishes plain / abstract / library variants. See
	// abstract_library.go and docs/graph/design/solidity-inheritance-and-
	// interface-dispatch.md §2.1 / §4.4.
	//
	// interface_declaration gets its own emit path so it lands as
	// NodeInterface (Q1: reuse the existing NodeInterface enum, shared
	// with TS/Go idiom). Inheritance / implementation detection runs
	// after contract / interface decls have been emitted — the inheritance
	// PendingRefs are name-resolved in Pass 2 (resolveInheritance) so
	// they can see nodes from other files.
	v.runContractDecl()
	v.runLibraryDecl()
	v.runInterfaceDecl()
	// Function emit is SubKind-aware (virtual / override / virtual_override
	// / function) and queues the override references in the same walk.
	v.runFunctionDecl()
	v.runDecl(queryModifier, types.NodeModifier)
	// Parameter and local variable types of every modifier, so a receiver
	// used inside a modifier body resolves like one in a function body.
	v.runModifierMeta()
	// Emit NodeFunction (synthetic name
	// "constructor") + parameter / local-var meta for every
	// constructor_definition. Constructors share function_body shape
	// but have no `name` field — runConstructorDecl uses a synthetic
	// identifier and hashes the id off the declaration's StartByte.
	v.runConstructorDecl()
	// Emit NodeFunction (synthetic name
	// "fallback" / "receive") + parameter / local-var meta for every
	// fallback_receive_definition. Tree-sitter lumps both forms into
	// a single kind; the walker disambiguates by reading the leading
	// source token.
	v.runFallbackReceiveDecl()
	v.runDecl(queryEvent, types.NodeEvent)
	// Custom errors: the system contracts declare 91 and the graph had no
	// node for any of them, so a revert had nothing to point at.
	v.runDecl(queryError, types.NodeError)
	v.runDecl(queryStruct, types.NodeStruct)
	v.runDecl(queryEnum, types.NodeEnum)
	v.runImports()
	v.runStateVarDecl()
	v.runEmits()
	v.runHasModifier()
	v.runBodyRefs()
	v.runTypeUses()
	// Detect `modifier m() override {}` and
	// emit EdgeOverrides PendingRefs. Re-uses resolveOverridesRef
	// in Pass 2 (NodeType-agnostic — modifier qnames live in funcByQName
	// alongside functions).
	v.runModifierOverride()
	v.runInheritance()
	// Struct field type metadata. Walks each
	// struct_declaration's struct_body children and emits a side-channel
	// PendingRef per struct_member carrying (structName, fieldName,
	// fieldType). Resolver builds the structFieldTypes index for
	// struct-field receiver dispatch (`obj.field.method()`).
	v.runStructFieldMeta()
	// Interface dispatch: walks body member_expression nodes that
	// fit the `IFoo(addr).bar()` shape and queues EdgeInvokes PendingRefs
	// resolved in Pass 2 (resolveInterfaceDispatch). Confidence is always
	// ConfAmbiguous per §5.0 Q5 — see dispatch.go preamble.
	v.runDispatch()
	// Contract-type cast sibling. Walks the same
	// AST shape but resolves the leading type against byName[NodeContract]
	// instead of NodeInterface. DispatchKind="contract_cast".
	v.runContractCastDispatch()
	// Walk import_directive nodes and fill
	// v.importAliases per-file. Must run before runUsingFor so the
	// using-for walker can resolve aliased library names.
	v.runImportAliases()
	// Using-for: emits EdgeUsesFor PendingRefs from `using LibName for
	// TypeName;` directives nested inside a contract / library / interface
	// body. Q9-1 (b) decision. The same call
	// additionally emits typebind PendingRefs (for the
	// per-contract binding map) and method-call PendingRefs (via the
	// separate runUsingForCalls below).
	v.runUsingFor()
	// ERROR-tolerant recovery for file-level
	// `using LibName for T [global];` directives (Sol 0.8.13+). Vendored
	// grammar v1.2.11 misparses these into ERROR nodes at source_file
	// scope; runFileLevelUsingFor walks the recoverable shape and emits
	// the same PendingRef pair runUsingFor produces, fanned out per
	// container in the file (file-level binding applies to all).
	v.runFileLevelUsingFor()
	// File-level operator-form using directive
	// ERROR recovery. `using {f as +, g as -} for T [global];` at
	// source_file scope parses to an ERROR child whose braced body
	// isn't surfaced as named children; the walker extracts the bound
	// function names and bound type from the raw ERROR text and emits
	// one binding pair per (container, function).
	v.runFileLevelOperatorForm()
	// Operator-form using directive ERROR
	// recovery. `using {f as +} for T;` parses to a misclassified
	// state_variable_declaration; pattern-match the shape and emit
	// the same binding pair runUsingFor produces.
	v.runOperatorFormRecovery()
	// Method-call dispatch detector. Walks every
	// member_expression that fits `<identifier>.<identifier>(...)` and
	// queues a PendingRef that Pass 2 resolves through the binding map.
	// State-variable receivers here (Q9-2 (a)); parameter
	// receivers come from emitParameterMetaPending in overrides.go.
	v.runUsingForCalls()
	// Low-level call dispatch detector. Walks
	// member_expression nodes matching `target.call/delegatecall/
	// staticcall(...)` and emits EdgeInvokes with DispatchKind=
	// "low_level_call" + ConfAmbiguous. Receiver resolution chain
	// re-uses lookupReceiverType (state-var / param / local-var).
	v.runLowLevelCalls()
	// Mark HasLowLevelCall / HasValueTransfer on
	// callables containing any .call / .delegatecall / .staticcall or
	// .send / .transfer invocation, regardless of receiver resolvability.
	// Superset signal complementing runLowLevelCalls's edge emission.
	v.runLowLevelCallMarker()
	// Mark NodeFunction / NodeModifier with
	// HasAssembly=true when the body contains `assembly { ... }`.
	// Post-Pass-1 sweep — mutates v.nodes in place.
	v.runAssemblyMarker()
	// Enumerate security-relevant Yul EVM
	// builtins (delegatecall, sstore, sload, selfdestruct, call,
	// staticcall) per callable. Sorted, deduped slice on
	// Node.YulBuiltins.
	v.runYulBuiltins()
	// Resolve the target address argument of
	// Yul delegatecall / call / staticcall to a Sol scope receiver.
	// Re-uses resolveLowLevelCallRef so Yul and Sol low-level
	// calls produce identical edge shapes.
	v.runYulLowLevelCalls()
	// Mark callables that own a function-typed
	// parameter or local variable with HasFunctionTypedVar=true.
	// Complements the NodeField function-typed marker and surfaces indirect
	// dispatch surfaces for security tooling.
	v.runFunctionTypedVarMarker()
	// Queue `<receiver>.<method>(...)` PendingRefs
	// so Pass 2 can resolve cross-contract function-pointer calls
	// (receiver typed as another contract whose method is a function-
	// typed state-var). Pass 2 marks HasFunctionPointerCall when the
	// chain resolves; the walker itself does not emit any edge.
	v.runCrossContractFnPointerCall()
	// Mark HasExternalCall=true on callables
	// that perform `address(x).call(...)` / `payable(x).call(...)`
	// cast-shape low-level calls. Complements the Pass-2 mark
	// for bare-identifier address-typed receivers.
	v.runExternalCallCastMarker()
	// Mark HasHighLevelSelfCall=true on callables
	// that perform a typed self-call — `this.foo()`,
	// `MyContract(address(this)).foo()`, `IFoo(address(this)).bar()`,
	// or nested cast chains that bottom out at `this`. Parallel to
	// the low-level call surface but independent: the EVM message-
	// call boundary still applies, so the callee can re-enter through
	// typed dispatch just as effectively as through `.call(...)`.
	v.runHighLevelSelfCallMarker()
	// Queue PendingRefs for chained-call
	// shape (`getTarget().call(...)`). Pass 2 looks up the inner
	// function's first return type via funcReturnTypes and marks
	// HasExternalCall on the source callable when the type is
	// address / address payable.
	v.runChainedExternalCall()
	v.collectABI()

	// canonical id (ADR-0001): no import path in Solidity, so the relative file
	// path is the qualifier — <relpath>:<qualified_name>. Functions already set
	// their canonical id inline in runFunctionDecl (with the overload
	// parameter-type signature), so this fills the remaining symbol nodes
	// (contracts/interfaces/libraries/modifiers/events/structs/enums/fields/
	// mappings) uniformly. File and import nodes are not symbols and are skipped.
	for i := range v.nodes {
		n := &v.nodes[i]
		if n.CanonicalID != "" || n.Type == types.NodeFile || n.Type == types.NodeImport {
			continue
		}
		n.CanonicalID = v.rel + ":" + n.QualifiedName
	}
}

// runDecl emits one node per match of a declaration query that captures the
// declaration as @decl and its name identifier as @name. It serves modifiers,
// events, errors, structs and enums; a struct also gets its members and an
// enum its values.
func (v *declVisitor) runDecl(q string, nt types.NodeType) {
	query, qErr := sitter.NewQuery(v.lang, q)
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
		var nameNode, declNode *sitter.Node
		for _, c := range m.Captures {
			n := c.Node
			switch names[c.Index] {
			case "name":
				nameNode = &n
			case "decl":
				declNode = &n
			}
		}
		if nameNode == nil || declNode == nil {
			continue
		}
		ident := nameNode.Utf8Text(v.src)
		qname := v.memberQName(nameNode, ident)
		id := v.emitDecl(declSpec{
			nt: nt, name: ident, qname: qname, decl: declNode,
			idByte: int(nameNode.StartByte()), definer: v.definerOf(declNode),
			signature: declHeader(declNode, v.src),
		})
		switch nt {
		case types.NodeStruct:
			v.emitStructMembers(id, qname, declNode)
		case types.NodeEnum:
			v.emitEnumValues(id, qname, declNode)
		}
	}
}

// runStateVarDecl walks all state_variable_declaration nodes once. Non-mapping
// state vars become Field nodes; declarations whose type_name has key_type +
// value_type fields are emitted as Mapping nodes. Unifying both kinds in one
// pass lets us avoid a separate queryMappingDecl (which the grammar doesn't
// expose as a distinct node type) and keeps mapping detection adjacent to its
// type-introspection logic.
func (v *declVisitor) runStateVarDecl() {
	query, qErr := sitter.NewQuery(v.lang, queryStateVarAll)
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
		for _, c := range m.Captures {
			if names[c.Index] != "decl" {
				continue
			}
			declNode := c.Node
			// A misparsed operator-form using directive (`using {f as +}
			// for T;`) looks like a state variable. The operator-form
			// recovery walker emits its binding; it is no variable and
			// takes no storage.
			if _, _, ok := matchOperatorFormStateVar(&declNode, v.src); ok {
				continue
			}
			v.emitVariable(&declNode)
		}
	}
	// A constant declared at file level (`uint256 constant TOP_C = 7;`) is
	// its own node kind in the grammar. It had no node at all.
	for i := uint(0); i < v.root.NamedChildCount(); i++ {
		if c := v.root.NamedChild(i); c != nil && c.Kind() == kindConstantVariableDecl {
			v.emitVariable(c)
		}
	}
}

// emitVariable emits the node of one state variable, mapping or constant
// declaration.
func (v *declVisitor) emitVariable(decl *sitter.Node) {
	nameNode := decl.ChildByFieldName(fieldName)
	typeNode := decl.ChildByFieldName(fieldType)
	if nameNode == nil {
		return
	}
	name := nameNode.Utf8Text(v.src)
	isMapping := typeNode != nil && typeNameIsMapping(typeNode, v.src)
	isConstant := hasKeywordChild(decl, kindConstantKeyword)
	isImmutable := hasKeywordChild(decl, kindImmutableKeyword)
	visibility := visibilityOf(decl, v.src)
	// A state variable is a Mapping when its type is a mapping, a
	// Constant when it is declared `constant`, and a Field
	// otherwise. A constant used to be a Field with a storage sub
	// kind and a slot; it is compiled into the code and has neither.
	nt := types.NodeField
	subKind := ""
	// The type is stored in Signature: Pass 2 reads a Field's
	// Signature to find the type of a receiver (`set.add(x)`), and a
	// reader sees the declared type. A mapping keeps its whole type
	// expression; it never is a using-for receiver.
	signature := ""
	switch {
	case isMapping:
		nt = types.NodeMapping
		signature = typeText(typeNode, v.src)
	case isConstant:
		nt = types.NodeConstant
		subKind = subKindConstant
		signature = extractTypeNameText(typeNode, v.src)
	default:
		signature = extractTypeNameText(typeNode, v.src)
		// Sub kind precedence: immutable, then the written
		// visibility; no visibility stays empty so a reader can
		// tell "written internal" from "not written".
		switch {
		case isImmutable:
			subKind = subKindImmutable
		case visibility != "":
			subKind = subKindStoragePrefix + visibility
		}
	}
	v.emitDecl(declSpec{
		nt: nt, name: name, qname: v.memberQName(nameNode, name), decl: decl,
		idByte: int(nameNode.StartByte()), definer: v.definerOf(decl),
		subKind: subKind, signature: signature, visibility: visibility,
	})
	// The storage slot is assigned in Pass 2 (layoutStateVariables),
	// which knows the types of every file.
	//
	// A function-typed state variable (`function(uint256) external
	// handler;`) is marked so callers through it can be found. The
	// helper recurses into arrays and mapping values.
	v.lastNode().IsFunctionTyped = typeNameIsFunctionTyped(typeNode)
}

// runEmits queues one reference per `emit Name(...)` statement, from the
// enclosing callable to the event name (`Name`, or `Owner.Name` when the
// statement qualifies it). Pass 2 resolves the name in the scope of the
// contract that emits.
func (v *declVisitor) runEmits() {
	query, qErr := sitter.NewQuery(v.lang, queryEmit)
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
		for _, c := range m.Captures {
			if names[c.Index] != "stmt" {
				continue
			}
			stmt := c.Node
			event := unwrapExpression(stmt.ChildByFieldName(fieldName))
			if event == nil {
				continue
			}
			name := dottedName(event, v.src)
			srcID, ok := v.callableIDAt(&stmt)
			if name == "" || !ok {
				continue
			}
			v.queue(srcID, types.EdgeEmitsEvent, refEmit, name, event)
		}
	}
}

func (v *declVisitor) runHasModifier() {
	query, qErr := sitter.NewQuery(v.lang, queryHasModifier)
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
		var mod string
		var fnQ string
		var fnStart int
		var fnOK bool
		var line int
		var stmtStart uint
		var stmtParent *sitter.Node
		for _, c := range m.Captures {
			if names[c.Index] == "mod" {
				node := c.Node
				mod = node.Utf8Text(v.src)
				fnQ, fnStart, fnOK = nearestFunctionQnameAndStart(&node, v.src)
				line = int(node.StartPosition().Row) + 1
			}
			if names[c.Index] == "stmt" {
				node := c.Node
				stmtStart = node.StartByte()
				stmtParent = node.Parent()
			}
		}
		if mod == "" || !fnOK {
			continue
		}
		// Compute source-order index. Count
		// modifier_invocation siblings whose StartByte precedes the
		// current statement node. The grammar lists modifier_invocation
		// children of function_definition / constructor_definition in
		// source order, so the count equals the 0-indexed position.
		order := 0
		if stmtParent != nil {
			for i := uint(0); i < stmtParent.NamedChildCount(); i++ {
				sibling := stmtParent.NamedChild(i)
				if sibling == nil || sibling.Kind() != "modifier_invocation" {
					continue
				}
				if sibling.StartByte() >= stmtStart {
					break
				}
				order++
			}
		}
		v.pending = append(v.pending, parse.PendingRef{
			SrcID:        v.declID(fnQ, fnStart),
			EdgeType:     types.EdgeHasModifier,
			TargetQName:  mod,
			Line:         line,
			Order:        order,
			DispatchKind: refModifier,
		})
	}
}

// collectABI populates p.abi from the discovered Contract / Function nodes.
// Iteration order matches v.nodes (append order from visit()): the function
// is attributed to the contract node seen last before it. Name match is all
// the cross-language linker needs.
func (v *declVisitor) collectABI() {
	currentContract := ""
	for _, n := range v.nodes {
		switch n.Type {
		case types.NodeContract:
			currentContract = n.Name
		case types.NodeFunction:
			if currentContract == "" {
				continue
			}
			v.abi[currentContract] = append(v.abi[currentContract], ABISig{
				ContractName: currentContract,
				FunctionName: n.Name,
				ParamTypes:   nil, // placeholder — name-match is sufficient.
			})
		}
	}
}

// helpers

// runModifierMeta walks every
// modifier_definition node and emits the parameter / local-var meta PendingRefs
// against its modifier ID. modifier_definition has the same shape as
// function_definition for parameter children and body field, so the
// existing emit helpers work without modification. (Modifiers have no
// return_type, so emitFunctionReturnMetaPending / emit-named-return are
// not applicable here.)
//
// The modifier id comes from callableIDAt, the same rule that keyed the
// Modifier node.
func (v *declVisitor) runModifierMeta() {
	query, qErr := sitter.NewQuery(v.lang, queryModifier)
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
		var nameNode, declNode *sitter.Node
		for _, c := range m.Captures {
			switch names[c.Index] {
			case "name":
				n := c.Node
				nameNode = &n
			case "decl":
				n := c.Node
				declNode = &n
			}
		}
		if nameNode == nil || declNode == nil {
			continue
		}
		id, ok := v.callableIDAt(declNode)
		if !ok {
			continue
		}
		emitParameterMetaPending(v, id, declNode)
		emitLocalVarMetaPending(v, id, declNode)
	}
}

// runImportAliases walks every import_directive in the file and records
// (alias → originalName) pairs into v.importAliases. Tree-sitter-
// solidity v1.2.13 exposes the directive's `import_name` and `alias`
// as multiple-cardinality identifier fields.
//
// Pairing is by source order, not by bucket index. An earlier
// approach bucketed `import_name` and `alias` separately
// and zipped them at the end, which mis-paired heterogeneous
// statements like `import {SafeMath, Address as A}`:
//
//	buckets: importNames=[SafeMath, Address], aliases=[A]
//	zip:     A ↔ SafeMath  (WRONG — A actually aliases Address)
//
// The fix: walk identifier children in source order, keep the most
// recent `import_name` in a one-slot buffer, and pair it with the
// next `alias` when one appears. An `import_name` with no following
// `alias` is bare and needs no alias mapping (the bare name already
// resolves via the global byName index).
//
// Shapes covered:
//   - `import {SafeMath as SM} from "./util.sol"` — single aliased.
//     alias map: SM → SafeMath.
//   - `import {SafeMath as SM, Address as A} from "./util.sol"` —
//     all-aliased multi-entry.
//   - `import {SafeMath, Address as A} from "./util.sol"` — mixed
//     bare + aliased.
//   - `import {SafeMath} from "./util.sol"` — bare-only; no mapping
//     recorded, bare name resolves directly.
//   - `import "./util.sol" as L` — whole-file alias.
//     Detected as alias-without-preceding-import_name, recorded in
//     namespaceAliases.
func (v *declVisitor) runImportAliases() {
	if v.root == nil {
		return
	}
	for i := uint(0); i < uint(v.root.NamedChildCount()); i++ {
		child := v.root.NamedChild(i)
		if child == nil || child.Kind() != "import_directive" {
			continue
		}
		// Capture the directive's source path
		// (string node) for namespace-alias correlation. The walk
		// below records the leading `string` child once and pairs it
		// with any `alias` that turns out to be a namespace alias
		// (whole-file form, no preceding `import_name`).
		sourcePath := ""
		for j := uint(0); j < uint(child.ChildCount()); j++ {
			c := child.Child(j)
			if c != nil && c.Kind() == "string" {
				sourcePath = trimStringQuotes(c.Utf8Text(v.src))
				break
			}
		}
		// Single-pass walk in source order. `lastImportName` holds
		// the most recently seen `import_name` (empty if none, or
		// if already consumed by a paired `alias`). An `alias`
		// encountered with `lastImportName == ""` is a whole-file
		// namespace alias, recorded separately.
		var lastImportName string
		for j := uint(0); j < uint(child.ChildCount()); j++ {
			c := child.Child(j)
			if c == nil || c.Kind() != "identifier" {
				continue
			}
			switch child.FieldNameForChild(uint32(j)) {
			case "import_name":
				lastImportName = c.Utf8Text(v.src)
			case "alias":
				aliasName := c.Utf8Text(v.src)
				if aliasName == "" {
					continue
				}
				if lastImportName != "" {
					v.importAliases[aliasName] = lastImportName
					lastImportName = "" // consumed
					// Record the named-
					// import alias source path so resolveUsingFor
					// Ref can disambiguate cross-file free-
					// function homonyms the same way the
					// namespace-alias path hint does.
					if sourcePath != "" {
						v.importPaths[aliasName] = sourcePath
					}
				} else {
					// Whole-file namespace alias.
					v.namespaceAliases[aliasName] = true
					if sourcePath != "" {
						v.namespacePaths[aliasName] = sourcePath
					}
				}
			}
		}
	}
}

// trimStringQuotes strips a single layer of surrounding single or
// double quotes from a Sol string literal text. Defensive against
// the rare case where tree-sitter exposes the literal with or
// without quote characters depending on grammar revision.
func trimStringQuotes(s string) string {
	if len(s) >= 2 {
		first := s[0]
		last := s[len(s)-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			return s[1 : len(s)-1]
		}
	}
	return s
}

// runConstructorDecl emits one Function node per constructor_definition,
// named "constructor" (the grammar gives the declaration no name field) with
// qualified name `Container.constructor` and sub kind "constructor", then
// queues the parameter and local variable types of its body.
func (v *declVisitor) runConstructorDecl() {
	query, qErr := sitter.NewQuery(v.lang, queryConstructor)
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
		var declNode *sitter.Node
		for _, c := range m.Captures {
			if names[c.Index] == "decl" {
				n := c.Node
				declNode = &n
			}
		}
		if declNode == nil {
			continue
		}
		// A constructor is a Function node named "constructor" with that
		// sub kind: it has parameters, a body and modifiers like any
		// function, and every consumer of callables already reads
		// Function nodes.
		id := v.emitCallable(declNode, nameConstructor)
		// Meta pipeline — constructor has no return_type, so
		// the return-type emits don't apply.
		emitParameterMetaPending(v, id, declNode)
		emitLocalVarMetaPending(v, id, declNode)
	}
}

// runFallbackReceiveDecl emits one
// NodeFunction per fallback_receive_definition with synthetic name
// "fallback" or "receive", qname "<Container>.<name>", SubKind matches
// the name. Then runs the meta-emission pipeline against the
// body.
//
// Tree-sitter quirk: the v1.2.13 grammar uses a single node kind for
// both `fallback() { ... }` and `receive() external payable { ... }`,
// with no field that disambiguates them. The leading keyword in the
// source text is the only reliable signal — we read up to the first
// whitespace / `(` after the node's StartByte to recover the name.
//
// Why NodeFunction (not new types)? Same rationale as runConstructorDecl: keep the
// type-level contract intact for containerIDByFuncID, lookupReceiverType,
// EdgeHasModifier consumers. SubKind carries the role distinction.
func (v *declVisitor) runFallbackReceiveDecl() {
	query, qErr := sitter.NewQuery(v.lang, queryFallbackReceive)
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
		var declNode *sitter.Node
		for _, c := range m.Captures {
			if names[c.Index] == "decl" {
				n := c.Node
				declNode = &n
			}
		}
		if declNode == nil {
			continue
		}
		ident := fallbackOrReceiveKeyword(declNode, v.src)
		if ident == "" {
			continue // defensive — neither keyword found at node start
		}
		id := v.emitCallable(declNode, ident)
		// Meta pipeline — fallback can carry parameters (Sol
		// 0.6+, e.g. `fallback(bytes calldata input) external returns
		// (bytes memory)`); receive cannot by language rule, but the
		// walker is uniform — parameter children will be empty for
		// receive and the helper is a no-op.
		emitParameterMetaPending(v, id, declNode)
		emitLocalVarMetaPending(v, id, declNode)
	}
}

// fallbackOrReceiveKeyword reads the leading source keyword at the node
// (either "fallback" or "receive") and returns it, or "" when neither
// keyword matches (defensive — shouldn't happen for valid Sol code).
func fallbackOrReceiveKeyword(n *sitter.Node, src []byte) string {
	start := int(n.StartByte())
	if start >= len(src) {
		return ""
	}
	// Length 8 covers "fallback"; length 7 covers "receive".
	if start+len("fallback") <= len(src) && string(src[start:start+len("fallback")]) == "fallback" {
		return "fallback"
	}
	if start+len("receive") <= len(src) && string(src[start:start+len("receive")]) == "receive" {
		return "receive"
	}
	return ""
}

// nearestContractName walks the parent chain looking for an enclosing
// contract-like declaration and returns its name (empty if none).
//
// Also recognises `library_declaration` (Sol libraries hold function
// definitions just like contracts do; their methods should be qualified
// as "Library.func" the same way contract methods are). Reserved for
// future extension: `interface_declaration` (interface methods).
func nearestContractName(n *sitter.Node, src []byte) string {
	if decl := nearestContractNode(n); decl != nil {
		if id := decl.ChildByFieldName(fieldName); id != nil {
			return id.Utf8Text(src)
		}
	}
	return ""
}

// callableKey returns the name of a function, modifier, constructor,
// fallback or receive declaration and the byte that keys its node id: the
// start of the name identifier, or the start of the declaration for the
// three kinds the language gives no name. ok is false when decl is not such
// a declaration or its name could not be read. The emitters and every walker
// that finds the callable around a node take the pair from here, so both
// compute the same id.
func callableKey(decl *sitter.Node, src []byte) (name string, idByte int, ok bool) {
	switch decl.Kind() {
	case kindFunctionDef, kindModifierDef:
		ident := decl.ChildByFieldName(fieldName)
		if ident == nil {
			return "", 0, false
		}
		return ident.Utf8Text(src), int(ident.StartByte()), true
	case kindConstructorDef:
		return nameConstructor, int(decl.StartByte()), true
	case kindFallbackReceiveDef:
		// One node kind serves both; the leading keyword tells them apart.
		keyword := fallbackOrReceiveKeyword(decl, src)
		return keyword, int(decl.StartByte()), keyword != ""
	}
	return "", 0, false
}

// isCallableDecl reports whether n is a function, modifier, constructor,
// fallback or receive declaration.
func isCallableDecl(n *sitter.Node) bool {
	switch n.Kind() {
	case kindFunctionDef, kindModifierDef, kindConstructorDef, kindFallbackReceiveDef:
		return true
	}
	return false
}

// nearestFunctionQnameAndStart walks the parent chain to the enclosing
// function, modifier, constructor, fallback or receive and returns its
// qualified name (`Contract.name`, or the bare name at file level) and the
// byte that keys its node id (see callableKey). ok is false outside any
// callable.
func nearestFunctionQnameAndStart(n *sitter.Node, src []byte) (string, int, bool) {
	for cur := n; cur != nil; cur = cur.Parent() {
		if !isCallableDecl(cur) {
			continue
		}
		name, idByte, ok := callableKey(cur, src)
		if !ok {
			return "", 0, false
		}
		return qualify(nearestContractName(cur, src), name), idByte, true
	}
	return "", 0, false
}

// runStructFieldMeta walks every
// struct_declaration in the file and emits a PendingRef per struct_member
// carrying (structName, fieldName, fieldType) so Pass 2 can build a
// structFieldTypes index for struct-field receiver dispatch.
//
// tree-sitter-solidity v1.2.13 shape:
//
//	struct_declaration
//	  name: identifier (structName)
//	  body: struct_body
//	    struct_member (multiple)
//	      name: identifier (fieldName)
//	      type: type_name (fieldType)
//
// Emit decisions:
//   - mapping fields drop (extractTypeNameText returns "" for them) —
//     they have no method-dispatch semantics.
//   - struct definitions outside any contract / library are still
//     captured (file-level struct declarations are common in Sol).
//   - SrcID = v.fileID; the index keys on the encoded TargetQName
//     `<structName>|<fieldName>|<fieldType>`, so multiple PendingRefs
//     per file/struct fan out naturally.
//
// Tree-sitter query reuses queryStruct (the existing NodeStruct emitter)
// to find struct_declaration nodes — saves a per-file query compilation
// cost. struct_body / struct_member walking is hand-rolled because the
// nested shape is awkward to express in a single query alternation.
func (v *declVisitor) runStructFieldMeta() {
	query, qErr := sitter.NewQuery(v.lang, queryStruct)
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
		var nameNode *sitter.Node
		var declNode *sitter.Node
		for _, c := range m.Captures {
			switch names[c.Index] {
			case "name":
				n := c.Node
				nameNode = &n
			case "decl":
				n := c.Node
				declNode = &n
			}
		}
		if nameNode == nil || declNode == nil {
			continue
		}
		structName := nameNode.Utf8Text(v.src)
		body := declNode.ChildByFieldName("body")
		if body == nil {
			continue
		}
		line := int(nameNode.StartPosition().Row) + 1
		for i := uint(0); i < uint(body.NamedChildCount()); i++ {
			memberNode := body.NamedChild(i)
			if memberNode == nil || memberNode.Kind() != "struct_member" {
				continue
			}
			fieldNameNode := memberNode.ChildByFieldName("name")
			fieldTypeNode := memberNode.ChildByFieldName("type")
			if fieldNameNode == nil || fieldTypeNode == nil {
				continue
			}
			fieldName := fieldNameNode.Utf8Text(v.src)
			fieldType := extractTypeNameText(fieldTypeNode, v.src)
			if fieldName == "" || fieldType == "" {
				continue
			}
			v.pending = append(v.pending, parse.PendingRef{
				SrcID:        v.fileID,
				EdgeType:     types.EdgeUsesFor, // unused — routed by DispatchKind
				TargetQName:  structName + "|" + fieldName + "|" + fieldType,
				Line:         line,
				DispatchKind: dispatchKindUsingForStructField,
			})
		}
	}
}

// dispatchKindUsingForStructField carries struct field
// (structName, fieldName, fieldType) bindings as side-channel data.
// Resolver sweeps these into structFieldTypes index — no graph edge.
const dispatchKindUsingForStructField = "using_for_struct_field"

// extractTypeNameText returns the user-written type expression for a
// state_variable_declaration's type_name child. Used by using-for to
// stamp NodeField.Signature with the declared type so Pass 2 can resolve
// `<stateVar>.<method>(...)` against the using-for binding map.
//
// Three shapes covered:
//   - primitive_type / user_defined_type → identifier text (`uint256`,
//     `MyContract`).
//   - mapping → returns "" (mapping receivers don't participate in
//     using-for dispatch; the resolver naturally drops them).
//   - other compound types (array_type, function_type) → raw subtree text
//     normalised by stripping outer whitespace. This is a permissive
//     fallback so future receiver shapes don't crash the parse; the
//     binding map lookup will simply miss when the typeName doesn't match
//     any directive's bound type.
func extractTypeNameText(typeNode *sitter.Node, src []byte) string {
	if typeNode == nil {
		return ""
	}
	if typeNameIsMapping(typeNode, src) {
		return ""
	}
	// First, look for a direct named child (primitive_type /
	// user_defined_type). The grammar wraps these in type_name; the inner
	// named child is the one carrying the identifier we care about.
	for i := uint(0); i < uint(typeNode.NamedChildCount()); i++ {
		c := typeNode.NamedChild(i)
		switch c.Kind() {
		case "primitive_type":
			return strings.TrimSpace(string(src[c.StartByte():c.EndByte()]))
		case "user_defined_type":
			// user_defined_type may be `Foo` or `Ns.Foo` — this takes the
			// trailing identifier (same idiom as TS heritage resolution).
			if id := lastIdentifier(c); id != nil {
				return id.Utf8Text(src)
			}
			return strings.TrimSpace(string(src[c.StartByte():c.EndByte()]))
		}
	}
	// Permissive fallback — raw subtree text. Whitespace-trimmed so
	// downstream string compares stay clean.
	return strings.TrimSpace(string(src[typeNode.StartByte():typeNode.EndByte()]))
}

// lastIdentifier returns the rightmost identifier-like named child under n,
// flattening `Ns.Foo`-style user_defined_type references to their final
// segment.
func lastIdentifier(n *sitter.Node) *sitter.Node {
	if n == nil {
		return nil
	}
	for i := int(n.NamedChildCount()) - 1; i >= 0; i-- {
		c := n.NamedChild(uint(i))
		if c.Kind() == "identifier" {
			return c
		}
	}
	return nil
}

// typeNameIsMapping reports whether a type_name node represents a mapping
// declaration. The grammar models mappings as a hidden _mapping rule inlined
// into type_name, so we detect them by the presence of `key_type` /
// `value_type` fields, falling back to a textual `mapping(` prefix check.
func typeNameIsMapping(n *sitter.Node, src []byte) bool {
	if n == nil {
		return false
	}
	if n.ChildByFieldName("key_type") != nil || n.ChildByFieldName("value_type") != nil {
		return true
	}
	txt := strings.TrimSpace(string(src[n.StartByte():n.EndByte()]))
	return strings.HasPrefix(txt, "mapping")
}
