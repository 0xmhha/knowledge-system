# Graph architecture

CKG is the graph engine in the consolidated knowledge-system module. `cmd/graph` owns the CLI, `internal/graph` implements parsing/build/storage, and `pkg/graph` exposes the contracts consumed by the other engines. Shared lexical ranking is in `pkg/bm25`.

## Source to graph

The build pipeline detects source files, parses Go/TypeScript/Solidity/Proto, resolves and links references, composes graph relationships, clusters and scores, then persists the result. Incremental routing and input-sensitive cache keys are described in [INCREMENTAL.md](./INCREMENTAL.md). Policy/security overlays and cross-function lock propagation are explicit build options; they are not independent policy verification or a general dataflow/taint engine.

Structural, semantic, execution, concurrency, distributed and temporal relations use the types in [SCHEMA.md](./SCHEMA.md). Schema/version/type counts belong there rather than being repeated in each guide. Canonical identity joins graph symbols to vector chunks; internal node IDs are not the cross-engine join contract.

SQLite is the default store. The PostgreSQL integration is opt-in; supported selection and connection arguments come from `ckg build --help`. Graph builds are deterministic and do not call an LLM. Optional native/parser coverage and unresolved dynamic calls must not be interpreted as complete whole-program semantics.

## Public surfaces

- `ckg build`: build or update a graph.
- `ckg api`: HTTP graph API; `cks viewer` serves the unified dashboard.
- `ckg mcp`: stdio graph tools, registered in [RegisterAll](../../pkg/graph/mcphandlers/registerall.go).
- `ckg audit` and `ckg validate`: source/database parity and graph consistency.
- `ckg eval-retrieval`, `ckg benchmark`, `ckg bench-mcp`: deterministic measurement tools; [evaluation](./EVAL.md) records their limits.

The twelve graph MCP registrations include symbol/callers/callees/subgraph/text/context/impact/concurrency/history/evidence queries and the bounded retained Go [value/control dependence query](./VALUE-DEPENDENCIES.md), plus opt-in [Go rule candidates](./GO-RULE-CANDIDATES.md). Storage-reader validation is shared by registration; a custom integration must preserve the retrieval boundary.

## Developer boundaries

Consumer engines use `pkg/graph` rather than importing graph implementation internals. New fields must round-trip through readers/writers and supported old stores. A cache schema change and a public manifest compatibility change are distinct decisions. [Verification checklist](./VERIFICATION-CHECKLIST.md) covers surface and negative-path testing; [purpose](./VISION.md) defines the responsibility boundary.

Current graph support does not establish independent coding-agent answer quality, external benchmark completion, or whole-program CFG/PDG precision. The on-demand Go layer has explicit scalar/normal-exit, type/import, alias/effect and cost boundaries; follow-ups remain in [remaining work](../REMAINING-WORK.md).

CKS exposes33 tools, including the read-only [policy/security annotation query](./POLICY-SECURITY.md). CKG remains12 tools; this query reuses the retained graph through CKS.
