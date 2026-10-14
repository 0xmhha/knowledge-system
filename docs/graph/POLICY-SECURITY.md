# Retained policy and security annotations

CKS `context.policy_security` reads annotations for one exact code symbol from
its retained graph. Resolve a symbol first, then supply exactly one `node_id`
or `canonical_id`. Optional `expected_commit`, `expected_source_sha256` and
`expected_snapshot_sha256` reject mismatches; commit and blob pins describe the
indexed code, not the original annotation document.

```json
{"node_id":"selected-graph-node-id","max_annotations":32,"timeout_ms":1000}
```

Defaults are32 annotations and1000ms. Caps are128 and5000ms. Only outgoing
`governed_by` and `has_security_pattern` edges are fetched using SQL LIMIT+1.
The result includes the selected symbol, retained blob/metadata and manifest
hashes, annotation explanations, modification hints, categories and severity,
manual provenance, limits and elapsed time. A truncated page is `partial` with
`more=true` and `reason=max_annotations`. Unsupported bounded backends return
`unavailable`; they never fetch an unlimited neighborhood and slice afterward.

Fresh policy/security loaders store separate description/remediation/severity
fields in the existing node attrs payload. This avoids guessing a remediation
boundary from prose containing the word Remediation. Older graphs keep their
merged explanation; remediation and severity remain `unknown`. Empty retained
operator fields are explicit empty values, distinct from fields never retained.

The current loaders do not retain YAML bytes, actual row, document version,
reviewer, review time or approval. Those provenance fields return `unknown`,
even when the legacy node has placeholder line1 or an indexed Go commit.
`metadata_sha256` hashes the retained graph node; it is not an original YAML
hash. The source path is an operator-provided overlay path, potentially absolute;
it provides no reviewer identity or signed approval proof. Private `.claude`
and log source boundaries remain excluded, as do ambiguous history records.

Manual annotations have `origin=operator_curated_metadata`. They are operator
assertions, not authenticated judgments or confirmed vulnerabilities. The
existing `context.rule_candidates` tool is a separate opt-in automatic Go
review query. `policy_security` does not execute that detector or merge its
results. Agent reasoning is also separate. `no_annotations` and `not_found`
are distinct; neither proves policy compliance or safety. `safety_verdict`
always remains `unknown`.

Manifest, selected node/blob, returned annotation metadata, fetched edge page
and truncation flag are rechecked before returning. Detected drift clears all
symbol and annotation results. These checks do not provide a database snapshot
transaction or detect changes that revert between reads. Cooperative deadlines
and SQL row caps do not bound individual retained row/blob bytes, forcefully
interrupt synchronous legacy methods, or guarantee process memory limits.

The tool uses the existing stdio/HTTP server and HTTP peer-IP/CIDR ACL. Its
readOnlyHint is descriptive metadata, not authentication. HTTP ACL uses the
direct peer and ignores spoofed X-Forwarded-For; reviewer identity verification
and signed annotation trust remain unsupported. Unit HTTP transport tests cover
rejection before tool invocation; the owned public evaluator separately covers
real stdio index/query/EOF execution.

`scripts/eval-policy-security.py` compiles and race-tests an owned Go corpus,
indexes policy and security overlays, checks typed fields and unknown provenance,
manual/automatic separation, positive/untagged/missing/canonical results, caps,
stale pins and invalid input. It then deletes live Go and overlay files and
requires identical retained results. Raw protocol, source bundle and receipts
provide implementation evidence; independent production acceptance is separate.
