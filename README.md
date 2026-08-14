# knowledge-system

Build queryable knowledge from a source tree. Three engines compose:

- **graph/** — parses source (Go / TypeScript / Solidity) into a deterministic
  graph DB: symbols, call edges, change history. SQLite by default,
  PostgreSQL opt-in.
- **vector/** — semantic search over the same source: embeddings plus a
  vocabulary bridge from natural-language queries to exact code keywords.
- **system/** — orchestrator that fuses graph and vector retrieval behind a
  single MCP server, so coding agents get precise code context in one call.

The three engines were formerly separate repositories
(`code-knowledge-graph`, `code-knowledge-vector`, `code-knowledge-system`)
and are consolidated here into a single Go module with full history.

Per-project data lives in `projects/<name>/` packs: policies, domain
knowledge, eval ground truth, dataset parameters. Packs may import engine
code; engines never reference packs, and `scripts/check-boundaries.sh`
enforces it. `projects/stablenet/` is the reference pack and the worked
example below.

## Prerequisites

The first four are toolchain; Ollama is needed to build a vector index;
the source checkout is whatever tree the pack you build indexes.

### Go

`go.mod` declares `go 1.25.13`, and Go manages the rest: whatever release
you install, `GOTOOLCHAIN=auto` (the default) downloads and uses the
declared toolchain on the first build. Installing the latest release is
enough:

```sh
brew install go       # or download from https://go.dev/dl/
go version
```

If a build fails with `go.mod requires go >= 1.25.13` the machine has
`GOTOOLCHAIN` pinned (usually `local`, common on machines that forbid
toolchain downloads). Either install go1.25.13+ directly, or allow the
download for that one command: `GOTOOLCHAIN=auto make build`.

### C toolchain

The vector engine links SQLite through cgo (`sqlite-vec-go-bindings`,
`mattn/go-sqlite3`), so a build without a C compiler fails at link time
rather than at `go vet`. No system SQLite install is needed — both
libraries bundle their own amalgamation and compile it into the binary.

```sh
xcode-select --install    # macOS (also installs git)
cc --version              # expect clang/gcc version output
```

On Linux: `sudo apt install build-essential` (Debian/Ubuntu) or the
distro equivalent.

### git

Indexing records the commit each chunk and node came from; the freshness
guard compares that against the working tree. Already present if you
installed the Xcode Command Line Tools above; otherwise
`brew install git` / `sudo apt install git`.

### Python 3

Two things need it, so a machine without it hits the gap partway through
a build rather than here: `make -C system dogfood-eval-summary` renders
the evaluation report table, and the 1024-dimension check in the next
section is a `python3 -c` one-liner. Any Python 3 works — nothing here
uses recent syntax. macOS ships one, so check before installing:

```sh
python3 --version
# if absent: brew install python@3.12   (or: sudo apt install python3)
```

### Ollama with the bge-m3 model

Embeddings for the vector index. Skip this if you only build graph
indexes (`cks setup --skip-vector`).

```sh
brew install --cask ollama          # see the caveat below
ollama serve &                      # if not already running
ollama pull bge-m3
```

**Install the app cask, not the brew formula.** The formula ships without
`llama-server` on Apple Silicon and embedding requests fail against it.

Check it answers and returns the expected 1024 dimensions:

```sh
curl -fsS localhost:11434/api/version
curl -fsS localhost:11434/api/embed -d '{"model":"bge-m3","input":"x"}' \
  | python3 -c 'import json,sys; print(len(json.load(sys.stdin)["embeddings"][0]))'
# 1024
```

A different endpoint goes in `CKV_OLLAMA_ENDPOINT` (or `--ollama-url`). The
build preflights reachability and model presence, so a missing daemon fails
immediately instead of part-way through an embed run.

### A source checkout for the pack

A pack points at the tree it indexes through the `CKS_CODE_ROOT`
environment variable rather than a committed path, because the path is
per-machine (`projects/<name>/setup.yaml` and
`domain-knowledge/project.yaml` both resolve `code_root` from it). For the
reference pack that tree is a go-stablenet checkout:

```sh
git clone https://github.com/stable-net/go-stablenet.git
export CKS_CODE_ROOT="$PWD/go-stablenet"
```

The variable selects which tree is *read*; where the dataset is *written*
is `--out`'s job.

## Quick start

```sh
# 1. binaries  (`make build` only compiles; it writes nothing to bin/)
make build-bins

# 2. dataset — cks setup finds ckg/ckv beside its own binary and runs its
#    domain/filelist steps by re-invoking itself; no PATH setup needed
export DATASET=/abs/path/for/the/dataset   # build output = what the server serves
bin/cks setup --config projects/stablenet/setup.yaml \
    --src "$CKS_CODE_ROOT" \
    --out "$DATASET"

# 3. server config, then serve — reads the dataset built in step 2
bin/cks mcp gen-config --dataset-dir "$DATASET" \
    --name stablenet --port 8930 --out cks.yaml
bin/cks mcp --config cks.yaml
```

Every binary here takes GNU-style `--flag` options — consistently, across
all commands and subcommands. Single-dash long flags (`-config`) are
rejected.

Everything binds loopback until you say otherwise, and nothing quietly
narrows a binding you already chose. `gen-config --lan` binds this host's
auto-detected LAN IP; `cks mcp up --lan` binds every interface, which is the
form that survives the host's address changing. Either way the generated
config carries the `allow_remote` opt-in, without which the server refuses
non-local clients even when the socket is reachable.

`--port` names the port at every step and never moves the host with it:
`gen-config` writes it into the config, `cks mcp --port` moves a running
server (keeping the configured host, so a LAN or wildcard deployment stays
reachable), and `cks mcp up --port` pins the port of a single-instance
registry. A registry declaring several instances assigns their ports itself,
so `--port` is refused there rather than silently applied to one of them.
`--http-addr host:port` remains the escape hatch for naming an interface
yourself, is mutually exclusive with `--port`, and does not apply to `up`.

An instance that ends up on loopback says so in the `up` output, because a
supervised deployment usually exists to serve agents on other machines.

MCP tool names are bare `cks.context.*` / `cks.ops.*` by default; a
distribution can stamp its own namespace root at build time with
`make build-bins NAMESPACE=<root>`.

## Building a dataset

`cks setup` runs the graph index, the vector index aligned to it, and
the alignment gate in one command. Output lands in `<out>/graph/graph.db`
and `<out>/vector/`. The graph build is incremental, so the same command
serves first-time setup and refresh.

| Flag | Effect |
|---|---|
| `--skip-vector` | graph only — skips the embed pass when you want a fast check |
| `--version <name>`, `--version auto` | blue-green: build into `<out>/<version>`, gate it, then swap `<out>/current` atomically |
| `--progress json` | one JSON event per line, for orchestrators |

A `current` swap is not visible to an already-running server, which holds
handles into the dataset it resolved at startup — restart it. The command
prints this reminder after a swap.

The build verifies itself: graph/vector alignment (same source commit),
that the declared domain and flow corpora actually reached the index, and
that the committed policy/glossary renderings still match the domain
entries they derive from. A build that would silently ship a smaller index
fails instead of exiting zero.

## Development

```sh
make build test lint      # lint includes the engine-boundary rules
```

Retrieval changes are judged by the evaluation suite rather than by
inspection:

```sh
make -C system dogfood-eval USE_CKV=1 CKV_EMBEDDER=ollama CKV_MODEL=bge-m3 \
     CKG=$PWD/bin/ckg CKV=$PWD/bin/ckv
```

It rebuilds the index, runs 15 scenarios, and reports recall and MRR. A
scenario whose expected span has drifted fails before any measurement runs,
and a missing knowledge scope fails the run outright — both guard against a
change that scores well while quietly delivering less.

Per-engine detail lives in [`graph/README.md`](graph/README.md),
[`vector/README.md`](vector/README.md), and
[`system/README.md`](system/README.md).

## License

AGPL-3.0 — see [LICENSE](LICENSE).
