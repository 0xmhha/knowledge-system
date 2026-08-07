# filelist-gen — derived build-scope file lists for knowledge indexing

Status: IMPLEMENTED (2026-07-29) — v2 design (review round 1: build-context

> 2026-08-06: the tool has since moved into the cks command tree — it is
> `cks filelist` (cmd/cks/filelistcli). Path and script mentions below
> reflect the layout at implementation time.
pinning, source-state discipline, scope-transition semantics, check
semantics, integration keys) implemented as `cmd/filelist-gen` with the
setup-pipeline `filelist:` key. §9 decisions confirmed: build_roots =
`./cmd/gstable` + `./cmd/genesis_generator`; build_context =
linux/amd64/cgo-on/no-tags. Spot-check vs go-stablenet `0bf2f4d1b`: see
§8.1. Tier 2 design doc.

## 1. Problem

Knowledge datasets index a *scoped* subset of a project: the files that are
actually part of the shipped build, plus the tests and assets that belong to
them. Today that scope is expressed as a hand-curated glob list
(`projects/stablenet/eval/graph/stablenet-files-with-tests.json`) and a shell
generator (`projects/stablenet/scripts/gen-filelist.sh`). Both drift:

- The curated globs absorb new files inside listed directories but silently
  miss **new directories** (and already miss `systemcontracts/test/*.go` —
  the top-level glob `systemcontracts/*.go` does not descend).
- The shell generator produced a *different* file set than the curated list
  (966 vs 988 go files at the same commit) — two sources of truth, both
  wrong in different ways.
- A shell script is outside the toolchain conventions of this repo: every
  other pack artifact (domain corpus, engine views) is produced by a Go
  binary with testable behavior.

When the indexed project changes, nothing follows up: the list is a tracked
artifact, not a derivation, so it needs a human to notice.

## 2. Requirements

| # | Requirement | Origin |
|---|---|---|
| R1 | Generator is a **Go binary** (like `domain-export`), not a shell script | operator request |
| R2 | Include the **tests of every package in the build closure** (`*_test.go`), not just the shipped files | operator request; a build-only closure misses all 321 test files |
| R3 | Include **Solidity system-contract sources and their tests**, and test-only Go packages that exercise them | operator request; see §6 lifecycle |
| R4 | **Multiple build roots** (e.g. `./cmd/gstable`, `./cmd/genesis_generator`) with union semantics | operator request |
| R5 | The list is **derived, not curated**: re-running the tool at a new commit reflects code changes with no manual edits | delta-tracking discussion |
| R6 | Output carries **provenance** (commit, config, build context, per-root contribution) so a dataset records exactly how its scope was computed | reproducibility discipline |
| R7 | **Fail closed**: an unresolvable root or package is an error, never a silent omission | audit lesson |
| R8 | The tool imports **no engine internals**; `go list` is invoked as a subprocess (the CLI is the contract), keeping it clean under `scripts/check-boundaries.sh` | repo boundary rules |
| R9 | The derivation is computed under a **pinned build context** (GOOS/GOARCH/CGO/tags from config, recorded in provenance) — `go list` file sets are build-constraint-sensitive, and a platform-dependent scope would break cross-machine reproducibility | review C1 |
| R10 | **Source-state discipline**: the derivation refuses a dirty tracked tree by default and resolves every file against the **git-tracked** state, so the emitted list always corresponds to the recorded `src_commit` | review C2; the 2026-07-28 dataset-pollution incident |

## 3. Empirical grounding (go-stablenet, measured)

All numbers measured against the go-stablenet tree; the canonical curated
list (`stablenet-files-with-tests.json`) is the comparison baseline that past
datasets (pr-77-2 graph, digest `4be26516…`) were built with.

| Set | Files | Note |
|---|---|---|
| `go list -deps ./cmd/gstable` (GoFiles+CgoFiles, in-module) | 668 | build-only — **all tests missing** |
| + `TestGoFiles`+`XTestGoFiles` of the same closure | +321 → **989** | **matches the canonical list (988) exactly** (+1 was a test file added by a work branch) — the derivation reproduces the curated scope |
| `systemcontracts/**/*.sol` | 22 | matches canonical; includes `solidity/test` contracts |
| `systemcontracts/test/*.go` (test-only package) | missing from both | outside the dep graph **and** outside the canonical glob — R3 improves on the historical baseline |
| `go:embed` artifacts (`contracts.go:37-52` embeds `artifacts/{v1,v2}`) | n/a | compiled bytecode IS part of the binary via embed; see §6 step 3 |

