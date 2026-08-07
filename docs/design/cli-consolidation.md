# CLI consolidation: 17 binaries → 3 engine commands

Status: IMPLEMENTED — 2026-08-07 (all three phases plus the §11 viewer
amendment landed; pre/post behavioral equivalence verified by side-by-side
execution against the pre-refactor baselines in both distributions)
Owner: repo operators
Scope: upstream-first change; applied to both knowledge-system and
stablenet-knowledge-mcp in lockstep, like the pflag migration.

## 1. Motivation

The repository ships three engines but seventeen binaries. Operating the
system today means knowing which of seven dataset-toolchain binaries must
exist in `bin/`, exporting `PATH` so `knowledge-setup` can find five of
them, and telling apart `graph-mcp` / `vector-mcp` / `system-mcp` /
`ckg mcp` / `ckv mcp`, two of which duplicate two others with a different
implementation. Every one of these seams surfaced as a real operator
failure during the 2026-08 README verification pass (missing-binary
refusals, PATH resolution errors, wrong `--src` diagnosis).

The architecture is "three engines compose" (graph, vector, system). The
CLI should say the same thing: **three binaries, one per engine, each a
cobra command tree.**

## 2. Current inventory

| Binary (17) | Parsing | Role |
|---|---|---|
| `graph` (built as `ckg`) | cobra | graph engine CLI: build/query/serve/mcp/eval/... |
| `vector` (built as `ckv`) | cobra | vector engine CLI: build/query/mcp/model/... |
| `graph-mcp` | pflag | standalone namespaced graph MCP server (pkg/graph/mcphandlers) |
| `vector-mcp` | pflag | standalone namespaced vector MCP server |
| `system-mcp` | pflag | fused MCP server + hand-rolled subcommands gen-config / daemon / print-mcp-config |
| `knowledge-setup` | pflag | dataset build orchestrator (graph → vector → verify) |
| `filelist-gen` | pflag | build-scope file list derivation |
| `eval-gate` | pflag | eval drift comparator (CI gate) |
| `cmd/system/*` ×9 | pflag | domain-export, domain-sync, glossary-gen, entry-verify, inventory-check, anchor-refresh, promotion-worksheet, agent, eval |

Pain points, concretely:

- `knowledge-setup` needs 5 sibling binaries resolved on PATH or via 5
  `--*-bin` flags (wrapper script exists solely to pass them).
- `make build-dataset-bins` (7 outputs) and `make build-mcp` (3 outputs)
  are two build stories for one system.
- `ckg mcp` (internal/graph/mcp, un-namespaced, legacy) and `graph-mcp`
  (pkg/graph/mcphandlers, namespaced) are different servers with the same
  purpose; same for `ckv mcp` / `vector-mcp`.
- `ckg serve` serves the human dashboard, not MCP — the verb hides what
  is served.

## 3. Target shape

Three binaries. Everything below is a cobra command.

```
ckg                          # graph engine
├── build / query / eval / validate / watch / report / export / ...
├── viewer [--open]          # 3D viewer + REST API   (was: ckg serve)
└── mcp                      # namespaced graph MCP server (absorbs graph-mcp)

ckv                          # vector engine
├── build / query / eval / freshness / model / migrate / promote / reindex / ...
└── mcp                      # namespaced vector MCP server (absorbs vector-mcp)

cks                          # system engine — everything that composes engines
├── mcp                      # fused MCP server, foreground (was: system-mcp)
│   ├── up|down|reload|status|list    # supervised instances (was: daemon ...)
│   ├── gen-config           # write server config          (was: gen-config)
│   └── client-config        # client registration JSON     (was: print-mcp-config)
├── setup                    # dataset build orchestration  (was: knowledge-setup)
├── filelist                 # build-scope derivation       (was: filelist-gen)
├── eval                     # scenario eval harness        (was: cks-eval)
├── eval-gate                # drift comparator, CI          (was: eval-gate)
├── agent                    # coding-agent CLI             (was: cks-agent)
└── domain [--project P]     # domain-knowledge toolchain (--project is a
    │                        # persistent flag — see D8)
    ├── export               # was: cks-domain-export
    ├── sync                 # was: cks-domain-sync
    ├── glossary-gen         # was: cks-glossary-gen
    ├── verify               # was: cks-entry-verify
    ├── check                # was: cks-inventory-check
    ├── anchors              # was: cks-anchor-refresh
    └── worksheet            # was: cks-promotion-worksheet
```

### Naming rules (to be documented in CLAUDE.md)

1. **`mcp`** names the MCP protocol server, identically on all three
   engines. `serve` is retired.
