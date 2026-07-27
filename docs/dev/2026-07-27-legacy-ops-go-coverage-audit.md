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
`setup-all.sh` (prereqs / ollama pull / binary builds / config orchestration),
`apply-cc-settings.sh` / `coding-agent.sh` / `enable-autopilot.sh` /
`activate.sh` (Claude Code env + launch glue — belongs to the coding-agent
plugin, not an engine), `system/dataset-toolkit/scripts/*` (specialized
dataset prep), `vector/scripts/pr-retrieval-eval.sh` (a specific eval harness;
`ckv eval` is close but uses a different fixture format).

Now superseded by the Go path (retire on the next cleanup, per backlog items
5/6/8 above): `serve-cks-http.sh` / `cks-mcpd.sh` (→ `system-mcp daemon`),
`reindex-dataset.sh` (→ `knowledge-setup --version` / `ops.reindex`). Only the
`cks.env` env-file half of `gen-cks-config.sh` still lacks a Go equivalent
(`system-mcp gen-config` covers the config YAML + LAN autodetect).

## Go-refactoring backlog (retire the script when its item lands)

1. **graph `--files-from` generation** — a `ckg`/`knowledge-setup` mode that
   builds the include/exclude list from a binary's package closure
   (`go list -deps`), matching `index-project.sh`. (retires `index-project.sh`) [LANDED — graph build --files-from-main; index-project.sh retired]
2. **vector build preflight** — ollama reachability + model existence; ckg
   `schema_version >= 1.19` gate before aligning. (part of `build-knowledge.sh`) [LANDED: ollama reachability + model existence — internal/setup preflight]
3. **vector build verify** — surface `canonical_id` match-rate and flow/doc
   chunk counts in the build summary. (part of `build-knowledge.sh`) [LANDED]
4. **dataset publish** — `wal_checkpoint(TRUNCATE)` + sha256 manifest as a
   `knowledge-setup`/`ckv` step. (part of `build-knowledge.sh`) [LANDED]
5. **fused-server config generation** — a Go generator for the `system-mcp`
   config + env (today only `config.Load` exists). (retires `gen-cks-config.sh`) [LANDED: `system-mcp gen-config` (config YAML) + `--lan` LAN-IP autodetect
   (`internal/system/netutil`, #15). The manifest-consistency assertion is
   **superseded by design** — the server derives `source_root` from the graph
   manifest when config leaves it empty (#14), so there is nothing to cross-check.
   Only the `cks.env` env-file half remains shell, so `gen-cks-config.sh` is kept
   for that alone.]
6. **reindex orchestration** — wire `ckg validate` / manifest-align /
   chunk-count / B7 / `ckg audit` into a coordinated blue-green pipeline with
   digest-named version dirs, atomic `current` promote for **both** engines
   (graph promote is missing; `ckv promote` is vector-only), a per-family build
   lock, rollback, and serving restart. (retires `reindex-dataset.sh`)
   [LANDED (orchestration): `knowledge-setup --version/--rollback` in
   internal/setup/reindex.go — versioned build → gate suite (ckg validate,
   verify-align commit+digest+schema, vector chunk_count>0, canonical-ratio in
   place of the B7 live test, soft ckg audit) → **dataset-level** atomic
   `current` swap (one flip covers graph+vector, so no separate graph promote)
   → advisory lock → rollback. Instance-level blue-green serving restart now
   LANDED too: `system-mcp daemon reload` health-gates a green instance on
   `/healthz` before swapping (#16), and reindex is exposed as the async
   `ops.reindex` MCP tool (#18). `reindex-dataset.sh` is now fully superseded
   (retire on the next cleanup).]
7. **verify-align schema gate** — `internal/setup/verify.go` reads
   `SchemaVersion` but never asserts `>= 1.19` / ADR-007 canonical_id; add the
   assertion (a coverage regression vs `reindex-dataset.sh` gate 2). [LANDED]
8. **multi-instance MCP daemon management** — start/stop/list several
   `system-mcp` HTTP instances. (retires `cks-mcpd.sh` / `serve-cks-http.sh`)
   [LANDED: `system-mcp daemon up|down|reload|start|stop|restart|status|list`
   (internal/system/daemon) supervises named instances via pidfiles, driven by
   an `instances.yaml` registry with auto-port picking (#15), LAN-IP autodetect
   (`internal/system/netutil`), health-gated blue-green reload on `/healthz`
   (#16), and `print-mcp-config` for client registration (#18). `cks-mcpd.sh` /
   `serve-cks-http.sh` are now fully superseded (retire on the next cleanup);
   only the coding-agent plugin's own MCP-client wiring stays shell.]
9. **semantic-validation harness** — reconcile `build-knowledge.sh`'s
   paraphrase→expected-file check with `ckv eval` (fixture format differs).
   [LANDED — `ckv eval --fixture <semantic.json>` accepts the JSON set
   (substring-in-top-k) with a `--min-pass-rate` gate]

Out of Go scope (stay shell / move to the coding-agent plugin, not archived
here): `activate.sh`, `apply-cc-settings.sh`, `coding-agent.sh`,
`enable-autopilot.sh`.

## Disposition

- **Ready to retire (Go equivalent landed):** `serve-cks-http.sh`, `cks-mcpd.sh`,
  `reindex-dataset.sh`, and `index-project.sh` (already removed). The MCP-server
  hardening sequence (#14–#18, 2026-07-27) closed backlog items 5/6/8; only the
  `cks.env` half of `gen-cks-config.sh` and the plugin-glue scripts still lack a
  Go path.
- Retire each remaining script in the same change that lands its backlog item above.
- The 3-repo path staleness is tracked separately and is lower priority than
  the coverage gaps.
