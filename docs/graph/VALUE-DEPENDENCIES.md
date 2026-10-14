# Retained Go value and control dependence

CKG computes bounded on-demand dependence from the function source stored in the
index. `pkg/graph/valueflow.Query` is the shared read-only API; the graph stdio
MCP registers `value_dependencies`. A namespace root changes its name to
`<root>.context.value_dependencies`. The query never falls back to the live file.

First resolve a function with `find_symbol`. Select one exact `node_id` or
`canonical_id`; the query rejects both together or neither. Same-name functions
are separate selections, not merged graphs. Only named function/method blobs are
supported. Go is the first supported language; TS/Sol return unsupported.

```sh
ckg build --src ./source --out ./graph --lang go
ckg mcp --graph ./graph --namespace flow_probe
```

Call `flow_probe.context.value_dependencies` with this input shape. Omit limits
to use defaults and replace the selected ID/pins with values from your index.
The actual registered tool schema is authoritative for input/output types.

```json
{
  "node_id": "1111111111111111",
  "limits": {
    "max_source_bytes": 262144,
    "max_ast_nodes": 4096,
    "max_blocks": 256,
    "max_nodes": 1024,
    "max_edges": 4096,
    "max_iterations": 128,
    "max_facts": 65536,
    "timeout_ms": 1000
  }
}
```

Optional `expected_commit` requires a full40/64-character lowercase Git commit;
`expected_source_sha256` pins the function bytes and `expected_snapshot_sha256`
pins the retained manifest. Query results include the snapshot digest and
analysis source: node/canonical ID, function name, file, commit, byte SHA and
original line/byte origin. The query re-reads manifest, node and blob after
calculation. Drift, missing blob, invalid coordinates or another function name
cannot produce a normal verified result. Empty/abbreviated revision provenance
is explicit uncertainty; no current-checkout revision is inferred.

## Computation and supported precision

The Go CFG uses x/tools CFG successors, branch joins, loops, returns and
unreachable blocks. It is not AST statement order. Isolated Go type bindings
identify declarations and references; name equality does not merge shadow
scopes. Missing imports/package types produce partial binding diagnostics.
Type errors are not claims that the original project failed to compile: the
query has one retained function rather than the complete package.

Reaching definitions preserve simultaneous RHS evaluation, redefinition kills,
branch joins and loop fixed points. Parameters/receiver/named-result zero values
enter at the synthetic entry. Local constants are compile-time source nodes
with `block=-1` and `kind=compile_time_constant`; they are not invented runtime
CFG statements. Def-use relations bind occurrence IDs and lexical variable IDs
with original coordinates. Implicit named-return uses cite the actual return.

| Relation | Meaning and boundary |
|---|---|
| `cfg` | Structural successors. Boolean branch labels apply only to their actual condition expression. Path feasibility is not proven. |
| `data_dependence` | Local scalar definitions may reach value uses. This does not prove a feasible runtime trace. |
| `address_dependence` | A container/address binding reaches an indirect access. Field/pointer contents remain unknown. |
| `reference_dependence` | Reference binding copy/use; no memory alias or effect proof. |
| `aggregate_dependence` | Aggregate binding use; contents remain outside scalar precision. |
| `unknown_dependence` | Uninstantiated generic value kind remains unknown. |
| `control_dependence` | Normal-exit postdominators, including loop self-control. Possible non-exiting regions are excluded and diagnosed. |

Field/index/pointer memory, aggregate contents, external/callback calls,
reflection/dynamic effects, closure captures, async/deferred effects, exceptions
and short-circuit expression flow are unavailable or conservative diagnostics.
The typed capability map records these boundaries. This is not whole-program
alias/taint analysis or a validated vulnerability verdict. There is no LLM in
this computation and no automatic manual-policy/security approval.

## Results, limits and uncertainty

`analyzed_bounded` means the requested supported calculation completed.
`partial` retains usable scoped relations with explicit unknowns.
`unsupported` has no supported analysis for the selected language/construct.
`budget_exhausted` is truncated/incomplete, never safe. Missing identity/source,
stale pins and snapshot changes are distinct query statuses.

Source/AST/block/node/edge caps stop oversized work. Iteration limits count actual
completed iterations; facts reserve variable/occurrence, reaching-state and
postdominator storage before allocation where applicable. `facts_peak` is peak
accounted/reserved relation facts, not process-memory bytes. Parse/type analysis
and reader calls cannot be preempted mid-call; the bounded source/AST and
cooperative deadline checks restrict work and report expiration. This is not a
hard real-time deadline or a total SQLite/index-memory bound. `elapsed_micros`
reports measured query/analysis cost rather than a throughput guarantee.

A missing edge never means safe, independent, sanitized or irrelevant. Consumers
must retain source pins, capability, diagnostics, truncated status and certainty
when combining functions. Existing calls/curated flow are different relations.
The query reads no node key material and adds no authentication bypass. Existing
reader/MCP retrieval boundaries continue to apply; private source paths are
excluded from the retained query.

## Verification

Independent expected fixtures check kills, shadow scopes, parallel assignments,
branch/early-exit control, loops/self-control, named returns, constants,
reference/generic distinctions, fields/closures/nontermination, original spans,
budgets, source drift and actual runtime schema validation. The opt-in
`scripts/eval-value-dependencies.py` compiles/race-tests an owned Go fixture,
builds its index and exercises namespaced stdio MCP. It preserves raw requests,
responses, source bundle and receipts. Source deletion confirms no live fallback.
Independent production and whole-goal human acceptance remain separate.

## Bounded multi-function context in CKS

CKS `context.value_dependency_expansion` composes the same local Go computation
through retained outgoing calls/invokes. Its required `function` object has the
exact selector/source pins/limits above. Optional `limits` defaults to4 functions,
1 hop,32 unique candidates,2048 total local nodes,8192 local edges plus call hops,
131072 cumulative analyzed facts and1000ms cooperative total time. Caps are8/3/
128/4096/16384/262144/5000 respectively. Logical fact/output budgets do not claim
process-byte limits or parser/typechecker preemption.

Each retained function is computed once within an expansion, cycles and repeated
targets are deduplicated, and function/blob/manifest pins are rechecked along with
consulted metadata and edge pages. SQL retrieves a bounded stable prefix before
Go materialization; legacy readers lacking this optional capability report
partial/unavailable, never an unbounded fallback. Candidate/function/hop/output/
fact/time bounds and source changes remain explicit. More rows at a page boundary
are a conservative incomplete signal even when some returned rows are filtered.

Call hops have `origin=retained_call_graph` and `argument_mapping=not_computed`.
They are context links rather than calculated parameter/return or memory transfer.
Each function's local def-use/value/control relations remain under its own source
identity. Curated operator flows retain the existing separate `context.get_flow`
surface: the expansion labels them `separate_not_joined`, with source association
`not_verified`. It does not borrow their reads/writes/order as computed dependence.
This endpoint does not automatically perform a curated-flow lookup or dataset join.

A controlled6-function corpus compares independent relevance labels: baseline
returns1 of4 relevant functions; expansion returns all4 plus1 noise function.
Report denominators, caps and elapsed cost for that fixture; they are not global
recall, a relevance proof, whole-program transfer or an optimization claim.
