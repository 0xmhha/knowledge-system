# Guided setup and ontology PR preparation — 2026-10-13

This is a local review packet, not a created GitHub PR. The remote audit before
documentation cleanup found no feat/setup-tool branch, no matching PR and no
open PRs. Inspected head d5784a20 had 44 commits and 908 changed paths against
origin/main21e26ac2; documentation cleanup adds local commits and paths.
The earlier PR #98 merge does not publish these changes.

## Proposed target and content

Repository: 0xmhha/knowledge-system. Head: feat/setup-tool. Base: main.
The existing release tag/deployment hold remains separate from PR review.

Proposed title:

```text
feat: add guided setup and ontology inspection tools
```

Proposed body:

```markdown
Guided setup builds and checks coordinated graph/vector datasets with visible
progress. MCP clients can inspect aligned ontology readiness, read terms and
concepts, trace requirements, and request bounded explanations of changes to
v2 evidence. Optional ontology retrieval stays off by default.

The branch also fixes engine error handling and input validation, reuses small
coordinate decoding results only after hashing fresh file bytes, and documents
source-bound evaluation limits and the current handoff.

Validation:
- The latest frozen WBFT comparison verified 702 responses / 351 pairs with no
  failed or missing calls; all response/source hashes and replayed scores match.
- Recall stayed equal and the expected-rank measure declined slightly.
  Independent quality remains inconclusive; proposed clusters are not approved
  independent units. No new FINAL, operating activation or criterion approval.
- Aggregate warm p95 ratio was 1.086; three questions exceeded 1.25. Overall
  performance acceptance remains false and earlier observations stay separate.
- Ontology Python checks 80, progress guard tests 11 and documentation checks
  passed. Prior targeted Go race evidence is retained at its recorded revision;
  the documentation cleanup does not claim a fresh Go regression run.

N16 public-key fingerprint/trust conditions and public tag/release stay on hold.
This PR is for code/document review and does not authorize deployment.
```

## Before creating the PR

Confirm the final local head and whole branch diff, integrate the current main
if GitHub reports conflicts, preserve original evidence commit objects, and
push the intended branch. Create a normal reviewable PR and attach its actual
URL to the task. CI for this new branch has not run because it is not pushed.
A prepared title/body, a local commit or a prior merged PR does not complete
push/PR/CI/main synchronization.

## Evidence and current documents

[Git/PR snapshot](../../system/eval/b0-knowledge-system/documentation-status-review-2026-10-13/git-status-before-cleanup.json),
[current handoff](./CURRENT-HANDOFF.md),
[worklist](./REFACTORING-REMAINING-WORKLIST.md),
[ontology final verdict](./ONTOLOGY-FINAL-VERDICT-2026-10-13.md).
