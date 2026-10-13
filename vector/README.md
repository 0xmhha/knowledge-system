# Code Knowledge Vector (CKV)

> The three engines (ckg · ckv · cks) now live in one module — the
> [`knowledge-system`](https://github.com/0xmhha/knowledge-system) monorepo.
> "CKV"/"CKG"/"CKS" name the engines, not separate repositories.

Semantic code search over a local vector index. CKV indexes a source
repository as embedding vectors at function / type / heading
granularity, stores them in an embedded SQLite + `sqlite-vec`
database, and serves retrieval over a CLI, an in-process Go API, and
an MCP server. The in-repo graph engine (CKG) provides symbol-graph
search; the two are combined by the system engine (CKS) for hybrid
retrieval.

> Start with the [current handoff](../docs/spec-driven/CURRENT-HANDOFF.md) and [remaining work](../docs/REMAINING-WORK.md).

## Features

- **Languages**: Go (`go/parser`), TypeScript / TSX, JavaScript / JSX / MJS / CJS, Solidity, Markdown.
- **Embedders**: `ollama` (default, BGE-M3), `mock` (deterministic feature-hash), and optional `bgeonnx` (ONNX Runtime + tokenizers).
- **CLI**: `build`, `query`, `eval`, `freshness`, `mcp`, `model`.
- **MCP server**: stdio JSON-RPC. Tools: `cks.context.semantic_search`, `cks.ops.health`, `cks.ops.warmup`, `cks.ops.get_freshness`. Every response carries a top-level `schema_version`.
- **Go API**: import `github.com/0xmhha/knowledge-system/pkg/vector/ckv` for `Open` / `SemanticSearch` / `Warmup` / `Manifest` / `Close` in the calling process.
- **Operational**: host memory pre-check + adaptive batching (`CKV_MEM_GUARD`), CoreML execution provider tuning on macOS (`CKV_COREML_*`), ORT thread overrides, panic-safe MCP middleware.

## Quickstart

### CLI with the mock embedder (no system dependencies)

```bash
make build
./bin/ckv build --src /path/to/repo --out ./ckv-data --embedder mock
./bin/ckv query "TCP socket bind on port" --out ./ckv-data --embedder mock
```

### CLI with `bgeonnx` (real semantic embeddings)

Requires `libonnxruntime`, `libtokenizers.a`, and a downloaded model.
See [`docs/vector/d1-installation-guide.md`](../docs/vector/d1-installation-guide.md).

```bash
CGO_LDFLAGS="-L$HOME/lib" go build -tags bgeonnx -o ./bin/ckv ./cmd/ckv
./bin/ckv build --embedder bgeonnx --src /path/to/repo --out ./ckv-data
./bin/ckv query "..." --embedder bgeonnx --out ./ckv-data
```

### In-process Go API

```go
import (
    "context"

    "github.com/0xmhha/knowledge-system/pkg/vector/ckv"
)

func search() error {
    engine, err := ckv.Open(".ckv-data", ckv.OpenOptions{
        Embedder: ckv.MockEmbedder(),
    })
    if err != nil {
        return err
    }
    defer engine.Close()

    if err := engine.Warmup(context.Background()); err != nil {
        // log and continue; first query will pay the cost instead
    }

    resp, err := engine.SemanticSearch(context.Background(),
        "TCP socket bind on port",
        ckv.SearchOptions{K: 5})
    if err != nil {
        return err
    }
    _ = resp.Hits // []ckv.Hit — citation, snippet, score per result
    return nil
}
```

See [`docs/vector/embedder-integration.md`](../docs/vector/embedder-integration.md)
for the production embedder path, environment overrides, and
migration off subprocess MCP.

### MCP server

```bash
./bin/ckv mcp --out ./ckv-data --embedder mock
```

Speaks MCP JSON-RPC over stdio. Configure the client to run the absolute binary path with the index path and matching embedder arguments. The complete tool list is in [the MCP reference](../docs/vector/mcp-tools.md).

## Supported languages

| Language | Parser | Extensions |
|---|---|---|
| Go | `go/parser` | `.go` |
| TypeScript | tree-sitter | `.ts`, `.tsx` |
| JavaScript | tree-sitter (via TS grammar) | `.js`, `.jsx`, `.mjs`, `.cjs` |
| Solidity | tree-sitter | `.sol` |
| Markdown | heading-section chunks | `.md`, `.markdown` |

## Embedders

| Backend | Build tag | System deps | Use case |
|---|---|---|---|
| `ollama` | none | running Ollama and matching model | default BGE-M3 semantic search |
| `mock` | none | none | tests, smoke checks — no semantic signal |
| `bgeonnx` | `-tags bgeonnx` | `libonnxruntime`, `libtokenizers.a`, model files | production semantic search |

The model registry is [internal/vector/embed/registry](../internal/vector/embed/registry). A registered model without an ONNX export cannot use `bgeonnx`; model files and exact identity are not bundled. The direct CoreML adapter is not wired into the ordinary CLI dispatch.

## Architecture

```
ckv build   discover ── parse ── chunk ── embed ── sqlite-vec
                                                    │
                                                    └─ manifest.json
ckv query   embed(intent) ── store.Search ── citation enforce ── snippet ── top-K
ckv mcp     JSON-RPC stdio ── vector tool registrations in pkg/vector/mcp
pkg/vector/ckv     Engine wrapper around internal/vector/query (in-process consumers)
```

## Build requirements

- Go 1.25.13 minimum; tool/lint CI uses Go 1.26.9
- CGO enabled (for `sqlite-vec` via `mattn/go-sqlite3`)
- `gcc` or `clang` toolchain
- `libonnxruntime` and `libtokenizers.a` only when building with
  `-tags bgeonnx`

## Documentation

- [`docs/vector/README.md`](../docs/vector/README.md) — **documentation index** (living reference vs design record vs archive). Start here.
- [Remaining work](../docs/REMAINING-WORK.md) — required holds and separately scoped follow-ups.
- [Current handoff](../docs/spec-driven/CURRENT-HANDOFF.md) — current decisions and verification limits.
- [`docs/vector/ARCHITECTURE.md`](../docs/vector/ARCHITECTURE.md), [`docs/vector/SCHEMA.md`](../docs/vector/SCHEMA.md) — module map + chunk/store schema.
- [`docs/vector/mcp-tools.md`](../docs/vector/mcp-tools.md) — MCP tool I/O specs (19 tools).
- [`docs/vector/embedder-integration.md`](../docs/vector/embedder-integration.md) — consumer integration: in-process API, MCP, env overrides.
- [`docs/vector/d1-installation-guide.md`](../docs/vector/d1-installation-guide.md) — building the optional `bgeonnx`/ONNX path (default runtime is ollama/bge-m3).
- [`docs/vector/eval-metrics.md`](../docs/vector/eval-metrics.md) — `ckv eval` metrics.
- [Purpose and scope](../docs/spec-driven/PURPOSE-AND-SCOPE.md) — retained decisions, ownership and limits; removed historical records remain recoverable through Git.

"What is true now" = code + git.

## License

AGPL-3.0. See [`LICENSE`](../LICENSE).
