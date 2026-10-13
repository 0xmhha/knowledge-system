# Guided setup and ontology PR preparation — 2026-10-13

[Normal PR #99](https://github.com/0xmhha/knowledge-system/pull/99) is OPEN and not a draft. The approved branch was pushed at initial head `82dbb684`. The original preparation snapshot below is historical; publication does not mean CI acceptance, main merge or release.

The remote audit before
documentation cleanup found no feat/setup-tool branch, no matching PR and no
open PRs. Inspected head d5784a20 had 44 commits and 908 changed paths against
origin/main21e26ac2; documentation cleanup adds local commits and paths.
The earlier PR #98 merge does not publish these changes.

## Published target and original prepared content

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

The upstream main documentation commit was integrated locally as 0c87bee9;
the four documentation conflicts were resolved while retaining current
progress, upstream release clarification and historical evidence.

## Publication checklist and remaining delivery work

- [x] Inspect the whole branch and integrate upstream documentation changes.
- [x] Obtain explicit approval for all 48 commits / 939 changed paths.
- [x] Push feat/setup-tool and verify initial remote head `82dbb684`.
- [x] Create normal PR #99 and attach its actual URL to this task.
- [x] Pass required CI: patched head `de3dcee1`, [run 38359250800](https://github.com/0xmhha/knowledge-system/actions/runs/38359250800), all five jobs succeeded. Original run38356674327 failed on the Go/linter mismatch; its failure receipt remains historical.
- [x] Review advisory vulnerability findings: Go1.26.9 reports no reachable or imported-package vulnerabilities. Two module-only findings in x/mod remain documented; the scan does not establish that older installed binaries or minimum-version compatibility artifacts were remediated.
- [ ] Complete PR review. Main merge requires a separate instruction.

At the initial published head, vector/graph evaluation gates and the native
installation contract passed. The patched headde3dcee1 subsequently passed all five CI jobs. Later commits
have their own CI run; do not treat this initial snapshot as their results.
[CI correction and final verification](./CI-TOOLCHAIN-VALIDATION-2026-10-13.md).
Original development remains 17/18, with N16 on hold; ontology is 4/4.
Tag and public deployment remain on hold. Publication does not change the
independent quality verdict, performance acceptance or optional defaults.

[Initial publication verification](../../system/eval/b0-knowledge-system/setup-ontology-publication-2026-10-13/publication-verification.json),
[exact PR metadata](../../system/eval/b0-knowledge-system/setup-ontology-publication-2026-10-13/pr-at-initial-publication.json).

## Evidence and current documents

[Git/PR snapshot](../../system/eval/b0-knowledge-system/documentation-status-review-2026-10-13/git-status-before-cleanup.json),
[current handoff](./CURRENT-HANDOFF.md),
[worklist](./REFACTORING-REMAINING-WORKLIST.md),
[ontology final verdict](./ONTOLOGY-FINAL-VERDICT-2026-10-13.md).
