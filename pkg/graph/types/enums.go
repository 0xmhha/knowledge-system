package types

// NodeType enumerates the 35 node kinds (spec §5.1; v0.2 schema 1.1 added
// Mutex; schema 1.3 appended Endpoint + MessageType for distributed
// topology; schema 1.4 appended Commit for temporal git history nodes;
// schema 1.8 appended Hunk for the temporal Hunk-graph — one block of
// changed lines per (commit, file); schema 1.10 appended AwaitPoint for
// TS async/await suspension points).
type NodeType string

const (
	NodePackage       NodeType = "Package"
	NodeFile          NodeType = "File"
	NodeStruct        NodeType = "Struct"
	NodeInterface     NodeType = "Interface"
	NodeClass         NodeType = "Class"
	NodeTypeAlias     NodeType = "TypeAlias"
	NodeEnum          NodeType = "Enum"
	NodeContract      NodeType = "Contract"
	NodeMapping       NodeType = "Mapping"
	NodeEvent         NodeType = "Event"
	NodeFunction      NodeType = "Function"
	NodeMethod        NodeType = "Method"
	NodeModifier      NodeType = "Modifier"
	NodeConstructor   NodeType = "Constructor"
	NodeConstant      NodeType = "Constant"
	NodeVariable      NodeType = "Variable"
	NodeField         NodeType = "Field"
	NodeParameter     NodeType = "Parameter"
	NodeLocalVariable NodeType = "LocalVariable"
	NodeImport        NodeType = "Import"
	NodeExport        NodeType = "Export"
	NodeDecorator     NodeType = "Decorator"
	NodeGoroutine     NodeType = "Goroutine"
	NodeChannel       NodeType = "Channel"
	// NodeMutex: schema 1.1 slot — emitted by the Go concurrency pass for
	// sync.Mutex / sync.RWMutex fields, package-level vars, and
	// function-local vars. See
	// internal/parse/golang/concurrency.go:emitMutexNode. Cross-function
	// lock chain propagation (caller holds X, callee touches field) is a
	// separate opt-in pass — see
	// docs/graph/design/go-cross-function-lock-propagation.md. Kept adjacent to the other concurrency nodes
	// (Goroutine/Channel) for grouping.
	NodeMutex      NodeType = "Mutex"
	NodeIfStmt     NodeType = "IfStmt"
	NodeLoopStmt   NodeType = "LoopStmt"
	NodeCallSite   NodeType = "CallSite"
	NodeReturnStmt NodeType = "ReturnStmt"
	NodeSwitchStmt NodeType = "SwitchStmt"
	// Schema 1.3 (distributed topology): handler/route topology entries.
	// NodeEndpoint  : an HTTP/RPC route literal. Qname follows protocol-
	//                  specific format (schema 1.9 §6.2):
	//                    - http  : `http:METHOD /route`   (METHOD=`*` for any)
	//                    - rpc   : `rpc:Service.Method`
	//                    - grpc  : `grpc:pkg.Service.Method`
	//                    - ws    : `ws:/route[#msg]`          (later)
	// NodeMessageType: a request/response message type a handler dispatches on
	//                  (e.g. `pkg.MyRequest`). Appended at the end so existing
	//                  positional indices stay stable (see TestAllNodeTypes_Stable).
	NodeEndpoint    NodeType = "Endpoint"
	NodeMessageType NodeType = "MessageType"
	// Schema 1.4 (temporal): a git commit that touched one or
	// more source files. Name = first 12 chars of SHA, QualifiedName =
	// `commit:<full-sha>`. SubKind = "git". StartLine/EndLine = 1 (commits
	// have no source range). Appended at the end so existing positional
	// indices stay stable (TestAllNodeTypes_Stable).
	NodeCommit NodeType = "Commit"
	// Schema 1.8 (temporal Hunk-graph): one contiguous
	// block of changed lines in one file in one commit, as defined by
	// unified-diff `@@` headers. Name = "<sha12>:<file>:<idx>",
	// QualifiedName = `hunk:<full-sha>:<file>:<idx>` (idx = 0-based per-commit
	// hunk position so multiple hunks per commit get distinct IDs via MakeID).
	// SubKind = "git". StartLine/EndLine = the hunk's @@ header new-file
	// line range; StartByte = 0 / EndByte = 1 sentinels (the patch text
	// lives in blobs.source, gzip-compressed; see hunk-graph.md §2.2-2.3).
	// Confidence semantics (hunk-graph.md §11.3):
	//   - EXTRACTED: HEAD-reachable hunks (the only kind currently collected).
	//   - AMBIGUOUS: reserved for unreachable hunks that a future PR will
	//                collect via reflog/fsck. The EvidencePack assembler
	//                MUST filter to confidence='EXTRACTED' so the LLM never
	//                sees code paths that were rolled back by force-push.
	// Appended at the end so existing positional indices stay stable
	// (TestAllNodeTypes_Stable).
	NodeHunk NodeType = "Hunk"
	// Schema 1.10 (TS async/await): statement-level node emitted at each
	// `await` expression in TypeScript source. Marks an async suspension
	// point so graph queries can answer "where does control yield, and to
	// which AsyncCallSite?". See
	// docs/graph/design/ts-async-await-and-interface.md §2.1 + §3.2 and
	// docs/graph/archive/DISPATCH-WITHIN-LANG-SEMANTICS.md §2. Appended at the
	// end so existing positional indices stay stable
	// (TestAllNodeTypes_Stable).
	NodeAwaitPoint NodeType = "AwaitPoint"

	// NodePolicy (schema 1.14) — governance/protocol policy
	// metadata loaded from an external YAML rather than the parsed
	// source tree. Surfaces "why does this code exist?" — fork blocks,
	// gas schedules, consensus parameters, security policies — so an
	// LLM can answer policy-driven questions ("which fork activated
	// this?", "what gas schedule governs this?") without first
	// searching the code itself. See pkg/policy for the YAML loader. FilePath /
	// StartLine cite the YAML entry's source location so citations
	// stay grounded.
	NodePolicy NodeType = "Policy"

	// NodeSecurityPattern (schema 1.15) — security risk
	// pattern annotations loaded from an external YAML. Captures
	// "this symbol is reachable in a reentrancy / access-control /
	// Byzantine / overflow scenario" so an LLM modifying the code
	// can see the risk surface at retrieval time instead of having
	// to run a separate static analyser. SubKind carries the
	// category (reentrancy / access-control / …), the attrs JSON
	// blob carries severity (info / low / medium / high / critical)
	// and an optional remediation hint. See pkg/security for the
	// YAML loader.
	NodeSecurityPattern NodeType = "SecurityPattern"

	// NodeExternal (schema 1.36): a function or method the build calls but
	// does not contain (standard library, a module dependency, a package
	// outside the build scope). One node per symbol, shared by every caller,
	// so "what calls time.AfterFunc" is one lookup. External carries the
	// package (import path, name, module, version); SubKind is func, method
	// or interface_method; Signature is the go/types signature. FilePath is
	// ExternalFilePath, not a file of the repository.
	NodeExternal NodeType = "External"

	// NodeError (schema 1.38): a Solidity custom error declaration
	// (`error NotOwner(address who);`), at file level or inside a contract,
	// interface or library. It is none of the other kinds: it is not emitted
	// like an event and holds no value like a constant. reverts_with edges
	// end at it. Signature is the declaration header.
	NodeError NodeType = "Error"

	// Schema 1.45 (control transfer and code extents), statement nodes of
	// the Go frontend, each with a contains edge from its function and an
	// encloses edge from the statement around it:
	//   BranchStmt  a break, continue, goto or fallthrough (sub kind the
	//               keyword); jumps_to names where it goes.
	//   CaseClause  one clause of a switch, type switch or select, spanning
	//               from its case (or default) keyword to the end of its
	//               last statement (sub kind case or default); its switch
	//               encloses it with kind clause.
	//   DeferStmt   a defer statement, spanning the whole statement.
	//   FuncLit     a function literal, spanning func to its closing brace.
	NodeBranchStmt NodeType = "BranchStmt"
	NodeCaseClause NodeType = "CaseClause"
	NodeDeferStmt  NodeType = "DeferStmt"
	NodeFuncLit    NodeType = "FuncLit"
)

