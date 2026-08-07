# CLAUDE.md — working agreement for this repo

Guidance for Claude Code (and any agent) working in **CKG (Code Knowledge
Graph)**. Loaded automatically every session — these are the standing rules,
not per-task instructions.

## What this project is (1 paragraph)

CKG turns a source tree (Go / TypeScript / Solidity) into a deterministic
**graph DB** (SQLite default, PostgreSQL opt-in). It is one of three sister
projects — **CKG** (graph) · **CKV** (vector + vocab bridge) · **CKS**
(orchestrator). CKG's job: *given exact keywords + a project graph, return
precisely the code an agent needs.* **Keyword-retrieval accuracy is the
first-class metric.** Full statement: **[../docs/graph/VISION.md](../docs/graph/VISION.md)** —
read it before any design discussion. Doc index: **[../docs/graph/DOC-MAP.md](../docs/graph/DOC-MAP.md)**.

## Build / test / lint (use the Makefile)

| Task | Command | Notes |
|---|---|---|
| Build (ckg only, default) | `make build` | Go binary only (API-only engine; the dashboard embed lives with cks). `build-no-viewer` is a back-compat alias |
| Build the dashboard | `make -C graph viewer` then `make -C .. build-bins` | Next.js build lands in internal/system/viewer/web_assets; `cks viewer` serves it |
| Test | `make test` (= `go test ./...`) | |
| Test + race + coverage | `make test-race` | what to run before claiming concurrency-safe |
| Lint | `make lint` | = `go vet ./...` + `fmt-check` + viewer eslint |
| Format | `make fmt` | gofmt -w over `.go` under graph/ (fixtures); engine-wide = root `make fmt` |
| Audit (parity) | `make audit` | `go/packages.Load` vs DB parity; exits 0/1/2 for CI |
| Eval | `make eval` | retrieval / validate / benchmark; gated by `cmd/eval-gate` |

CI (root `.github/workflows/ci.yml`) runs `make build`, `make lint`,
`make test-race` (whole module) plus two eval gates: `eval-gate-graph`
(`make -C graph eval` + `cmd/eval-gate` vs the committed baseline) and
`eval-gate-vector` (mock-embedder recall gate). The viewer pipeline and
`make audit` are local-only (not in CI). **Match CI locally before pushing.**

- Go **1.25.12+**. Module: `github.com/0xmhha/knowledge-system`.
- **gofmt drift is a hard gate** (`fmt-check`). Run `make fmt` before commit;
  `make -C .. install-hooks` (root Makefile) wires the local pre-commit hook (opt-in).

## Code structure

Paths are for the consolidated module (the three engines share one repo; the
graph engine's code moved from a standalone `cmd/`+`internal/`+`pkg/` into the
root under an engine-scoped prefix).

- `../cmd/graph/` — cobra root + subcommands. **Five production surfaces:**
  `build`, `serve`, `mcp`, `eval`, `audit`. The rest are utilities.
  (`ckg mcp` serves the namespaced MCP tool set.)
- `../cmd/eval-gate/` — eval regression gate (CI).
- `../internal/graph/` — implementation. **Private**: other engines must not
  import it. Key: `buildpipe` (7-pass pipeline + cache),
  `parse/{golang,typescript,solidity}`, `persist` (SQLite/Postgres store,
  schema, migrations), `graph`, `link`, `temporal`, `score`, `cluster`,
  `server`, `mcp`, `eval`, `audit`, `detect`.
- `../pkg/graph/` — **public API / stable contract** (`types`, `store`,
  `smartctx`, `evidence`, `impact`, `policy`, `security`, `mcphandlers`, …);
  `../pkg/bm25` is the shared BM25 core. CKV/CKS consume these.
- `../tools/viewer/` — Next.js dashboard (graph + vector viewers), embedded
  into the cks binary: `make -C graph viewer` then `make build-bins` at the root.
- `graph/`-local: `docs/` (see `../docs/graph/DOC-MAP.md`), `policies/`,
  `eval/`, `testdata/`, `scripts/`, `Makefile`.

## Codebase conventions (non-obvious, easy to get wrong)

- **Public boundary:** never make CKV/CKS reach into `internal/`. New
  cross-repo API goes under `pkg/` and is a **contract change** — treat it as
  such (back-compat, review).
- **SchemaVersion bumps are for BREAKING changes only** (`internal/graph/persist/manifest.go`
  policy comment). Additive optional fields with `omitempty` do **NOT** bump it
  — old readers ignore unknown JSON and decode unset fields as zero. Spurious
  bumps force a full rebuild of every existing graph DB.
- **Two distinct version-ish constants — don't confuse them:**
  - `internal/graph/persist/manifest.go` `SchemaVersion` (manifest/back-compat policy), and
  - `internal/graph/buildpipe/cache.go` `SchemaVersion` (the **cache-key** contributor;
    bumping it invalidates the build cache and forces a reindex). A change that
    must repopulate node columns on rebuild needs the **cache.go** bump.
- **Additive node/edge fields:** prefer additive + `omitempty`; round-trip
  through `sqlite_writer.go`/`sqlite_reader.go`; add an idempotent `ALTER` in
  `sqlite_migrate.go` for pre-existing DBs (see `ensureCanonicalIDColumn` as the
  pattern).
- **The graph build is LLM-free and deterministic.** Keep it that way; LLM use
  belongs to the `eval` surface only.
- **Concurrency claims require `make test-race`.** Don't assert race-safety
  without it.

## Verifying "what is true now"

When docs disagree with each other or with reality, **code + git are ground
truth.** Verify against the tree (and `git log` / `git -L`), cite `file:line`,
and report which doc is stale rather than trusting the prose. Don't claim a thing
is done/green without having run the relevant `make` target.

## Documentation discipline (ALWAYS apply when creating/editing docs)

The docs are **three tiers** (full map: `../docs/graph/DOC-MAP.md`). Before creating a
new `.md`, decide which tier it is — and prefer updating an existing doc over
spawning a new one.

- **Tier 1 — `../docs/graph/VISION.md`** (purpose/vision): **read-only input.** Never
  delete or shrink it during cleanup. If purpose/vision prose is scattered in
  other docs, **move it into VISION.md — do not drop it.** (This directly
  prevents vision being lost when status docs are pruned.)
- **Tier 2 — design/specs incl. `docs/adr/`**: a decision = one ADR file. To
  change a decision, **add a new ADR and mark the old one `Superseded by …` —
  never delete it.** This is what stops repeated "which design is right?"
  re-litigation: the supersession chain is the answer.
- **Tier 3 — state / remaining-work / status**: date it; freely disposable and
  regenerable from code + git.

Rules:
1. **Ground truth = code + git** for "current state"; ADR for "why decided";
   VISION for "what we aim at".
2. **Don't proliferate docs.** New `.md` only when it's genuinely a new decision
   (→ ADR) or a new dated status snapshot. Otherwise update in place.
3. **Supersede, don't delete.** Move superseded design docs to `docs/archive/`
   with a one-line "superseded by X".
4. **Update the index.** Any doc add/move/supersede → update `../docs/graph/DOC-MAP.md`
   (and the ADR index) in the same change.
5. **Destructive cleanup = plan first.** Present a move/supersede/delete table
   and get approval before executing.

## Git / PR

- Don't commit or push unless asked. If on `main`, branch first.
- Keep PRs small; concurrent sessions edit ckg/ckv/cks — sync at phase
  boundaries to avoid rebase churn.
- No commit trailers — do NOT add `Co-Authored-By` (matches git history and
  the commit convention in ../docs/graph/CODE-STRUCTURE.md).
