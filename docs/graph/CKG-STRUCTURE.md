# ckg structure: from project source to graph.db, and every way it is queried

Status: 2026-11-16, code at main 9b8d456d; section 8 records the fixes made after the first version of this page. A map of the code graph engine (ckg) as it is, with sequence diagrams:
how `ckg build` turns a project's source into `graph.db`, what each stored table holds and who writes it, and every
surface that reads the graph (`ckg mcp`, `ckg api`, the CLI, and the CKS search composer). The design the engine
moves to is [the shared symbol contract](./SYMBOL-CONTRACT-DESIGN.md); logging and the short flow summary are in
[the build and query flow](./BUILD-AND-QUERY-FLOW.md). Section 8 lists defects found while mapping; each is marked
as read in the code or inferred.

## 1. Components

```mermaid
flowchart LR
  SRC[project source] --> BUILD[ckg build<br/>cmd/graph/build.go]
  PROF[build profile + this build's options] --> SC[ckg scope<br/>cmd/graph/scope_cmd.go]
  SRC --> SC
  SC --> SCOPE[build scope JSON]
  SCOPE --> BUILD
  BUILD --> BP[internal/graph/buildpipe]
  BP --> DET[detect, filterlist]
  BP --> PARSE[parse: golang, typescript,<br/>solidity, proto]
  BP --> GR[graph.Build, link,<br/>score, cluster, temporal]
  BP --> DB[(graph.db<br/>internal/graph/persist)]
  DB --> MCP[ckg mcp<br/>pkg/graph/mcphandlers]
  DB --> API[ckg api<br/>internal/graph/server]
  DB --> CLI[ckg query, path, report,<br/>verify, export ...]
  DB --> CKS[CKS composer<br/>internal/system/ckgclient]
  CKV[(ckv vector index)] --> CKS
```

Every reader opens the database read-only (`persist.OpenReadOnly`, `mode=ro&immutable=1`); CKS opens it in its own
process through `ckgclient.NewReal`, not over MCP or HTTP.

### 1.1 Where the build scope JSON comes from

The scope is extracted before the graph is built, every time, from the options the build at hand uses; the graph
then reads exactly the files the scope lists. Without a scope every file of the tree is read.

```
ckg scope --src <go-stablenet> --profile projects/stablenet/filelist.yaml \
  [--goos linux --goarch amd64 --cgo 1 --tags urfave_cli_no_docs,ckzg] [--binary build/bin/gstable] \
  --out <dataset>/build-scope.json
ckg build --src <go-stablenet> --scope <dataset>/build-scope.json --out <graph dir> --no-cache
```

The profile (`projects/<pack>/filelist.yaml`) holds the project's usual build: `build_context` (goos, goarch, cgo,
tags), `build_roots` (the entry packages), `include_package_tests`, `inclusions` and `exclude_globs`; unknown keys are
an error. A flag given to `ckg scope` overrides the profile for that run (`--tags`, even empty, replaces the tags;
`--entry` replaces the roots; `--include` adds globs as the inclusion `command-line`), so a build with other options
gets the scope of that build. `cmd/buildscope` runs the same extraction (`internal/buildscope.ExtractProject`).

`inclusions` is the one place for exceptions: code the build does not compile that belongs to the scope because the
compiled code cannot be understood without it, such as the system contracts of an EVM chain. Each entry has an `id`,
a `reason`, `files` (globs) or `go_packages` (main-module patterns whose own files are added), an optional `exclude`,
and an optional `linked_by.embed`, globs of files the build must embed:

```yaml
inclusions:
  - id: system-contracts
    reason: genesis contracts; contracts.go embeds their bytecode and the Go code calls them by address
    files: ["systemcontracts/**/*.sol"]
    linked_by: {embed: ["systemcontracts/artifacts/v1/*"]}
  - id: system-contract-tooling
    reason: their end-to-end tests and the solc tooling; no shipped binary imports them
    go_packages: [./systemcontracts/test, ./systemcontracts/compile/...]
```

Extraction fails when an inclusion has no id or reason, when one of its globs matches no file, when it adds nothing
the build does not already have, and when the build no longer embeds a file its `linked_by` names, so an exception
cannot outlive its reason unnoticed. An inclusion without `linked_by` rests on its reason alone. Every file an
inclusion adds carries its id as `included_by`; the scope lists each inclusion with its reason, file count and link,
and the files the build embeds as `embed_files`. The older `extra_packages` and `extra_globs` keys still work as the
inclusions of those names, without a reason and without the match check. `cks filelist` refuses a profile with
`inclusions`.

