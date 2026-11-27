// Package buildpipe — cache.go holds the schema version and the per-file
// keys the manifest records: SHA256, cache key, parser version. No build
// reads them back (every build is cold); they say what a graph was built
// from.
package buildpipe

import (
	"crypto/sha256"
	"encoding/hex"
	"runtime"
	"runtime/debug"
)

// SchemaVersion is the cache-key contributor for the extraction schema. Bumped
// from "1.1" to "1.2" for FK ON DELETE CASCADE on edges/blobs/pkg_tree/
// topic_tree. Bumped from "1.2" to "1.3" because new node kinds
// (Endpoint, MessageType) and edge kinds (listens_on, handles_message,
// rpc_calls) materially change the extraction surface — pre-1.3 DBs are
// missing those rows, so incremental invalidation must force a cold rebuild
// on first run with this binary. Bumped from "1.3" to "1.4" for temporal
// edges: NodeCommit + changed_in/blame edges are emitted by the new
// post-Build temporal pass; pre-1.4 DBs are missing those rows. Bumped from
// "1.4" to "1.5" for pending_refs persistence: Pass 1's unresolved
// cross-file references are now persisted per-file so the partial-cache
// rebuild path can reconstruct Pass 2's input without re-parsing cached
// files. Pre-1.5 DBs are missing the table, so the first 1.5 build is forced
// cold by ManifestUsable's version check. Bumped from "1.5" to "1.6" for
// control-flow context propagation: timeout_path /
// cancellation_path self-loop edges are emitted from Go context.With* call
// sites; pre-1.6 DBs are missing those rows so the first 1.6 build must run
// cold. Bumped from "1.6" to "1.7" for detector gap fill: the
// edges row gains an optional `dispatch_kind` TEXT column populated for
// `invokes` edges, plus three new emit sites — `uses_type`,
// `instantiates`, and the lock-edge fix inside goroutine bodies.
// Pre-1.7 DBs are missing the column AND the new edges; opening
// such a DB triggers an idempotent ALTER ADD COLUMN via Migrate(), and
// ManifestUsable's version check forces a cold rebuild on first 1.7 run
// so the new edges land in their natural emission order.
// Bumped from "1.7" to "1.8" for the hunk graph (temporal extension):
// new node type NodeHunk + new edges has_hunk / adjacent + gzip-compressed
// unified-diff blobs persisted under the existing blobs.node_id PK. No
// schema DDL change (the new rows reuse existing tables); pre-1.8 DBs are
// missing the rows + the new node/edge type literals so ManifestUsable's
// version check forces a cold rebuild on first 1.8 run.
// Bumped from "1.8" to "1.9" for cross-language interop expansion:
// TypeScript HTTP server endpoint detection (Express / Fastify
// / Hono / Next.js App Router). Reuses the existing NodeEndpoint type +
// `listens_on` edge — no new enum literals, no new columns. The bump is
// purely a cache-key contributor so pre-1.9 DBs don't carry forward a
// missing-Endpoint TS graph view on first 1.9 build. Per §6.1 of the
// design spec, later interop stages (HTTP client matching, gRPC,
// message queue) stay on 1.9 and append-only.
// Bumped from "1.9" to "1.10" for within-language semantics slot
// reservation: `NodeAwaitPoint` + `EdgeAwaits` (TS async/await suspension
// flow) and `EdgeOverrides` (Solidity virtual/override semantics). The
// detectors landed separately — this bump is slot-only, so pre-1.10 DBs are byte-identical in their existing
// rows but the cache key flip forces a cold rebuild on first 1.10 run
// for symmetry with prior bumps. No new DDL (the new enum literals
// fit existing nodes.type / edges.type TEXT columns); see
// docs/graph/archive/DISPATCH-WITHIN-LANG-SEMANTICS.md §2 and
// docs/design/{ts-async-await-and-interface,solidity-inheritance-and-interface-dispatch}.md.
//
// Bumped from "1.10" to "1.11": the nodes table gains the `attrs` JSON-blob column that carries
// every types.Node marker without its own SQLite column (SlotIndex,
// HasAssembly, HasLowLevelCall, HasValueTransfer, YulBuiltins,
// IsFunctionTyped, HasFunctionTypedVar, HasFunctionPointerCall,
// HasFunctionPointerPropagation, HasExternalCall,
// HasInheritanceMROFallback, HasSelfReentrantCall,
// HasSelfDelegatecallDead). Migrate() ALTER-adds the column on
// pre-1.11 DBs via ensureAttrsColumn — incremental builds still
// upgrade — but the cache-key flip forces a cold rebuild on first
// 1.11 run so the new attrs land for every node, not just the
// ones touched by dirty files. See internal/persist/node_attrs.go
// for the JSON encoding.
//
// Bumped from "1.13" to "1.14" for policy metadata: introduces
// NodePolicy + EdgeGovernedBy slots and the buildpipe Options.PolicyFile
// path that loads them from a YAML and resolves governs[] qnames
// against the parsed graph. No DDL change — the new enum literals fit
// the existing nodes.type / edges.type TEXT columns. The bump forces
// a cold rebuild on first 1.14 run so existing DBs that pre-date the
// PolicyFile flag don't carry forward a stale "no policy nodes" view
// when the operator subsequently turns the flag on. Empty PolicyFile
// is the no-op default; only operators who opt in see new rows. See
// pkg/policy.
//
// Bumped from "1.14" to "1.15" for security pattern metadata:
// introduces NodeSecurityPattern + EdgeHasSecurityPattern slots and
// the buildpipe Options.SecurityPatternFile path that loads them
// from a YAML and resolves matches[] qnames against the parsed
// graph. Same shape as the policy bump — no DDL change, additive,
// cold-only re-ingest. See pkg/security.
//
// Kept here (not in pkg/types) because only the cache key needs it;
// pkg/types schema version bumps already trigger rebuilds via this constant.
//
// Bumped from "1.15" to "1.16" for two Go-extraction accuracy changes:
//  1. formatSignature now emits the full parameter and result lists (was the
//     "func name(...) ..." placeholder) — a stored node column.
//  2. call resolution no longer bare-name-binds builtin calls (len/cap/...) or
//     interface-method dispatch to same-named decoys; both are now qualified or
//     dropped — changes calls/invokes edges.
//
// Both repopulate stored rows, so existing graphs are stale until reindexed —
// bump to force a cold re-extraction on first run with this binary.
//
// Bumped from "1.16" to "1.17": EmitPromotedMethods materialises a
// method node (+ defines edge) for each method an in-module struct promotes
// from an embedded type, so find_symbol("T.M") resolves promoted methods. New
// rows → existing graphs are stale until reindexed.
//
// Bumped from "1.17" to "1.18": EmitFieldWriteEdges emits
// writes_field edges (function -> struct field it assigns) for Go, so an agent
// can find who mutates a field. New edges → reindex required.
//
// Bumped from "1.18" to "1.19" by symbol identity (ADR-0001): every
// parser now populates nodes.canonical_id. Go covers types/structs/interfaces,
// fields, package-level const/var, and interface methods (previously
// func/method only); Solidity, TypeScript, and proto use the relative file path
// as the qualifier (<relpath>:<qname>), with Solidity appending the
// parameter-type signature to functions to separate overloads. The column
// already exists (idempotent ALTER); pre-1.19 DBs carry it empty, so the
// cache-key flip forces a cold rebuild to repopulate canonical_id graph-wide.
// Additive — qualified_name, node IDs, edges unchanged.
//
// Bumped from "1.19" to "1.20" by a symbol-identity refinement: the blank
// identifier `_` (package-level `var _`, struct padding fields) no longer
// receives a canonical_id (it was a non-unique <pkg>._ before). Stored
// canonical_id values change for those nodes, so force a cold rebuild.
//
// Bumped from "1.20" to "1.21" by further symbol-identity refinements: function-local
// `var` declarations no longer receive a canonical_id (only package-level
// const/var do — the local <pkg>.<name> id collided across functions); proto
// canonical ids drop the doubled `proto:` prefix (<relpath>:<pkg>.<Sym>); and
// a canonical_id shared by >1 node in the same file is line-qualified with
// "@<line>" (e.g. minified-JS `function t(){}`, multiple Go `init`). All change
// stored canonical_id values, so force a cold rebuild.
//
// Bumped from "1.21" to "1.22" by the simple_name column: nodes now store the
// short name (last dotted segment) so suffix lookups use an indexed equi-join
// instead of a leading-wildcard LIKE. The column is populated at write time, so
// pre-1.22 DBs must be rebuilt to fill it graph-wide (read-only opens cannot
// run the ALTER); the cache-key flip forces that cold rebuild.
//
// Bumped from "1.22" to "1.23" by ADR-0002 (staged graph composition): the Go
// loader's buildFileIndex now gives primary (non-test-variant) packages
// deterministic ownership of production files, instead of order-dependent
// first-seen-wins. ~17.5% of production files previously landed on a test
// variant (whose TypesInfo also feeds the concurrency / field-touch passes),
// making which package resolved them depend on packages.Load's unguaranteed
// order. A cold rebuild pins them to the primary package deterministically.
// (This does not change canonical_id values — test variants share the import
// path; it removes a latent non-reproducibility in resolution context.)
//
// 1.24 (2026-10-11): review index — nodes.is_test, code_edges and
// node_commits, recomputed after every build (RebuildReviewIndex). The bump
// forces a cold rebuild so existing graphs gain the index.
//
// 1.25 (2026-11-15): Go node ids include the declaring file (parse.NodeID) and
// package nodes are keyed by package directory, so packages that declare the
// same name (crypto/bn256/google and crypto/bn256/cloudflare, two package
// main) no longer share node ids; call sites find their function by span, not
// by qualified name. Stored node ids change, so force a cold rebuild.
//
// 1.26 (2026-11-16): Mutex node ids carry the node kind, so a package-level
// `var mu sync.Mutex` keeps both its Variable and its Mutex node (they shared
// one id and graph.Build kept one). Stored node ids change.
//
// 1.27 (2026-11-16): scores are computed before the topic tree, so topic_tree
// labels name each community after its highest-PageRank member instead of its
// first member. Stored topic labels change; force a cold rebuild so an
// unchanged source tree does not keep the old labels through the all-cached
// short circuit.
//
// 1.28 (2026-11-16): communities of one resolution get distinct topic labels
// (repeats are numbered), so topic_tree keeps every member; before, a
// community whose label repeated another's replaced its members. Stored
// topic rows change.
//
// 1.29 (2026-11-17): Go value uses. EmitValueAccessEdges replaces
// EmitFieldWriteEdges: reads_field and writes_field (now also composite
// literal fields and promoted fields, with line and count) and references
// from a function or package-level initializer to the package-level
// variables and constants it uses (dispatch_kind read or write). New edges;
// force a cold rebuild.
//
// 1.30 (2026-11-17): one value edge per use line. reads_field, writes_field
// and references had one edge per (source, value, access) with the first
// line; they now have one per line, as calls do, with count the uses on that
// line. Edges change; force a cold rebuild.
//
// 1.31 (2026-11-18): runtime links (design decision D3). dispatches_to from
// an interface method to each implementation whose values reach one of its
// calls, and from a function that calls a function value to each function
// the value can hold, from SSA + VTA in the Go frontend. New edges; force a
// cold rebuild.
//
// 1.32 (2026-11-18): statement links and channel links (design decision
// D3 and the user's 2026-11-18 instruction that both be supported).
// site_calls, cond_reads, cond_calls, body_writes and encloses connect the
// statement nodes of a Go function body; sends_to and recvs_from gain edges
// from SSA to the channel a send or receive operates on (make site, field or
// package-level variable), followed through parameters, closures, stores
// and returns. The SSA pass now covers every method body, not only those
// AllFunctions reaches. New edges; force a cold rebuild.
//
// 1.33 (2026-11-18): fixes found by ckg verify's statement-link check. The
// statements of a goroutine's function literal get nodes, and the calls in a
// go statement's arguments get call sites and calls edges (static call
// recall on go-stablenet 0.9997 to 1.0000); an init function's value uses
// and statement links stay with the init of their own file (all init
// functions of a package share one canonical id); a write in a condition is
// no longer a cond_reads; a call of an interface method through a struct
// that embeds the interface is interface_method. Nodes and edges change;
// force a cold rebuild.
//
// 1.34 (2026-11-18): channel aliases and type-hierarchy dispatch, from the
// recall ckg verify --observed measured. flows_to joins the names of one
// channel (a make site and the field or variable it is stored in);
// dispatches_to of kind type_hierarchy adds, for each invoked interface
// method, the implementations go/types allows that the value flow did not
// link (reflection, values kept in standard-library containers); a channel
// a closure captures by reference is followed through the cell it points
// to. New edges; force a cold rebuild.
//
// 1.35 (2026-11-18): type-hierarchy dispatches_to edges stay out of
// PageRank, as the statement links do. A replay of recorded calls showed
// them reordering search_code hits (13 of 214 answers): they say what
// go/types allows, not a dependency the analysis found. Scores change;
// force a cold rebuild. Bumped from "1.35" to "1.36" for External nodes:
// calls, interface calls and type uses whose target is outside the build
// (standard library, dependencies) now point at one External node per
// symbol instead of being dropped; pre-1.36 DBs have none.
// Bumped from "1.36" to "1.37": value-use edges (reads_field, writes_field,
// references) counted every use twice in packages with in-package tests,
// whose production files were walked again through the test variant; counts
// change, so force a cold rebuild.
// Bumped from "1.37" to "1.38": node identity and coverage changed. A call
// site is named and keyed by its start and end byte (the calls of a chain no
// longer share one node), embedded fields and the fields of anonymous nested
// structs have Field nodes, a method of a generic type is named after its
// type, field uses are tied to the field's declaration position (a use of a
// function-local struct's field no longer points at the same-named
// package-level field), statement-link counts no longer double in packages
// with in-package tests, and handles_message edges need a net/rpc import.
// Added in the same version: a package-level variable is the caller of the
// calls in its initialiser, whose statements and call sites have nodes; a
// local declaration is named under its function; fields of anonymous struct
// types have Field nodes; a name made by := or a range clause is a
// LocalVariable node with references edges from its function; uses_type
// covers receivers, bodies and type declarations (interface signatures,
// defined types). Solidity in the same version: node ids are keyed by file,
// a node spans its whole declaration, members are named under and defined by
// their contract, names resolve in the scope of the contract that uses them,
// and errors, struct members, enum values, imports, calls, reverts_with and
// state variable reads and writes have nodes and edges. Node IDs, edges and
// scores change; force a cold rebuild.
//
// 1.39 (2026-11-26): the Go graph agrees with its source where an independent
// comparison on go-stablenet found it did not. An existing name on the left
// of := is written, not read (1,117 uses). Lock edges and accessed_under_lock
// are made after Resolve, so a lock on a mutex declared in another file has
// its edge (171 sites); a mutex declared by a composite literal or new has a
// node. uses_type also starts at package-level variables and constants
// (1,854 references), ends at types declared inside a function, and carries
// the line of the reference; new(T) of an alias instantiates the alias. Code
// of the module that the build scope leaves out and the builtin error method
// are External nodes (445 calls, 71 type references, 222 err.Error() calls).
// Methods on an alias receiver have a canonical id; methods of interfaces
// declared in a function or written without a name have nodes and are called
// by position; a call through an interface embedded in a struct is invokes;
// a closure called in a package-level initialiser leaves its marker on the
// variable. The variable of a type switch is a LocalVariable; local constants
// and array-length constants in signatures and type declarations are read.
// implements covers types of test files and types declared in functions;
// extends reaches interfaces outside the build. Every make(chan) has a
// Channel node with its element type, a range over a channel is a receive,
// send and receive sites and statements in function literals have encloses
// edges, and the calls in a go statement's arguments belong to the Goroutine.
// Single var, const and type declarations keep their doc comment and grouped
// ones their own. TypeAlias nodes say alias or defined in sub_kind. is_test
// follows the build scope's file role when a scope is given
// (Options.FileRoles). In the same version: the span of a type, var, const
// or import declaration starts at its first token, the keyword when it
// stands alone, and every name of one specification spans the whole
// specification (node ids stay keyed by the declaring identifier, and the
// signature of each such name says which it is, "name 2 of 4"); the span of
// a specification, a struct field, an interface method, a function and a
// method runs through the comment after it on its last line; a Mutex node has the span of its
// Variable or Field; a package
// node shows its first file that is not test code; instantiates also starts
// at the Variable of a package-level initialiser; a channel operation in a go
// literal has an edge from its function; a read or call in a function
// literal written in a statement header belongs to that header; a lock on a
// mutex returned by a getter has its edge. Nodes, edges and scores change;
// force a cold rebuild.
//
// 1.40 (2026-11-26): the span of a Solidity declaration runs through the
// plain comment after it on its last line (after the comma of an enum
// value), as a Go declaration's does; the comment was in no node's text.
// accessed_under_lock names a field used where the mutex is held, not every
// field of a function that locks it somewhere; a function with a goto gets
// none. Node ids stay; Solidity end bytes and accessed_under_lock edges
// change; force a cold rebuild.
//
// 1.41 (2026-11-26): accessed_under_lock counts the fields a struct literal
// sets while the mutex is held, as writes_field does (one notion of a field
// use, fieldUsesAt), and the receiver path of a deferred Unlock, read where
// the defer stands. accessed_under_lock edges change; force a cold rebuild.
//
// 1.42 (2026-11-26): no field is under the mutex it declares (a getter that
// returns &s.mu while s.mu is held). --lock-propagation follows only calls
// made where the caller's mutex is held and marks its edges with the
// dispatch kind "propagated". Interface dispatches_to no longer end at an
// interface method and reach the methods of interfaces declared in a
// function or written inline; a Goroutine's sends_to/recvs_from come from
// the channel linker. Every channel operation has its site, also on a bound
// channel. A Goroutine is a caller: calls, reads_field and writes_field
// from it (dispatch kind goroutine), which the walks and the tools keep apart
// from the function-level edges. New edge types handles (a receive to the
// code its message runs) and triggers (a sender to a receive it can wake),
// and flows_to of kind received. accessed_under_lock, dispatches_to,
// sends_to and recvs_from edges and CallSite nodes change. Every build is
// cold.
//
// 1.43 (2026-11-27): literal_passed_to, a statement link from the call site
// of each call written in a function literal passed as an argument to the
// function it is passed to. New edges; force a cold rebuild (every build is).
// The pending_refs table is no longer written (only the removed incremental
// build read it); an older graph.db keeps it until the next build replaces
// the file.
//
// 1.44 (2026-11-27): facts for reading code in execution order. closes (a
// closer to the channel it closes; its CallSite has sub kind close) and
// triggers from the closer to every receive on the channel; a select
// statement is a SwitchStmt of sub kind select or select_default that
// encloses its cases' sends and receives as cond; runs_under_lock (a call,
// send or receive site to each mutex held there); operates_on (a call or
// receive on a sync primitive to the declaration that holds it) and
// runs_func (to the function sync.Once.Do, time.AfterFunc or
// sync.WaitGroup.Go runs later); literal_passed_to also from the Goroutine
// of a go statement in the literal. SwitchStmt nodes, encloses, cond_reads,
// cond_calls, body_writes and triggers change. Every build is cold.
//
// 1.45 (2026-11-27): control transfer and code extents. Node types
// BranchStmt (break, continue, goto, fallthrough), CaseClause (one clause
// of a switch or select), DeferStmt and FuncLit; edge type jumps_to (a
// BranchStmt or ReturnStmt to where it goes); literal_passed_to also from a
// FuncLit to its callee, its kind what the callee does with it; flows_to of
// kind sync (a sync primitive stored into a field); a CallSite may carry
// sync_unknown; Channel qualified names name the line. Every build is cold.
const SchemaVersion = "1.45"

