# Spec-driven documentation

This directory contains current task contracts and progress records alongside
historical design, review and evaluation documents. The live worklist, execution
specification, ledger and current handoff are updated when work changes. Dated
captures and decisions retain the facts and limitations recorded at the time.

## Current entry points

| Question | Document |
|---|---|
| Where should a new session start? | [CURRENT-HANDOFF.md](./CURRENT-HANDOFF.md) |
| Which tasks are complete or remain? | [REFACTORING-REMAINING-WORKLIST.md](./REFACTORING-REMAINING-WORKLIST.md) — original 17/18, ontology 4/4; N16 on hold |
| What must each task prove? | [REFACTORING-EXECUTION-SPEC.md](./REFACTORING-EXECUTION-SPEC.md) |
| Which committed evidence supports completion? | [REFACTORING-PROGRESS.json](./REFACTORING-PROGRESS.json) |
| How do clients use ontology over MCP? | [Ontology MCP reference](../../system/docs/ontology-mcp.md), [tool mapping](../../system/docs/coding-agent-mcp-mapping.md) |
| What did the latest ontology comparison establish? | [ONTOLOGY-FINAL-VERDICT-2026-10-13.md](./ONTOLOGY-FINAL-VERDICT-2026-10-13.md) — observations complete, independent quality inconclusive, default off |
| Has this branch been pushed or given a PR? | [SETUP-ONTOLOGY-PR-PREPARATION-2026-10-13.md](./SETUP-ONTOLOGY-PR-PREPARATION-2026-10-13.md) — normal PR #99 created; initial publication receipt and remaining CI/review work |
| What release scope was accepted? | [N18-RELEASE-VERDICT-DRAFT.md](./N18-RELEASE-VERDICT-DRAFT.md) — local operation, prior PR #98 merged; public tag/release on hold |

## Design and original purpose

Start with [original-goal audit](./REFACTORING-ORIGINAL-GOALS-REAUDIT.md) and
[original/v2 trace](./REFACTORING-ORIGIN-AND-V2-TRACE.md). Public dataset,
source-snapshot and domain-pack contracts are indexed in [docs/README.md](../README.md).
The twenty ontology types are explained in [ONTOLOGY-20-USAGE.md](./ONTOLOGY-20-USAGE.md).

## Historical records

| Record | How to read it |
|---|---|
| [REFACTORING-STATUS-HISTORY-2026-10-13.md](./REFACTORING-STATUS-HISTORY-2026-10-13.md) | Older worklist/specification opening logs; “current” counts refer to their recorded dates |
| [FINAL2-RESULT.md](./FINAL2-RESULT.md), [N11-FINAL-RESULT.md](./N11-FINAL-RESULT.md) | Earlier frozen evaluations, never reused as fresh holdouts |
| ONTOLOGY dated review, DEV and cost reports | Preparation and observations leading to the final verdict; their earlier pending states are historical |
| CLAUDE-CODE and REMOTE-SESSION handoffs | Instructions and transfer scopes from their original sessions; use CURRENT-HANDOFF for present work |
| A/B/C/D and N-prefixed records | Earlier requirements, reviews, tests and decisions; follow their explicit scope |

Original raw inputs, protocols and manifests under system/eval remain immutable.
No document cleanup turns source-range coverage into independent quality approval.

## Verify completion

```sh
make refactoring-check
```

The checker reads each evidence manifest at its recorded Git revision and
verifies current sources against the active binding. Retain the original
feat/refactoring-followup and feat/setup-tool evidence commits when squash
merging; a squash commit alone does not contain their historical objects.
