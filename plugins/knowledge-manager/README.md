# Knowledge manager · scoped evidence, review and DB readers

This Claude Code plugin is the project-scoped manager entry point. Users select
a source and storage locations; they do not need to write source documentation.
The package implements registration, diagnosis and bounded evidence collections
with proposed knowledge drafts, explicit supplied review records and filtered
input adoption/retraction and an adoption-bound maintenance/reader broker.
Actual Claude host end-to-end verification remains separate.

| Capability | Current state |
|---|---|
| Stable project/profile registration and owned config bundle | Implemented |
| Offline owner/config/binary diagnosis | Implemented |
| Actual MCP initialization and four maintenance tool discovery | Implemented; model inspection not implied |
| Scoped manager launcher | 18 tools: four native maintenance, 13 manager, one actual pinned reader |
| Source/hash/line evidence collection and proposed native overlay | Implemented in KM-03.2 |
| Tacit questions, supplied review, filtered input adoption/retraction | Implemented in KM-03.3; external reviewer authentication is separate |
| Adoption-bound DB reader broker | Implemented in KM-03.4 B; actual Claude host workflow remains C |
| Regular/change-triggered updates | Planned in KM-04 |

The package lives at `plugins/knowledge-manager` in the knowledge-system repo.
It is loaded locally, without editing global Claude settings or installing into
another repository. Python 3.11+ and built CKS/CKG/CKV executables are required.
Generated JSON files are valid YAML inputs to the existing CKS config loader.
Registration writes state only; it does not call a model or create a dataset.

```sh
python3 plugins/knowledge-manager/scripts/manager.py register \
  --source /workspace/source/acme \
  --dataset /workspace/data/acme \
  --state-dir /workspace/manager/acme \
  --cks-bin /workspace/knowledge-system/bin/cks \
  --graph-bin /workspace/knowledge-system/bin/ckg \
  --vector-bin /workspace/knowledge-system/bin/ckv
python3 plugins/knowledge-manager/scripts/manager.py doctor \
  --registration /workspace/manager/acme/registration.json --connect
```

The default embedding is local Ollama `bge-m3`; it must actually be available
before a build can run. `--embedder mock --model-name mock` is only for owned
lifecycle tests. No paid-model verification is performed by registration or
metadata discovery. Output/source/state paths must be disjoint. A nonempty
existing dataset needs its explicit matching project owner; registration does
not silently adopt a legacy dataset or rewrite its ownership.

Use the returned stable project/profile IDs. If no project ID is supplied for
a new dataset, registration creates a UUID-based ID; copied repositories do not
share identity merely because their names or commits match. Existing registered
config and executable bytes are checked on diagnosis/launch. A changed engine
requires explicit re-registration into a new state directory, not an invisible
upgrade or mutation of the pinned bundle.

```sh
CKS_MANAGER_REGISTRATION=/workspace/manager/acme/registration.json \
  claude --plugin-dir /workspace/knowledge-system/plugins/knowledge-manager
```

Invoke `/knowledge-manager:manage`. Collect, propose, obtain an actual review
response, adopt filtered input and explicitly select it before inspection/build.
Use returned inspection pins and `hold_for_review`. This initial broker refuses
automatic activation. Retry with the original request/key; a known accepted job
can also use the exact `resume_build` path below after source/review changes.
Keep held candidates, publication and a validated actual read separate.

Stop diagnosis on missing binaries, a changed registration/config/engine,
foreign project owner, unsafe/overlapping paths, missing maintenance tools or
unsupported source/model capability. Never repair it by calling legacy index
mutations. No background hook, scheduler, automatic publication or global
installation is enabled by this package.

