# Spec-driven WBS execution log

Source baseline: `main` at `1ded9b3e47bc2e09062dba329c2f918423746fa4` (2026-09-29). Implementation branch: `feat/spec-driven-knowledge-system` in a separate worktree. Planning documents are currently in the `study/docs/reviews/knowledge-system/` workspace; this log records code changes and gate evidence in the product repository.

## Gate 0: baseline and correctness

| WBS | Status | Evidence | Remaining gate work |
|---|---|---|---|
| W0.1 | Structural baseline complete; real-model quality baseline pending | `scripts/wbs-smoke.sh` builds a tiny committed fixture through `cks setup` with the deterministic mock backend, audits CKG, and evaluates one answerable and one no-citation case through CKS MCP. This checks wiring and citations, **not semantic quality**. | Run CKV+CKS scenarios with a real embedding model and pinned question set; record Recall/MRR/p95, model identity, hardware. |
| W0.2 | Implemented and focused tests passed | A failing fixture proved that historical `Hunk` paths entered the SQLite current-file set. `DistinctFilePaths` now selects `File` nodes in SQLite and PostgreSQL. The previously built target graph audits at `build_count=877`, `db_count=877`, `in_both=877`. | Full regression and race gate. |
| W0.3 | Project-scale structural alignment measured; real-model and multi-project cases pending | `cks setup` indexed this project at commit `41fa881`: 883 Go files in CKG audit (883/883 parity), 12,784 CKV chunks, 7,567 code-symbol chunks, 7,134 genuinely aligned symbols (94.28%), zero orphan canonical IDs, one consistent source commit. The original manifest claimed 7,421 aligned symbols because it counted 285 aligned file headers and two invariants in the numerator. `scripts/wbs-alignment-report.py` exposes and fails that mismatch. Build and reindex now derive symbol and canonical counts from the authoritative store validation. Eight existing CKG schema warnings remain for call endpoints (Interface/Modifier). | Rebuild with corrected manifest, multi-project isolation fixture, real-model semantic quality and long-doc cases. |
| W0.4 | Implemented and focused tests passed | Explicit `expect_no_citations` scenario guard, report result, CLI failure on violation; empty expected citations no longer reward unrelated actual citations with precision 1. | Wider no-answer/conflict dataset with real embedder. |

An additional baseline failure was found: without a writable Go build cache, the Go package loader previously produced `go=0` while `cks setup` succeeded on a Go fixture. Fileless package list errors now fail the build. The same no-cache fixture run now exits nonzero; with a writable cache it detects one Go file and passes graph audit. This prevents an empty-but-successful index.

## Gate 1: CKV retrieval

| WBS | Status | Evidence | Remaining gate work |
|---|---|---|---|
| W1.1 | Initial contract defined | `types.Filter.Matches` remains the final predicate; SQL selection pushes only equivalent metadata constraints. | Lock real-corpus oracle and p95 budget. |
| W1.2 | Initial implementation and focused test passed | A new test first reproduced `K=2` returning 0 hits with 12 excluded global neighbors. Small filtered candidate sets now use exact distance search; larger sets use vector search and exact fallback if results are short. | Large-set path, combined filters, exact-oracle and performance tests. |
| W1.4–W1.5 | Parsing, splitting, storage, and bounded CKS context implemented; real-model A/B pending | Long Markdown sections now split without losing the tail under a capped embedder. Children keep original line ranges, stable parent ID, parent line range, heading path and ordinal; SQLite persists these fields. A repeated oversized line first exposed ID collisions; byte-offset fragment IDs now keep children distinct. CKV carries parent bounds through CKS; the budget allocator fetches up to six neighboring lines on each side as separately cited bodies, only after the matched child is selected. An older chunk table without the new columns reopens and migrates idempotently. | Real-model tail-query A/B, code-fence/list golden fixtures, context selection under crowded budget. |
| W1.3, W1.6–W1.7 | Pending | — | No real-model quality or performance gate passed yet. |

## Reproducible commands

Set writable Go caches when the host's defaults are sandboxed:

```sh
export GOCACHE=/private/tmp/ks-wbs-go-cache
export GOMODCACHE=/private/tmp/ks-wbs-mod-cache
make build-bins
scripts/wbs-smoke.sh
go test ./internal/graph/detect ./internal/graph/audit ./internal/graph/persist ./internal/vector/store/sqlitevec ./internal/system/eval
```

The smoke script writes its dataset and reports to a temporary directory and prints its path. `mock-feature-hash-v1` is a deterministic test backend. A real-model evaluation is required before turning quality changes on by default. Full `go test ./...`, race, vet, docs, boundaries, and installation platform gates are tracked separately; passing the focused suite does not imply a phase is complete.

The whole-module `go test ./...` passed after the parent-context change (2026-09-29); `go vet ./...`, `make boundaries`, and `make docs-check` also passed. The structural smoke script passed again with one indexed Go file and seven vector chunks. In this host's restricted environment, use the writable Go cache paths above and `GIT_CONFIG_COUNT=1 GIT_CONFIG_KEY_0=commit.gpgsign GIT_CONFIG_VALUE_0=false` for test fixtures that create Git commits. The real-model quality gate remains open.