// ComputeCacheKey returns the SHA256 of:
//
//	file_content + "|ckg:" + ckgVersion + "|parser:" + parserVersion + "|schema:" + SchemaVersion
//
// Any change in the four contributors invalidates the cache for that file
// and forces a reparse on next build (spec v0.2 § 4 design).
func ComputeCacheKey(content []byte, ckgVersion, parserVersion string) string {
	h := sha256.New()
	h.Write(content)
	h.Write([]byte("|ckg:"))
	h.Write([]byte(ckgVersion))
	h.Write([]byte("|parser:"))
	h.Write([]byte(parserVersion))
	h.Write([]byte("|schema:"))
	h.Write([]byte(SchemaVersion))
	return hex.EncodeToString(h.Sum(nil))
}

// SHA256Hex returns the hex SHA256 of content. Exposed separately because
// FileEntry.SHA256 stores content-only hash (used for the "mtime changed but
// content identical" fast/slow path), distinct from the full cache key.
func SHA256Hex(content []byte) string {
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// parserVersionFor returns a deterministic identifier for the parser used on
// language lang. Simplification: Go ties to runtime.Version()
// (the host toolchain that drives go/packages.Load); TS/Sol tie to the
// tree-sitter wrapper module pseudo-version read from BuildInfo, falling back
// to a constant if BuildInfo is unavailable (e.g. in unit tests via `go run`).
//
// Perfect introspection (e.g. embedded grammar SHAs) is out of scope. The bar
// is "any change to the parser binary changes this string".
func parserVersionFor(lang string) string {
	switch lang {
	case "go":
		// runtime.Version() flips when the toolchain that linked ckg
		// changes — Go builds use go/packages.Load with the same toolchain.
		return "go/" + runtime.Version()
	case "ts", "sol":
		return "tree-sitter/" + treeSitterModuleVersion()
	case "proto":
		// Hand-rolled lexer/parser (schema 1.9 §6.4) — ties to the host
		// toolchain so a rebuild after parser edits invalidates the cache.
		return "proto/handrolled/" + runtime.Version()
	default:
		return lang + "/unknown"
	}
}

// treeSitterModuleVersion reads the resolved version of the tree-sitter
// runtime binding from the embedded BuildInfo. Returns "unknown" when
// BuildInfo isn't available (test runs without -buildvcs, for instance).
//
// Pinned to the exact runtime path github.com/tree-sitter/go-tree-sitter
// (after the go-tree-sitter migration). An earlier substring match on "tree-sitter"
// matched THREE deps (the runtime + tree-sitter-typescript +
// tree-sitter-javascript grammar bindings) and returned whichever
// BuildInfo's Deps iteration happened to surface first — non-deterministic
// across builds. Pinning to the runtime gives a single deterministic
// version contributor to the cache key. The grammar bindings are
// transitively versioned through the runtime release; bumping either
// grammar without bumping the runtime is unusual.
//
// Cache invalidation across the migration: pre-migration manifests recorded
// "tree-sitter/v0.0.0-20240827..." (the smacker pseudo-version);
// post-migration records "tree-sitter/v0.25.0". The string mismatch
// fires the slow-path reparse on first build of any pre-migration graph dir.
const tsRuntimePath = "github.com/tree-sitter/go-tree-sitter"

func treeSitterModuleVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	for _, dep := range bi.Deps {
		if dep.Path != tsRuntimePath {
			continue
		}
		if dep.Replace != nil && dep.Replace.Version != "" {
			return dep.Replace.Version
		}
		if dep.Version != "" {
			return dep.Version
		}
	}
	return "unknown"
}

// DiscoveredFile describes one file produced by detect.* — used as input to
// the diff. Path is srcRoot-relative slash form.
type DiscoveredFile struct {
	Path     string
	Language string
}
