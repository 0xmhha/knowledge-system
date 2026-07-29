# filelist-gen — derived build-scope file lists for knowledge indexing

Status: DESIGN (2026-07-29) — review pending, implementation follows on this
branch. Tier 2 design doc.

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
| R3 | Include **Solidity system-contract sources and their tests**, and test-only Go packages that exercise them | operator request; see §5 lifecycle |
| R4 | **Multiple build roots** (e.g. `./cmd/gstable`, `./cmd/genesis_generator`) with union semantics | operator request |
| R5 | The list is **derived, not curated**: re-running the tool at a new commit reflects code changes with no manual edits | delta-tracking discussion |
| R6 | Output carries **provenance** (commit, config, per-root contribution) so a dataset records exactly how its scope was computed | reproducibility discipline |
| R7 | **Fail closed**: an unresolvable root or package is an error, never a silent omission | audit lesson |
| R8 | The tool imports **no engine internals**; `go list` is invoked as a subprocess (the CLI is the contract), keeping it clean under `scripts/check-boundaries.sh` | repo boundary rules |

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
| `go:embed` artifacts (`contracts.go:37-52` embeds `artifacts/{v1,v2}`) | n/a | compiled bytecode IS part of the binary via embed; see §5 step 3 |

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
  resolves `extra_globs` against the source tree.
- Flags:

```
filelist-gen -src <project root> -config <filelist.yaml> -out <files-from.json>
             [-check]          # verify the existing output matches; exit 1 on drift
```

- `-check` makes it a CI/update-verb gate: "does the committed/stored list
  still equal the derivation at this commit?"

### 4.2 Config: `projects/<pack>/filelist.yaml` (tracked, per pack)

```yaml
# Scope derivation for knowledge indexing. The ground truth of "in the
# build" is `go list -deps` over build_roots; everything else is explicit.
build_roots:                       # R4 — union, deduped, single go list call
  - ./cmd/gstable
  - ./cmd/genesis_generator
include_package_tests: true        # R2 — TestGoFiles+XTestGoFiles of the closure
include_embed_files: false         # go:embed assets (e.g. compiled contract
                                   # artifacts). They ARE part of the binary,
                                   # but bytecode is retrieval noise — excluded
                                   # by default, switch exists to be honest
                                   # about the build fact (§5 step 3).
extra_packages:                    # R3 — packages OUTSIDE the dep graph that
  - ./systemcontracts/test         #      belong to the workflow: test-only
  - ./systemcontracts/compile/...  #      integration packages and the solc
                                   #      compile tooling (+ their tests)
extra_globs:                       # R3 — non-Go assets
  - "systemcontracts/**/*.sol"     #      contract sources + test contracts
exclude_globs: []                  # escape hatch; empty by default
```

Field semantics:

- `build_roots`: explicit package paths recommended (auditable); go-style
  patterns (`./cmd/...`) are accepted but discouraged in packs.
- `extra_packages`: resolved via `go list` (files, incl. their tests when
  `include_package_tests`), so they stay glob-free and fail closed. `/...`
  suffix expands subpackages.
- `extra_globs`: for non-Go files only. Doublestar matching, evaluated
  against the git-tracked tree.
- Precedence: (union of roots ∪ package tests ∪ extra packages ∪ extra
  globs) − exclude_globs, emitted as a sorted, deduplicated explicit list.

### 4.3 Output: `generated/filelist/files-from.json` (untracked, derived)

Same consumer format the engines already accept (`--files-from`, an
`{include: [...]}` object), plus a provenance block:

```json
{
  "_provenance": {
    "tool": "filelist-gen <version>",
    "src_commit": "<HEAD of -src>",
    "config_sha256": "<hash of filelist.yaml>",
    "roots": {"./cmd/gstable": 988, "./cmd/genesis_generator": 4},
    "counts": {"build": 672, "tests": 321, "extra_packages": 9, "extra_globs": 22}
  },
  "include": ["accounts/abi/abi.go", "..."]
}
```

