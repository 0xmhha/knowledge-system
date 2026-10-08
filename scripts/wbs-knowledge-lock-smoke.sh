#!/usr/bin/env bash
# Model-independent D5 lock/build identity and stale-lock gate.
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
scratch="${KS_KNOWLEDGE_SMOKE_DIR:-$(mktemp -d)}"
src="$scratch/src"
data="$scratch/data"
mkdir -p "$src"
cp "$repo_root/testdata/wbs-smoke/go.mod" "$repo_root/testdata/wbs-smoke/main.go" "$repo_root/testdata/wbs-smoke/README.md" "$src/"
"$repo_root/bin/cks" knowledge init --project-root "$src" --project-id knowledge-fixture > "$scratch/init.json"
"$repo_root/bin/cks" knowledge lock --project-root "$src" > "$scratch/lock-1.json"
"$repo_root/bin/cks" knowledge validate --project-root "$src" > "$scratch/validate-1.json"
"$repo_root/bin/cks" setup --src "$src" --out "$data" --project-id knowledge-fixture \
  --source-mode snapshot-only --version first --embedder mock > "$scratch/first.log" 2>&1
printf 'status: proposed\n' > "$src/.cks/knowledge/domain/fixture.yaml"
if "$repo_root/bin/cks" setup --src "$src" --out "$data" --project-id knowledge-fixture \
  --source-mode snapshot-only --version stale --embedder mock > "$scratch/stale.log" 2>&1; then
  echo "stale knowledge lock was accepted" >&2; exit 1
fi
"$repo_root/bin/cks" knowledge lock --project-root "$src" > "$scratch/lock-2.json"
"$repo_root/bin/cks" setup --src "$src" --out "$data" --project-id knowledge-fixture \
  --source-mode snapshot-only --version second --embedder mock > "$scratch/second.log" 2>&1
mkdir -p "$src/vendor/engineering-decisions"
cat > "$src/vendor/engineering-decisions/pack.yaml" <<'YAML'
pack_schema_version: 1
pack_id: engineering.decisions
version: 1.0.0
owner: project
scope: fixture
requires: []
concepts:
  - {id: business-policy, kind: rule, definition: A project business rule.}
  - {id: design-decision, kind: entity, definition: A reviewed design choice.}
relation_types:
  - predicate: motivates
    subject_type: {pack_id: engineering.decisions, local_id: business-policy}
    object_type: {pack_id: engineering.decisions, local_id: design-decision}
    direction: forward
    cardinality: many-to-many
    required_evidence: true
    review_rule: human
constraints: []
competency_questions: ["Which policy governs the transfer?"]
YAML
"$repo_root/bin/cks" knowledge digest --project-root "$src" --source vendor/engineering-decisions > "$scratch/pack-digest.json"
python3 - "$src/.cks/knowledge/manifest.yaml" "$scratch/pack-digest.json" <<'PY'
import json, pathlib, sys
manifest, digest_path = map(pathlib.Path, sys.argv[1:])
digest = json.loads(digest_path.read_text())['sha256']
text = manifest.read_text()
assert 'selected_packs: []' in text
text = text.replace('selected_packs: []', 'selected_packs:\n  - pack_id: engineering.decisions\n    version: 1.0.0\n    source: ./vendor/engineering-decisions\n    sha256: '+digest)
manifest.write_text(text)
PY
cat > "$src/.cks/knowledge/policies/BR-17.yaml" <<'YAML'
id: BR-17
type: {pack_id: engineering.decisions, local_id: business-policy}
statement: Transfers require separate approval.
owner: payments-team
scope: {subsystem: transfers}
effective_from: "2026-01-01"
status: verified
reviewed_by: fixture-reviewer
review_reason: Approved against the project policy source.
visibility: public
conflicts_with: [BR-18]
source_ref: {origin_id: repo, path: .cks/knowledge/policies/BR-17.yaml}
YAML
cat > "$src/.cks/knowledge/policies/BR-18.yaml" <<'YAML'
id: BR-18
type: {pack_id: engineering.decisions, local_id: business-policy}
statement: Transfers do not require separate approval.
owner: payments-team
scope: {subsystem: transfers}
effective_from: "2026-01-01"
status: verified
reviewed_by: fixture-reviewer
review_reason: Retained as a conflicting reviewed source for audit.
visibility: public
conflicts_with: [BR-17]
source_ref: {origin_id: repo, path: .cks/knowledge/policies/BR-18.yaml}
YAML
cat > "$src/.cks/knowledge/decisions/ADR-1.md" <<'MD'
---
id: ADR-1
type: {pack_id: engineering.decisions, local_id: design-decision}
problem: Which transfer approval path should be used?
decision: Require an independent approver.
rationale: Preserve separation of duties.
alternatives: [Single approver]
assumptions: [Approval service is available]
scope: {subsystem: transfers}
date: "2026-01-01"
status: proposed
visibility: public
source_ref: {origin_id: repo, path: .cks/knowledge/decisions/ADR-1.md}
---
# Transfer approval
The proposed decision needs a human review.
MD
cat > "$src/.cks/knowledge/relations/REL-1.yaml" <<'YAML'
id: REL-1
type: {pack_id: engineering.decisions, local_id: motivates}
subject: {id: BR-17, type: {pack_id: engineering.decisions, local_id: business-policy}}
object: {id: ADR-1, type: {pack_id: engineering.decisions, local_id: design-decision}}
status: proposed
visibility: public
source_ref: {origin_id: repo, path: .cks/knowledge/relations/REL-1.yaml}
evidence_refs:
  - {origin_id: repo, path: .cks/knowledge/policies/BR-17.yaml}
  - {origin_id: repo, path: .cks/knowledge/decisions/ADR-1.md}