Caveat on the 989 match (motivates R9): curated globs are
platform-neutral, but `go list` file sets honor build constraints. The
exact match indicates this scope currently has no platform-split files —
that is a property of today's tree, not of the method. R9 pins the context
so the derivation cannot silently vary by the machine that ran it.

Per-root union cost (single `go list` invocation, go dedups packages):

| Additional root | Union delta vs gstable-only (988) |
|---|---|
| `./cmd/genesis_generator` | +4 (contains the file the A8 entry anchors) |
| `./cmd/bootnode`, `db_migrator`, `era` | +1 each |
| `./cmd/ethkey` +8 · `abidump` +7 · `abigen` +14 · `p2psim` +19 · `clef` +25 · `devp2p` +29 · `evm` +48 | developer tooling, larger closures |
| `./cmd/utils` | +0 (fully shared closure) |

Note: `./cmd/logrot` (used as an example in discussion) does not exist in
go-stablenet; R7's fail-closed behavior makes such a typo an immediate error
rather than a silent gap.

## 4. Design

### 4.1 Tool: `cmd/filelist-gen`

- Standalone Go binary. Imports **no** engine packages (R8): it shells out to
  `go list -deps -json <roots...>` (one invocation regardless of root count)
  and to `go list -json <extra_packages...>` for test-only packages, then
  resolves `extra_globs` against the git-tracked tree.
- Assumptions (validated, fail closed): `--src` is the root of a **single Go
  module** and of a **git repository**. Multi-module trees and non-git
  sources are out of scope for v1 and rejected with a clear error.
- The `go list` subprocess runs with the environment derived from the
  config's `build_context` (R9): `GOOS`, `GOARCH`, `CGO_ENABLED`, and
  `-tags` are set explicitly, never inherited silently from the invoking
  machine.
- Flags:

```
cks filelist --src <project root> --config <filelist.yaml> --out <files-from.json>
             [-check]        # compare a fresh derivation against the existing
                             # --out content; exit 1 on any difference
             [-allow-dirty]  # permit a dirty tracked tree (recorded in provenance)
             [-strict]       # zero-match extra_globs become errors
```

- `--check` semantics (two supported uses, same flag):
  - **dataset check**: point `--out` at the list copied into a built dataset
    → "does this dataset's scope still equal the derivation at HEAD?" This
    is the hook for the setup `update` verb and freshness reporting.
  - **self check**: point `--out` at `generated/filelist/files-from.json`
    → CI-style "derivation is current" gate (generate, then `--check`).
  - Comparison covers the `include` list only, but the tool refuses to
    compare outputs produced under a **different `build_context`** (the
    provenance block records it) — cross-context comparison is a config
    error, not a drift signal.

### 4.2 Config: `projects/<pack>/filelist.yaml` (tracked, per pack)

```yaml
# Scope derivation for knowledge indexing. The ground truth of "in the
# build" is `go list -deps` over build_roots; everything else is explicit.
build_context:                     # R9 — pinned, never inherited from the
  goos: linux                      #      invoking machine. Pin to the
  goarch: amd64                    #      deployment target of the indexed
  cgo: true                        #      project. Recorded in provenance.
  tags: []
build_roots:                       # R4 — union, deduped, single go list call
  - ./cmd/gstable
  - ./cmd/genesis_generator
include_package_tests: true        # R2 — TestGoFiles+XTestGoFiles of the closure
include_embed_files: false         # go:embed assets (e.g. compiled contract
                                   # artifacts). They ARE part of the binary,
                                   # but bytecode is retrieval noise — excluded
                                   # by default. WARNING: enabling this feeds
                                   # non-source files to the engine parsers;
                                   # consumers must tolerate them (M2). The
                                   # switch exists to be honest about the
                                   # build fact (§6 step 3).
extra_packages:                    # R3 — packages OUTSIDE the dep graph that
  - ./systemcontracts/test         #      belong to the workflow: test-only
  - ./systemcontracts/compile/...  #      integration packages and the solc
                                   #      compile tooling (+ their tests)
extra_globs:                       # R3 — non-Go assets
  - "systemcontracts/**/*.sol"     #      contract sources + test contracts
exclude_globs: []                  # escape hatch; empty by default
```