// Sub kinds of the schema 1.45 statement nodes.
const (
	SubKindBreak       = "break"
	SubKindContinue    = "continue"
	SubKindGoto        = "goto"
	SubKindFallthrough = "fallthrough"
	SubKindCase        = "case"
	SubKindDefault     = "default"
)

// Sub kinds of a Go TypeAlias node. The node type covers every named type
// that is neither a struct nor an interface; the sub kind tells the two Go
// declarations apart.
const (
	// SubKindAlias is `type A = B`: another name for the type B.
	SubKindAlias = "alias"
	// SubKindDefinedType is `type A B`: a new type with B's underlying type
	// and its own method set.
	SubKindDefinedType = "defined"
)

// ExternalFilePath is the file_path of every External node. It names no file
// of the repository, so per-file deletes and reloads never touch these nodes.
const ExternalFilePath = "(external)"

// AllNodeTypes returns all 39 node types in a stable order.
// NOTE: identifier names are stable; positional indices are load-bearing
// only for tests that snapshot the full slice (TestAllNodeTypes_Stable).
// NodeMutex was inserted at index 24 to keep the concurrency family
// (Goroutine/Channel/Mutex) contiguous, which shifted the statement
// nodes (NodeIfStmt..NodeSwitchStmt) from indices 24-28 to 25-29 — no
// callers key on those indices, so the shift is safe; future additions
// should prefer append over insert when no grouping reason argues
// otherwise. NodeEndpoint + NodeMessageType (schema 1.3) are appended
// (indices 30-31) — distributed topology is a distinct family from
// concurrency / statements, no grouping argument applied. NodeCommit
// (schema 1.4) is appended at index 32 — temporal/git history is a
// distinct family from everything above. NodeHunk (schema 1.8, Hunk-graph)
// is appended at index 33 — same temporal family as NodeCommit but
// finer-grained (one block of changed lines, not a whole commit).
// NodeAwaitPoint (schema 1.10) is appended at index 34 — TS async
// suspension family.
// NodePolicy (schema 1.14) is appended at index 35 — domain
// governance/protocol policy metadata loaded from an external YAML.
// NodeSecurityPattern (schema 1.15) is appended at index 36 —
// security risk pattern annotations loaded from an external YAML.
// NodeExternal (schema 1.36) is appended at index 37 — symbols the build
// calls but does not contain.
// NodeError (schema 1.38) is appended at index 38 — Solidity custom errors.
func AllNodeTypes() []NodeType {
	return []NodeType{
		NodePackage, NodeFile, NodeStruct, NodeInterface, NodeClass,
		NodeTypeAlias, NodeEnum, NodeContract, NodeMapping, NodeEvent,
		NodeFunction, NodeMethod, NodeModifier, NodeConstructor,
		NodeConstant, NodeVariable, NodeField, NodeParameter, NodeLocalVariable,
		NodeImport, NodeExport, NodeDecorator,
		NodeGoroutine, NodeChannel, NodeMutex,
		NodeIfStmt, NodeLoopStmt, NodeCallSite, NodeReturnStmt, NodeSwitchStmt,
		NodeEndpoint, NodeMessageType,
		NodeCommit,
		NodeHunk,
		NodeAwaitPoint,
		NodePolicy,
		NodeSecurityPattern,
		NodeExternal,
		NodeError,
		NodeBranchStmt, NodeCaseClause, NodeDeferStmt, NodeFuncLit,
	}
}