2. **`viewer`** names the human-facing dashboard (ckg only).
3. **`up/down/reload/status/list`** exist only where a supervisor exists
   (`cks mcp`). A bare command runs in the foreground; lifecycle verbs
   imply supervision.
4. Flags are GNU `--flag` everywhere (already enforced via pflag/cobra).
5. New binaries: subcommands ⇒ cobra; a hypothetical single-command tool
   ⇒ pflag. (After this consolidation all three are cobra.)

## 4. Design decisions

**D1 — cobra shims over existing run functions.** Each absorbed tool's
`main()` becomes a `cobra.Command{RunE: ...}` that calls the existing
`run*(args...)` function. Business logic and its tests do not move
libraries; only flag wiring changes. This mirrors how `runGenConfig` /
`runDaemon` are already factored.

**D2 — one MCP implementation per engine.** `ckg mcp` and `ckv mcp` are
re-pointed at the namespaced standalone implementations
(`pkg/graph/mcphandlers`, `pkg/vector` equivalent + `pkg/mcp` namespace
rule) — the ones deployed today as `graph-mcp` / `vector-mcp`. The legacy
`internal/graph/mcp` stdio server is retired with a note in its package
doc. Tool-namespace stamping (`pkg/mcp.BuildRoot` via `NS_LDFLAGS`) is
applied to **all three** binaries at build time.

**D3 — `cks setup` self-execs for its own subcommands.** The five
`--*-bin` flags shrink to two. Domain-corpus, glossary, and filelist
steps invoke `os.Executable()` + `["domain", "export", ...]` — same
subprocess/step/journal model as today, no in-process shortcut, so the
step logs and the CLI contract stay identical.

**D4 — engine binaries stay subprocesses, resolved as siblings first.**
`cks setup` keeps `--graph-bin` / `--vector-bin` (CLI contract = engine
boundary), but the default changes from "PATH lookup" to "`ckg`/`ckv`
next to the running `cks` executable, then PATH". `export PATH=$PWD/bin`
disappears from the README quick start.

**D5 — `cmd/` layout.** `cmd/graph` and `cmd/vector` keep their
directories (churn without behavior change); a new `cmd/cks` absorbs
`cmd/system-mcp`, `cmd/system/*`, `cmd/knowledge-setup`,
`cmd/filelist-gen`, `cmd/eval-gate`. Absorbed packages move as
subpackages or files under `cmd/cks/`; their `*_test.go` files move with
them. Old `cmd/` directories are deleted in the same phase that absorbs
them — no transition aliases.

**D6 — clean break on names.** No compatibility symlinks or hidden
aliases for retired binary names (`knowledge-setup`, `cks-domain-export`,
`system-mcp`, ...). Both repos, their Makefiles, CI, scripts, configs and
docs are updated in the same phase. External references are ours only
(both repos + operator shells); a mapping table ships in the release
notes section of this doc.

**D7 — build targets.** `make build-bins` produces exactly
`bin/{ckg,ckv,cks}`, namespace-stamped. `build-dataset-bins` and
`build-mcp` are removed (README and wrapper script updated). `make -C
<engine>` specialized targets keep working against the new trees.

**D8 — `--project` as a persistent flag on `cks domain`.** Six of the
seven domain tools take `--project` (the domain-knowledge project dir);
`domain sync`'s `--entries` names the same directory and is renamed to
`--project` for uniformity. The flag is declared once, persistently, on
the `domain` group: `cks domain --project P export --out D` and
`cks domain export --project P --out D` both work (cobra persistent-flag
semantics).

## 5. Old → new mapping (operator reference)

| Old | New |
|---|---|
| `graph-mcp --graph D` | `ckg mcp --graph D` |
| `ckg serve --graph D --open` | `cks viewer --graph D --open` (§11) |
| `vector-mcp ...` | `ckv mcp ...` |
| `system-mcp --config F` | `cks mcp --config F` |
| `system-mcp daemon up ...` | `cks mcp up ...` |
| `system-mcp gen-config ...` | `cks mcp gen-config ...` |
| `system-mcp print-mcp-config ...` | `cks mcp client-config ...` |
| `knowledge-setup --config S --src R --out D` | `cks setup --config S --src R --out D` |
| `filelist-gen --src R --config F --out J` | `cks filelist --src R --config F --out J` |
| `eval-gate --baseline B --latest L` | `cks eval-gate --baseline B --latest L` |
| `cks-domain-export --project P --out D` | `cks domain export --project P --out D` |
| `cks-domain-sync --entries E ...` | `cks domain sync --entries E ...` |
| `cks-glossary-gen --project P --out F` | `cks domain glossary-gen --project P --out F` |
| `cks-entry-verify ...` | `cks domain verify ...` |
| `cks-inventory-check ...` | `cks domain check ...` |
| `cks-anchor-refresh ...` | `cks domain anchors ...` |
| `cks-promotion-worksheet ...` | `cks domain worksheet ...` |
| `cks-agent ...` | `cks agent ...` |
| `cks-eval ...` | `cks eval ...` |

