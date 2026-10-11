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
| Running the tool | Python 3.9+ and [`rich`](https://github.com/Textualize/rich) (installed automatically by `uv`) |
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
```

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
| 6 | Start the server and check health | `cks mcp --config <out>-cks-stdio.yaml`, then `initialize`, `tools/list`, `cks.ops.health` |

### Files it writes

Next to the dataset directory `<out>`:

| Path | Content |
|---|---|
| `<out>/` | the dataset: versions `v<date>-<time>-<commit8>/` and the `current` link to the served one |
| `<out>-setup.yaml` | project setup config from `cks init` (keeps the project ID across rebuilds) |
| `<out>-cks.yaml` | MCP server config, HTTP on `127.0.0.1:<port>` |
| `<out>-cks-stdio.yaml` | the same config for stdio clients |
| `<out>-setup-logs/` | one full log per run, including every command line |

Rerunning the tool on the same `--out` builds a new version and promotes it
only after its gates pass; the previous version stays available for
`cks setup --rollback`. Servers that are already running keep the dataset
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
| `--source-mode` | `committed` | `working-tree` also indexes uncommitted edits (less reproducible) |
| `--version` | `v<date>-<time>-<commit8>` | version label |
| `--no-build`, `--bin-dir` | build | reuse existing binaries |
| `--no-verify` | verify | skip the server health check |
| `--allow-missing-embeddings` | strict | do not fail when some chunks could not be embedded |
| `--plain` | dashboard on a terminal | one line per change |
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