// IsSymbol reports whether t is a "symbol-level" node — a code unit
// (function, type, field, package, file, endpoint, …) that a coding
// agent's keyword search would normally want to surface. Returns
// false for:
//
//   - Statement nodes (IsStatement) — control-flow markers whose qname
//     carries the enclosing function's prefix, which makes them
//     false-positive FTS hits on a keyword that names the enclosing
//     symbol.
//   - Meta nodes (Commit, Hunk) — git-history rows surfaced via
//     evidence_for_intent, not the symbol search path.
//   - Path-only nodes (Import, Export) — their FTS columns carry
//     module paths, so a query like "Vault" matches every import of
//     contracts/Vault even when the consumer wanted the class itself.
//   - External nodes — code outside the build; a search for repository
//     symbols should not return time.Now.
//   - Local names (LocalVariable, schema 1.38) — a name made by := or a
//     range clause inside a function body. There are tens of thousands
//     (err, ok, i), each known only inside its function; a keyword search
//     for a symbol should not return them. A caller that wants them
//     passes NodeKinds.
//
// search_text (pkg/mcphandlers + internal/persist.SearchFTS) uses
// IsSymbol as the default whitelist when SearchFTSOptions.NodeKinds
// is nil. Callers that want the full surface (statement nodes
// included) pass an explicit NodeKinds slice — typically
// types.AllNodeTypes().
func (t NodeType) IsSymbol() bool {
	if t.IsStatement() {
		return false
	}
	switch t {
	case NodeCommit, NodeHunk,
		NodeImport, NodeExport,
		NodeExternal,
		NodeLocalVariable:
		return false
	default:
		return true
	}
}

// IsStatement reports whether t is a statement-level node: a piece of a
// function body (IfStmt, LoopStmt, CallSite, ReturnStmt, SwitchStmt,
// BranchStmt, CaseClause, DeferStmt, FuncLit, AwaitPoint) whose qname
// carries the enclosing function's prefix. It is the one rule the tools
// and the graph passes use to tell such a node from a declaration an agent
// asks about (IsSymbol); a new statement kind is added here only.
func (t NodeType) IsStatement() bool {
	switch t {
	case NodeIfStmt, NodeLoopStmt, NodeCallSite, NodeReturnStmt, NodeSwitchStmt,
		NodeBranchStmt, NodeCaseClause, NodeDeferStmt, NodeFuncLit,
		NodeAwaitPoint:
		return true
	}
	return false
}

// SymbolNodeTypes returns the symbol-level subset of AllNodeTypes —
// the default whitelist for search_text when no explicit NodeKinds
// is supplied. Stable ordering tracks AllNodeTypes (callers must not
// key on positional indices; see TestAllNodeTypes_Stable for the
// invariant).
func SymbolNodeTypes() []NodeType {
	all := AllNodeTypes()
	out := make([]NodeType, 0, len(all))
	for _, t := range all {
		if t.IsSymbol() {
			out = append(out, t)
		}
	}
	return out
}

