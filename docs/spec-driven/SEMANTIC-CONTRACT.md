# Source-backed semantic projection contract

The current implementation lives in internal/system/semantic and cmd/cks/semanticcli. It supports core projection schema3 and tuple-scoped selected-pack projection schema4; recorded v1/v2 documents retain immutable bytes and read compatibility. Meaning records describe reviewed source-backed concepts over CKV/CKG rather than replacing AST taxonomy or approving business truth.

## Identity, source and review

A projection records project/dataset/snapshot/source-mode identity. Evidence spans identify exact relative files, source lines and content hashes, optionally joined to canonical graph symbols or vector chunks. Concepts, terms, requirements, criteria and assertions have separate provenance and proposed/verified/rejected review states. Source text and approved metadata must agree; editing JSON alone does not approve changed YAML.

Versioned commands accept a pinned version-dir and validate retained source/graph/vector coordinates. Legacy committed-source commands require matching repo/graph/vector/store inputs. Missing/stale/unavailable data is an explicit failure or bounded diagnostic state. Reads must not create a missing store, activate a candidate or read unrelated current source bytes.

## Supported workflow

| Command | Behavior |
|---|---|
| cks semantic build | Extract source/spec/ontology, validate and save a projection; extract-only leaves a reviewable proposal |
| cks semantic review | Enumerate review targets and source evidence; no automatic meaning approval |
| cks semantic promote | Revalidate reviewed projection/source/graph/vector anchors before storing; activation is explicit |
| cks semantic lookup-term | Resolve exact multilingual term candidates under aligned source coordinates |
| cks semantic trace / plan | Return requirement paths, gaps/conflicts and next actions; do not execute tests or approve criteria |
| cks semantic export-text | Produce reviewed meaning text with source/projection/file hash provenance |
| cks semantic annotate-pack | Add source-validated meaning links to an existing pack without replacing its code evidence |
| cks semantic test | Record snapshot-bound execution; go-test-exact verifies the named test run/pass events |
| cks semantic compare-responses | Validate two saved v2 packs and report observed differences; no quality/latency verdict |

Use each command's --help for required project/store/source or version-dir arguments. [Patch workflow](./PATCH-WORKFLOW.md) binds actual test reports and separate reviewer decisions to held candidates.

## Relation and acceptance semantics

New criterion→test assertions use CHECKED_BY; old ACCEPTED_BY is read-only compatibility and does not mean accepted. IMPLEMENTED_BY/TESTED_BY/CHECKED_BY require reviewed source endpoints and valid graph/test anchors. Source binding, linked paths, successful test execution and CriterionDecision approval are distinct states. Exact Go execution proves the selected test function ran and passed, not that a natural-language requirement is satisfied.

The runtime supports read-only status/term/concept/requirement MCP tools and optional retrieval modes off/baseline/relations/concept_text/combined. Default is off. Verified, aligned inputs are required for retrieval. Original-query CKV+CKG evidence remains primary; ambiguity, missing inputs and budgets do not create facts. [Ontology MCP](../../system/docs/ontology-mcp.md) specifies the current read limits and bounded v2 explanation.

The [public contract](./PUBLIC-CONTRACT-V2.md) owns response/integrity compatibility. [Core20](./ONTOLOGY-20-USAGE.md) and [pack contract](./DOMAIN-PACK-CONTRACT-V1.md) own type/instance separation. [Evaluation status](./EVALUATION-STATUS.md) records observed effects, missing independent approval and performance limits; implementation completion does not change those verdicts.
