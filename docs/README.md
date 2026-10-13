# Documentation index

Start here to find the document that answers your question. When code and a
document disagree, the code wins; the reference documents below are kept in
step with it, while design notes and records describe how a decision was made
at the time.

## Getting started

- [`../README.md`](../README.md) — install, build the binaries, build a
  dataset (`cks setup`), and serve it over MCP.
- [`../tools/setup/README.md`](../tools/setup/README.md) — `ks-setup`, a guided
  setup that builds and verifies a dataset with a live progress view.
- [`../system/README.md`](../system/README.md) — the `cks` command, its
  subcommands and the agent-facing MCP tools.
- [`docs/downstream-sync.md`](./downstream-sync.md) — porting this repository into a
  downstream distribution and verifying the port.

## Reference (kept in step with the code)

| Area | Documents |
|---|---|
| Graph engine (ckg) | [`docs/graph/DOC-MAP.md`](./graph/DOC-MAP.md) (index), [`docs/graph/ARCHITECTURE.md`](./graph/ARCHITECTURE.md), [`docs/graph/SCHEMA.md`](./graph/SCHEMA.md), [`docs/graph/INCREMENTAL.md`](./graph/INCREMENTAL.md), [`docs/graph/adr/`](./graph/adr/) |
| Vector engine (ckv) | [`docs/vector/README.md`](./vector/README.md) (index), [`docs/vector/ARCHITECTURE.md`](./vector/ARCHITECTURE.md), [`docs/vector/SCHEMA.md`](./vector/SCHEMA.md), [`docs/vector/mcp-tools.md`](./vector/mcp-tools.md), [`docs/vector/adr/`](./vector/adr/) |
| Dataset contracts | [`docs/spec-driven/PUBLIC-CONTRACT-V2.md`](./spec-driven/PUBLIC-CONTRACT-V2.md), [`docs/spec-driven/SOURCE-SNAPSHOT-ADR.md`](./spec-driven/SOURCE-SNAPSHOT-ADR.md), [`docs/spec-driven/DOMAIN-PACK-CONTRACT-V1.md`](./spec-driven/DOMAIN-PACK-CONTRACT-V1.md), [`docs/spec-driven/WORKSHEET-CATALOG-CONTRACT-V1.md`](./spec-driven/WORKSHEET-CATALOG-CONTRACT-V1.md) |
| Ontology MCP | [system/docs/ontology-mcp.md](../system/docs/ontology-mcp.md), [tool mapping](../system/docs/coding-agent-mcp-mapping.md) |
| Evaluation | [`docs/spec-driven/EVAL-CONTRACT.md`](./spec-driven/EVAL-CONTRACT.md), [`docs/graph/EVAL.md`](./graph/EVAL.md), [`docs/vector/eval-metrics.md`](./vector/eval-metrics.md) |

## Design notes

- [`docs/design/`](./design/) — implemented cross-engine designs (CLI
  consolidation, build-scope file lists).
- [`docs/graph/design/`](./graph/design/) — parser and detector designs per language.

## Records

The spec-driven index separates live task state from dated evidence. Historical
records describe work as it happened and retain their original limitations.

- [`docs/spec-driven/`](./spec-driven/README.md) — the spec-driven refactoring:
  current handoff, execution spec, worklist, evidence and release verdict.
- [`docs/dev/`](./dev/) — dated investigation notes and reviews; superseded notes
  live in [`docs/dev/archive/`](./dev/archive/).
- [`docs/graph/archive/`](./graph/archive/), [`docs/vector/archive/`](./vector/archive/) —
  retired engine documents.