// EdgeType enumerates the 41 edge kinds (spec §5.2; v0.2 schema 1.1 added 3
// lock edges; schema 1.3 appended listens_on / handles_message / rpc_calls
// for distributed topology; schema 1.4 appended changed_in / blame for
// temporal git history; schema 1.6 appended timeout_path /
// cancellation_path for Go context.With* propagation; schema 1.8 appended
// has_hunk / adjacent for the Hunk-graph, then `modifies` for hunk/AST
// overlap; schema 1.9 appended `http_calls` — caller Function → Endpoint
// (HTTP client call sites) — and `grpc_listens_on` + `grpc_calls` — Go
// gRPC server/client detection; schema 1.10 appended `awaits` (TS async
// suspension flow) + `overrides` (Solidity virtual/override semantics)).
type EdgeType string

const (
	EdgeContains      EdgeType = "contains"
	EdgeDefines       EdgeType = "defines"
	EdgeCalls         EdgeType = "calls"
	EdgeInvokes       EdgeType = "invokes"
	EdgeUsesType      EdgeType = "uses_type"
	EdgeInstantiates  EdgeType = "instantiates"
	EdgeReferences    EdgeType = "references"
	EdgeReadsField    EdgeType = "reads_field"
	EdgeWritesField   EdgeType = "writes_field"
	EdgeImports       EdgeType = "imports"
	EdgeExports       EdgeType = "exports"
	EdgeImplements    EdgeType = "implements"
	EdgeExtends       EdgeType = "extends"
	EdgeHasModifier   EdgeType = "has_modifier"
	EdgeEmitsEvent    EdgeType = "emits_event"
	EdgeReadsMapping  EdgeType = "reads_mapping"
	EdgeWritesMapping EdgeType = "writes_mapping"
	EdgeHasDecorator  EdgeType = "has_decorator"
	EdgeSpawns        EdgeType = "spawns"
	EdgeSendsTo       EdgeType = "sends_to"
	EdgeRecvsFrom     EdgeType = "recvs_from"
	EdgeBindsTo       EdgeType = "binds_to"
	// Schema 1.1 (concurrency lock semantics) — emitted by the Go
	// concurrency pass:
	//   acquires_lock / releases_lock: from
	//     internal/graph/parse/golang/lock_edges.go — a Lock/Unlock/RLock/
	//     RUnlock call whose receiver go/types resolves to a declaration
	//     with a Mutex node, in whichever file it is declared (EXTRACTED).
	//     A user-defined Lock() on a non-mutex type matches nothing (spec
	//     §2 R2.1).
	//   accessed_under_lock: from the same pass — intra-function
	//     lexical heuristic (any field access in a body that holds any lock
	//     gets one edge per (field, mutex) pair). Cross-function chain
	//     propagation is a separate pass behind the --lock-propagation
	//     opt-in flag — see
	//     docs/graph/design/go-cross-function-lock-propagation.md.
	// Appended (not interleaved) so existing edge-type hash positions /
	// test snapshots stay stable.
	EdgeAcquiresLock      EdgeType = "acquires_lock"
	EdgeReleasesLock      EdgeType = "releases_lock"
	EdgeAccessedUnderLock EdgeType = "accessed_under_lock"
	// Schema 1.3 (distributed topology): handler/RPC topology edges.
	// listens_on:      handler function/method → endpoint route
	// handles_message: handler function/method → message type it dispatches on
	// rpc_calls:       caller function → server method (or message-type placeholder)
	// Appended (not interleaved) so existing edge-type hash positions / test
	// snapshots stay stable.
	EdgeListensOn      EdgeType = "listens_on"
	EdgeHandlesMessage EdgeType = "handles_message"
	EdgeRPCCalls       EdgeType = "rpc_calls"
	// Schema 1.4 (temporal): git-history derived edges, recorded for File
	// nodes and declarations (functions, methods, types, contracts and the
	// other kinds in buildpipe's lineHistoryNodeTypes).
	// changed_in: node → a commit that changed the node's lines (the
	//             commits git log -L lists for the node's range, renames
	//             followed): a File node's edges are the commits that changed
	//             the file, a declaration's the commits that changed its own
	//             lines, located at each commit. Every commit of the walked
	//             history unless the build caps it (Options.TemporalDepth).
	// blame:      node → a commit that last changed some of its lines today
	//             (git blame), Count = how many lines.
	// Appended (not interleaved) so existing edge-type hash positions /
	// test snapshots stay stable.
	EdgeChangedIn EdgeType = "changed_in"
	EdgeBlame     EdgeType = "blame"
	// Schema 1.6 (control-flow context propagation): Go
	// context.With* creation sites. Self-loop edges anchored on the
	// enclosing Function/Method:
	//   timeout_path:      context.WithTimeout / context.WithDeadline call
	//                      site. Deadline is treated as a timeout variant —
	//                      both express "this work is bounded by a wall-clock
	//                      budget" and consumers (graph queries / viewer)
	//                      benefit from collapsing them.
	//   cancellation_path: context.WithCancel / context.WithCancelCause call
	//                      site (Go 1.20+ for the latter). Distinct from
	//                      timeout_path because cancellation is event-driven,
	//                      not deadline-driven.
	// TODO: retry_path is intentionally NOT emitted yet — the heuristic
	// (loops around RPC calls? error-handling branches?) is too noisy to
	// ship without false-positive cleanup. Reserved for a follow-up once
	// we have a typed retry pattern (e.g. detecting backoff libraries
	// like cenkalti/backoff or built-in `for { ...err... }` loops with
	// rpc_calls inside).
	// Appended (not interleaved) so existing edge-type hash positions /
	// test snapshots stay stable.
	EdgeTimeoutPath      EdgeType = "timeout_path"
	EdgeCancellationPath EdgeType = "cancellation_path"
	// Schema 1.8 (temporal Hunk-graph):
	//   has_hunk: Commit → Hunk. One per Hunk; "this commit produced this
	//             block of changed lines". Confidence mirrors the Hunk's
	//             own (EXTRACTED for HEAD-reachable, AMBIGUOUS for
	//             unreachable hunks added by a future reflog-collection PR).
	//   adjacent: Hunk → Hunk between same-commit, same-file hunks
	//             ordered by their @@ header start line. Provides a
	//             deterministic "next-in-this-file" traversal so the
	//             EvidencePack assembler can stitch a multi-hunk view
	//             of a commit's edits without a separate ORDER BY query.
	//             Emitted only between hunks within one (commit, file)
	//             pair — never across commits or files. Out-of-scope edges:
	//             modifies (Hunk → CodeNode interval overlap) is a separate edge;
	//             same_logical_change clustering across commits is out of
	//             scope (see hunk-graph.md §11.5 decision).
	// Appended (not interleaved) so existing edge-type hash positions /
	// test snapshots stay stable.
	EdgeHasHunk  EdgeType = "has_hunk"
	EdgeAdjacent EdgeType = "adjacent"
	// Schema 1.8 (Hunk-graph AST overlap):
	//   modifies: Hunk → declaration when a line the hunk added, replaced
	//             or deleted (context lines excluded) lies in the
	//             declaration's lines as they were in the hunk's commit — the
	//             changed_in rule applied to one hunk.
	// Appended at the end so existing hash positions stay stable.
	EdgeModifies EdgeType = "modifies"
	// Schema 1.9 (cross-language interop — HTTP client calls):
	//   http_calls: caller Function/Method → Endpoint when the function
	//               invokes an HTTP client (TS: fetch / axios / useSWR /
	//               useQuery; Go: http.Get / http.Post / http.NewRequest /
	//               (*http.Client).Get/Post/Do).
	//
	// Target resolution uses 2-stage cascade (schema-1.9-spec §6.9):
	//   1. Specific-verb lookup: `http:METHOD /path` exact match.
	//   2. Wildcard fallback: `http:* /path` exact match.
	// On miss the matcher synthesises an AMBIGUOUS placeholder Endpoint
	// (schema-1.9-spec §6.3 (B)) so the call site stays surfaceable for
	// monorepo external-API audits. Path matching is EXACT (schema-1.9-spec
	// §3.3 decision: exact-match is chosen over suffix-match because
	// false-positives across distinct services with overlapping path
	// suffixes are far worse than the false-negatives exact-match incurs
	// in well-curated monorepos).
	//
	// Appended at the end so existing edge-type hash positions / test
	// snapshots stay stable.
	EdgeHTTPCalls EdgeType = "http_calls"
	// Schema 1.9 (cross-language interop — Go gRPC server/client
	// detection):
	//   grpc_listens_on: server impl Method → Endpoint when the file calls
	//                    `pb.RegisterXXXServer(s, &impl{})`. Each method on
	//                    the impl receiver type whose name matches an rpc
	//                    method on the generated XServer interface emits
	//                    one edge to a `grpc:Service.Method` Endpoint
	//                    (language="go", sub_kind="grpc").
	//   grpc_calls:      caller Function/Method → Endpoint when the body
	//                    calls `<stub>.RpcMethod(ctx, req)` where `stub`
	//                    was assigned from `pb.NewXXXClient(conn)`. Like
	//                    http_calls, on miss the matcher synthesises an
	//                    AMBIGUOUS placeholder Endpoint (language="external")
	//                    so external-API call sites stay surfaceable.
	//
	// Confidence split (schema-1.9-spec §6.5 (C) Both with split confidence):
	//   - typesInfo available + method matches generated XServer interface
	//     → EXTRACTED.
	//   - AST-only (no typesInfo) suffix-matcher on RegisterXXXServer →
	//     INFERRED.
	//   - Miss / unresolved stub var type → AMBIGUOUS placeholder.
	//
	// Appended at the end so existing edge-type hash positions / test
	// snapshots stay stable.
	EdgeGRPCListensOn EdgeType = "grpc_listens_on"
	EdgeGRPCCalls     EdgeType = "grpc_calls"
	// Schema 1.10 (within-language semantics):
	//   awaits:         Function/Method → AwaitPoint, and AwaitPoint →
	//                   AsyncCallSite. Marks async suspension flow in
	//                   TypeScript source so graph queries can answer
	//                   "where does control yield, and what call awaits
	//                   here?". See docs/graph/design/ts-async-await-and-interface.md
	//                   §2.1 + §3.2.
	//   overrides:      Method → Method between a child contract's method
	//                   that overrides a parent's `virtual` method in
	//                   Solidity. Direction = child → parent (Q4 decision
	//                   in solidity-inheritance spec §5.0). Distinct from
	//                   `implements` (interface satisfaction) because
	//                   `overrides` is concrete-to-concrete virtual
	//                   dispatch resolution. See
	//                   docs/graph/design/solidity-inheritance-and-interface-dispatch.md
	//                   §2.1 + §3.3.
	//   using_for:      Contract → Library binding emitted from
	//                   `using SafeMath for uint256` directives. First-class
	//                   so binding queries can run as a single edge-type
	//                   filter (matches the extends/implements/overrides/
	//                   has_modifier idiom — every other Sol-specific
	//                   semantic relation is already first-class). Direction =
	//                   contract → library; the call site of `balance.add(...)`
	//                   still produces a separate EdgeCalls into the
	//                   library function. See solidity-inheritance-and-interface-
	//                   dispatch.md §4.6.
	//
	// Appended at the end so existing edge-type hash positions / test
	// snapshots stay stable.
	EdgeAwaits    EdgeType = "awaits"
	EdgeOverrides EdgeType = "overrides"
	EdgeUsesFor   EdgeType = "using_for"
	// Schema 1.14 (policy metadata): code symbol → Policy node.
	// Emitted from pkg/policy.Resolve when a YAML policy entry's
	// governs[].qname matches an existing Function/Method/Field/Struct/
	// Constant/Variable in the parsed graph. Direction = governed node
	// → policy that constrains it, matching the natural query "what
	// rules apply to this symbol?". Multiple policies can govern the
	// same symbol; the Policy node side fans in.
	EdgeGovernedBy EdgeType = "governed_by"
	// Schema 1.15 (security pattern annotations): code symbol
	// → SecurityPattern node. Emitted from pkg/security.Resolve when
	// a YAML security pattern's matches[].qname hits an existing
	// Function/Method/Field/Struct/Modifier in the parsed graph.
	// Direction = at-risk code symbol → pattern label so an LLM
	// modifying X can read "what security patterns does X exhibit?"
	// as a single edge-type lookup on src.
	EdgeHasSecurityPattern EdgeType = "has_security_pattern"
	// Schema 1.31 (runtime links, design decision D3): a dynamic call's
	// possible targets, from the Go frontend's SSA + VTA pass.
	// dispatch_kind "interface_method": interface method -> each
	// implementation whose values reach a call of it; "func_value": the
	// function that calls a function value -> each function that value can
	// hold, with the line of the call. Confidence INFERRED (static_may).
	EdgeDispatchesTo EdgeType = "dispatches_to"
	// Schema 1.32 (statement links): what happens inside a function, from
	// the statement nodes (IfStmt, LoopStmt, SwitchStmt, ReturnStmt,
	// CallSite, Goroutine) of the Go frontend. Line is the line of the use;
	// dispatch_kind names the part of the statement it lies in.
	//   site_calls   CallSite or Goroutine -> the function, method or
	//                interface method it calls (dispatch_kind static or
	//                interface_method)
	//   cond_reads   IfStmt, LoopStmt, SwitchStmt -> a field, package-level
	//                variable or constant its condition (init, condition,
	//                post, range expression, tag, case expressions) reads
	//   cond_calls   the same -> a function its condition calls
	//   body_writes  IfStmt, LoopStmt, SwitchStmt, Goroutine -> a value its
	//                body writes outside any statement nested in it
	//                (dispatch_kind then, else, body, case, go)
	//   encloses     a statement -> each statement directly inside it
	//                (dispatch_kind cond, then, else, body, case, go)
	EdgeSiteCalls  EdgeType = "site_calls"
	EdgeCondReads  EdgeType = "cond_reads"
	EdgeCondCalls  EdgeType = "cond_calls"
	EdgeBodyWrites EdgeType = "body_writes"
	EdgeEncloses   EdgeType = "encloses"
	// Schema 1.34 (channel aliases): flows_to from a channel's identity (its
	// make site, a field or a package-level variable) to a field or
	// variable the channel is stored in, with the line of the store
	// (dispatch_kind chan, confidence INFERRED). One channel has several
	// names; sends_to and recvs_from point at the name each operation uses,
	// and flows_to joins the names.
	EdgeFlowsTo EdgeType = "flows_to"
	// Schema 1.38 (Solidity custom errors): reverts_with from a function,
	// modifier or constructor to the Error it can revert with, one edge per
	// line of `revert X(...)` or `require(cond, X(...))`. No existing kind
	// fits: emits_event ends at an Event and calls at a callable.
	EdgeRevertsWith EdgeType = "reverts_with"
	// Schema 1.42 (message-driven code), from the Go frontend's SSA pass,
	// confidence INFERRED:
	//   handles   a receive operation's CallSite -> the code its message
	//             makes run: the CallSite of each call and channel
	//             operation, the Goroutine of each go statement, and each
	//             Field used, in its range (a plain receive: the rest of
	//             its block; range over a channel: the loop body; a select
	//             case: the case body). dispatch_kind call, send, recv,
	//             go, read or write; line of the handled code.
	//   triggers  a sender (the Function, Method or Goroutine whose send
	//             reaches the channel by its SSA origins) -> each receive
	//             operation's CallSite on that channel it can wake.
	//             dispatch_kind is the channel's origin (the qualified
	//             name of its make site, field or variable); line and file
	//             are the send's, the receive's line is its CallSite's.
	//             The send and the receive pair when their operands can be
	//             one channel by their own possible make sites; one edge per
	//             such make site, and the kind ends in "~aliasing" when
	//             there are several or the channel was received from a
	//             channel of channels (schema 1.45).
	//             A channel is identified by where it is made or held, not
	//             by object: two values of a struct type whose field holds
	//             a channel are one channel to the graph, so an edge through
	//             a field joins every instance's sends and receives on it.
	// Sender to handled code is two hops: triggers, then handles.
	EdgeHandles  EdgeType = "handles"
	EdgeTriggers EdgeType = "triggers"
	// Schema 1.42 (literals passed to calls), a statement link of the Go
	// frontend: literal_passed_to from the CallSite of a call written in a
	// function literal (not in a literal nested in it), or the Goroutine of
	// a go statement written there (schema 1.44), to the function or method
	// the literal is passed to as an argument, with the line of that call;
	// dispatch_kind arg; confidence EXTRACTED. The literal's code runs when
	// that function calls it. A literal assigned, returned, called in
	// place, started with go or deferred gives no edge, nor does one passed
	// to a function value.
	EdgeLiteralPassedTo EdgeType = "literal_passed_to"
	// Schema 1.44 (execution order), from the Go frontend:
	//   closes           a closer (a Function, Method or Goroutine) -> the
	//                    origin of the channel it closes (a make site, a
	//                    field, a package-level variable), as sends_to;
	//                    dispatch_kind make, field or global; confidence
	//                    INFERRED. A close wakes every receive on the
	//                    channel: the closer has a triggers edge to each,
	//                    on the line of the close.
	//   runs_under_lock  a CallSite (a call, a send, a receive) -> each
	//                    Mutex held there by the must-hold rule; dispatch
	//                    kind propagated when the mutex is held by a caller
	//                    (ckg build --lock-propagation), none when the
	//                    site's own function holds it; confidence INFERRED.
	//   operates_on      a CallSite -> the declaration of the sync primitive
	//                    it operates on (a Field, a Variable, a
	//                    LocalVariable; for a timer channel no declaration
	//                    holds, <-time.After(d), the CallSite of the call
	//                    that makes the timer); dispatch_kind the operation
	//                    (SyncOp); confidence EXTRACTED, INFERRED when the
	//                    primitive is a parameter followed to its origins.
	//   runs_func        a CallSite of sync.Once.Do, time.AfterFunc or
	//                    sync.WaitGroup.Go -> the declared function or
	//                    method passed to it, which runs later (once_do,
	//                    after_func, wg_go); confidence EXTRACTED. A literal
	//                    passed there is told by literal_passed_to.
	EdgeCloses        EdgeType = "closes"
	EdgeRunsUnderLock EdgeType = "runs_under_lock"
	EdgeOperatesOn    EdgeType = "operates_on"
	EdgeRunsFunc      EdgeType = "runs_func"
	// Schema 1.45 (control transfer), a statement link of the Go frontend:
	// jumps_to from a statement that leaves the normal order to where it
	// goes, dispatch_kind the kind of jump; confidence EXTRACTED:
	//   break        BranchStmt -> the LoopStmt or SwitchStmt it leaves
	//   continue     BranchStmt -> the LoopStmt it goes on with
	//   goto         BranchStmt -> the statement node its label stands on
	//   fallthrough  BranchStmt -> the CaseClause it goes on into
	//   return       ReturnStmt -> the Function, Method or FuncLit it
	//                returns from
	EdgeJumpsTo EdgeType = "jumps_to"
)

