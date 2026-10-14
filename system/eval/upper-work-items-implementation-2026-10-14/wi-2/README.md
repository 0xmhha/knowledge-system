# Bounded opt-in Go rule implementation evidence

Source commit: `2e1e7791f8159aa008f9f9c797255488d18f4f4b`.
Both buildinfo receipts report this revision and `vcs.modified=false`.
Execution: 23 compiled/race-tested Go review-condition cases,39 actual CKG/CKS
MCP checks, CKG12/CKS31 tools, both EOF exit0. Previous value evaluator:15 actual
checks, EOF0. Full build/test/fmt/vet/pinned lint/boundaries/docs and affected race
passed before source commit. Verified source bytes were unchanged afterward.

Fixture review conditions: TP10 TN7 FP2 FN4; precision10/12 and recall10/14.
Sanitizer helper and same-name fake selector are deliberate false positives;
renamed imports, pointer/closure and renamed conversion paths are false negatives.
These denominators only concern the owned fixture, not vulnerabilities. Target
resolution, sanitizers and runtime feasibility remain unverified. One lexical
witness per source/sink; empty candidates never prove safety. Manual YAML and
persisted graph stayed unchanged. CKS forwards the same engine without a second
matcher or new transport/authorization surface.

The requirement audit maps scope to actual evidence. The manifest binds every
artifact and maintained Go/dependency input. Source bundle is incremental from
retained c0f816d7. Fixture bundles preserve original source; live files deleted
during retained-index tests are intentionally absent from execution/source,
with original text preserved separately. Historical failures are labelled and
are not passing evidence. Independent human/production/whole-goal acceptance
is not recorded.
