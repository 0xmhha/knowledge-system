package solidity

// Tree-sitter queries for the JoranHonig/tree-sitter-solidity grammar
// (vendored under ./binding, v1.2.11). Captures use `@name` markers that the
// declVisitor reads positionally.
//
// Notes on this grammar's quirks (verified against node-types.json):
//   - `mapping` is an anonymous keyword, not a top-level node — mapping
//     state-vars are detected separately in runMappingDecl by walking the
//     state_variable_declaration's `type_name` field for key_type/value_type.
//   - `emit_statement` exposes its event reference under field `name` whose
//     type is `expression`, so we descend through the expression to grab the
//     leading identifier.
//   - `modifier_invocation` has no fields; the modifier name is the first
//     `identifier` named child.
const (
	queryContract  = `(contract_declaration name: (identifier) @name) @decl`
	queryLibrary   = `(library_declaration name: (identifier) @name) @decl`
	queryInterface = `(interface_declaration name: (identifier) @name) @decl`
	queryFunction  = `(function_definition name: (identifier) @name) @decl`
	queryModifier  = `(modifier_definition name: (identifier) @name) @decl`
	// constructor_definition has no `name`
	// field — only `body` (function_body) + direct `parameter` children
	// + optional modifier_invocation. We synthesise "constructor" as the
	// identifier and pin id-hashing to the declaration node's StartByte.
	queryConstructor = `(constructor_definition) @decl`
	// fallback_receive_definition is a single
	// node kind for both `fallback()` and `receive()` — tree-sitter does
	// not disambiguate them via fields. The walker reads the leading
	// source token at the declaration's StartByte to pick the synthetic
	// identifier ("fallback" or "receive").
	queryFallbackReceive = `(fallback_receive_definition) @decl`
	queryEvent           = `(event_definition name: (identifier) @name) @decl`
	queryStruct          = `(struct_declaration name: (identifier) @name) @decl`
	queryEnum            = `(enum_declaration name: (identifier) @name) @decl`
	// queryStateVar was replaced by queryStateVarAll + runStateVarDecl
	// (mapping detection unified into one visitor pass).
	// queryStateVar = `(state_variable_declaration name: (identifier) @name) @decl`
	queryStateVarAll = `(state_variable_declaration) @decl`
	queryEmit        = `(emit_statement name: (expression (identifier) @event)) @stmt`
	queryHasModifier = `(modifier_invocation (identifier) @mod) @stmt`
	// Sol inheritance — the `is`-clause exposes each parent as its own
	// inheritance_specifier sibling under contract_declaration /
	// interface_declaration (verified via AST dump 2026-05-11). Each
	// specifier wraps a user_defined_type whose first identifier is the
	// parent name (qualified names like `pkg.Type` are nested deeper but
	// the leading identifier still drives resolution).
	queryInheritance = `[
		(contract_declaration
			name: (identifier) @child
			(inheritance_specifier
				(user_defined_type (identifier) @parent)))
		(interface_declaration
			name: (identifier) @child
			(inheritance_specifier
				(user_defined_type (identifier) @parent)))
	]`
	// Sol using For — `using LibName for TypeName;` directives inside
	// a contract / library / interface body.
	//
	// tree-sitter-solidity v1.2.11 grammar (vendored under ./binding):
	//   using_directive
	//     - `type_alias`     (legacy form: `using SafeMath for uint256`)
	//        └── identifier  ← library name
	//     - source field     (type_name OR any_source_type for `for *`)
	//
	// The legacy form's library identifier is captured via type_alias,
	// plus an additional `@type` capture on the source field so the bound
	// type can drive method-call dispatch resolution.
	//
	// Note: there is no `using_alias` node type for the 0.8.13+ free-
	// function form child. Empirical AST dump 2026-05-17 against the
	// vendored grammar shows NO such node — `using_alias` is not a valid
	// node type, and any query referencing it fails to compile. The
	// operator-form variant `using {f as +} for T;` (Sol 0.8.19+) is
	// further grammar-blocked: the parser misinterprets the alias body
	// as a state_variable_declaration sequence wrapped in ERROR nodes.
	// Conclusion: operator-form is category A (grammar reject), NOT
	// category B (query gap) — the gap is upstream of the query.
	//
	// The incidental capture of `using {Math.add, Math.sub} for T;`
	// works because the grammar interprets the qualified `Math.add` as
	// a type_alias-wrapped identifier (not a using_alias node). This
	// is also why the query's existing `type_alias (identifier)`
	// pattern hits it.
	//
	// contract_body is the `body:` field of contract_declaration /
	// library_declaration / interface_declaration; using_directive nests
	// inside it.
	//
	// The `@stmt` capture is required because the matched node — the
	// using_directive itself — needs to be addressable for binding-map
	// PendingRef emission; without a separate capture, the runUsingFor
	// loop can only see the leaf identifiers.
	queryUsingFor = `[
		(contract_declaration
			name: (identifier) @container
			body: (contract_body
				(using_directive
					(type_alias (identifier) @lib)
					source: (_) @type) @stmt))
		(library_declaration
			name: (identifier) @container
			body: (contract_body
				(using_directive
					(type_alias (identifier) @lib)
					source: (_) @type) @stmt))
		(interface_declaration
			name: (identifier) @container
			body: (contract_body
				(using_directive
					(type_alias (identifier) @lib)
					source: (_) @type) @stmt))
	]`
)

// Grammar limitation note
//
// File-level using directives (Solidity 0.8.13+, `using LibName for T;`
// at module scope outside any contract body) cannot be matched by the
// queries above because of tree-sitter-solidity v1.2.13: the
// grammar's node-types.json lists `using_directive` as a valid
// source_file child, but the parser actually wraps such directives in
// ERROR nodes — meaning the lexer/grammar rules don't recognise this
// 0.8.13+ syntax yet (verified via AST dump 2026-05-12). Until the
// vendored grammar supports the new directive position,
// runFileLevelUsingFor recovers these directives from the ERROR nodes.
