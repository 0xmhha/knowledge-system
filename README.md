# knowledge-system

Build queryable knowledge from a source tree. Three engines compose:

- **graph/** — parses source (Go / TypeScript / Solidity) into a deterministic
  graph DB: symbols, call edges, change history. SQLite by default,
  PostgreSQL opt-in.
- **vector/** — semantic search over the same source: embeddings plus a
  vocabulary bridge from natural-language queries to exact code keywords.
- **system/** — orchestrator that fuses graph and vector retrieval behind a
  single MCP server, so coding agents get precise code context in one call.

The three engines were formerly separate repositories
(`code-knowledge-graph`, `code-knowledge-vector`, `code-knowledge-system`)
and are consolidated here into a single Go module with full history.

## License

AGPL-3.0 — see [LICENSE](LICENSE).
