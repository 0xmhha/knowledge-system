---
name: manage
description: Register and diagnose a project-scoped knowledge DB manager, then use the registered knowledge-system tools. Invoke when asked to build or maintain a project's knowledge system.
disable-model-invocation: true
argument-hint: "<source directory> <dataset directory> <state directory>"
---

Manage the selected project's knowledge system. Read the current capability and
phase boundaries in `${CLAUDE_PLUGIN_ROOT}/README.md` first.

1. Use the source, dataset and state directories the user selected. Registration
   keeps state outside the source and dataset. A generated project ID is stable
   in the registration; a repository name or shared commit is not project identity.
   Never silently switch to another registration, project or dataset.
2. Register with `python3 "${CLAUDE_PLUGIN_ROOT}/scripts/manager.py" register`
   and its explicit source/dataset/state/binary arguments. This prepares JSON
   configs; it does not build or activate a DB, install globally or change source.
   Use the configured local embedding provider. Mock embeddings are test-only.
3. Run `doctor --registration <absolute registration.json>` on that script.
   `--connect` additionally checks actual MCP initialization and discovery. It
   does not claim model readiness, retrieval quality, a successful build or host
   acceptance. Missing dependencies or mismatched owner/config/binary bytes stop
   the workflow; do not bypass them with a legacy operation or direct DB writes.
4. Launch Claude with this plugin and `CKS_MANAGER_REGISTRATION` set to that
   registration. Discover the actual MCP aliases. Use the registered
   `cks.dataset.inspect_profile`, `cks.dataset.build`, `cks.dataset.status`,
   `cks.dataset.cancel` contracts. Copy all source/input/model/profile/base pins
   from the current inspection. Use a fresh version/key for a new intended build;
   after interruption replay the original request/key, not a new guessed request.
5. Treat accepted/queued/cancel_requested/recovering/held_for_review as distinct
   from succeeded/activated. A past publication receipt is not serving adoption.
   On cancellation poll until actual termination; do not say canceled on receipt
   of an accepted intent. Never fabricate source facts, design reasons, reviewers
   or approval. Preserve unknowns and conflicts for human review.

This initial package implements registration, offline diagnosis, metadata-only
MCP discovery and the maintenance launcher. Draft collection/review and the
managed reader broker are subsequent roadmap stages. Do not execute an invented
`draft`, `review`, `adopt-reader` or scheduling command. Until the reader broker
is implemented, maintenance server context tools are not a verified read of the
managed DB. Record the missing step instead of returning dummy evidence.
