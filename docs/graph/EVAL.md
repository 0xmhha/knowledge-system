# CKG Eval

> Supersedes the V0 LLM-baseline harness this file used to describe
> (`ckg eval` with alpha/beta/gamma/delta baselines and an LLM judge).
> That subcommand was excised — the binary stays deterministic; the
> LLM-driven half lives at the agent/session layer. The V0 design and
> its 2026-05 measurement notes remain in this file's git history.

## Run

```bash
make -C graph eval
```

Five deterministic steps, all against a self-index of this repository
(the corpus must stay stable so expected node IDs hold):

| Step | Command under the hood | Output |
|---|---|---|
| 1. self-index | `ckg build --src=.. --files-from-main ./cmd/graph --fail-on-parse-errors` | `graph/eval/.ckg-data` |
| 2. validate | `ckg validate --format=json` | `validate.json` (schema integrity; issue count must not rise) |
| 3. benchmark | `ckg benchmark --format=json` | `benchmark.json` (token reduction) |
| 4. bench-mcp | `ckg bench-mcp --depth-sweep --iterations=50` | `bench-mcp.json` (tool latency p50/p95/p99) |
| 5. retrieval | `ckg eval-retrieval --fixtures=eval/retrieval` | `retrieval.json` (recall/precision/F1 over pinned fixtures) |

Results land in `graph/eval/results/latest/`; the committed baseline
lives in `graph/eval/baseline/`.

## Gate (CI)

```bash
cks eval-gate --baseline graph/eval/baseline --latest graph/eval/results/latest
```

Fails when aggregate recall/precision/F1 drop below the baseline beyond
the tolerance (default 0.02), or when the validate issue count rises.
Improvements never fail the gate — after reviewing a genuine improvement,
refresh the baseline:

```bash
make -C graph eval-baseline-update
```

CI runs both steps in the `eval-gate-graph` job (`.github/workflows/ci.yml`)
with a full-history checkout — the temporal pass walks git history, so a
shallow clone would index a different corpus.

## Related

- Keyword-retrieval fixtures against the stablenet pack:
  `make -C graph eval-stablenet-keyword` (fixtures under
  `projects/stablenet/eval/graph-keyword/`).
- CKV-mirror comparison: `make -C graph eval-ckv-mirror`.