Field semantics:

- `build_context`: required. The stablenet pack pins the gstable deployment
  target (linux/amd64, cgo on). Cross-derivation on a darwin operator
  machine is fine — `go list` honors the env for file-set resolution.
- `build_roots`: explicit package paths recommended (auditable); go-style
  patterns (`./cmd/...`) are accepted but discouraged in packs.
- `extra_packages`: resolved via `go list` (files, incl. their tests when
  `include_package_tests`), so they stay glob-free and fail closed. `/...`
  suffix expands subpackages.
- `extra_globs`: for non-Go files only. Matched against the output of
  `git ls-files` (R10 — tracked files only, so an untracked stray can never
  enter the scope). Doublestar semantics: `**` crosses directory
  separators; matching is by `bmatcuk/doublestar`-style rules, documented
  in the tool's help text.
- Precedence: (union of roots ∪ package tests ∪ extra packages ∪ extra
  globs) − exclude_globs, emitted as a sorted, deduplicated explicit list.

### 4.3 Output: `generated/filelist/files-from.json` (untracked, derived)

Same consumer format the engines already accept (`--files-from`, an
`{include: [...]}` object), plus a provenance block:

```json
{
  "_provenance": {
    "tool": "filelist-gen <version>",
    "src_commit": "<HEAD of --src>",
    "dirty": false,
    "build_context": {"goos": "linux", "goarch": "amd64", "cgo": true, "tags": []},
    "config_sha256": "<hash of filelist.yaml>",
    "roots": {"./cmd/gstable": 988, "./cmd/genesis_generator": 4},
    "counts": {"build": 672, "tests": 321, "extra_packages": 9, "extra_globs": 22}
  },
  "include": ["accounts/abi/abi.go", "..."]
}
```

- Consumers ignore unknown keys (`include` is all they read) — verified
  against the graph/vector `--files-from` loaders before shipping (§7).
- `dirty` appears only when `--allow-dirty` was used; a clean derivation
  omits it.
- Datasets keep their current practice of copying the used list next to the
  built DBs; the provenance block makes that copy self-describing —
  including **which scope generation** (config hash) produced it (§5).

### 4.4 Failure semantics (R7, R10)

- `--src` not a git repo, or not a single-module root → error.
- **Dirty tracked files** (modified/staged/deleted; untracked files are
  ignorable noise) → error unless `--allow-dirty`, which records
  `dirty: true` in provenance. Default is fail-closed: the
  2026-07-28 incident (a reindex silently embedding a work branch) is the
  class of bug this kills.
- Any root or extra package that `go list` cannot resolve → non-zero exit,
  no output written.
- An extra glob that matches zero files → warning (legitimate during
  refactors) unless `--strict` is set.
- `--check` against an output produced under a different `build_context` or
  tool major version → error (not drift).

### 4.5 Integration

- **setup pipeline**: `setup.yaml` gains an optional key
  `filelist: <path to filelist.yaml>` (M4). When present, the plan inserts
  a `filelist-derive` step before the graph build: run filelist-gen, feed
  the fresh list to both engine builds via `--files-from`. The dataset then
  always matches the indexed commit's build scope, and the `update` verb
  gets scope-following for free.
- **script retirement**: `projects/stablenet/scripts/gen-filelist.sh` is
  deleted once the parity test (§7) passes. Before deletion, sweep for
  references to the script (docs, other scripts, HANDOFF notes) and update
  them.
- The curated glob list under `eval/graph/` is retained untouched — it is
  the *historical* fixture that reproduces past datasets (digest
  `4be26516…`); new datasets use the derivation. See §5.

## 5. Scope transition (one-time baseline shift)

