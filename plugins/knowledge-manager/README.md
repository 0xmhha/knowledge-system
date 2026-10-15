# Knowledge manager · initial package

This Claude Code plugin is the project-scoped manager entry point. Users select
a source and storage locations; they do not need to write source documentation.
The package currently implements registration and diagnosis. Grounded drafts,
review/adoption and actual host end-to-end management remain separate stages.

| Capability | Current state |
|---|---|
| Stable project/profile registration and owned config bundle | Implemented |
| Offline owner/config/binary diagnosis | Implemented |
| Actual MCP initialization and four maintenance tool discovery | Implemented; model inspection not implied |
| Maintenance launcher | Implemented; exposes the configured CKS server |
| Evidence-backed draft collection and tacit knowledge review | Planned in KM-03.2/03 |
| Managed DB reader broker and full Claude host workflow | Planned in KM-03.4 |
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

Invoke `/knowledge-manager:manage`. Use inspection pins and `hold_for_review`
unless activation is authorized for the intended job. Retry an interrupted job
with its original key/request. Keep `held_for_review` and publication separate.
The current maintenance server does not dynamically adopt newly built readers;
its default context tools must not be treated as verified reads of that DB.
The reader broker is an explicit KM-03.4 implementation and test requirement.

Stop diagnosis on missing binaries, a changed registration/config/engine,
foreign project owner, unsafe/overlapping paths, missing maintenance tools or
unsupported source/model capability. Never repair it by calling legacy index
mutations. No background hook, scheduler, automatic publication or global
installation is enabled by this initial package.

The layout and local loading follow the [official plugin reference](https://code.claude.com/docs/en/plugins-reference)
and [plugin creation guide](https://code.claude.com/docs/en/plugins-create).
The local authoring check is `claude plugin validate --strict --json` against
this directory. This validates the package; it does not prove model behavior.