- Consumers ignore unknown keys (`include` is all they read) — verified
  against the graph/vector `--files-from` loaders before shipping.
- Datasets keep their current practice of copying the used list next to the
  built DBs; the provenance block makes that copy self-describing.

### 4.4 Failure semantics (R7)

- Any root or extra package that `go list` cannot resolve → non-zero exit,
  no output written.
- An extra glob that matches zero files → warning (legitimate during
  refactors) unless `-strict` is set.
- `-check` compares the freshly derived list against `-out`'s existing
  content (ignoring provenance) and exits 1 on any difference — the delta
  hook for CI and the setup `update` verb.

### 4.5 Integration

- **setup pipeline**: `knowledge-setup` (and later the `update` verb) gains
  an optional derivation step before the graph build: run filelist-gen, feed
  the fresh list to both engine builds. The dataset then always matches the
  indexed commit's build scope.
- **script retirement**: `projects/stablenet/scripts/gen-filelist.sh` is
  deleted once the parity test (§6) passes.
- The curated glob list under `eval/graph/` is retained untouched — it is
  the *historical* fixture that reproduces past datasets (digest
  `4be26516…`); new datasets use the derivation.

## 5. System-contract lifecycle → domain knowledge

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
- `code_anchors` (measured):
  - `systemcontracts/compile/main.go` + `compile/compiler/compiler.go`
    (`solcVersion`) — step 2
  - `systemcontracts/artifacts/{v1,v2}` — step 3 (kind: loc, path anchor)
  - `systemcontracts/contracts.go:37` — the `go:embed` of artifacts (step 4)
  - `systemcontracts/systemcontracts.go` — load/DB-store/initialization
    (step 4)
  - `systemcontracts/test/` — step 5 scope
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

## 6. Testing

- **Parity test** (locks R2/R3): against the vendored synthetic corpus or a
  minimal fixture module, assert the derivation = build + closure tests +
  extra packages + globs, deterministic ordering, dedup.
- **Real-tree spot check** (documented, not CI): gstable root reproduces the
  canonical 988-file scope (+ the deltas R3 adds intentionally:
  `systemcontracts/test/*.go`).
- **Fail-closed tests**: unknown root, unknown extra package, zero-match
  glob with `-strict`.
- **`-check` drift test**: derive, mutate a file list entry, `-check` exits 1.
- **Consumer compatibility**: graph and vector `--files-from` loaders accept
  the provenance-bearing output (unknown-key tolerance).

## 7. Implementation plan

1. `cmd/filelist-gen` (+ unit tests, fixture module under its testdata).
2. `projects/stablenet/filelist.yaml` (§4.2 values) + pack README row.
3. Delete `projects/stablenet/scripts/gen-filelist.sh` (superseded).
4. New domain entry `A4.system_contracts.build_deploy_pipeline` +
   `entry-verify` promotion + inventory refresh.
5. Boundary script: add `cmd/filelist-gen` (forbid all engine internals).
6. Downstream sync PR after upstream merge; corpus regen + reindex ride the
   next propagation pass.

## 8. Non-goals / open items

- **Per-file root attribution** in provenance (needs one `go list` per
  root): v1 records per-root union counts only.
- **ABI JSON indexing** (artifacts contain ABIs which could aid retrieval):
  future consideration; today the workflow entry carries that knowledge.
- **Semantic-drift limits of delta tracking** (recorded in the earlier
  discussion): anchored-file deltas are a first-pass filter; indirect
  behavior changes still warrant the cheap periodic full audit
  (`anchor-refresh -check`).
- **Decision pending (operator)**: final `build_roots` set — proposal is
  `gstable` + `genesis_generator`; `bootnode`/`db_migrator` (+1 each) if
  operationally relevant; `evm`/`devp2p`-class developer tools excluded by
  default (+29–48 files of retrieval noise).