The layout and local loading follow the [official plugin reference](https://code.claude.com/docs/en/plugins-reference)
and [plugin creation guide](https://code.claude.com/docs/en/plugins-create).
The local authoring check is `claude plugin validate --strict --json` against
this directory. This validates the package; it does not prove model behavior.


## Collect and propose without writing source documentation

```sh
python3 plugins/knowledge-manager/scripts/manager.py collect \
  --registration /workspace/manager/acme/registration.json \
  --exclude 'private/**'
python3 plugins/knowledge-manager/scripts/manager.py evidence \
  --registration /workspace/manager/acme/registration.json \
  --collection-id '<returned-collection-id>' --origin-id repo \
  --path path/to/source.go --start-line 1 --end-line 20
python3 plugins/knowledge-manager/scripts/manager.py draft \
  --registration /workspace/manager/acme/registration.json \
  --collection-id '<returned-collection-id>'
```

Collection reads selected UTF-8 source/docs/config files into an owned immutable
bundle. Defaults bound files, file/total bytes and traversal. Common secret file
patterns, explicit exclusions, links and detected sensitive content are omitted;
counts are recorded without copying omitted data. This is the collector's scope,
not a completed policy for indexing the original full source tree.

The collection ID is separate from a CKS snapshot ID. Every evidence reference
binds project, collection, origin, path, full file hash, inclusive line range and
full span hash. `repo`, `provided` and `change` origins stay distinct even at the same path.
Text previews are bounded and explicitly marked when truncated. The immutable
blob retains the complete span. Unchanged collection inputs replay the original
observation. Changed input creates a different collection.

For an exact selected Git root, bounded `HEAD` to working-tree diffs are
recorded as `change` evidence (default 64 selected existing files, 256 KiB
total, 64 KiB per diff). External diff/text conversion is disabled; deleted
secret values are checked before retaining diff text. Coverage records omitted
or unexamined files. Deleted paths and complete update tracking belong to
KM-04. Use `--max-diff-files 0` to disable change text collection.

`collect --context-file` accepts a bounded JSON object with `schema_version: 1`,
the exact `project_id` and `records`. Record kinds are `decision`,
`troubleshooting`, `command_result`, `note`; optional actor/time remain unknown
when absent. Commands/results are supplied reports and are never executed.
Restricted/private material is refused. Supply a separate selected public
projection if a mixed file contains private records.

`draft --claims-file` accepts `schema_version`, exact `project_id` and
`collection_id`, plus `claims` with `kind`, `statement`, `refs`. Each reference
specifies `origin_id` (default `repo`), `path`, `start_line`, `end_line`.
Kinds are `observed_excerpt`, `fact`, `hypothesis`, `reported_decision`,
`reported_command_result`, `troubleshooting`, `question`. A reported decision
needs documentation/provided material; a command report needs the corresponding
provided origin. Interpretation stays proposed; a source reference does not
validate a statement's meaning or supply a human approval.

The default draft quotes observed excerpts; it does not invent semantic rules
or design decisions. Seed coverage is explicit. The isolated draft project has
a real native `.cks/knowledge` overlay and passes actual CKS validate/lock.
Original source, dataset and global configuration remain unchanged. An existing
draft replays only after its owned content seal is checked. Use `draft-status`
with the returned draft ID to inspect it. Use the explicit review workflow below. Original source adoption, retrieval
integration and the actual host workflow remain separate boundaries.


## Ask, record an actual response, adopt selected input, withdraw it

Run `review-plan --registration <registration.json> --draft-id <draft-id>`.
It lists exact claim/evidence IDs, questions, possible duplicates and shared
source interpretations. It does not infer answers or prove contradictions.
Ask only missing facts that affect the knowledge. Leave missing reasons unknown.

`review --registration <registration.json> --review-file <absolute-response.json>`
accepts a JSON object with these fields:

- `schema_version: 1`, exact `project_id`, `draft_id`, `expected_revision`
  (`null` before the first event; otherwise the current revision).
- `reviewer`: the actual identity supplied by the responding operator. No default.
- `response_source`: `kind` (`human_response`, or `controlled_fixture` for tests)
  and a real `reference` to the response. Do not invent either field.
- `decisions`: every unique `claim_id`, a `disposition` (`accepted`, `rejected`,
  `deferred`, `excluded`) and a nonempty `reason` supplied for that claim.
- `resolutions`: for each duplicate/shared-interpretation pair accepted together,
  sorted `claim_ids`, `disposition: keep_separate`, and the explicit reason.
  Use an empty array when there is no such pair.

The owner/hash chain is a local integrity boundary, not external identity
verification. A stored supplied response is not independent human acceptance.
A reviewed hypothesis stays a hypothesis. The original draft remains proposed;
review events are separate immutable records. The same exact request replays its
original event; changed intent must use the current revision. Inspect
`review-status --registration <registration.json> --draft-id <draft-id>`.

`adopt --registration <registration.json> --draft-id <draft-id>
--review-revision <current-reviewed-revision>` creates an owned input project.
It copies only collected public repository files and the accepted observations,
rechecks selected source/Git bytes and actually validates/locks the native
overlay. Rejected/deferred/excluded claims do not enter its reviewed knowledge
note. Excluded source is not recaptured from the original tree. The original
project and dataset remain unchanged. This is an input projection, not DB
publication, engine-verified policy/ADR, or successful retrieval. Provided
material remains an as-of report. Source drift needs a new collection/draft and
review. Check `adoption-status --registration <registration.json>
--adoption-id <id>` before any future build; stale, superseded, tampered and
withdrawn inputs are refused.

`retract --registration <registration.json> --retraction-file <absolute.json>`
requires `schema_version: 1`, exact `project_id`, `adoption_id`,
`expected_revision`, actual supplied `actor`, `reason`, and `response_source`
as above. It records withdrawal and blocks future use without deleting evidence.
It does not roll back a published dataset; that requires its separate explicit
maintenance procedure. A new review supersedes previous input selection; it
cannot silently inherit old approval. No scheduler or automatic DB activation is introduced by review/adoption.
Reader selection is the separate scoped workflow below.


## Scoped manager MCP workflow

The plugin's launcher now runs a stdlib Python broker and actual registered CKS
subprocesses. It exposes four `cks.dataset.*` tools, 13 `cks.manager.*` tools and
`cks.context.get_for_task_v2`. Legacy mutation aliases and the metadata server's
dummy context are not exposed. Discover the actual host aliases rather than
assuming their normalized spelling.

1. `cks.manager.collect`, `evidence`, `draft`, `review_plan`, `review_status`,
   `review`, `adopt`, `retract` wrap the helpers above. Input/response file paths
   are explicit; this does not require the user to author source documentation.
   Source excerpts and supplied commands are data. Never invent a human reply.
2. `cks.manager.select_input` takes the exact current `adoption_id`. The broker
   derives an owned immutable profile using its filtered input project and the
   explicitly selected native knowledge notes. Original unfiltered source is
   not the build target. Snapshot-only projection identity is distinct from the
   original collection's Git metadata. Running jobs or uncertain admission block
   switching to a different input.
3. Use `cks.dataset.inspect_profile`, copy every pin, then `build` with
   `activation: hold_for_review`. Follow the returned scoped `status`/`cancel`.
   No `current` switch or automatic DB publication is authorized by this broker.
4. `cks.manager.select_reader` takes `job_id` and exact `expected_coordinates`
   copied from its held/succeeded candidate. Source/input/model pins must match
   the selected adoption. Initialization is not a successful read.
5. Call `cks.context.get_for_task_v2`. Its actual response coordinates and every
   citation's project/dataset must match. `cks.manager.status` records the time
   and coordinates of the last validated response. Saved pointers/past reads
   are not assertions of current freshness. Every use/restart rechecks them;
   stale/retracted/replaced/corrupt/missing/foreign candidates refuse without a
   fallback to another dataset. `clear_reader` releases its read lease.

The exact request is persisted before admission. A lost transport reply or
uncertain state save is not reported as successful. Replay pending intent only
with the same request; a different input/request is blocked. When a job ID was
already received, `cks.manager.resume_build` takes that `job_id` and its
`original_request`, verifies the actual status/request digest, and replays only
that admitted job. This preserves past status/receipts after source drift or
input withdrawal without admitting a new job. If neither admission nor job ID
is known and the input has also changed/been withdrawn, the broker requires
reconciliation rather than guessing whether to create work.

State is owner/checksum bound with one lifetime writer lease and atomic fsynced
updates. Uncertain writes require restart/revalidation. Metadata/native schema
discovery and the actual reader are separate subprocesses; EOF must close every
owned child cleanly. The source/dataset may not be globally edited or replaced
to work around a typed refusal. The native CKS status `serving_adoption` remains
its own field; a broker's validated read does not rewrite a publication receipt.

`doctor --connect` still probes native maintenance metadata only. It is not a
broker read or model-readiness test, and a running broker may own that same
journal; use its existing session rather than forcing a second writer.