// What a callee does with a function literal it is passed: the dispatch
// kind of a literal_passed_to edge from a FuncLit node (schema 1.45), the
// kinds of its uses joined by LiteralUseSep in this order (call+go). A use
// inside a function literal of the callee's own takes the kind of how that
// literal runs: started by go, go; deferred, defer; called, call; stored or
// returned, store; sent, send; passed on, pass. The edge is EXTRACTED when
// one use of the kind list runs on every path through the callee, INFERRED
// when each depends on a branch (a select case is a branch).
const (
	LiteralUseCall  = "call"  // the callee calls it before it returns
	LiteralUseGo    = "go"    // the callee starts it in a goroutine
	LiteralUseDefer = "defer" // the callee defers it: it runs as the callee returns
	LiteralUseStore = "store" // the callee stores it (a field, a variable, a map) or returns it
	LiteralUseSend  = "send"  // the callee sends it on a channel: another goroutine may run it
	LiteralUsePass  = "pass"  // the callee passes it on as an argument
	LiteralUseSep   = "+"
)

// LiteralUses returns the literal use kinds in the order they are joined.
func LiteralUses() []string {
	return []string{LiteralUseCall, LiteralUseGo, LiteralUseDefer, LiteralUseStore, LiteralUseSend, LiteralUsePass}
}

