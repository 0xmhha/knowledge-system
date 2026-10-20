# Completion evidence and removed-document recovery

Completion evidence is read at each task's recorded `evidence_revision` and `manifest` in [the progress ledger](./REFACTORING-PROGRESS.json). Historical reports are not rewritten to match current sources. Removing a completed report from the working tree does not remove the commit that the ledger verifies.

The original evidence commits are retained by the immutable tags listed in [HANDOFF-HISTORY.json](./HANDOFF-HISTORY.json). Merged development branches may be deleted after their recorded tips are preserved. The main squash tree alone cannot supply historical artifacts. A fresh clone must fetch the evidence tags before running progress checks:

```sh
git fetch origin '+refs/tags/evidence/2026-10-20/*:refs/tags/evidence/2026-10-20/*'
python3 scripts/check-handoff-history.py --restore-generated-inputs
python3 scripts/check-refactoring-progress.py
```

[New-machine handoff](./MACHINE-HANDOFF.md) records the current bootstrap, source/model pins, external data requirements, verification boundaries and next task. Historical branches are no longer prerequisites; their original commit objects remain available through the pinned evidence tags.

Inspect a recorded artifact with `git show REVISION:PATH`. Replace both arguments with the exact values from the ledger. To read one of the prose documents removed in this cleanup, use its entry in [DOCUMENTATION-CLEANUP.json](../DOCUMENTATION-CLEANUP.json); the complete pre-cleanup tree is [available at 9a397188](https://github.com/0xmhha/knowledge-system/tree/9a39718807ed25a0add30405f7cd1f1ebff4a629).

Original inputs, source captures, protocols and raw results under the retained evaluation directories remain byte-identical. Links inside a frozen artifact belong to its capture revision; use that revision when resolving them. Current reference links are checked against the current working tree instead.

No recovery command runs a model, modifies a dataset, approves a concept or activates a feature. If a required evidence object is unavailable, completion verification must fail rather than substituting a current report.
