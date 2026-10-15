# Architecture and source boundaries

The graph, vector and composition engines share the Go module `github.com/0xmhha/knowledge-system`. Project-specific policies, domain inputs and evaluation fixtures belong in project packs. Runtime implementation is under `internal/<engine>`; cross-engine contracts are under `pkg/`. Engine boundaries are enforced by `scripts/check-boundaries.sh`.

## Build and serving paths

| Path | Current behavior | Source |
|---|---|---|
| Graph | Go/TypeScript/Solidity/Proto parsing, symbols and relationships, temporal history, deterministic graph storage | [graph architecture](./graph/ARCHITECTURE.md) |
| Vector | Code/document/flow chunks, embedding identity, SQLite vector search and canonical symbol alignment | [vector architecture](./vector/ARCHITECTURE.md) |
| Coordinated setup | Capture source, build graph/vector candidates, verify coordinates and gates, promote an immutable version | [setup](../system/docs/SETUP.md), [source contract](./spec-driven/SOURCE-SNAPSHOT-ADR.md) |
| Composition | Classify intent, combine vector/lexical/graph evidence, enforce source validation, body budget, sanitization and integrity | [system](../system/README.md), [public contract](./spec-driven/PUBLIC-CONTRACT-V2.md) |
| Meaning | Source-backed reviewed concepts and relations, bounded optional retrieval, read-only readiness/lookup/trace | [semantic contract](./spec-driven/SEMANTIC-CONTRACT.md), [ontology MCP](../system/docs/ontology-mcp.md) |
| Review | Hold a patch candidate, record exact tests, record separate criterion decisions, then promote | [patch workflow](./spec-driven/PATCH-WORKFLOW.md) |

Dataset identity separates project, snapshot and dataset. A snapshot identifies captured source bytes; a dataset also pins model, knowledge inputs and build recipe. `committed`, `working-tree` and `snapshot-only` have distinct history semantics. Source files are retained as content-addressed blobs; moving or deleting the original checkout does not justify reading unrelated current bytes. The pinned MCP server reads verified blobs directly for citation checks and body selection; it does not create a temporary source tree that must be recursively deleted at shutdown.

## Safety and publication boundaries

Versioned builds use shared writer locking, candidate verification, durable pointer transitions and reader leases. Garbage collection protects current, rollback, held-review and live-reader references. Flat legacy maintenance and filesystem body reads are retained; checkout drift warnings do not inherit immutable-version guarantees. See [body provenance](../system/docs/design-pack-body-provenance.md). Low-level engine writes are outside coordinated management.

An aligned semantic store can be readable while ontology retrieval is off. Verified input means reviewed content under a recorded snapshot, not independently established answer quality. Trace links, test execution and criterion acceptance are separate records. No automatic review, organization authentication, independent quality approval or deployment follows from a successful readiness probe.

The current decisions and remaining scope are in [purpose and scope](./spec-driven/PURPOSE-AND-SCOPE.md), [remaining work](./REMAINING-WORK.md) and [release policy](./spec-driven/RELEASE-POLICY.md).

The requested next scope is a [Claude knowledge manager, continuous updates and cross-project comparison](./spec-driven/KNOWLEDGE-MANAGER-AND-COMPARISON-SPEC.md). It is a design and implementation backlog, not a currently available federation/agent capability. Existing tuple-scoped citations, reviewed packs and coordinated version builds are foundations. The new manager requires version-aware maintenance tools: current ops.setup/reindex MCP tools remain legacy-only and must not bypass pinned-data write guards. Ontology effect evaluation is a required priority item under that roadmap.
