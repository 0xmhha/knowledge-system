# Spec-driven refactoring records

This directory records the spec-driven refactoring of the knowledge system:
what each task had to prove, how it was tested, and what was decided. The
documents are kept as a record and are written in Korean. They are not
updated when the code moves on; for the current behaviour read the code and
the [reference documents](../README.md#reference-kept-in-step-with-the-code).

## Where to start

| Question | Document |
|---|---|
| What was the outcome? | [`N18-RELEASE-VERDICT-DRAFT.md`](./N18-RELEASE-VERDICT-DRAFT.md) — feature defaults and release scope |
| Which tasks are done and which remain? | [`REFACTORING-REMAINING-WORKLIST.md`](./REFACTORING-REMAINING-WORKLIST.md) |
| What did each task have to prove? | [`REFACTORING-EXECUTION-SPEC.md`](./REFACTORING-EXECUTION-SPEC.md) |
| Where is the evidence? | [`REFACTORING-PROGRESS.json`](./REFACTORING-PROGRESS.json) — machine-readable ledger; every completed task points at a committed evidence manifest under `system/eval/b0-knowledge-system/` |
| What did the final independent evaluation show? | [`FINAL2-RESULT.md`](./FINAL2-RESULT.md), [`N11-FINAL-RESULT.md`](./N11-FINAL-RESULT.md) |

## Verifying the record

```sh
make refactoring-check   # or: python3 scripts/check-refactoring-progress.py
```

The checker re-hashes every evidence manifest at the commit that recorded it
(`git show <revision>:<path>`) and the current sources against the active
binding. The recorded revisions are commits of the `feat/refactoring-followup`
branch, which was squash-merged into `main`; keep that branch (local or remote)
so the checker can still read them.

## File names

| Prefix | Meaning |
|---|---|
| `N01`–`N18` | Records for the eighteen refactoring tasks in the worklist |
| `FINAL`, `FINAL2` | Independent held-out evaluations run after development was frozen |
| `A*`, `B0`, `B1`, `C0`, `C1`, `D1`, `D5` | Earlier delivery stages: preparation, measurement, gates and reviews |
| `*-CONTRACT*`, `*-ADR*` | Contracts and decisions that still describe the shipped behaviour |
| `CLAUDE-CODE-*`, `REMOTE-SESSION-*` | Hand-over notes between working sessions |
| `*.yaml`, `*.json` | Pilot inputs and scopes bound by the evidence manifests; do not edit |
