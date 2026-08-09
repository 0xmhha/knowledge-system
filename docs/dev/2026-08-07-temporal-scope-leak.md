# The temporal pass ignores the build-scope file list

Found while verifying a dataset built from go-stablenet `0bf2f4d1b` with a
derived build scope (`projects/stablenet/filelist.yaml`). The symbol graph
honours the scope; the temporal pass does not, so the database carries hunk
history for files the dataset deliberately excluded.

## What the scope says

`cks filelist` derived 1044 entries — 671 files in the dependency closure of
`./cmd/gstable` + `./cmd/genesis_generator`, 322 tests of those packages, 29
explicitly configured extra packages, and 22 Solidity assets. The source tree
tracks 1297 Go files, so 275 are out of scope by design. The graph build
applied it: `files-from applied go=1256 go_after=1018`.

## What the database holds

| Measure | Value |
|---|---|
| Distinct Go file paths in `nodes` | 1241 |
| …of which outside the include list | 223 |
| Node types those 223 files produced | `Hunk` only — 5319 nodes |
| Hunk nodes in total | 9072 (**58% out of scope**) |
| Patch blobs for out-of-scope hunks | 5288 blobs, 5.1 MiB gzipped |
| Edges touching them | 8184 |
| Share of all nodes | 2.8% |

No `Function`, `Method`, `CallSite` or other structural node exists for an
out-of-scope file. The leak is confined to the temporal axis.

## Root cause

`buildPipeline` calls `emitTemporalEdges(g, srcRoot, log, temporalDepth)` — the
filter is never passed. Inside:

- `temporal.LoadHistory(repoRoot, maxPerFile)` walks the repository's whole git
  log, and `remapToSrcRel` keeps every path under `srcRoot`. The scope is a
  *directory*, not the include list.
- `emitHunkGraph` → `temporal.LoadHunks(repoRoot, 0)` does the same for hunks,
  and `buildHunkNodes` creates a `Hunk` node per (file, commit) straight from
  git output.

The file-level pass survives this by accident: `buildTemporalEdges` resolves
`changed_in` / `blame` through `nodesByPath` / `fileByPath`, which only contain
in-scope files, so those edges cannot reach outside. Hunks have no such
lookup — they are keyed by the path git reports.

`emitModifiesEdges` then finds no CodeNode to attach out-of-scope hunks to, so
they sit in the graph with `has_hunk` / `adjacent` edges and a patch blob, but
no link to any symbol.

## Why it matters

Storage is the smaller half: 5.1 MiB of patches here, but the ratio scales with
how narrow the scope is — a monorepo indexed for one binary would carry history
for everything beside it.

The retrieval concern is sharper. Hunks feed change-history and evidence
surfaces, so a query can return a citation to a file the dataset otherwise does
not know: no symbols, no bodies, no conventions. An agent receiving that
citation cannot follow it into anything else in the pack.

## Options

1. **Filter hunks by the include list.** Plumb the loaded `filterlist.Filter`
   into `emitTemporalEdges` and drop hunks whose file it excludes. Exact, and
   it matches what the operator declared.
2. **Filter by the graph's own File nodes** — mirror what `buildTemporalEdges`
   already does. No new plumbing, but it also drops hunks for files deleted
   from the tree, which the recovery track (hunk-graph §11.3, unreachable
   history) exists to surface. In this dataset every out-of-scope file still
   exists on disk, so the two options differ only for deleted files.
3. **Leave it and document the axis as repository-wide.** Cheapest, but the
   `--files-from` contract then means "scopes parsing" rather than "scopes the
   dataset", and the citation problem stands.

Option 1 with a note about deleted files is the honest fix; option 2 is
tempting because it needs no plumbing, and would silently change what the
recovery track can see.

## Reproducing

```sh
python3 - <<'PY'
import json, sqlite3
V = "<dataset>/<version>"
inc = set(json.load(open(f"{V}/files-from.json"))["include"])
db = sqlite3.connect(f"{V}/graph/graph.db")
rows = db.execute("SELECT id, file_path, type FROM nodes WHERE file_path LIKE '%.go'").fetchall()
out = [r for r in rows if r[1] not in inc]
print(len(out), "nodes outside the include list;", {t for _, _, t in out}, "types")
PY
```
