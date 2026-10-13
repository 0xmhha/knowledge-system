# ks-setup — guided dataset setup

`ks-setup` builds a knowledge dataset for one project and shows every step
live in the terminal: which prerequisite checks passed, which binary is being
compiled, which engine stage (graph, vector, alignment, promotion gate) is
running and for how long, the last lines of output, and any warnings. At the
end it starts the MCP server once, calls `cks.ops.health`, and prints the
commands to serve the dataset.

It does not replace `cks setup`; it runs the commands documented in the
[repository README](../../README.md#quick-start) in order and stops at the
first failure.

```
╭─ knowledge-system setup ──────────────────────────────────────────╮
│ Source       /Users/me/work/my-project                           │
│ Dataset      /Users/me/work/knowledge-data/my-project            │
│ Index        graph + vector (ollama: bge-m3:latest, 1024d)       │
╰───────────────────────────────────────────────────────────────────╯
╭─ Progress 3/6  ·  4m12s ──────────────────────────────────────────╮
│ ✓  1. Check prerequisites                 0s  9 checks passed     │
│ ✓  2. Build ckg / ckv / cks              38s  …/knowledge-system/bin│
│ ✓  3. Write the project setup config      0s  my-project-setup.yaml│
│ ⠋  4. Build the knowledge dataset      3m34s  2140 source files … │
│ ✓       Graph index (ckg)              1m02s  built 81234 nodes … │
│ ⠋       Vector index (ckv)             2m32s  embedded 4100/9800  │
│ ·  5. Write the MCP server config                                 │
│ ·  6. Start the server and check health                           │
╰───────────────────────────────────────────────────────────────────╯
```

## Requirements

| Needed for | Requirement |
|---|---|
| Running the tool | Python 3.9+, [`rich`](https://github.com/Textualize/rich) and PyYAML (installed automatically by `uv`) |
| Building the binaries | Go and `make` (skip with `--no-build` or `--bin-dir`) |
| Indexing | Git; the project must be a Git repository with at least one commit |
| Vector index | [Ollama](https://ollama.com) with `bge-m3` pulled (`ollama pull bge-m3`); not needed with `--graph-only` |

## Usage

From the repository root:

```sh
# Interactive: asks for the project and dataset paths, then confirms.
tools/setup/ks-setup

# Non-interactive.
tools/setup/ks-setup --src ~/work/my-project --out ~/work/knowledge-data/my-project --yes

# Graph only (no Ollama), plain line output for logs or CI.
tools/setup/ks-setup --src ~/work/my-project --out /tmp/kd/my-project --graph-only --plain

# A project with a pack: governance policy, vector chunk policy, domain
# knowledge, glossary, flow corpus and build scope from projects/stablenet.
tools/setup/ks-setup --src ~/work/go-stablenet --out ~/work/knowledge-data/go-stablenet \
    --pack projects/stablenet --compare ~/work/knowledge-data/go-stablenet@0bf2f4d1b
```

### Project packs

`--pack projects/<name>` reads the pack's `setup.yaml` and passes each input
to `cks setup` as an absolute path: `policy_file`, `security_pattern_file`,
`vector_policy`, `domain_knowledge`, `glossary_file`, `flow_corpus`,
`filelist`, `code_root` (`${CKS_CODE_ROOT}` expands to `--src`) and
`vector_build_sources`. The server config gets the same governance policy,
glossary and domain corpus, so queries see what the build consumed. The
rendered domain corpus goes to `<out>-derived/`, one directory per dataset.

`--skip-pack-input KEY` leaves one input out; for example
`--skip-pack-input filelist` indexes the whole repository instead of the
pack's build closure. A pack file list narrows the graph to the files its
build roots compile for one platform; for go-stablenet that left out 238 Go
files the promotion gate's audit still counts, along with the TypeScript and
proto sources, so the tool raises a warning whenever the audit reports files
missing from the graph. With `--graph-only`, inputs that only feed the vector
index are dropped automatically. Preflight fails when an input is missing and
warns when the pack declares a different embedding model.

### Go sources under build/ directories

The vector indexer skips every directory named `build/`, at any depth, unless
a `vector_build_sources` list names the exact Go files to keep; the graph has
no such rule. Without a list, a package such as go-stablenet's
internal/build package is in the graph but has no vector chunks. Before the build
the tool asks `go list` which Go files the graph loads under `build/`
directories (same build constraints, so `//go:build none` scripts such as
`build/ci.go` stay out; `vendor/` and `testdata/` are skipped), writes them to
`<out>-derived/vector-build-sources.json` and passes the file to `cks setup`.
A pack that declares its own `vector_build_sources` keeps it, and
`--no-build-sources` turns the list off. The list is part of the dataset
identity, so turning it on or off yields a different dataset.

After the build the tool compares the Go files in the graph manifest with
the files the vector index read and warns about any Go file that is only in
the graph. Other languages are not compared, since the vector indexer has no
reader for some graph languages (proto).

### Measuring and comparing builds

After the health check the tool profiles the served dataset: graph files,
nodes, edges, functions, commits, policy nodes and `governed_by` edges,
node-PR links; vector chunks by kind, categorized chunks, chunks with
guidance or a `canonical_id`, domain and flow chunks. The profile and the
step timings are written to `<out>-setup-report.json`. `--compare <dataset>`
profiles another dataset (versioned or flat layout) the same way and prints
the two side by side, marking the rows that differ.

The launcher uses `uv` when it is installed (it creates a `.venv` beside the tool
with `rich`); otherwise it runs `python3 -m ks_setup` and falls back to plain
output when `rich` is missing. Without the launcher:

```sh
pip install -e tools/setup     # or: uv tool install --editable tools/setup
ks-setup --src ~/work/my-project
```

### Steps

| # | Step | Command it runs |
|---|---|---|
| 1 | Check prerequisites | git/go/make in PATH, the source is a Git repository with a commit, the dataset is outside the source tree, disk space, Ollama answering with the model pulled |
| 2 | Build ckg / ckv / cks | `make build-bins` in this repository |
| 3 | Write the project setup config | `cks init` (skipped when `<out>-setup.yaml` exists, so the project ID stays stable) |
| 4 | Build the knowledge dataset | `cks setup --config … --version … --progress json`; engine stages appear as nested rows |
| 5 | Write the MCP server config | `cks mcp gen-config` → `<out>-cks.yaml` (HTTP) and `<out>-cks-stdio.yaml` (stdio) |
| 6 | Start the server and check health | `cks mcp --config <out>-cks-stdio.yaml`, then `initialize`, `tools/list`, `cks.ops.health`, and two first questions an agent asks: `review_function` on the most-called production function and `get_for_task_v2`, which must return citations |

### Files it writes

Next to the dataset directory `<out>`:

| Path | Content |
|---|---|
| `<out>/` | the dataset: versions `v<date>-<time>-<commit8>/` and the `current` link to the served one |
| `<out>-setup.yaml` | project setup config from `cks init` (keeps the project ID across rebuilds) |
| `<out>-cks.yaml` | MCP server config, HTTP on `127.0.0.1:<port>` |
| `<out>-cks-stdio.yaml` | the same config for stdio clients |
| `<out>-setup-logs/` | one full log per run, including every command line |
| `<out>-setup-report.json` | step timings, health, the build-source list and Go scope gap, dataset profile and the comparison, if any |
| `<out>-derived/` | the pack's rendered domain corpus (with `--pack` and domain knowledge) and `vector-build-sources.json` |

Rerunning the tool on the same `--out` builds a new version and promotes it
only after its gates pass; the previous version stays available for
`cks setup --rollback`. The summary shows how many versions are retained and
how much disk they use, with the `cks gc --dry-run` command that reviews a
cleanup; the tool never deletes versions itself.

`--min-canonical RATIO` turns on the promotion gate's join-density check
(the share of symbol chunks that carry a `canonical_id`). Without it the
engine skips the check and warns; A historical project ratio is not a guarantee for another source scope. Servers that are already running keep the dataset
they opened, so restart them after a rebuild.

### Options

| Option | Default | Effect |
|---|---|---|
| `--src` | asked | Git repository to index |
| `--out` | `../knowledge-data/<name>` beside the source | dataset directory |
| `--name` | source directory name | MCP instance name |
| `--port` | 8930 | HTTP port in the server config |
| `--graph-only` | off | graph index only; unversioned layout; no Ollama |
| `--embedder` | `ollama` | `mock` builds without embeddings, to try the flow; such a dataset cannot answer semantic queries |
| `--model`, `--embed-dim`, `--ollama-url` | `bge-m3:latest`, 1024, `http://127.0.0.1:11434` | embedding settings |
| `--source-mode` | `committed` | `working-tree` captures uncommitted edits into its own snapshot; it does not claim a clean committed tree |
| `--version` | `v<date>-<time>-<commit8>` | version label |
| `--no-build`, `--bin-dir` | build | reuse existing binaries |
| `--no-verify` | verify | skip the server health check |
| `--allow-missing-embeddings` | strict | do not fail when some chunks could not be embedded |
| `--pack` | none | project pack whose `setup.yaml` inputs the build uses |
| `--skip-pack-input` | none | leave one pack input out (repeatable) |
| `--no-build-sources` | listed | do not list Go sources under `build/` directories for the vector index |
| `--compare` | none | dataset to profile side by side after the build |
| `--min-canonical` | off | promotion gate: minimum canonical_id coverage |
| `--plain` | dashboard on a terminal | one line per change, flushed immediately |
| `-y`, `--yes` | ask | start without confirmation |

Exit status: 0 success, 1 failure, 2 usage error, 130 cancelled. Ctrl-C stops
the running command; `current` keeps pointing at the last promoted version, and
the cancelled version's directory is left in place for inspection (delete it
when no longer needed; a later run picks a new label).

## Tests

```sh
python3 -m unittest discover -s tools/setup/tests
# End to end with the mock embedder (needs `make build-bins` first):
KS_SETUP_E2E=1 python3 -m unittest discover -s tools/setup/tests
```

The end-to-end test builds a dataset from a throwaway repository twice and
checks that the second run promotes a new version and keeps the first as a
rollback target.

### Reviewed meaning and ontology readiness

`--semantic-store PATH`, `--ontology-mode off|baseline|concept_text|relations|combined`
and `--ontology-budget-ms 1..5000` connect a pinned semantic store. The default
is off and preserves the existing query path. An explicitly enabled mode needs
meaning data; `cks.semantic.status` is checked separately from backend health.
A reachable graph/vector index does not imply that ontology data is ready.

To prepare source-backed meaning, keep `ontology.yaml` and `spec.yaml` inside
the committed source tree and give both the same stable project ID:

```sh
tools/setup/ks-setup --src /path/to/project --out /path/to/data \
  --project-id my-project --ontology ontology.yaml --spec spec.yaml \
  --ontology-mode off --ontology-budget-ms 5000
```

Semantic inputs or an existing semantic store make the graph/vector build a
**held candidate**. The tool calls `cks semantic build` on that candidate's
retained files and writes a reviewable projection and a candidate-specific MCP
config. It never supplies `--activate`, never approves records, and keeps the
old `current` and operator MCP configs. If preparation fails or the projection
belongs to another snapshot, it refuses readiness and keeps that old state.
The held candidate can be inspected with the read-only semantic tools before
review/release; proposed data is readable but supplies no reviewed ranking boost.

`--project-id` must agree with an existing setup config. No signing key or
reviewer label is generated. Preparing definitions/specs does not invent
IMPLEMENTED_BY/TESTED_BY/CHECKED_BY edges: code anchors and reviewed links need
separate authoring and validation. Actual project-content approval and an
independent effect evaluation remain distinct from the setup smoke check.