The graph digest is a deterministic function of **(commit, scope)**. The
derivation changes the scope definition once (adds `systemcontracts/test`,
the compile tooling, and future files the globs would have missed), so:

- **At the same commit**, a dataset built with the derived scope carries a
  **different `graph_digest`** than the historical `4be26516…` lineage.
  This is a one-time lineage break, not a recurring cost: after adoption,
  same commit + same config ⇒ same digest, exactly as before. Per-commit
  digest changes remain what they always were — the code changed.
- **Reproducing the old lineage stays possible forever**: the historical
  fixture list is retained (§4.5), so a byte-identical `4be26516…` dataset
  can be rebuilt on demand (e.g. to re-run past experiments under identical
  conditions).
- **Eval baselines are regenerated once** on adoption: retrieval fixtures
  and stored results that assume the old scope (e.g. precision cases
  sensitive to `test.*` symbols) get one refresh against the first
  derived-scope dataset, and are tagged with the scope generation.
- **Scope generation is self-recording**: the provenance `config_sha256`
  plus the dataset's `@<commit8>-<digest8>` naming already distinguish
  lineages; no extra naming scheme is needed.
- Rollout rule: do not mix lineages inside one comparison table. Existing
  datasets stay valid for their own lineage; new experiments start on the
  derived scope.

## 6. System-contract lifecycle → domain knowledge

The system-contract change workflow (operator-provided, verified against the
tree) is load-bearing context for both this design and future agent tasks:

1. `systemcontracts/solidity/**/*.sol` — contract sources, compiled to
   deployable bytecode.
2. `systemcontracts/compile/` — the solc compile tooling (Go).
3. Compiled output lands in `systemcontracts/artifacts/{v1,v2}`.
4. Native Go under `systemcontracts/` loads the artifacts — they are
   **`go:embed`-ded into the binary** (`contracts.go:37-52`) — stores them
   into the DB and runs initialization.
5. `systemcontracts/test/` exercises steps 1–4 end to end (native +
   contract behavior).

Design consequences:

- Steps 1, 2, 5 join the file list via `extra_globs`/`extra_packages`
  (§4.2). Step 3's artifacts stay excluded by default (bytecode ≠
  retrieval text) with `include_embed_files` as the honest switch. Step 4
  is already in the build closure.
- A new domain-knowledge entry captures the workflow itself:

**`projects/stablenet/domain-knowledge/entries/A4.system_contracts.build_deploy_pipeline.yaml`**

- `knowledge_type`: procedure (uses the existing `procedure_steps` field —
  precedent: `A14.foundations.cherry_pick_principle`).
- `procedure_steps`: the five steps above.
- `code_anchors` (measured; anchor forms to be validated against
  `ValidateEntry` before authoring — M3):
  - `systemcontracts/compile/main.go` + `compile/compiler/compiler.go`
    (`solcVersion`) — step 2
  - step 3 is anchored through the **embed site**
    `systemcontracts/contracts.go:37` (a directory path is likely not a
    valid mechanical anchor; if `ValidateEntry` accepts loc-style path
    anchors, add `artifacts/v1` as a secondary, else the embed line plus
    the entry prose carry it)
  - `systemcontracts/systemcontracts.go` — load/DB-store/initialization
    (step 4)
  - `systemcontracts/test/` — step 5 scope (same anchor-form caveat)
- Cross-references: `A10.codegen.contract_regen_procedure` (how to
  regenerate — mechanics) and `A4.system_contracts.*` (addresses,
  governance). The new entry owns the *why-shaped-this-way* lifecycle view;
  no duplication.
- Flows into the corpus via `domain-export` — an agent picking up a
  system-contract task retrieves the five steps.

This also resolves an audit classification: the A10 anchors into
`systemcontracts/compile/` were flagged "outside the gstable build set" —
they are intentional knowledge targets of this workflow, which the pack
config now states explicitly.

## 7. Testing

- **Parity test** (locks R2/R3): against a minimal fixture module vendored
  under the tool's testdata, assert the derivation = build + closure tests
  + extra packages + globs, deterministic ordering, dedup. The fixture
  imports **stdlib only** so `go list` needs no network or module cache
  (M5) — CI-safe.
