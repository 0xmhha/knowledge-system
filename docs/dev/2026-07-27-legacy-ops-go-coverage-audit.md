# Legacy operational scripts — Go-coverage audit (2026-07-27)

Status: Tier-3 (dated backlog / status). Disposable once the backlog below is
worked off and the scripts are retired.

## Why this exists

The top-level engine dirs still carry the pre-consolidation (3-repo era)
operational shell scripts (`system/scripts/`, `vector/scripts/`,
`graph/scripts/`, `system/dataset-toolkit/`, `system/activate.sh`). The plan
was to archive them as "superseded by the Go pipeline" (`knowledge-setup`,
`ops.setup`/`ops.index`, `system-mcp`, `ckg`/`ckv` subcommands). A rigorous,
line-by-line comparison of each script against the Go code showed **most are
only partially covered** — they still encode reproducibility / orchestration
logic that Go has not absorbed. **Decision: keep them; do not archive.** Retire
each script individually when its Go equivalent lands (the backlog below).

The scripts default to the old 3-repo layout (`../code-knowledge-graph`,
`bin/ckg`/`bin/ckv`/`bin/cks-mcp`). That path staleness is a **separate,
lower-priority** issue — it does not change the coverage judgment (the engines
are now in-repo as `internal/graph` + `internal/vector`, exposed by
`cmd/graph`=`ckg`, `cmd/vector`=`ckv`, `cmd/system-mcp`).

## Coverage verdict (per script)

| Script | Go coverage | Uncovered logic (lost if archived) |
|---|---|---|
| `system/scripts/cks-health.sh` | **full** | none — health is computed by the `cks.ops.health` tool (`internal/system/mcp/health.go`); the script is a thin stdio client |
| `vector/scripts/build-vector-stablenet.sh` | **core** | embedding (multi `--docs` + `--flow-corpus`) is fully covered by `vector build`; only `DRY_RUN`, input-existence preflight, and a `chunk_kind` summary remain (convenience) |
| `graph/scripts/index-project.sh` | partial | `--files-from` generation from a **main-package closure** (`go list -deps <MAIN_PKG>` → module-relative `*.go` globs + Solidity include + extra exclude). `internal/graph/filterlist` only *consumes* the JSON; generation is unimplemented |
| `vector/scripts/build-knowledge.sh` | partial | ollama-reachability + model-existence preflight; ckg `schema_version >= 1.19` gate; `canonical_id` match-rate + flow/doc chunk-count verify; `wal_checkpoint(TRUNCATE)` + sha256 publish |
| `system/scripts/reindex-dataset.sh` | partial | coordinated **blue-green reindex**: digest-named version dirs, a 5-part gate (`ckg validate` / manifest-align / chunk-count / B7 canonical match-rate / `ckg audit`), atomic `current` promote **for the graph side too**, per-family build lock, rollback, serving restart. Primitives exist (`ckg validate`, `ckg audit`, `ckv promote`, `TestLive_B7`) but nothing wires them into the pipeline |

Kept as-is (no Go equivalent, and mostly out of engine scope):
`gen-cks-config.sh` (fused-server config generation — `config.go` only `Load()`s),
`serve-cks-http.sh` / `cks-mcpd.sh` (multi-instance HTTP daemon management),
`setup-all.sh` (prereqs / ollama pull / binary builds / config orchestration),
`apply-cc-settings.sh` / `coding-agent.sh` / `enable-autopilot.sh` /
`activate.sh` (Claude Code env + launch glue — belongs to the coding-agent
plugin, not an engine), `system/dataset-toolkit/scripts/*` (specialized
dataset prep), `vector/scripts/pr-retrieval-eval.sh` (a specific eval harness;
`ckv eval` is close but uses a different fixture format).

## Go-refactoring backlog (retire the script when its item lands)

1. **graph `--files-from` generation** — a `ckg`/`knowledge-setup` mode that
   builds the include/exclude list from a binary's package closure
   (`go list -deps`), matching `index-project.sh`. (retires `index-project.sh`)
2. **vector build preflight** — ollama reachability + model existence; ckg
   `schema_version >= 1.19` gate before aligning. (part of `build-knowledge.sh`) [LANDED: ollama reachability + model existence — internal/setup preflight]
3. **vector build verify** — surface `canonical_id` match-rate and flow/doc
   chunk counts in the build summary. (part of `build-knowledge.sh`) [LANDED]
4. **dataset publish** — `wal_checkpoint(TRUNCATE)` + sha256 manifest as a
   `knowledge-setup`/`ckv` step. (part of `build-knowledge.sh`) [LANDED]
5. **fused-server config generation** — a Go generator for the `system-mcp`
   config + env (today only `config.Load` exists). (retires `gen-cks-config.sh`)
6. **reindex orchestration** — wire `ckg validate` / manifest-align /
   chunk-count / B7 / `ckg audit` into a coordinated blue-green pipeline with
   digest-named version dirs, atomic `current` promote for **both** engines
   (graph promote is missing; `ckv promote` is vector-only), a per-family build
   lock, rollback, and serving restart. (retires `reindex-dataset.sh`)
7. **verify-align schema gate** — `internal/setup/verify.go` reads
   `SchemaVersion` but never asserts `>= 1.19` / ADR-007 canonical_id; add the
   assertion (a coverage regression vs `reindex-dataset.sh` gate 2). [LANDED]
8. **multi-instance MCP daemon management** — start/stop/list several
   `system-mcp` HTTP instances. (retires `cks-mcpd.sh` / `serve-cks-http.sh`)
9. **semantic-validation harness** — reconcile `build-knowledge.sh`'s
   paraphrase→expected-file check with `ckv eval` (fixture format differs).

Out of Go scope (stay shell / move to the coding-agent plugin, not archived
here): `activate.sh`, `apply-cc-settings.sh`, `coding-agent.sh`,
`enable-autopilot.sh`.

## Disposition

- **Archive: none now.** The scripts are stale-pathed but functionally
  load-bearing.
- Retire each script in the same change that lands its backlog item above.
- The 3-repo path staleness is tracked separately and is lower priority than
  the coverage gaps.
