# Project domain knowledge

Each project pack owns its entries, provenance, code anchors, glossary and rendered corpus. The repository example is [projects/stablenet/domain-knowledge](../../../projects/stablenet/domain-knowledge); shared rules live here. Engine code must not embed project-specific catalog data.

| Input | Current boundary |
|---|---|
| project.yaml, subsystems.yaml, entries/*.yaml | [inventory loader/validator](../../../internal/system/inventory) resolves project context and anchor paths |
| verified entry status | Recorded content-review state, not independent answer-quality proof |
| selected worksheet_packs | Pinned optional catalogs; keyword mappings are review aids, not automatic decisions |
| glossary and corpus | Derived by domain commands from explicit project inputs, then passed into setup/config |

The inventory directory is an input path, not a new global projects/active_project server configuration contract. CKS serves one configured backend/dataset context per instance. Coordinate isolation and immutable snapshots are defined by [the source contract](../../../docs/spec-driven/SOURCE-SNAPSHOT-ADR.md); reading a pack or generating a worksheet does not activate ontology.

See [registration](./shared/PROJECT_REGISTRATION.md), [entry fields](./shared/SCHEMA.md), [knowledge types](./shared/KNOWLEDGE_TYPES.md), [status lifecycle](./shared/STATUS_LIFECYCLE.md) and [worksheet catalog contract](../../../docs/spec-driven/WORKSHEET-CATALOG-CONTRACT-V1.md).
