# Versioned reindex, restart and rollback

Run coordinated builds through cks setup rather than writing a served pinned database with an engine command. The current implementation is in internal/setup and cmd/cks/setupcli. Flat legacy maintenance is supported separately; ops.index refuses versioned/pinned inputs before modifying engines.

## Dataset lifecycle

A versioned build captures declared source/knowledge/model inputs, writes a new candidate, validates alignment and gates, then switches current. Reusing a version name is rejected. hold-for-review keeps a candidate inactive until the explicit patch/criterion workflow allows promotion. Failed or cancelled candidates cannot replace the previously active complete version.

```sh
bin/cks setup --help
bin/cks setup --src /abs/source --out /abs/datasets --version next --config /abs/setup.yaml
bin/cks setup --out /abs/datasets --rollback previous
```

Choose project/source-mode and selected pack/model identity through the reviewed setup configuration. Resource limits, deadlines and Git history bounds are explicit. Shared writer locks and durable candidate/pointer steps protect managed transitions; network filesystems or direct low-level writes are outside those guarantees.

A running server pins its resolved dataset and reader lease. After promotion, use the explicit instance restart/reload path to serve the new version; promotion alone does not swap an already-pinned reader. Retention/GC protects current, rollback, held review and live reader references. Inspect dry-run output before reclaiming storage.

## Instance management

cks mcp up/down/status/list manage configured instances. cks mcp reload starts a candidate serving process, checks readiness, and changes the serving instance only through its gated path. Inspect health/alignment and the generated client configuration after a change. [Availability](./ops-availability.md) covers supervision, dependency recovery, networking and power policy.

```sh
bin/cks mcp status --help
bin/cks mcp reload --help
bin/cks mcp client-config --help
```

Do not confuse the dataset current pointer with HTTP instance routing. Rollback is a managed pointer transition to a valid retained version, not a way to bypass review hold. A durability_uncertain result after rename/sync failure cannot be treated as proof current never changed. Re-read status and follow the recorded recovery intent.

[Source identity](../../docs/spec-driven/SOURCE-SNAPSHOT-ADR.md), [public write boundary](../../docs/spec-driven/PUBLIC-CONTRACT-V2.md), [patch review](../../docs/spec-driven/PATCH-WORKFLOW.md) and [remaining work](../../docs/REMAINING-WORK.md) define the current contracts and limits.
