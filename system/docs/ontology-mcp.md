# Ontology over MCP

CKS exposes four read-only semantic tools and an optional explanation on the
v2 task context tool. These use the same pinned retained source, graph, vector
and semantic coordinates. The tool schemas are defined in
[agent-mcp.schema.json](../../internal/system/mcp/testdata/agent-mcp.schema.json).

## Read meaning before interpreting search effects

| Tool | Arguments | Meaning of the result |
|---|---|---|
| cks.semantic.status | none | Configured mode, aligned data availability, read_tools_ready and retrieval_ready; readiness is separate from quality |
| cks.semantic.lookup_term | language, term; optional limit | Exact term candidates, ambiguity, review status and source evidence |
| cks.semantic.get_concept | concept_id; optional limit | Definition, scope, multilingual terms, reviewer/evidence and typed incident relations |
| cks.semantic.trace_requirement | requirement_id; optional limit | Requirements/criteria, available code/test paths and missing/unapproved/conflicting links; no test execution |

Read limits are integers from 1 to 100, default 20. Missing/stale/invalid data and
budget limits remain explicit states. Reads do not create or approve data,
activate a dataset, call an embedding model or certify criterion acceptance.
Proposed/rejected meanings may be visible for review; retrieval uses only
verified aligned inputs.

Example lookup arguments for the approved WBFT subset:

```json
{"language":"ko","term":"쿼럼","limit":20}
```

Example requirement trace arguments:

```json
{"requirement_id":"req-wbft-quorum","limit":20}
```

The WBFT held pilot currently reports missing_code and
acceptance_evaluated=false: approved requirement text does not create reviewed
implementation/test relationships or prove that the criteria passed.

## Explain a task response

Use cks.context.get_for_task_v2 with explain_ontology=true when an explanation
is needed. The request does not enable ontology or change the configured mode.
It returns bounded matched concepts and candidate score/rank changes, separates
off/no_match/no_candidates/fallback/applied/no_effect, and preserves v2
integrity. At most 8 concepts and 20 changes are reported. Uncited candidate keys
are hashed; scores are decimal strings. selected_by_composer describes composer
selection before the final body budget, so inspect the published bodies for
actual output.

Ontology mode remains off by default. With an explicitly configured verified
store, concept_text can search reviewed concept definitions and adjust candidate
scores; reviewed relations can contribute when such links exist. It keeps the
CKV+CKG evidence path and source validation. Status/lookup/trace availability is
independent of enabling retrieval.

## What the evaluation supports

The [final observation](../../docs/spec-driven/ONTOLOGY-FINAL-VERDICT-2026-10-13.md)
verified 702 responses / 351 pairs without errors. Recall stayed equal and the
expected-rank measure declined slightly. Aggregate warm p95 increased 8.6%;
three questions exceeded the descriptive 1.25 ratio. Independent approval was
not established. Quality remains inconclusive, the feature stays opt-in and
no operating dataset/configuration was activated by the evaluation.

For registration, all tool options and controlled capture guidance, use the
[agent mapping](./coding-agent-mcp-mapping.md). For implementation and historical
validation, use the [implementation record](../../docs/spec-driven/ONTOLOGY-MCP-IMPLEMENTATION.md).