- **Build-context test** (locks R9): a fixture file guarded by a build tag
  or GOOS constraint appears/disappears exactly per the configured context,
  and provenance records the context.
- **Source-state tests** (lock R10): dirty tracked file → error;
  `--allow-dirty` → succeeds with `dirty: true`; an untracked file matching
  an extra glob is NOT included (git-tracked resolution).
- **Real-tree spot check** (documented, not CI): gstable root reproduces the
  canonical 988-file scope (+ the deltas R3 adds intentionally).
- **Fail-closed tests**: unknown root, unknown extra package, zero-match
  glob with `--strict`, non-git `--src`, cross-context `--check`.
- **`--check` drift test**: derive, mutate a list entry, `--check` exits 1.
- **Consumer compatibility**: graph and vector `--files-from` loaders accept
  the provenance-bearing output (unknown-key tolerance), verified by an
  integration test that runs both loaders on a generated file.

## 8. Implementation plan

1. `cmd/filelist-gen` (+ unit tests, fixture module under its testdata).
2. `projects/stablenet/filelist.yaml` (§4.2 values) + pack README row.
3. Sweep references to `gen-filelist.sh`, then delete it (superseded).
   [Already satisfied: the script was deleted earlier by the safety-net
   restore (#33); the sweep found no live references — only historical
   session records, left as-is.]
4. New domain entry `A4.system_contracts.build_deploy_pipeline` (validate
   anchor forms first — M3) + `entry-verify` promotion + inventory refresh.
5. Boundary script: add `cmd/filelist-gen` (forbid all engine internals).
6. setup.yaml `filelist:` key + plan step (can land with or after 1–5).
7. Downstream sync PR after upstream merge; corpus regen + reindex ride the
   next propagation pass; first derived-scope dataset starts the new
   lineage per §5.

### 8.1 Implementation spot-check (2026-07-29, go-stablenet @ `0bf2f4d1b`)

Derivation with the confirmed pack config: **1,044 files** (build 671,
tests 322, extra_packages 29, extra_globs 22; roots: `./cmd/gstable` 989 +
`./cmd/genesis_generator` 4). Reconciliation against the historical curated
list (segment-boundary glob resolution, 1,016 go + 22 sol):

- **shared core = 989 go files** — exactly the §3 parity measurement;
- **+33 designed additions**: `systemcontracts/test` (25),
  `systemcontracts/compile` (4), `cmd/genesis_generator` (4) — R3/R4;
- **−27 build-constraint variants** dropped by the linux/amd64/cgo-on pin
  (windows/bsd/darwin files, `*_nocgo`, js/wasm stubs, and two
  `integrationtests`-tagged `cmd/gstable` test files) — the R9 caveat
  materialized; the curated globs were platform-neutral, the derived scope
  is the shipped build's;
- `.sol` set identical (22).

Known scope choice surfaced by the check: **test-helper packages imported
only by tests** (e.g. `consensus/wbft/testutils`, `internal/testlog`,
`tests/`) are outside `go list -deps` (which does not follow test imports)
and are not in the derived scope. They were also outside the historical
988-file core, so this is parity, not regression; individual helpers can be
opted in via `extra_packages` if retrieval gaps show up.

## 9. Non-goals / open items

- **Per-file root attribution** in provenance (needs one `go list` per
  root): v1 records per-root union counts only.
- **ABI JSON indexing** (artifacts contain ABIs which could aid retrieval):
  future consideration; today the workflow entry carries that knowledge.
- **Semantic-drift limits of delta tracking** (recorded in the earlier
  discussion): anchored-file deltas are a first-pass filter; indirect
  behavior changes still warrant the cheap periodic full audit
  (`anchor-refresh --check`).
- **Multi-module trees**: out of scope for v1 (§4.1); revisit if a target
  project needs it.
- **Decision pending (operator)**: final `build_roots` set — proposal is
  `gstable` + `genesis_generator`; `bootnode`/`db_migrator` (+1 each) if
  operationally relevant; `evm`/`devp2p`-class developer tools excluded by
  default (+29–48 files of retrieval noise). Also confirm the pinned
  `build_context` for the stablenet pack (proposal: linux/amd64, cgo on).
