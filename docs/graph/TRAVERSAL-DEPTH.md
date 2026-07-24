# TRAVERSAL-DEPTH — why the call-graph tools default to depth=2

Tier 2 (design rationale). Authoritative for "why `depth=2` is the default" on
the call-graph traversal tools. The `depth` argument is caller-overridable; this
doc explains the default the tools ship with.

## Scope

Applies to the reverse/forward call-graph traversals and their reachability
cousins:

| Tool | Default | Traversal |
|---|---|---|
| `find_callers` | `depth=2` | reverse BFS over `calls` + `invokes` edges |
| `find_callees` | `depth=2` | forward BFS over `calls` + `invokes` edges |
| `impact_of_change` | `depth=2` | reverse-dependency closure (broader than callers) |

## Decision

**Default `depth=2`.** One hop of indirection past the direct neighbours.

## Why not `depth=1`

A 1-hop traversal returns only the *direct* callers/callees of the seed. It
misses the common helper-chain pattern

```
f  →  lockedHelper  →  reallyTouchesField
```

where the caller the user actually cares about sits two hops away. Direct-only
retrieval scores recall 1.0 on direct edges but 0 on anything reached through an
intermediate — a frequent and silent false-negative.

## Why not deeper

BFS fan-out grows super-linearly with depth: each extra hop multiplies the
frontier by the average out-degree, pulling in progressively lower-signal nodes
(noise) for progressively more latency. `depth=2` is the sweet spot that
captures indirect neighbours while keeping the result set and the query cost
bounded. Depth 3+ is retained only as a regression/determinism guardrail in the
eval suite, not as an interactive default.

## Evidence

Recall targets validated in the call-graph eval (`core.NewBlockChain` seed,
ground truth by grep):

| Depth | Target |
|---|---|
| 1 | precision 1.0 / recall 1.0 (direct callers) |
| 2 | recall ≥ 0.9 (zero missed indirect callers as the goal) |
| 3 | regression only — determinism of results confirmed |

Latency envelope (go-stablenet graph, ~10K call edges; see
[ARCHITECTURE-DETAILED.md](ARCHITECTURE-DETAILED.md) §17.2):

| Tool | Depth | Time |
|---|---|---|
| `find_callers` / `find_callees` | 1 | < 50 ms |
| `get_subgraph` | 2 | < 100 ms |

Both sit well inside the interactive budget, so latency is not the binding
constraint at `depth=2` — signal-to-noise is. The default trades a small,
bounded latency increase over `depth=1` for the recall of indirect edges.

## Overriding

Pass the `depth` argument to widen or narrow a single call. Raise it when a seed
is known to sit behind deep helper chains; lower it to `1` when only direct
neighbours are wanted and the result set must stay minimal.

## Provenance

The `depth=2` recommendation originated in the Stage-A depth analysis of
[design/go-cross-function-lock-propagation.md](design/go-cross-function-lock-propagation.md)
§Q1 ("(b) depth=2 fixed — sweet spot, noise controllable"). This doc
consolidates that decision and the measured latency/recall evidence into the
standing reference the tool descriptions point at.