YAML
"$repo_root/bin/cks" knowledge lock --project-root "$src" > "$scratch/lock-3.json"
"$repo_root/bin/cks" knowledge validate --project-root "$src" > "$scratch/validate-3.json"
"$repo_root/bin/cks" knowledge review --project-root "$src" > "$scratch/review-3.json"
"$repo_root/bin/cks" setup --src "$src" --out "$data" --project-id knowledge-fixture \
  --source-mode snapshot-only --version third --embedder mock > "$scratch/third.log" 2>&1
dataset_id="$(python3 - "$data/third/dataset-identity.json" <<'PY'
import json, sys
print(json.load(open(sys.argv[1]))['dataset_id'])
PY
)"
"$repo_root/bin/cks" semantic build --repo "$src" --project-id knowledge-fixture \
  --dataset-id "$dataset_id" --graph "$data/third/graph" --vector "$data/third/vector" \
  --store "$scratch/semantic.db" --out "$scratch/pack-v4.json" \
  --docs README.md --include-packs --version-dir "$data/third" --extract-only \
  > "$scratch/pack-v4-build.json"
"$repo_root/bin/cks" semantic promote --input "$scratch/pack-v4.json" --repo "$src" \
  --graph "$data/third/graph" --vector "$data/third/vector" --store "$scratch/semantic.db" \
  --version-dir "$data/third" --activate > "$scratch/pack-v4-promote.json"
python3 - "$scratch/pack-v4.json" "$scratch/pack-v4-tampered.json" <<'PY'
import json, pathlib, sys
original, tampered = map(pathlib.Path,sys.argv[1:])
p = json.loads(original.read_text())
assert p['schema_version'] == 4 and len(p['knowledge']['packs']) == 1
assert {c['local_id'] for c in p['knowledge']['packs'][0]['concepts']} == {'business-policy','design-decision'}
assert {r['predicate'] for r in p['knowledge']['packs'][0]['relations']} == {'motivates'}
p['knowledge']['packs'][0]['concepts'][0]['definition'] = 'Forged definition.'
tampered.write_text(json.dumps(p))
PY
if "$repo_root/bin/cks" semantic promote --input "$scratch/pack-v4-tampered.json" --repo "$src" \
  --graph "$data/third/graph" --vector "$data/third/vector" --store "$scratch/semantic.db" \
  --version-dir "$data/third" > "$scratch/pack-v4-tampered.log" 2>&1; then
  echo "forged v4 pack type was promoted" >&2; exit 1
fi
printf '\n# Patch candidate\n' >> "$src/README.md"
"$repo_root/bin/cks" setup --src "$src" --out "$data" --project-id knowledge-fixture \
  --source-mode snapshot-only --version held --embedder mock --hold-for-review \
  --gate-test-bin true > "$scratch/held.log" 2>&1
"$repo_root/bin/cks" patch --dataset "$data" --patch-id fixture-change register --version held > "$scratch/patch.json"
if "$repo_root/bin/cks" patch --dataset "$data" --patch-id fixture-change promote --semantic-store "$scratch/missing-semantic.db" > "$scratch/patch-premature.log" 2>&1; then
  echo "unreviewed patch was promoted" >&2; exit 1
fi
if "$repo_root/bin/cks" setup --out "$data" --rollback held > "$scratch/held-rollback.log" 2>&1; then
  echo "held candidate bypassed review through rollback" >&2; exit 1
fi
"$repo_root/bin/cks" knowledge review --version-dir "$data/third" > "$scratch/review-archived.json"
"$repo_root/bin/cks" knowledge review record --project-root "$src" --version-dir "$data/third" \
  --kind decision --id ADR-1 --decision verified --reviewer fixture-reviewer \
  --reason "Compared the retained ADR with the policy and alternatives." > "$scratch/review-record.json"
"$repo_root/bin/cks" knowledge lock --project-root "$src" > "$scratch/lock-4.json"
"$repo_root/bin/cks" setup --src "$src" --out "$data" --project-id knowledge-fixture \
  --source-mode snapshot-only --version fourth --embedder mock > "$scratch/fourth.log" 2>&1
"$repo_root/bin/cks" knowledge review --version-dir "$data/fourth" > "$scratch/review-fourth.json"
printf 'status: proposed\n' > "$src/.cks/knowledge/policies/BR-17.yaml"
"$repo_root/bin/cks" knowledge review --version-dir "$data/third" > "$scratch/review-after-edit.json"
"$repo_root/bin/cks" mcp gen-config --dataset-dir "$data/current" \
  --name knowledge-fixture --source-root "$src" \
  --sanitize-rules "$repo_root/system/policies/sanitization_rules.yaml" \
  --semantic-store "$scratch/semantic.db" --out "$scratch/mcp.yaml" > "$scratch/mcp-config.log"