// Dispatch kinds of a jumps_to edge.
const (
	JumpBreak       = "break"
	JumpContinue    = "continue"
	JumpGoto        = "goto"
	JumpFallthrough = "fallthrough"
	JumpReturn      = "return"
)

// AllEdgeTypes returns all 59 edge types in stable order.
// Append-only: existing positions are load-bearing for hash-derived IDs.
// EdgeAwaits + EdgeOverrides are appended at indices 38-39 for schema 1.10.
// EdgeUsesFor is appended at index 40 (schema 1.10 — using-for library
// extension as a first-class binding edge).
// EdgeGovernedBy is appended at index 41 (schema 1.14 — code
// symbol → external Policy node loaded from YAML; first-class so
// policy queries can run as a single edge-type filter).
// EdgeHasSecurityPattern is appended at index 42 (schema 1.15 —
// code symbol → SecurityPattern node loaded from YAML).
// EdgeDispatchesTo is appended at index 43 (schema 1.31 — runtime links).
// The five statement-link types are appended at indices 44-48 (schema 1.32).
// EdgeFlowsTo is appended at index 49 (schema 1.34 — channel aliases).
// EdgeRevertsWith is appended at index 50 (schema 1.38 — Solidity errors).
// EdgeHandles and EdgeTriggers are appended at 51-52 (schema 1.42 —
// message-driven code), EdgeLiteralPassedTo at 53 (schema 1.42), and
// EdgeCloses, EdgeRunsUnderLock, EdgeOperatesOn and EdgeRunsFunc at 54-57
// (schema 1.44 — execution order), EdgeJumpsTo at 58 (schema 1.45).
func AllEdgeTypes() []EdgeType {
	return []EdgeType{
		EdgeContains, EdgeDefines, EdgeCalls, EdgeInvokes, EdgeUsesType,
		EdgeInstantiates, EdgeReferences, EdgeReadsField, EdgeWritesField,
		EdgeImports, EdgeExports, EdgeImplements, EdgeExtends,
		EdgeHasModifier, EdgeEmitsEvent, EdgeReadsMapping, EdgeWritesMapping,
		EdgeHasDecorator, EdgeSpawns, EdgeSendsTo, EdgeRecvsFrom, EdgeBindsTo,
		EdgeAcquiresLock, EdgeReleasesLock, EdgeAccessedUnderLock,
		EdgeListensOn, EdgeHandlesMessage, EdgeRPCCalls,
		EdgeChangedIn, EdgeBlame,
		EdgeTimeoutPath, EdgeCancellationPath,
		EdgeHasHunk, EdgeAdjacent, EdgeModifies,
		EdgeHTTPCalls,
		EdgeGRPCListensOn, EdgeGRPCCalls,
		EdgeAwaits, EdgeOverrides,
		EdgeUsesFor,
		EdgeGovernedBy,
		EdgeHasSecurityPattern,
		EdgeDispatchesTo,
		EdgeSiteCalls, EdgeCondReads, EdgeCondCalls, EdgeBodyWrites, EdgeEncloses,
		EdgeFlowsTo,
		EdgeRevertsWith,
		EdgeHandles, EdgeTriggers,
		EdgeLiteralPassedTo,
		EdgeCloses, EdgeRunsUnderLock, EdgeOperatesOn, EdgeRunsFunc,
		EdgeJumpsTo,
	}
}

// Confidence labels (spec §4.8).
type Confidence string

const (
	ConfExtracted Confidence = "EXTRACTED"
	ConfInferred  Confidence = "INFERRED"
	ConfAmbiguous Confidence = "AMBIGUOUS"
)

// Valid reports whether c is one of the three known confidence labels.
func (c Confidence) Valid() bool {
	switch c {
	case ConfExtracted, ConfInferred, ConfAmbiguous:
		return true
	}
	return false
}
