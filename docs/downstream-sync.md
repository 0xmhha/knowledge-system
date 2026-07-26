# Downstream sync runbook

How to port this repository's code into a downstream deployment repo
(e.g. a stablenet-specialized distribution) and verify the port. The two
repos serve different purposes on purpose — upstream generalizes and grows
features; a downstream specializes for one project — so the sync is a
**deliberate, verified port**, not a git merge (the histories are unrelated).

Status: finalized (2026-07-27). Graph, vector, retrieval, and fused-server
equivalence to the pre-consolidation 3-repo tools is verified end to end —
see the review doc §8.16 for the evidence.

## What makes the port mechanical

Three structural properties keep the delta small:

1. **Deploy identity is injected, not edited** (pkg/mcp): tool namespaces,
   server names, and binary names come from config / `-ldflags` /
   `KNOWLEDGE_MCP_NAMESPACE`. A downstream needs zero code edits for
   branding:

   ```
   make build-mcp NAMESPACE=stablenet_knowledge
   ```

2. **Project data lives in a pack** (`projects/<name>/`): policies,
   domain knowledge, eval ground truth, dataset parameters. Packs may
   import engine code; engines never reference packs (enforced by
   `scripts/check-boundaries.sh`).

3. **Equivalence is checkable, not assumed**: the graph digest, the
   full-DB table-hash audit, and the eval fixtures give byte-level and
   behavior-level comparison tools (see "Verifying a port" below).

## Porting procedure

1. **Snapshot the source**: note the upstream commit being ported.
2. **Copy the tree** into the downstream repo (code directories: `cmd/`,
   `internal/`, `pkg/`, `graph/`, `vector/`, `system/`, `scripts/`,
   `Makefile`, `go.mod`/`go.sum` with the module path rewritten to the
   downstream module).
3. **Do NOT re-brand identifiers or comments.** The pre-consolidation
   downstream renamed ckg/ckv/cks throughout the code and lost
   readability; branding now belongs to the build (`NAMESPACE=...`) and
   config (`name:`), never to source edits.
4. **Carry the project pack**: the downstream keeps its own
   `projects/<name>/` content (or consumes this repo's pack verbatim).
5. **Record the port**: downstream commit message names the upstream
   commit it mirrors.

## Verifying a port

Run these against the ported tree; all of them exist in-repo:

| Check | Command | Passes when |
|---|---|---|
| Build/test/lint | `make build test lint` | green (lint includes engine-boundary rules) |
| Graph equivalence | build the same source with upstream and ported binaries; compare `graph_digest` in both manifests | digests byte-equal |
| Full-DB audit (optional, stronger) | sorted-dump hash per table (`nodes`, `edges` minus rowid, `blobs`, `node_prs`, `pending_refs`, `pkg_tree`, FTS content) | all hashes equal (`topic_tree` requires the deterministic-clustering fix; manifest differs only in additive identity keys) |
| Retrieval behavior | `eval-retrieval --fixtures projects/<name>/eval/graph-keyword/fixtures` on both binaries against their own builds | output JSON deep-equal (excluding the graph path field) |
| Tool names | start each MCP server with the downstream namespace; `tools/list` | names are `<root>.context.*` / `<root>.ops.*` with the intended root |
| Alignment gate | `knowledge-setup --config projects/<name>/setup.yaml ...` | verify-align step passes |

## Normalized-diff spot check

For auditing an existing downstream against upstream, normalize the only
legitimate differences and diff:

```sh
perl -pe 's|<downstream module path>|MODPATH|g; s|<upstream module path>|MODPATH|g' ...
```

Anything left beyond module paths and the project pack is unexplained
drift — either port it upstream (if it's a general improvement) or move
it into the pack (if it's project data). Code that is neither is a bug
in the port.

## Known caveats

- `topic_tree` (clustering) was run-nondeterministic before the
  deterministic-clustering fix; ports of older snapshots cannot hash-
  compare that table.
- Datasets built before the enrichment/digest split carry a combined
  digest; the first rebuild after porting changes `graph_digest` once
  when policy/security enrichment is in use (one vector realignment).
- Vector embeddings must be built one at a time when comparing against a
  reference: running several `vector build`s against the same Ollama daemon
  concurrently introduces batching jitter (~1% of vectors differ bit-for-bit)
  even though the inputs are identical. Input equality is provable from the
  `chunks` table hash; serve the equivalence run sequentially for a 100%
  vector match.
- `enrich_digest` was absent from the graph manifest before the 2026-07-27
  fix (enrichment rows were persisted to the store but not folded into the
  digest input, so it hashed to `""` and `omitempty` dropped the key).
  Datasets built before that fix show no `enrich_digest` even when policy /
  security enrichment was applied — rebuild to surface it. `graph_digest` is
  unaffected either way (enrichment is excluded from the coordinate pin).
