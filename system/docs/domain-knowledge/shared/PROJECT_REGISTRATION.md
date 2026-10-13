# Registering project knowledge inputs

Use a project-owned directory containing project.yaml, subsystems.yaml and entries/*.yaml. The committed example is [the stablenet pack](../../../../projects/stablenet/domain-knowledge). Shared entry definitions remain [here](./SCHEMA.md); project-specific knowledge is data, not engine behavior.

1. Choose a stable project identity and record the code root/source provenance. Branches and snapshots use their own snapshot identities rather than renaming the logical project each time.
2. Declare real subsystems and entries with code-relative anchors. Leave unreviewed entries in needs_author/draft/needs_verification.
3. Run mechanical validation, generate a worksheet, review substantive statements, and record reviewer/date only for an actual review. Optional catalog selections must be explicitly pinned; no catalog is implicitly embedded in the engine.
4. Derive glossary/engine views and render the corpus. Pass those explicit paths into the project setup inputs.
5. Build and check the actual dataset/model coordinates. A valid directory or zero verified entries does not prove a production-ready knowledge index.

```sh
cks domain check --project projects/stablenet/domain-knowledge
cks domain worksheet --project projects/stablenet/domain-knowledge
```

Replace the example with your own project directory and code-root environment. Use command help and [domain manuals](../../../../cmd/cks/domaincli/README-check.md) for options. [Coordinated setup](../../SETUP.md) and [source snapshot identity](../../../../docs/spec-driven/SOURCE-SNAPSHOT-ADR.md) define build/promotion behavior. The obsolete projects/active_project multi-project server YAML is not the current config surface; use one configured CKS instance/dataset context.

Keep historical source and reviewer information when archiving or superseding an entry. Editing verification state or changing anchor bytes requires renewed review; tools do not independently certify a human judgment.
