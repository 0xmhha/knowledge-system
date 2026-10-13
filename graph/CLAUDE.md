# Working agreement for the graph engine

This engine is part of the consolidated knowledge-system module. Start from [purpose](../docs/graph/VISION.md), [current architecture](../docs/graph/ARCHITECTURE.md), [remaining work](../docs/REMAINING-WORK.md) and [current handoff](../docs/spec-driven/CURRENT-HANDOFF.md).

## Verify against the source

Run commands below from the repository root. go.mod declares Go1.25.13; the pinned whole-module linter requires Go1.26, and tool-driven CI uses Go1.26.9. Native/vector CI still verifies the declared minimum version.

```sh
GOTOOLCHAIN=go1.26.9 make build
GOTOOLCHAIN=go1.26.9 make lint
GOTOOLCHAIN=go1.26.9 make test-race
make -C graph eval
make docs-check
make refactoring-check
```

Graph-scoped targets are in graph/Makefile. The API command is ckg api; the unified dashboard is served by cks viewer. Build dashboard assets with `make -C graph viewer`, then rebuild the root engine binaries. Viewer tooling has its own Node dependencies and checks.

## Boundaries and honest completion

Implementation lives under internal/graph; consumers use pkg/graph and pkg/bm25. The graph build remains deterministic and LLM-free. Preserve compatibility, reader/writer round-trips and cache invalidation when schemas change. Actual registered tools and CLI help take precedence over prose.

Purpose, active contracts, limitations and unfinished work remain necessary. Completed plans/session narratives may be removed after their relevant decisions and Git recovery coordinates are recorded. Update documentation indexes and the catalog together; user-authorized cleanup does not require an extra inferred approval. Retained raw evaluation/source snapshots and accepted input bytes stay unchanged. Original evidence commits remain available after squash merges.

Never turn linked into tested or accepted, or inconclusive into quality success. Check completed tasks with the progress guard. Respect the user's commit, push and publication scope; branch before editing main, use concise English conventional messages, and do not add co-author trailers. Do not perform unrelated host, model or dataset operations during a documentation task.
