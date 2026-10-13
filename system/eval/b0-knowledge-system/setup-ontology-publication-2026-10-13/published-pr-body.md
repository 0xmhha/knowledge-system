Guided setup now builds and checks coordinated graph/vector datasets with visible progress, project-pack selection, build-source inclusion, query checks and program-version comparisons. Configuration and graph-only errors identify the action callers need to take.

MCP clients can inspect aligned ontology readiness, read terms and concepts, trace requirements, and request bounded explanations of changes to v2 evidence. Optional ontology retrieval remains off by default. Coordinate decoding results are reused only after hashing fresh file bytes; source, index and evidence validation still run.

Documentation now separates the current handoff and live worklist from historical records, incorporates the existing main-branch release clarification, and records the actual limitations of the ontology evaluation.

Validation:
- The latest frozen consensus comparison verified 702 responses and 351 off/on pairs with no failed or missing calls. Response/source hashes and replayed scores match.
- Recall stayed equal and the expected-rank measure declined slightly. Independent quality remains inconclusive; proposed clusters are not approved independent units. No new independent holdout evaluation, operating activation or criterion approval is claimed.
- Aggregate warm p95 ratio was 1.086; three questions exceeded 1.25. Overall performance acceptance remains false, and earlier observations remain separate.
- Ontology Python checks: 80 passed. Progress guard tests: 11 passed. Documentation checks and 135 local documentation links passed. Current source/input/evidence bindings and recorded completion states are verified.
- Prior targeted Go race evidence is retained at its recorded revision. Documentation changes do not claim a new Go regression run; GitHub CI reports the submitted branch checks.

Public-key fingerprint/trust conditions and public tag/release remain on hold. This PR is for review and does not activate the operating dataset or authorize deployment. It can be squash-merged with a concise conventional commit message; the recorded evidence commits must remain available for historical manifest verification.