## 6. Impact map (what must change together)

| Surface | Change |
|---|---|
| `internal/setup/plan.go` | domain/filelist/glossary steps → self-exec argv (`[self, "domain", "export", ...]`); engine-bin sibling default (D4) |
| Root `Makefile` | `build-bins` replaces `build-dataset-bins` + `build-mcp`; `sync-domain-artifacts` recipes → `cks domain sync` / `cks domain glossary` |
| `system/Makefile` | `dogfood-eval` recipe → `cks eval ...`; `build-bins` target rewritten |
| `.github/workflows/ci.yml` | eval-gate step → `go run ./cmd/cks eval-gate ...` |
| `projects/stablenet/scripts/build-dataset.sh` | binary existence list `7 → 3`; invocation → `cks setup` |
| `projects/stablenet/mcp.yaml.example` | `binary_path` values → `bin/ckg`, `bin/ckv` |
| `cks mcp gen-config` | `--graph-binary`/`--vector-binary` sibling defaults |
| `internal/system/daemon` | self-exec argv gains the `mcp` subcommand prefix |
| `internal/system/eval/runner.go`, `cmd/.../agent` | spawn `cks mcp --config ...` |
| README, engine READMEs, docs (non-archive) | command surface rewrite; quick start loses the PATH export |
| Memory/runbooks | dataset-build memory: wrapper/PATH notes obsolete |

## 7. Phasing (each phase: build + full tests + smoke + upstream mirror)

- **Phase 1 — `cks` tree.** Create `cmd/cks` (cobra root). Absorb
  `system-mcp` (as `mcp`/`mcp up...`/`mcp gen-config`/`mcp client-config`)
  and the nine `cmd/system/*` tools (as `domain */agent/eval`). Update
  daemon self-exec, runner/agent spawn paths, Makefiles, docs. Delete
  absorbed cmd dirs.
  Smoke: `cks mcp` serves the existing dataset; `cks domain export` runs
  against the pack; `cks mcp up/down` cycle works.
- **Phase 2 — setup absorb.** Move `knowledge-setup`, `filelist-gen`,
  `eval-gate` under `cks` (`setup`/`filelist`/`eval-gate`); implement D3
  self-exec + D4 sibling resolution; update plan.go, wrapper script, CI,
  root Makefile (`build-bins`).
  Smoke: full dataset build from a clean clone with **no PATH export**;
  CI eval-gate job green.
- **Phase 3 — engine mcp/viewer.** `ckg mcp`/`ckv mcp` re-pointed at the
  namespaced implementations; `ckg serve` → `ckg viewer`; delete
  `cmd/graph-mcp`, `cmd/vector-mcp`, legacy `internal/graph/mcp` server
  wiring. Namespace stamping applied to all three binaries.
  Smoke: `ckg mcp` tool names carry `stablenet_knowledge.*` when stamped;
  viewer serves.

## 8. Resolved questions (2026-08-05 review)

1. `glossary` vs `glossary-gen` — **`glossary-gen`** (operator's call:
   the verb form keeps what the command does explicit).
2. References outside the two repos (operator cron, external scripts) —
   **none exist**; the clean break (D6) is safe.
3. `agent` placement — **stays embedded in `cks`**. It composes context
   through the same MCP surface any external agent uses, so shipping it
   inside the engine binary it talks to simplifies deployment (the
   spawned server is the same executable).

## 9. Non-goals

- No behavior changes to retrieval, indexing, config formats, or the MCP
  tool surface (names of MCP *tools* are unchanged; only process-level
  command names move).
- No in-process merging of the graph/vector build pipelines (the CLI
  contract between engines stays).
- No compatibility alias layer.

## 10. Phase 1 implementation plan (pre-checked 2026-08-05)

Verified impact inventory for the Phase 1 scope (system-mcp + the nine
cmd/system tools) — every consumer found by sweeping binary names, spawn
sites, and PATH-default fallbacks:

