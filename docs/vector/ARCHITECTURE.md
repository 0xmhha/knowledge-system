# Vector architecture

CKV supplies dense retrieval over code, documents and optional curated flow/history inputs. It lives in the same Go module as CKG and CKS. CKS composes dense and lexical/graph evidence; CKV similarity scores are not factual approval.

| Package | Responsibility |
|---|---|
| cmd/vector | Build/query/reindex/model/MCP/eval commands |
| pkg/vector/types | Chunk, hit, filter, embedder and storage contracts |
| pkg/vector/ckv | Public in-process open/search/warmup facade |
| pkg/vector/mcp | Registered vector MCP tools |
| pkg/vector/embed/ollama | Default HTTP embedding adapter and identity validation |
| internal/vector/discover, parse, chunk | Scope/ignore handling, language parsing and span-to-chunk generation |
| internal/vector/build, query | Write/read orchestration |
| internal/vector/ckgalign | Graph canonical symbol alignment |
| internal/vector/store/sqlitevec, manifest | SQLite vector storage and persisted index identity |
| internal/vector/flowcorpus | Explicit curated flow corpus ingestion |
| internal/vector/embed | Mock/optional native adapters and model registry/helpers |

## Write and read paths

Build detects eligible inputs, parses/chunks code and documents, applies explicit project guidance and prefix policy, embeds, persists and stamps a manifest. Incremental reindex updates changed/deleted input records. Canonical alignment uses graph symbol identity, not the retired positional graph-node column. Versioned CKS setup coordinates immutable candidates; direct vector maintenance does not provide the complete CKS promotion/recovery contract.

Query embeds with an identity matching the index, applies filters and bounded retrieval, and returns source citations. [SCHEMA.md](./SCHEMA.md) owns field/storage versions. [MCP reference](./mcp-tools.md) owns tool arguments; [integration](./embedder-integration.md) owns backend choices.

## Current decisions

SQLite vector storage is the default. Ollama/BGE-M3 is the default embedding path; mock is for deterministic structure tests. BGE ONNX requires its explicit build tag and native libraries. The native CoreML package is not wired into the general embedder dispatch. The LRU wrapper is not automatically applied by the CLI. Optional contextual-prefix generation is separate from deterministic prefix policy and is not enabled as a general quality guarantee.

CKV and CKG join by canonical_id. Project policy belongs in selected packs. Full PR evaluation must distinguish code history and the time scope of document/flow facts; frozen source coordinates do not by themselves establish historical knowledge validity. Large-corpus throughput, long-function splits and independent flow accuracy remain separately scoped measurements in [remaining work](../REMAINING-WORK.md).