The go command decides the Go files: `ExtractGo` runs it with the effective `GOOS`, `GOARCH`, `CGO_ENABLED` and tags
(`GOWORK=off`): `go env` for the target, `go version` and a sha256 of the go binary for the toolchain, `go list -e
-deps -json <entry points>` for the build and `go list -e -json <extra packages>` for the packages the profile adds.
Every main-module package gives its `GoFiles`, `CgoFiles` and `SFiles` (role production), its `TestGoFiles` and
`XTestGoFiles` (role test, unless the profile turns tests off), its `IgnoredGoFiles` and, for the build, its
`EmbedFiles`; every other module becomes an outside module. The inclusions then add their files, and `exclude_globs`
remove files from the whole scope. With the options go-stablenet's Makefile target gstable uses, this list is exactly the set of main-module files the compiler
receives: 673 of 673 on darwin/arm64 (recorded with `-toolexec`) and 675 of 675 on linux/amd64 (`go build -n`).

The extraction fails instead of writing a doubtful scope: when the go command reports load errors (a missing module
leaves packages out; `--allow-problems` keeps the scope anyway) and when a `--binary` was built with other options.
The binary check reads `go version -m` and compares its `GOOS`, `GOARCH`, `CGO_ENABLED`, the set of `-tags` and its
main package with the scope's target and entry points. The scope records the profile's sha256 and every binary that
matched.