python3 - "$scratch/mcp.yaml" <<'PY'
from pathlib import Path
import sys
p = Path(sys.argv[1])
s = p.read_text()
for before, after in (
    ('provider: ""', 'provider: mock'),
    ('embed_model: bge-m3', 'embed_model: mock-feature-hash-v1'),
    ('mcp_stdio: false', 'mcp_stdio: true'),
    ('transport: http', 'transport: stdio'),
):
    assert before in s, (before, p)
    s = s.replace(before, after)
p.write_text(s)
PY
python3 "$repo_root/scripts/wbs-mcp-pin-probe.py" "$repo_root/bin/cks" \
  "$scratch/mcp.yaml" --v2-once "Where is transfer approval handled?" \
  2026-06-01 transfers > "$scratch/mcp-v2-conflict.json"
python3 - "$data" "$scratch" <<'PY'
import json, pathlib, sys
data, scratch = map(pathlib.Path, sys.argv[1:])
first = json.loads((data/'first'/'dataset-identity.json').read_text())
second = json.loads((data/'second'/'dataset-identity.json').read_text())
third = json.loads((data/'third'/'dataset-identity.json').read_text())
archive = json.loads((data/'third'/'sources'/'manifest.json').read_text())
pack_files = [item for item in archive['files'] if item['origin_id'] == 'knowledge:engineering.decisions']
assert {item['path'] for item in pack_files} == {'pack.yaml'}
assert (data/'third'/'sources'/'blobs'/pack_files[0]['sha256']).read_bytes() == (scratch/'src'/'vendor'/'engineering-decisions'/'pack.yaml').read_bytes()
assert first['dataset_id'] != second['dataset_id']
assert first['source']['snapshot_id'] != second['source']['snapshot_id']
assert second['dataset_id'] != third['dataset_id']
assert (data/'current').resolve() == (data/'fourth').resolve()
v2 = json.loads((scratch/'mcp-v2-conflict.json').read_text())
assert v2['base_citation_count'] > 0 and v2['knowledge_citation_count'] >= v2['base_citation_count'], v2
assert v2['base_coordinates'] == v2['knowledge_coordinates'], v2
assert v2['knowledge_state'] == 'conflict' and v2['conflict_count'] == 1, v2
assert v2['trace_link_count'] == 0 and v2['required_behavior_count'] == 0, v2
assert json.loads((data/'held'/'review-hold.json').read_text())['base_version'] == 'third'
patch = json.loads((scratch/'patch.json').read_text())
assert patch['state'] == 'unconfirmed' and any(f['path'] == 'README.md' for f in patch['changed_files'])
assert not (data/'stale').exists()
assert json.loads((scratch/'validate-1.json').read_text())['status'] == 'locked'
assert json.loads((scratch/'lock-1.json').read_text())['lock_digest'] != json.loads((scratch/'lock-2.json').read_text())['lock_digest']
assert 'pack_lock_mismatch' in (scratch/'stale.log').read_text()
assert json.loads((scratch/'validate-3.json').read_text())['conflict_count'] == 1
review = json.loads((scratch/'review-3.json').read_text())
assert review == json.loads((scratch/'review-archived.json').read_text())
assert review == json.loads((scratch/'review-after-edit.json').read_text())
assert len(review['items']) == 4
assert len(review['conflicts']) == 1
assert any(item['id'] == 'ADR-1' and item['hold_reason'] == 'awaiting_human_review' for item in review['items'])
assert any(item['id'] == 'REL-1' and item['hold_reason'] == 'awaiting_human_review' for item in review['items'])
fourth = json.loads((scratch/'review-fourth.json').read_text())
assert any(item['id'] == 'ADR-1' and item['status'] == 'verified' and item['reviewed_by'] == 'fixture-reviewer' and item['review_count'] == 1 for item in fourth['items'])
assert json.loads((scratch/'review-record.json').read_text())['needs_relock'] is True
assert json.loads((scratch/'lock-4.json').read_text())['lock_digest'] != json.loads((scratch/'lock-3.json').read_text())['lock_digest']
PY
python3 - "$data/third" <<'PY'
import json, pathlib, sys
version = pathlib.Path(sys.argv[1])
manifest = json.loads((version/'sources'/'manifest.json').read_text())
policy = next(f for f in manifest['files'] if f['origin_id'] == 'repo' and f['path'].endswith('BR-17.yaml'))
blob = version/'sources'/'blobs'/policy['sha256']
blob.chmod(0o600)
blob.write_bytes(b'tampered\n')
PY
if "$repo_root/bin/cks" knowledge review --version-dir "$data/third" > "$scratch/review-corrupt.json" 2>&1; then
  echo "corrupt retained policy was accepted" >&2; exit 1
fi
echo "Knowledge lock and candidate identity smoke passed: $scratch"
