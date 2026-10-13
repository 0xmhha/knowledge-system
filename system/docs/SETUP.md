# Coordinated dataset setup

The supported entry points are the guided tools/setup/ks-setup workflow and the Go cks setup command. They build a graph/vector dataset from explicit source inputs; a model/daemon is required only for the chosen embedding backend. Mock is for wiring checks, not semantic-quality evaluation.

## Prepare and build

Build engine binaries with the root Makefile, then inspect current help:

```sh
make build-bins
bin/cks init --help
bin/cks setup --help
bin/cks doctor --help
```

cks init records stable project/source configuration. For a versioned dataset, specify the reviewed configuration, source and a new version name:

```sh
bin/cks setup --config /abs/setup.yaml --src /abs/source --out /abs/dataset --version next
```

committed captures a clean Git source; working-tree captures declared modified/new bytes with a separate snapshot identity; snapshot-only supports a non-Git source and does not invent history. [Source contract](../../docs/spec-driven/SOURCE-SNAPSHOT-ADR.md) specifies capture, mode/byte pinning and independent history replay. The captured build is separate from later changes to the original checkout.

The builder validates engine identities, model bytes/dimensions/prefix, selected knowledge lock, retained source and optional test/coverage gates. It promotes only a complete valid candidate. hold-for-review leaves a candidate inactive. Errors, limits, timeout and cancellation preserve earlier active data within the documented transition boundary.

## Configure and query

```sh
bin/cks mcp gen-config --dataset-dir /abs/dataset --name local --out /abs/cks.yaml
bin/cks mcp --config /abs/cks.yaml
```

The default binding is loopback. Remote access is an explicit opt-in and generated client registration must match the actual runtime host/port. [Root quick start](../../README.md), [tool mapping](./coding-agent-mcp-mapping.md) and [availability](./ops-availability.md) cover access and service management.

Ontology/spec inputs and semantic store/mode/budget options are explicit. Extraction, review, storing, readiness and retrieval activation are separate steps. A mode request is not a review approval. [Ontology MCP](./ontology-mcp.md) describes readiness and reads with mode off. Knowledge content is included only when explicitly requested under its reviewed scope/date.

For refresh/rollback use [versioned operations](./ops-blue-green-reindex.md), not an in-place write to pinned active data. For independent effect/scale or new OS/model support, follow [remaining work](../../docs/REMAINING-WORK.md); an installation smoke does not close those scopes.
