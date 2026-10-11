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
| Evaluation | [`docs/spec-driven/EVAL-CONTRACT.md`](./spec-driven/EVAL-CONTRACT.md), [`docs/graph/EVAL.md`](./graph/EVAL.md), [`docs/vector/eval-metrics.md`](./vector/eval-metrics.md) |

## Design notes

- [`docs/design/`](./design/) — implemented cross-engine designs (CLI
  consolidation, build-scope file lists).
- [`docs/graph/design/`](./graph/design/) — parser and detector designs per language.

## Records

These documents describe work as it happened. They are kept for traceability
and are not updated when the code moves on.

- [`docs/spec-driven/`](./spec-driven/README.md) — the spec-driven refactoring:
  execution spec, task records, evaluation results and the release verdict.
- [`docs/dev/`](./dev/) — dated investigation notes and reviews; superseded notes
  live in [`docs/dev/archive/`](./dev/archive/).
- [`docs/graph/archive/`](./graph/archive/), [`docs/vector/archive/`](./vector/archive/) —
  retired engine documents.
