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
relation_types: []
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
"$repo_root/bin/cks" knowledge lock --project-root "$src" > "$scratch/lock-3.json"
"$repo_root/bin/cks" knowledge validate --project-root "$src" > "$scratch/validate-3.json"
"$repo_root/bin/cks" knowledge review --project-root "$src" > "$scratch/review-3.json"
"$repo_root/bin/cks" setup --src "$src" --out "$data" --project-id knowledge-fixture \
  --source-mode snapshot-only --version third --embedder mock > "$scratch/third.log" 2>&1
"$repo_root/bin/cks" knowledge review --version-dir "$data/third" > "$scratch/review-archived.json"
printf 'status: proposed\n' > "$src/.cks/knowledge/policies/BR-17.yaml"
"$repo_root/bin/cks" knowledge review --version-dir "$data/third" > "$scratch/review-after-edit.json"
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
assert (data/'current').resolve() == (data/'third').resolve()
assert not (data/'stale').exists()
assert json.loads((scratch/'validate-1.json').read_text())['status'] == 'locked'
assert json.loads((scratch/'lock-1.json').read_text())['lock_digest'] != json.loads((scratch/'lock-2.json').read_text())['lock_digest']
assert 'pack_lock_mismatch' in (scratch/'stale.log').read_text()
assert json.loads((scratch/'validate-3.json').read_text())['conflict_count'] == 1
review = json.loads((scratch/'review-3.json').read_text())
assert review == json.loads((scratch/'review-archived.json').read_text())
assert review == json.loads((scratch/'review-after-edit.json').read_text())
assert len(review['items']) == 3
assert len(review['conflicts']) == 1
assert any(item['id'] == 'ADR-1' and item['hold_reason'] == 'awaiting_human_review' for item in review['items'])
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
