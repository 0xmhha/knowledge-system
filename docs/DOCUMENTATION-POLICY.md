# Documentation maintenance

Maintain one current reference for each product surface and one remaining-work entry point. A guide must describe registered commands, real flags, existing source paths and current defaults. Purpose and accepted contracts remain necessary even when implementation is complete. A completed implementation does not make its interface or safety contract disposable.

Completed plans, duplicate designs, dated debugging narratives and old session prompts do not remain as competing current instructions. Consolidate their still-relevant decisions and limits into current references, remove the obsolete prose files, and retain their Git revision/path/SHA in [the cleanup inventory](./DOCUMENTATION-CLEANUP.json). Recover them through [evidence recovery](./spec-driven/EVIDENCE-RECOVERY.md).

Retained evaluation/source snapshots, approved inputs, domain records and fixture text are product or validation data. Their recorded bytes must not be rewritten as if they came from the latest code. Interpret them at their recorded source/model/input revision. The catalog labels these separately; they are not current installation instructions.

Every maintained reference is listed in [DOCUMENTATION-CATALOG.json](./DOCUMENTATION-CATALOG.json). `make docs-check` validates catalog coverage, local Markdown targets, supported command/flag examples and code paths. A date or the word handoff must not exempt a catalogued current reference. New documentation must be classified explicitly rather than silently bypassing checks.

Before checking a development task complete, run `make refactoring-check`. Keep the original evidence commits available after squash merges; the main squash tree alone cannot supply historical artifacts. [Remaining work](./REMAINING-WORK.md) distinguishes required scope, user-held release decisions, unverified external responsibilities and optional new work.
