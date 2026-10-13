# Body provenance: current guarantees and the legacy gap

2026-10-13 · Current source: [MCP startup](../../cmd/cks/mcpcli/serve.go), [filesystem fetcher](../../internal/system/composer/budget/filesystem.go), [alignment](../../internal/system/mcp/alignment.go), [v2 pack](../../internal/system/evidencev2/pack.go).

| Serving path | Body source and guarantee | Limit |
|---|---|---|
| Coordinated immutable version with retained sources | Startup materializes the captured read tree and uses it as the fetch root. Public v2 evidence revalidates identity and captured bytes. | DocsRoots remains a separate document-source boundary; retained code is not a claim that arbitrary external docs are captured. |
| Legacy flat graph/vector config | FilesystemFetcher reads source_root (or cwd) and document roots. Alignment gates graph/vector coordinate mismatches. | Checkout HEAD/path drift is a warning; it does not replace the fetcher or independently prove every served body has indexed bytes. |

The old proposal for Store.BodyByRange, a CKV FetchBody API, SnapshotFetcher/FallbackVerified, and a health body_provenance field is **not implemented** under those interfaces. Retained-source v2 serving resolves much of the original code-body problem by a different design. Do not claim a snapshot-first vector-chunk fetcher or a verified legacy filesystem fallback exists.

A future legacy-hardening task must first decide whether to retain that serving mode. If retained, bind every served body to indexed bytes or return citation-only on a miss; cover code and DocsRoots, SQL/fetch failures, missing spans, changed checkout and health provenance. A HEAD match alone cannot verify dirty working-tree or external document bytes. Source span, public sanitized text and content hashes must keep their separate contracts.

This is a separately scoped follow-up in [remaining work](../../docs/REMAINING-WORK.md), not a reason to mark the existing v2 task incomplete or to call the legacy proposal complete. The original design is recoverable at [the pre-cleanup revision](https://github.com/0xmhha/knowledge-system/blob/9a39718807ed25a0add30405f7cd1f1ebff4a629/system/docs/design-pack-body-provenance.md).