`ckg build --scope` reads only the files the scope lists, of every language, type-checks the Go files with the
scope's target and compares the loaded modules with the scope's. `cks setup` with a `filelist:` profile runs `ckg
scope` as its first step (`--build-goos`, `--build-goarch`, `--build-cgo`, `--build-tags`, `--build-binary` pass this
run's options) and gives the scope to `ckg build --scope` and `ckv build --files-from`; the options are part of the
dataset's input digest. `cks.ops.index` does the same when `backends.ckg.build_profile` is set. The scope records the
toolchain that made it, but the build does not check it.

The scope stops at the compiler. The linker keeps only the functions reachable from `main`: built with
`-ldflags=-dumpdep` and inlining off, gstable keeps 7,205 of the graph's 9,498 production functions and methods
(75.9%); the graph still holds the other 2,293. Files compiled into the binary without being Go source, such as the
22 `go:embed` files and the C sources cgo includes, are not listed either.

## 2. Build: entry and cache decision

```mermaid
sequenceDiagram
  participant U as ckg build (cmd/graph/build.go)
  participant S as loadScope (cmd/graph/scope.go)
  participant R as buildpipe.Run (pipeline.go)
  participant D as discoveryAll (incremental.go)
  participant M as old manifest (graph.db)
  U->>U: --at-commit: git worktree add (temporary source)
  U->>U: graph.db exists and no --force/--no-cache: print and stop
  U->>U: logger: <out>/logs/build-<time>.jsonl and stderr
  U->>S: --scope: GOOS, GOARCH, cgo, tags, outside modules, the file list
  U->>R: Options
  R->>D: detect.Walk + detect.GoFilesFor + filter
  D-->>R: files per language
  R->>M: readOldManifestFromDB
  alt --no-cache, no usable manifest, or Go build configuration changed
    R->>R: runCold
  else every file cached (mtime or cache key)
    R->>R: runShortCircuit (manifest only)
  else some files changed or removed
    R->>R: runIncremental
  end
```

- A manifest is usable when its schema version equals `SchemaVersion` (`buildpipe/cache.go`, 1.28), the builder
  version equals the ckg version, and it lists files. `DiffManifest` marks a file cached when its mtime and parser
  version are unchanged, or when its cache key (sha256 of content, ckg, parser and schema version) is unchanged.
- `--force` only skips the "graph already exists" check; an unchanged tree still short-circuits, and its help text
  now says so. `--rebuild-metrics` had no effect and is deprecated.
- The short circuit rewrites the manifest (time, staleness) and leaves every graph table as it was. Like the
  incremental build it writes a copy, `graph.db.building`, and renames it over `graph.db` (section 4).

## 3. Cold build

```mermaid
sequenceDiagram
  participant C as runCold (pipeline.go)
  participant G as Go frontend (parse/golang)
  participant O as other frontends (ts, sol, proto)
  participant B as graph.Build
  participant P as emitDerivedPasses
  participant W as persist (sqlite_writer.go)
  C->>C: discovery again, filter applied
  C->>G: runGoPipeline (language_runners.go)
  G->>G: go/packages typed load, log "go packages loaded" (area load)
  G->>G: Pass 1: ParseFile per file, 8 workers
  G->>G: Pass 2: Resolve pending references
  G->>G: post-passes: implements, uses_type, instantiates, promoted methods, writes_field
  G->>C: resolved graph, "resolve summary", through contract records
  C->>O: runTSPipeline, runSolPipeline, runProtoPipeline (parse + name-based Resolve)
  C->>C: warnDuplicateNodeIDs (area graph)
  C->>B: merge by node id (last wins), edges deduplicated by type, src, dst, line
  C->>C: validateAndSanitize: drop dangling edges
  C->>P: Sol to TS binds_to, HTTP client matching, temporal history, lock propagation (flag), score, package tree, topic tree
  C->>W: nodes, edges, pkg_tree, topic_tree, blobs, nodes_fts (one transaction into graph.db.building)
  C->>W: pending_refs, node_prs (PR history), policy and security overlay, review index (is_test, code_edges, node_commits)
  C->>W: manifest (stats, digests, counts, go_build, files, staleness), manifest.json
  C->>C: rename graph.db.building to graph.db
```

### 3.1 The Go frontend

```mermaid
sequenceDiagram
  participant R as runGoPipeline
  participant L as detect.GoPackagesFor
  participant F as Parser.ParseFile (parser.go)
  participant V as Resolve (resolve.go)
  participant X as post-passes
  participant K as goThroughContract
  R->>L: packages.Load("./...") per go.mod, tests included, scope GOOS/GOARCH/cgo/tags
  L-->>R: packages with types.Info, errors counted, scope modules compared
  loop each file (parseConcurrent)
    R->>F: file path, source
    F->>F: concurrency declarations (Mutex, Channel nodes)
    F->>F: declarations: Package, File, Import, types, fields, consts, vars, funcs, methods, contains, defines, imports
    F->>F: function bodies: statements, call sites, goroutines, spawns, sends_to, recvs_from, lock edges
    F->>F: distributed: endpoints, message types, listens_on, http_calls, rpc_calls, grpc
    F->>F: context paths: timeout_path, cancellation_path
    F-->>R: nodes, edges, pending references (calls, invokes with target canonical id)
  end
  R->>V: all parse results
  V->>V: bind by canonical id (typed) or qualified name, count the rest by reason
  V-->>R: resolved graph
  R->>X: go/types packages, nodes
  X-->>R: implements, extends, uses_type, instantiates, promoted Method nodes, writes_field
  R->>K: resolved graph
  K-->>R: Decl and Ref records, turned back into the graph that goes on
```

Node ids come from `parse.NodeID(language, file, qualified name, start byte)`; call sites find their enclosing
function by byte span. Resolve binds a typed reference only by its canonical id and counts what it drops
(`outside_graph`, `no_target`, `bare_name`, `no_match`). The TypeScript, Solidity and Proto frontends resolve by name
and count nothing yet.

### 3.2 Derived passes

| Pass | Code | Adds |
|---|---|---|
| Sol to TS link | `link.SolToTS` (`link/xlang.go`) | `binds_to` from a Solidity contract to a TS class of the same name (INFERRED) |
| HTTP client matching | `link.MatchHTTPClients` (`link/http_match.go`) | rewires client endpoint placeholders (AMBIGUOUS) to server endpoints |
| Temporal | `emitTemporalEdges` (`buildpipe/temporal.go`), `temporal/git.go`, `temporal_hunks.go` | Commit and Hunk nodes, `changed_in`, `blame`, `has_hunk`, `adjacent`, `modifies`, patch blobs; capped by `--temporal-depth` (10) and 200 commits for hunks |
| Lock propagation | `propagateLockedFieldAccess` (`lock_propagation.go`), `--lock-propagation`, cold only | `accessed_under_lock` across calls, depth 5 |
| Scores | `score.Compute` (`score/score.go`) | degrees, usage score, PageRank (0.85, 30 iterations, Commit and Hunk excluded) |
| Package tree | `cluster.BuildPkgTree` | directory and package hierarchy |
| Topic tree | `cluster.BuildTopicTree` | Leiden communities at gamma 0.5, 1.0, 2.0 (seed 42), named after the highest-PageRank member |

Each pass that adds edges is followed by `validateAndSanitize`.

## 4. Incremental build

```mermaid
sequenceDiagram
  participant I as runIncremental (incremental.go)
  participant S as graph.db.building (a copy of graph.db)
  participant L as runLanguagePipelines
  participant B as graph.Build
  I->>S: delete edges of types changed_in, blame, binds_to, implements, extends, http_calls
  I->>S: ReverseDepsForFiles(changed + removed) via pending_refs target names
  I->>S: DeleteNodesByFilePath(changed + removed), foreign keys cascade
  I->>L: per language with changed or cached files
  L->>L: parse changed files, reload cached nodes, reload pending refs of reverse-dependent files
  L->>L: Resolve over all files, Go post-passes over all nodes, contract records
  I->>S: reloadCachedEdges (per file, plus cross-file edges between cached nodes)
  I->>B: reloaded edges first, then new ones
  I->>I: derived passes (no lock propagation)
  I->>S: insert new nodes and edges, trees, blobs of changed files, FTS, pending refs, overlay, review index, manifest
  I->>I: rename graph.db.building to graph.db
```

Known differences from a cold build (roadmap D10, measured on go-stablenet): duplicate `uses_type` edges, missing
`extends`, `http_calls`, `implements` and `imports` edges, two cross-file calls missing, scores of cached nodes not
rewritten in the store, no PR history and no lock propagation. Evaluation graphs are built with `--no-cache`.

## 5. Stored tables

| Table | Written by | Holds | Read by |
|---|---|---|---|
| `nodes` | `InsertNodes` (cold, overlay, new nodes on incremental); `RebuildReviewIndex` sets `is_test` | every node: id, type, names, canonical id, file, span, language, signature, doc, scores, confidence, sub kind, attributes, search tokens | every surface |
| `edges` | `InsertEdges` | relations: src, dst, type, file, line, count, confidence, dispatch kind | MCP traversals, impact, concurrency, evidence; HTTP `/api/edges`, impact; CLI query, path, report, verify, exports; CKS neighbors, subgraph, impact, review |
| `nodes_fts` | `RebuildFTS` | FTS5 over name, qualified name, signature, doc, search tokens | MCP `search_text`, `get_context_for_task`; HTTP `/api/search`; CKS BM25 search |
| `blobs` | `InsertBlobs` | source slice per node, gzip patch per Hunk | MCP bodies (`include_blobs`), evidence patches, value flow; HTTP `/api/blob`; exports; CKS value flow, review (not composer bodies) |
| `pending_refs` | `InsertPendingRefs` | Pass-1 references kept for incremental re-resolution | the incremental build only |
| `pkg_tree` | `InsertPkgTree` | package hierarchy | HTTP `/api/hierarchy`, `/api/nodes?parent`; export-static |
| `topic_tree` | `InsertTopicTree` | Leiden communities and labels per resolution | HTTP hierarchy and community labels; report, quickstart, export-static |
| `node_prs` | `InsertNodePRs` (cold only) | pull requests whose merge touched a node | MCP `change_history`, `get_context_for_task` (recent PRs); CKS |
| `code_edges`, `node_commits` | `RebuildReviewIndex` | code relations between production nodes; node to commit | CKS `review_function` only |
| `manifest` | `SetManifest` | build metadata (schema, versions, go_build, digests, counts, files, staleness) | readers' snapshot checks, HTTP `/api/manifest`, report, exports |

### 5.1 What the stored tables hold on go-stablenet

Measured on a cold build of go-stablenet 6e41d966 with the linux/amd64 scope (191,860 nodes, 265,620 edges; the
source copy has no git history, so `node_prs`, `node_commits` and the Commit and Hunk nodes are empty). That scope
was schema 1, made without the tags go-stablenet's gstable target uses, and its build still walked the tree for Solidity,
TypeScript, JavaScript and Proto files; a scope from the stablenet profile lists only the Solidity sources its
`system-contracts` inclusion names, so the JavaScript findings below do not arise in it.

Consistent:

- No edge points at a missing node, and no two edges share type, source, destination and line.
- `nodes_fts` has one row per node; every `code_edges` row is also an `edges` row; every `pending_refs` source exists.
- Every Go statement node (CallSite, IfStmt, LoopStmt, ReturnStmt, SwitchStmt, Channel, Goroutine) is contained by
  the function whose byte span holds it.

Not connected, or connected only to its parent:

| What | Count | Why |
|---|---|---|
| nodes with no edge at all | 33 | JavaScript files (rpc and console test data, tracers) that define nothing the TS frontend captures |
| File nodes without a parent | 62 | Solidity and TypeScript files: those frontends emit no Package node |
| AwaitPoint nodes without a parent | 54 | the TS frontend links them to their function by `awaits` only |
| Package nodes | 139 | roots; packages are linked to their files, not to each other |
| Import nodes linked only to their file | 6,870 | an import names a package, but no edge goes from the Import node to that Package node, so package dependencies cannot be walked |
| Variables and Constants linked only to their file | 7,392 and 1,522 | no edge records where a variable or constant is read or written (`writes_field` covers struct fields only; 4,098 of 6,848 Field nodes have no access edge) |
| Go functions and methods with no callee in the graph | 4,478 of 9,507 production ones | calls into the standard library and outside modules are counted (`outside_graph`), not stored |
| statement nodes | 96,149 call sites, all if, loop, return and switch nodes | leaves under their function; calls are lifted to the function, so a statement says where, not what |

Wrong:

- TypeScript and JavaScript declarations store the span of their name, not of their body (`runQuery` in
  `parse/typescript/declarations.go` reads the `@name` capture). Their blobs hold the name only, and none of the
  19,775 TS statement nodes lies inside its parent's span.
- In minified files (`graphql/internal/graphiql/graphiql.min.js`, 18,728 nodes) the TS frontend finds a call's
  function by name and line, which one-line files with names like `i` and `a` cannot tell apart: 4,485 call sites
  keep their own `calls` edge to an unrelated function. The three vendored `*.min.js` files hold 21,757 of the
  graph's 191,860 nodes.
- `references` edges do not exist in the graph, yet `RelationReferences` maps to them in CKS (`planTraversal`), so
  intents that ask for references (feature, refactor, concurrency, security) get nothing from that relation.
- 1,507 self loops: 1,402 `invokes` placeholders for calls through function values, 73 recursive `calls`, and 32
  `timeout_path` or `cancellation_path` edges from a function to itself.

## 6. Query surfaces

### 6.1 `ckg mcp` (stdio)

```mermaid
sequenceDiagram
  participant A as agent (MCP client)
  participant M as ckg mcp (cmd/graph/mcp.go)
  participant H as handler (pkg/graph/mcphandlers)
  participant Q as safeReader (safety.go)
  participant S as persist reader (sqlite_reader.go)
  A->>M: tools/call find_callers {qname, depth}
  M->>H: RegisterAll handlers (12 tools)
  M->>M: LogToolCalls: one record per call (area mcp, request id)
  H->>Q: resolveSeed: canonical id (that node only), then exact qualified name, then suffix match
  alt several qualified names match
    H-->>A: ambiguous + candidates
  else none
    H-->>A: not_found
  end
  H->>H: interface method seeds (1-hop implements)
  H->>Q: NeighborhoodByIDs(canonical seed) or NeighborhoodByQname(seed), depth <= 5, reverse, calls + invokes
  Q->>S: hop by hop: edges by src/dst index, then NodesByIDs
  S-->>Q: nodes, edges (AMBIGUOUS Commit/Hunk nodes removed)
  Q-->>H: subgraph
  H-->>A: {seed_qname, seed_id, nodes, edges} (bodies only with include_blobs)
```

| Tool | Seed | Reads and walk | Limits |
|---|---|---|---|
| `find_symbol` | name (exact by default) | `FindSymbol` | 100 rows, no order |
| `find_callers`, `find_callees` | `resolveSeed` | reverse or forward neighborhood over `calls`, `invokes`; callers add interface method seeds | depth 0 to 5, no node cap |
| `get_subgraph` | `resolveSeed` | forward and reverse walks, all edge types | depth 5, no node cap |
| `search_text` | text | FTS (or substring for non-ASCII) | `top_k` |
| `get_context_for_task` | task text | `smartctx.BuildContext`: search 100, expand hops (frontier 200 per hop), rank 0.5 BM25 + 0.3 PageRank + 0.2 usage, bodies for the top 5, summaries | `budget_tokens` (8000), reports `trimmed` |
| `impact_of_change` | exact qualified name or file | `impact.Compute`: reverse walk, grouped into callers, interface impact, type users, distributed, concurrent, other | depth 1 to 5, no cap |
| `concurrency_impact` | exact qualified name | spawns, sends_to, recvs_from, acquires_lock, accessed_under_lock, both directions | depth 1 to 5, `max_total` |
| `value_dependencies`, `rule_candidates` | node id or canonical id | `valueflow`: blob of one function, CFG and def-use, snapshot pinned by manifest hash | size, block, node, time limits |
| `change_history` | `resolveSeed` | `node_prs` per node | `k` |
| `evidence_for_intent` | intent text, optional seed | all nodes and edges into a hunk corpus with BM25 over commit subjects, patches and modified names | `k`, `budget_tokens` |

### 6.2 `ckg api` (HTTP, loopback only)

```mermaid
sequenceDiagram
  participant V as viewer (browser)
  participant H as server (internal/graph/server)
  participant C as cachedManifestStore
  participant S as persist reader
  V->>H: GET /api/nodes/top?metric=pagerank
  H->>C: TopNodes
  C->>S: nodes ORDER BY pagerank
  S-->>H: nodes
  H->>H: decorateNodes: community and topic label from topic_tree
  H-->>V: JSON
```

Endpoints: `/api/manifest` (with staleness from git), `/api/hierarchy`, `/api/nodes`, `/api/nodes/top`,
`/api/nodes/ambiguous`, `/api/nodes-by-ids`, `/api/edges`, `/api/edges/counts`, `/api/blob/{id}`, `/api/search`
(20 hits), `/api/impact`, `/api/evidence`, `/api/tickets`. Requests with a non-loopback Host header are refused.

### 6.3 CLI

| Command | Reads |
|---|---|
| `ckg query` | all nodes and edges in memory, token scoring, BFS depth 2 from 5 seeds, markdown within `--budget` |
| `ckg path` | all nodes and edges, undirected unweighted BFS, depth 12 |
| `ckg report`, `ckg quickstart` | manifest, nodes, edges, topic tree; quickstart builds first and then serves the API |
| `ckg verify` | nodes and edges by type against go/types truth, with the truth's load errors |
| `ckg validate`, `ckg audit` | nodes and edges; file paths |
| `ckg export-json`, `export-static`, `export-postgres` | everything |
| `ckg evidence` | the evidence pack of section 6.1 |

### 6.4 CKS search composer

```mermaid
sequenceDiagram
  participant A as agent
  participant T as CKS context.get_for_task
  participant P as Composer (internal/system/composer)
  participant V as ckv client
  participant G as ckgclient.Real
  participant S as persist reader
  A->>T: task text
  T->>P: ComposeWithIntent
  P->>P: intent.Classify
  loop stage 1, up to 2 rounds
    P->>V: SemanticSearch(query)
    P->>P: keyword candidates from hits
    P->>G: BM25Search(keyword, 5)
    G->>S: SearchFTS
  end
  P->>V: SemanticSearch for knowledge chunks
  P->>G: stage 2: BM25Search(keyword, 10), FindSymbol(keyword, kinds)
  G->>S: SearchFTS, FindByCanonicalID, then suffix FindSymbol
  P->>P: reciprocal rank fusion (ckv weight 5, BM25 1, symbols 1.5), join by file:start-end
  P->>G: stage 3: Neighbors(citation, relations, 3 hops) for the top 10 seeds
  G->>S: NodesByFilePath, span match, then one NeighborhoodByIDs per hop
  P->>P: budget.Allocate: bodies read from source files (not blobs)
  P-->>A: evidence pack (citations, bodies, graph neighbors, knowledge)
```

The composer joins ckv and ckg results by the citation key `file:start-end`, not by canonical id; ckv hits carry a
canonical id, but the composer uses it only for the ontology boost.

Stage 3 walks three hops for every intent (it was one or two). `Neighbors` starts from the node at the seed's file and
span, walks one hop per query, and stamps each neighbour with the hop it was reached at, so the expander's
`score / (1 + distance)` discounts far neighbours; each hop expands at most the 200 nodes with the highest PageRank,
and the per-seed cap (50) is shared between the hops. On go-stablenet a three-hop walk took at most 0.13 s (15 hub
functions and 40 sampled ones, both directions); from hub functions it returned on average 10, 14 and 14 callers at
hops 1, 2 and 3, where one hop returned 25.

## 7. Planned versus present

| Planned (contract design section 8, flow page section 3) | Present |
|---|---|
| start symbol by exact id, or short name with candidates | `resolveSeed` in four MCP tools; impact, concurrency and evidence take an exact qualified name; CKS returns an error instead of candidates |
| walk filters: edge kind, layer, configuration | edge kind only; no layer or configuration stored (`is_test` read only by review) |
| structure first, bodies on request | mostly; `get_context_for_task` includes bodies by default |
| a token budget on every walk, reporting what was cut | only `get_context_for_task` and evidence; traversals have no node cap or budget |
| in-memory adjacency index, root-cause walk (D14), A* path search (D10) | absent; walks are SQL hop by hop |
| MCP request records with one request id | `ckg mcp` writes one record per tool call (request id, tool, arguments, seconds, result size, outcome); the records of one task are not yet tied together |

## 8. Defects found while mapping

Read in the code, then fixed (2026-11-16; each fix has a test that fails without it):

1. `resolveSeed` (`pkg/graph/mcphandlers/helpers.go`) turned a canonical id into its qualified name, so every node
   sharing that name became a root, against its own comment. Fixed: a canonical id seed walks its own node only
   (`NeighborhoodByIDs`, new in the store reader); responses add `seed_id`.
2. `ckg mcp` discarded the logger it created, so no MCP request was logged. Fixed: `LogToolCalls` writes one record
   per tool call.
3. `ckgclient.Real.Neighbors` reported `Distance: 1` for every neighbour, also for 2-hop results, while stage 3
   scores by distance; it also started from every node with the seed's qualified name. Fixed: it walks hop by hop
   from the matched node and stamps the hop (section 6.4); `GetSubgraph` stamps distances too. Stage 3 walks three
   hops.
4. `--force` did not rebuild an unchanged tree while its help said "rebuild"; `--rebuild-metrics` was never read.
   Fixed: the help text says what `--force` does; `--rebuild-metrics` is deprecated and the unread option removed.
5. Readers open with `immutable=1`, which assumes the file is replaced by rename, while incremental builds and the
   short circuit wrote `graph.db` in place. Fixed: both write `graph.db.building` and rename it.
6. Topic rows are keyed by label (`cluster.TopicTree.ResolutionMembers`), so communities with the same label
   replaced each other. Fixed: repeated labels of one resolution are numbered (schema 1.28). On go-stablenet
   topic_tree held 117,099 rows for 191,860 nodes at three resolutions; it now holds all 575,580, with nodes and edges
   unchanged and `ckg verify` unchanged (0 misresolved static calls). The rows show a second problem, not fixed:
   521,933 of them (91%) are one-node communities, because Leiden sees only calls, invokes, uses_type, implements
   and extends edges, and statement, variable, constant and import nodes have none.

Reported by the mapping, then checked in the code and on the go-stablenet graph (not fixed):

7. Confirmed. `findSymbolQuery` (`persist/sqlite_reader.go`) ends in `LIMIT 100` with no `ORDER BY`, so a name with
   more than 100 rows gets an arbitrary subset, and `NeighborhoodByQname` roots come from the same query. On
   go-stablenet 27 simple names have more than 100 nodes (mostly `import:` nodes, `err`, `_`), 2 of them among
   functions and methods; 17 qualified names have more than 100 nodes, all imports.
8. Confirmed. `normalizeSearchHits` min-max normalises each FTS result set, so the best hit of any non-empty set
   scores 1.0. Stage 1's `rerankScored` keeps the best score per keyword, so every keyword with a hit scores 1.0,
   the sort keeps extraction order, and `computeConfidence` (top over sum) is 1/N: the 0.5 gate passes only with
   two keywords or fewer, so stage 1 nearly always runs its second round.
9. Confirmed. With `--at-commit` the build reads a temporary `git worktree` and records it as `src_root` unless
   `--logical-src-root` is given; the worktree is removed when the build ends, so readers that read bodies from
   `src_root` (the CKS composer) find no files.
10. Confirmed. `manifest.ckgignore` is never set (the stored value is `null`), and on go-stablenet all 197,043
    `edge_ids` in the file entries are 0: `computeColdFileEntries` takes them from the in-memory edges, and
    `InsertEdges` never writes the row ids back.
    Nothing reads `edge_ids`.
