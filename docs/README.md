# Documentation

These entry points describe the current repository. Source files, registered tool schemas and executable help are the authority for supported behavior. Completed plans and session reports have been removed from the working tree; their recorded revisions remain recoverable.

| Need | Read |
|---|---|
| Use an extracted host package | [package installation](./INSTALLATION.md) |
| Install and build a dataset | [repository quick start](../README.md), [guided setup](../tools/setup/README.md), [setup reference](../system/docs/SETUP.md) |
| Understand architecture and purpose | [architecture](./ARCHITECTURE.md), [purpose and scope](./spec-driven/PURPOSE-AND-SCOPE.md), [graph purpose](./graph/VISION.md) |
| Read completed dataset comparison and shutdown evidence | [go-stablenet dataset comparison](./spec-driven/GO-STABLENET-DATASET-COMPARISON.md) |
| Read the implementation-before audit | [upper work-item audit](./spec-driven/UPPER-WORK-ITEMS-AUDIT.md) |
| Read completed implementations and pending independent acceptance | [implementation specification and progress](./spec-driven/UPPER-WORK-ITEMS-IMPLEMENTATION.md) |
| Prepare operational and independent acceptance | [targets and criteria](./spec-driven/OPERATIONAL-ACCEPTANCE-PLAN.md), [acceptance ledger](./spec-driven/OPERATIONAL-ACCEPTANCE-PROGRESS.json), [protocol and freeze checklist](./spec-driven/OPERATIONAL-ACCEPTANCE-PROTOCOL.md) |
| Find all remaining work | [remaining work](./REMAINING-WORK.md) |
| Resume development and verify completion | [new-machine handoff](./spec-driven/MACHINE-HANDOFF.md), [current handoff](./spec-driven/CURRENT-HANDOFF.md), [execution specification](./spec-driven/REFACTORING-EXECUTION-SPEC.md), [progress ledger](./spec-driven/REFACTORING-PROGRESS.json) |
| Query code and meaning over MCP | [system](../system/README.md), [ontology](../system/docs/ontology-mcp.md), [tool mapping](../system/docs/coding-agent-mcp-mapping.md) |
| Graph APIs, schema and cache | [graph index](./graph/DOC-MAP.md) |
| Vector APIs, schema and models | [vector index](./vector/README.md) |
| Dataset and knowledge contracts | [contract index](./spec-driven/README.md) |
| Interpret evaluation and release limits | [evaluation status](./spec-driven/EVALUATION-STATUS.md), [release policy](./spec-driven/RELEASE-POLICY.md) |
| Operate and recover a service | [availability](../system/docs/ops-availability.md), [versioned reindex](../system/docs/ops-blue-green-reindex.md) |
| Port to a distribution | [downstream synchronization](./downstream-sync.md) |
| Recover a removed record | [evidence recovery](./spec-driven/EVIDENCE-RECOVERY.md), [cleanup inventory](./DOCUMENTATION-CLEANUP.json) |

[Documentation policy](./DOCUMENTATION-POLICY.md) defines current-document maintenance, immutable evidence and checks. The machine-readable [catalog](./DOCUMENTATION-CATALOG.json) distinguishes current references, source fixtures and retained evidence; the same rules are checked by `make docs-check`.
