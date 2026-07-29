# stablenet project pack

Project-owned knowledge data for indexing the public
[go-stablenet](https://github.com/stable-net) codebase with the engines in
this repository. Everything here is either curated from, or generated
against, public sources (the go-stablenet code, its developer docs and
readmes) — nothing in a project pack may be non-publishable.

## Layout

| Path | What | Consumed by |
|---|---|---|
| `policies/graph.yaml` | governance policy (invariants, lock discipline, quorum rules) | `graph build --policy-file` → `governed_by` edges |
| `policies/vector.yaml` | chunk categorization by path + watch-outs | `vector build --policy` |
| `domain-knowledge/` | master domain-knowledge entries (source of truth; `domain-sync` derives the engine views, `domain-export` renders the corpus) | system domain tools |
| `eval/graph/` | graph evaluation harness: tasks, ground truth, reports | `make -C graph eval-*` targets |
| `eval/graph-keyword/` | keyword-retrieval fixtures + results | `graph eval-retrieval` |
| `validate/` | Go tests that load this pack's data through the engine loaders | `go test ./projects/...` |
| `scripts/` | dataset build helpers carrying stablenet parameters | operators |
| `setup.yaml` | dataset build parameters for `knowledge-setup --config` | `cmd/knowledge-setup` |
| `filelist.yaml` | build-scope derivation config (build roots, pinned build context, extra packages/globs) | `cmd/filelist-gen` → `--files-from` list |
| `mcp.yaml.example` | fused-server config template — copy to `mcp.yaml` (gitignored; endpoints stay out of git) and fill deployment values | `cmd/system-mcp -config` |
| `generated/` (untracked) | derived outputs (domain corpus, engine policy views) — rebuild with the domain tools | build outputs |

## Rules

- **Dependency direction**: this pack may import engine code (its `validate/`
  tests do); engine code never references a project pack. Enforced by
  `scripts/check-boundaries.sh`.
- **Data only**: engine behavior changes belong in the engines. A pack carries
  data, configuration, and validation of that data.
