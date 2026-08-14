# What actually broke that dataset, and where the gate belongs

2026-08-14. Answers the open question in the downstream handover
`2026-08-14-ops-index-writes-live-without-a-gate.md`, which asked what rebuilt
the vector index alone — and notes that the answer changes where the check has
to sit.

## The handover's reading

```
graph.db   last written  2026-08-13 13:00:54
vector.db  last written  2026-08-14 14:03:34     ← a day later, alone
ALIGNMENT FAILED         2026-08-14 14:03:35
```

Read as: something rebuilt the vector index on the 14th without the graph.

## What the manifests say

File mtimes record when a file was last written, which for a SQLite database
includes being reopened. The manifests record when an index was *built*, and
they disagree with that reading:

| | recorded | source |
|---|---|---|
| vector index built | **2026-08-13 12:55:45** | `vector/manifest.json` `built_at` |
| graph written | **2026-08-13 13:00:54** | `graph/graph.db` mtime |
| vector.db touched | 2026-08-14 14:03:34 | mtime, `built_at` unchanged |

The vector index was built five minutes **before** the graph, on the same day,
and its content did not change afterwards — `built_at` is still the 13th. The
14th's mtime is the file being reopened, not rebuilt; the alignment assert
fired one second later, which is the restart that reopened it.

So the causality is the reverse of the handover's: **the graph was rebuilt
alone**, at 13:00 on the 13th, and the vector index's pin to the graph digest
went stale where it sat. Nothing rebuilt the vector on the 14th. The mismatch
was ~25 hours old when it surfaced, which is the gap the handover correctly
identifies — only the direction differs.

## Why the direction matters

It moves the check.

The handover's narrow fix — and the first cut of it here — ran the alignment
gate only when the deployment builds **both** indexes and both builds
succeeded. Under that guard, the refresh that actually broke this dataset
would have been skipped: it rebuilt the graph, the vector binary was not part
of that call, so there was "no pair to check".

Rebuilding one index is not the case to skip. It is the case that produces the
failure: one half moves and the other half's pin to it goes stale. Which half
moved does not matter.

So the gate applies whenever the deployment **has** both indexes on disk,
whatever this particular refresh rebuilt. It is skipped only when there is
genuinely nothing to compare — no vector index in the deployment, or one half
never built — and when a build already failed, since a precise error should
not be replaced by a vaguer one.

## What is in this tree now

`verifyIndexAlignment` in `internal/system/mcp/ops_index.go`, running
`setup.VerifyAlignment` — the same gate `cks setup` runs at its `verify-align`
step, rather than a second implementation of the same judgement — against the
dataset the refresh just wrote. A mismatch is reported as
`index refresh FAILED (both engines exited 0, but the dataset they wrote is
not servable)` with the reason, instead of `index refresh result`.

Tests cover the pair agreeing, both mismatch shapes, the graph-only refresh
above, and a deployment with no vector index at all. They fail with the gate
removed.

## What this does not fix

The write is still in place. The gate turns a silent corruption into a loud
one at the moment it is caused; it does not leave a previous version to return
to. That is the structural change the handover proposes — routing `ops.index`
through the blue-green path `ops.reindex` already uses — and it stays open,
with one thing worth noting for whoever takes it: those two tools already
exist side by side, one gated and versioned, one not. The structural fix is
mostly the question of why both do.
