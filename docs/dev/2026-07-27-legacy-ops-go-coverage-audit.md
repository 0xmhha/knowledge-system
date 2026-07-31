# Legacy operational scripts — Go-coverage audit (2026-07-27)

Status: Tier-3 (dated backlog / status). **백로그 9개 항목 전부 LANDED
(2026-07-30 확인)** — 이 문서가 계속 살아 있는 이유는 아래 **"잔여"** 2건과
처분 기록(Disposition) 때문이다. 그 2건이 닫히면 폐기 가능.

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

## 잔여 (이 문서의 유일한 열린 항목)

3-repo 통합 설계
([`archive/2026-07-28-gap-integration-design.md`](./archive/2026-07-28-gap-integration-design.md))가
2026-07-30에 구현 완료로 닫히면서, 그 §7이 열린 결정으로 남긴 항목이 아래
2건으로 수렴했다. 그중 1건은 2026-07-31에 닫혔다.

1. **`activate.sh` / `apply-cc-settings.sh`의 coding-agent repo 이관** — 열림.
   플러그인 `.mcp.json`이 아직 `${VAR}` 플레이스홀더를 요구하는 동안은
   `activate.sh`가 config에서 파생·export한다. 플러그인이 config를 직접
   소비하도록 바뀌는 시점에 export를 제거하고 두 스크립트를 이관한다.
2. ~~`system/dataset-toolkit/scripts/*` · `vector/scripts/build-vector-stablenet.sh`
   경로 갱신~~ — **종결(2026-07-31).** 전수 검증 결과 **구 3-repo 경로는 이미
   남아 있지 않았다**: 내부 파생은 `gen-dataset-config.sh`의 `KS_ROOT`(repo
   루트), `run-coding-agent.sh`의 `CKS_ROOT`(`system/`, `activate.sh` 실재),
   `build-vector-stablenet.sh`의 `KS_ROOT`(repo 루트)로 모두 정상 해석되고,
   스크립트가 인용하는 repo-상대 경로도 전부 실재한다. 나머지 경로는 호출자가
   env로 주는 값(`SRC`/`OUT`/`DATASET`)이라 레이아웃 문제가 아니다.
   실제로 남아 있던 결함은 경로가 아니라 **`build-vector-stablenet.sh` 헤더의
   재생성 안내 1건**이었고(존재하지 않는 `bin/system-domain-export`, 그리고
   `-code-root` 누락으로 지시대로 실행하면 exit 1), 이번에 교정했다.

## Disposition

- **Retired (2026-07-28, gap-integration — see
  `docs/dev/archive/2026-07-28-gap-integration-design.md` §4):** `serve-cks-http.sh`,
  `cks-mcpd.sh`, `reindex-dataset.sh`, `gen-cks-config.sh`, `cks-health.sh`,
  and `projects/stablenet/scripts/gen-filelist.sh` (superseded by
  `ckg build --files-from-main`). The "cks.env half" judgment above is
  **re-adjudicated as covered**: the env file's consumers are served by
  `system-mcp print-mcp-config` (URL/registration JSON) + the config YAML as
  single source of truth — `activate.sh` now derives its exports from those
  instead of sourcing cks.env, so no env-file generator is needed (and none
  should be added — it would duplicate the namespace-based structure).
- `setup-all.sh` / `build-dataset.sh` were re-pathed to the consolidated
  layout (in-repo builds + `knowledge-setup` delegation) — no longer
  sibling-repo-dependent. `activate.sh` / `apply-cc-settings.sh` remain
  plugin glue (candidates to migrate to the coding-agent repo).
- Still shell, still kept: `coding-agent.sh`, `enable-autopilot.sh` (plugin
  glue), `system/dataset-toolkit/scripts/*` and
  `vector/scripts/build-vector-stablenet.sh` (경로 검증 완료 2026-07-31 —
  위 `잔여` 2 참조).