| Consumer | Today | Phase 1 change |
|---|---|---|
| root `Makefile` | `build-mcp` builds `bin/system-mcp`; `build-dataset-bins` builds 3 domain bins; `sync-domain-artifacts` runs them | build `bin/cks` (NS-stamped); recipes → `cks domain sync` / `cks domain glossary-gen`; dataset-bins list: ckg ckv knowledge-setup filelist-gen **cks** |
| `system/Makefile` | `build-bins` builds cks-mcp/agent/eval/4 domain bins from old dirs; `CKS_LDFLAGS -X main.builderVersion`; dogfood recipe runs `cks-eval --cks-mcp bin/cks-mcp` | build one `bin/cks`; keep `main.builderVersion` (same var name in cmd/cks); dogfood → `cks eval --cks-mcp bin/cks` |
| `internal/setup/plan.go` | falls back to `cks-domain-export`/`-sync`/`cks-glossary-gen` on PATH; argv `{bin, --flags}` | three `--*-bin` flags collapse to one `--cks-bin` (default `cks` on PATH); argv gains subcommand words `{cks, domain, export, --flags}`. Removed again in Phase 2 (self-exec). |
| `projects/stablenet/scripts/build-dataset.sh` | checks 7 bins, passes 3 domain-bin flags | checks 5 (ckg ckv knowledge-setup filelist-gen cks), passes `--cks-bin` |
| `cmd/system/agent`, `internal/system/eval/runner.go` | spawn `CKSMCPBinary` (default `cks-mcp` on PATH) + `--config` | inside cks: default to `os.Executable()`, argv `{self, mcp, --config, ...}` |
| `internal/system/daemon` | self-exec `{self, --config, --name, [--http-addr]}` | argv gains the `mcp` word |
| `system/scripts/*` (activate, setup-all, apply-cc-settings, dataset-toolkit) | call `bin/system-mcp gen-config` / `print-mcp-config` | `bin/cks mcp gen-config` / `client-config` |
| README + system/README + dataset-toolkit docs + projects/stablenet/README | `system-mcp ...` invocations | `cks mcp ...` |
| Tests exec-ing binaries | `internal/system/daemon/daemon_test.go` builds/spawns the server | point at the cks binary with the `mcp` arg |

### Shim strategy inside `cmd/cks`

- **cobra root** `cmd/cks/main.go`: root command `cks`, version from
  `main.builderVersion` (keeps the existing ldflags injection).
- **`mcp` group**: `runServe` extracted from the old system-mcp `main()`
  as a plain function; `up/down/reload/start/stop/restart/status/list`,
  `gen-config`, `client-config` wrap the existing `runDaemon`,
  `runGenConfig`, `runPrintMCPConfig` with `DisableFlagParsing: true` —
  those functions already take `(args, stdout)` and own pflag FlagSets
  with passing tests, so tests move unmodified.
- **`domain` group**: native cobra flags (required for the persistent
  `--project`, D8); each tool's `main()` becomes a `newXxxCmd()` whose
  RunE calls the tool's existing logic. `domain sync --entries` is
  renamed `--project` here (D8).
- **`agent`, `eval`, `eval-gate`-style tools**: `DisableFlagParsing`
  shims around their existing arg-parsing run functions.

## 11. Amendment: the viewer belongs to cks (accepted 2026-08-06)

§3 placed `viewer` under ckg. Review after Phase 3 corrected this: the
dashboard (tools/viewer) is already the unified graph+vector UI — the
Atlas page is the vector viewer — so it is a composition artifact and
belongs to the composition engine. A cross-engine UI embedded inside one
engine's server was a layering error this consolidation exists to remove.

Target shape:

```
cks viewer [--graph D | --api-url U] [--port] [--open]
    serves the embedded dashboard; /api/* is reverse-proxied to a graph
    API backend — a sibling `ckg api` subprocess spawned on a loopback
    port when --graph is given, or an already-running server at
    --api-url. Vector's atlas backend joins the same proxy surface when
    it lands in Go.

ckg api --graph D --port P
    the graph REST API alone (was the data half of `ckg viewer`); no
    embedded assets, no --open, no --no-viewer.
```

Consequences:
- The dashboard embed (web_assets + go:embed) moves from
  internal/graph/server to internal/system/viewer; `make -C graph
  viewer` copies the Next build there instead. internal/graph/server
  keeps only the REST API.
- `ckg viewer` is retired; `ckg api` inherits its data-serving flags
  (--graph/--db/--port). `--no-viewer` disappears (api serves no UI;
  the UI command always serves it). quickstart wires `cks`? — no:
  quickstart stays within the graph engine and now runs `ckg api`,
  printing the `cks viewer` invocation instead of opening a browser.
- Boundary rules hold: cks reaches graph data over HTTP through the
  spawned engine CLI, exactly the subprocess contract `cks setup`
  already uses; no internal/graph import appears in cmd/cks.
