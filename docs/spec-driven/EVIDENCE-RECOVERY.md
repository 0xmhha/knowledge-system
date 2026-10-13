# Completion evidence and removed-document recovery

Completion evidence is read at each task's recorded `evidence_revision` and `manifest` in [the progress ledger](./REFACTORING-PROGRESS.json). Historical reports are not rewritten to match current sources. Removing a completed report from the working tree does not remove the commit that the ledger verifies.

The source/evidence branches `feat/refactoring-followup` and `feat/setup-tool` must be retained. PR #99 was squashed into main as `9a397188`; its original tip `1e3ace26` remains on `feat/setup-tool`. To obtain those objects in a fresh clone, fetch the evidence branches before running the progress check:

```sh
git fetch origin feat/refactoring-followup feat/setup-tool
python3 scripts/check-refactoring-progress.py
```

Inspect a recorded artifact with `git show REVISION:PATH`. Replace both arguments with the exact values from the ledger. To read one of the prose documents removed in this cleanup, use its entry in [DOCUMENTATION-CLEANUP.json](../DOCUMENTATION-CLEANUP.json); the complete pre-cleanup tree is [available at 9a397188](https://github.com/0xmhha/knowledge-system/tree/9a39718807ed25a0add30405f7cd1f1ebff4a629).

Original inputs, source captures, protocols and raw results under the retained evaluation directories remain byte-identical. Links inside a frozen artifact belong to its capture revision; use that revision when resolving them. Current reference links are checked against the current working tree instead.

No recovery command runs a model, modifies a dataset, approves a concept or activates a feature. If a required evidence object is unavailable, completion verification must fail rather than substituting a current report.
