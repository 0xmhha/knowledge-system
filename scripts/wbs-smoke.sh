#!/usr/bin/env bash
# Structural CKV+CKG+CKS smoke. The mock embedder is deterministic but does
# not measure semantic retrieval quality. Pass a real-model eval separately.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
scratch="${KS_SMOKE_DIR:-$(mktemp -d)}"
src="$scratch/src"
dataset="$scratch/dataset"
config="$scratch/cks.yaml"
mkdir -p "$src"
cp "$repo_root/testdata/wbs-smoke/go.mod" "$repo_root/testdata/wbs-smoke/main.go" \
  "$repo_root/testdata/wbs-smoke/main_test.go" "$repo_root/testdata/wbs-smoke/README.md" \
  "$repo_root/testdata/wbs-smoke/ontology.yaml" "$repo_root/testdata/wbs-smoke/spec.yaml" \
  "$repo_root/testdata/wbs-smoke/ontology-reviewed.yaml" \
  "$repo_root/testdata/wbs-smoke/spec-reviewed.yaml" "$src/"
git -C "$src" init -q
git -C "$src" add .
git -C "$src" -c commit.gpgsign=false -c user.name=Codex \
  -c user.email=codex@example.com commit -qm fixture

"$repo_root/bin/cks" setup --src "$src" --out "$dataset" \
  --embedder mock --version smoke --progress text
"$repo_root/bin/ckg" audit --src "$src" --graph "$dataset/current/graph"
"$repo_root/bin/cks" semantic build --repo "$src" --project-id ks-fixture \
  --dataset-id smoke --graph "$dataset/current/graph" \
  --vector "$dataset/current/vector" --store "$scratch/semantic.db" \
  --out "$scratch/projection.json" --docs README.md --ontology ontology.yaml --spec spec.yaml \
  --min-canonical-ratio 0.4 --activate \
  > "$scratch/semantic-build.json"
"$repo_root/bin/cks" semantic review --input "$scratch/projection.json" \
  --repo "$src" --sample 10 > "$scratch/semantic-review.json"
"$repo_root/bin/cks" semantic build --repo "$src" --project-id ks-fixture \
  --dataset-id smoke-code --graph "$dataset/current/graph" \
  --vector "$dataset/current/vector" --store "$scratch/semantic.db" \
  --out "$scratch/projection-code.json" --docs README.md --ontology ontology.yaml --spec spec.yaml \
  --min-canonical-ratio 0.4 --extract-only > "$scratch/semantic-extract.json"
python3 - "$scratch/projection-code.json" "$dataset/current/graph/graph.db" "$src" <<'PY'
import hashlib
import json
from pathlib import Path
import sqlite3
import subprocess
import sys
p = Path(sys.argv[1])
graph = sqlite3.connect(sys.argv[2])
row = graph.execute("SELECT canonical_id, file_path, start_line, end_line FROM nodes WHERE name='Alpha' AND type='Function'").fetchone()
assert row and row[0], row
canonical, file, start, end = row
projection = json.loads(p.read_text())
commit = projection['snapshot']['commit']
source = subprocess.check_output(['git', '-C', sys.argv[3], 'show', commit + ':' + file])
span = b''.join(source.splitlines(keepends=True)[start - 1:end])
projection['evidence'].append({
    'id': 'code:alpha', 'snapshot': projection['snapshot'], 'kind': 'code',
    'path': file, 'start_line': start, 'end_line': end,
    'content_sha256': hashlib.sha256(span).hexdigest(),
    'canonical_id': canonical, 'extractor': 'smoke-review-v1',
})
concept = next(c for c in projection['concepts'] if c['id'] == 'alpha-function')
projection['assertions'] = [{
    'id': 'assertion:alpha-implementation', 'predicate': 'IMPLEMENTED_BY',
    'subject_id': 'alpha-function', 'object_id': canonical,
    'evidence_ids': [concept['evidence_id'], 'code:alpha'], 'status': 'proposed',
}]
p.write_text(json.dumps(projection, indent=2) + '\n')
PY
"$repo_root/bin/cks" semantic promote --input "$scratch/projection-code.json" \
  --repo "$src" --graph "$dataset/current/graph" --vector "$dataset/current/vector" \
  --store "$scratch/semantic.db" --min-canonical-ratio 0.4 --activate \
  > "$scratch/semantic-promote.json"
"$repo_root/bin/cks" semantic review --input "$scratch/projection-code.json" \
  --repo "$src" --sample 10 > "$scratch/semantic-code-review.json"
"$repo_root/bin/cks" semantic trace --project-id ks-fixture --repo "$src" \
  --graph "$dataset/current/graph" --vector "$dataset/current/vector" \
  --store "$scratch/semantic.db" > "$scratch/semantic-trace.json"
python3 - "$scratch/projection-code.json" "$scratch/projection-invalid.json" <<'PY'
import json
import sys
p = json.load(open(sys.argv[1], encoding='utf-8'))
p['evidence'][-1]['canonical_id'] = 'missing.canonical.ID'
p['assertions'][0]['object_id'] = 'missing.canonical.ID'
json.dump(p, open(sys.argv[2], 'w', encoding='utf-8'))
PY
if "$repo_root/bin/cks" semantic promote --input "$scratch/projection-invalid.json" \
  --repo "$src" --graph "$dataset/current/graph" --vector "$dataset/current/vector" \
  --store "$scratch/semantic.db" --activate > "$scratch/invalid-promote.log" 2>&1; then
  echo "invalid code anchor was promoted" >&2
  exit 1
fi
python3 - "$scratch/semantic.db" <<'PY'
import sqlite3, sys
active = sqlite3.connect(sys.argv[1]).execute("SELECT dataset_id FROM semantic_current WHERE project_id='ks-fixture'").fetchone()
assert active == ('smoke-code',), active
PY
"$repo_root/bin/cks" semantic build --repo "$src" --project-id ks-fixture \
  --dataset-id smoke-reviewed --graph "$dataset/current/graph" \
  --vector "$dataset/current/vector" --store "$scratch/semantic.db" \
  --out "$scratch/projection-reviewed.json" --docs README.md \
  --ontology ontology-reviewed.yaml --spec spec-reviewed.yaml \
  --min-canonical-ratio 0.4 --extract-only > "$scratch/semantic-reviewed-extract.json"
python3 - "$scratch/projection-reviewed.json" "$dataset/current/graph/graph.db" "$src" <<'PY'
import hashlib, json, sqlite3, subprocess, sys
from pathlib import Path
target = Path(sys.argv[1])
p = json.loads(target.read_text())
graph = sqlite3.connect(sys.argv[2])
anchors = {}
for name, proof_id, kind in [('Alpha', 'code:alpha', 'code'), ('TestAlpha', 'test:alpha', 'test')]:
    row = graph.execute("SELECT canonical_id,file_path,start_line,end_line FROM nodes WHERE name=? AND type='Function'", (name,)).fetchone()
    assert row and row[0], row
    canonical, file, start, end = row
    source = subprocess.check_output(['git', '-C', sys.argv[3], 'show', p['snapshot']['commit'] + ':' + file])
    span = b''.join(source.splitlines(keepends=True)[start - 1:end])
    p['evidence'].append({'id': proof_id, 'snapshot': p['snapshot'], 'kind': kind, 'path': file,
                          'start_line': start, 'end_line': end, 'content_sha256': hashlib.sha256(span).hexdigest(),
                          'canonical_id': canonical, 'extractor': 'smoke-review-v1'})
    anchors[name] = canonical
concept = next(c for c in p['concepts'] if c['id'] == 'alpha-function')
criterion = p['requirements'][0]['acceptance_criteria'][0]
p['assertions'] = [
    {'id': 'reviewed:implementation', 'predicate': 'IMPLEMENTED_BY', 'subject_id': 'alpha-function',
     'object_id': anchors['Alpha'], 'evidence_ids': [concept['evidence_id'], 'code:alpha'],
     'status': 'verified', 'reviewed_by': 'fixture-reviewer'},
    {'id': 'reviewed:tested', 'predicate': 'TESTED_BY', 'subject_id': anchors['Alpha'],
     'object_id': anchors['TestAlpha'], 'evidence_ids': ['code:alpha', 'test:alpha'],
     'status': 'verified', 'reviewed_by': 'fixture-reviewer'},
    {'id': 'reviewed:checked', 'predicate': 'CHECKED_BY', 'subject_id': criterion['id'],
     'object_id': anchors['TestAlpha'], 'evidence_ids': [criterion['evidence_id'], 'test:alpha'],
     'status': 'verified', 'reviewed_by': 'fixture-reviewer'},
]
target.write_text(json.dumps(p, indent=2) + '\n')
PY
"$repo_root/bin/cks" semantic promote --input "$scratch/projection-reviewed.json" \
  --repo "$src" --graph "$dataset/current/graph" --vector "$dataset/current/vector" \
  --store "$scratch/semantic.db" --min-canonical-ratio 0.4 --activate \
  > "$scratch/semantic-reviewed-promote.json"
"$repo_root/bin/cks" semantic trace --project-id ks-fixture --repo "$src" \
  --graph "$dataset/current/graph" --vector "$dataset/current/vector" \
  --store "$scratch/semantic.db" > "$scratch/semantic-reviewed-trace.json"
"$repo_root/bin/cks" semantic lookup-term --project-id ks-fixture --repo "$src" \
  --graph "$dataset/current/graph" --vector "$dataset/current/vector" \
  --store "$scratch/semantic.db" --lang en --term 'ALPHA FUNCTION' \
  > "$scratch/semantic-term.json"
python3 - "$scratch/semantic-term.json" <<'PY'
import json, sys
lookup = json.load(open(sys.argv[1], encoding='utf-8'))
assert lookup['candidates'] == [{'concept_id': 'alpha-function', 'term': 'Alpha function',
                                 'status': 'verified', 'score': 1}], lookup
PY
"$repo_root/bin/cks" semantic plan --project-id ks-fixture --repo "$src" \
  --graph "$dataset/current/graph" --vector "$dataset/current/vector" \
  --store "$scratch/semantic.db" > "$scratch/semantic-reviewed-plan.json"
"$repo_root/bin/cks" semantic test --project-id ks-fixture --repo "$src" \
  --graph "$dataset/current/graph" --vector "$dataset/current/vector" \
  --store "$scratch/semantic.db" --criterion-id ac-alpha \
  --out "$scratch/test-pass.json" -- go test ./...
"$repo_root/bin/cks" semantic test --project-id ks-fixture --repo "$src" \
  --graph "$dataset/current/graph" --vector "$dataset/current/vector" \
  --store "$scratch/semantic.db" --criterion-id ac-alpha \
  --out "$scratch/test-exact.json" --go-test-exact
if "$repo_root/bin/cks" semantic test --project-id ks-fixture --repo "$src" \
  --graph "$dataset/current/graph" --vector "$dataset/current/vector" \
  --store "$scratch/semantic.db" --criterion-id ac-alpha \
  --out "$scratch/test-fail.json" -- go invalid-command > "$scratch/test-fail.log" 2>&1; then
  echo "failed test command reported success" >&2
  exit 1
fi
if "$repo_root/bin/cks" semantic test --project-id ks-fixture --repo "$src" \
  --graph "$dataset/current/graph" --vector "$dataset/current/vector" \
  --store "$scratch/semantic.db" --criterion-id missing-criterion \
  --out "$scratch/test-unlinked.json" -- go test ./... > "$scratch/test-unlinked.log" 2>&1; then
  echo "unlinked criterion was executed" >&2
  exit 1
fi
read -r code_file code_start code_end code_commit < <(python3 - "$scratch/projection-reviewed.json" <<'PY'
import json, sys
p = json.load(open(sys.argv[1], encoding='utf-8'))
e = next(e for e in p['evidence'] if e['id'] == 'code:alpha')
print(e['path'], e['start_line'], e['end_line'], p['snapshot']['commit'])
PY
)
(cd "$repo_root" && go build -o "$scratch/make-pack" ./testdata/wbs-smoke/make-pack/main.go)
"$scratch/make-pack" "$code_file" "$code_start" "$code_end" \
  "$code_commit" "$scratch/base-pack.json"
"$repo_root/bin/cks" semantic annotate-pack --project-id ks-fixture --repo "$src" \
  --graph "$dataset/current/graph" --vector "$dataset/current/vector" \
  --store "$scratch/semantic.db" --input "$scratch/base-pack.json" \
  --out "$scratch/annotated-pack.json"
"$repo_root/bin/cks" semantic export-text --project-id ks-fixture --repo "$src" \
  --graph "$dataset/current/graph" --vector "$dataset/current/vector" \
  --store "$scratch/semantic.db" --out "$scratch/semantic-corpus" \
  > "$scratch/semantic-corpus-export.json"
"$repo_root/bin/cks" setup --src "$src" --out "$dataset" --embedder mock \
  --version smoke-semantic --semantic-corpus "$scratch/semantic-corpus" \
  --gate-min-canonical 0.4 --progress text > "$scratch/semantic-corpus-reindex.log" 2>&1
"$repo_root/bin/ckv" --embedder mock query "Alpha behavior" \
  --out "$dataset/current/vector" --lang markdown --threshold -1 --json \
  > "$scratch/semantic-corpus-query.json" 2> "$scratch/semantic-corpus-query.log"
python3 - "$scratch/semantic-corpus-export.json" "$scratch/semantic-corpus-query.json" "$dataset/current/vector/manifest.json" <<'PY'
import json, sys
export = json.load(open(sys.argv[1], encoding='utf-8'))
query = json.load(open(sys.argv[2], encoding='utf-8'))
vector = json.load(open(sys.argv[3], encoding='utf-8'))
assert len(export['files']) == 2 and export['snapshot']['dataset_id'] == 'smoke-reviewed', export
assert vector['docs_roots'] and query['hits'], (vector, query)
assert any('concept-' in hit['citation']['file'] for hit in query['hits']), query
PY
cp -R "$scratch/semantic-corpus" "$scratch/semantic-corpus-tampered"
printf '\nUnreviewed edit.\n' >> "$(find "$scratch/semantic-corpus-tampered" -name 'concept-*.md' -print -quit)"
if "$repo_root/bin/cks" setup --src "$src" --out "$dataset" --embedder mock \
  --version smoke-semantic-tampered --semantic-corpus "$scratch/semantic-corpus-tampered" \
  --gate-min-canonical 0.4 --progress text > "$scratch/semantic-corpus-tampered.log" 2>&1; then
  echo "tampered semantic corpus was indexed" >&2
  exit 1
fi
test "$(readlink "$dataset/current")" = smoke-semantic
"$repo_root/bin/cks" setup --out "$dataset" --rollback smoke > "$scratch/semantic-corpus-rollback.log" 2>&1
"$repo_root/bin/cks" mcp gen-config --dataset-dir "$dataset/current" \
  --name ks-fixture --source-root "$src" --out "$config"
python3 - "$config" <<'PY'
from pathlib import Path
import sys
p = Path(sys.argv[1])
s = p.read_text()
s = s.replace('provider: ""', 'provider: mock')
s = s.replace('embed_model: bge-m3', 'embed_model: mock-feature-hash-v1')
s = s.replace('mcp_stdio: false', 'mcp_stdio: true')
s = s.replace('transport: http', 'transport: stdio')
p.write_text(s)
PY
fixture_commit="$(git -C "$src" rev-parse HEAD)"
cp "$repo_root/testdata/wbs-smoke/find-alpha.yaml" "$scratch/find-alpha-pinned.yaml"
cp "$repo_root/testdata/wbs-smoke/absent-api.yaml" "$scratch/absent-api-pinned.yaml"
printf '\nexpected_commit: %s\n' "$fixture_commit" >> "$scratch/find-alpha-pinned.yaml"
printf '\nexpected_commit: %s\n' "$fixture_commit" >> "$scratch/absent-api-pinned.yaml"
"$repo_root/bin/cks" eval --scenarios "$scratch/find-alpha-pinned.yaml" \
  --config "$config" --verify-anchors "$src" --output "$scratch/find-alpha-report.json"
"$repo_root/bin/cks" eval --scenarios "$scratch/absent-api-pinned.yaml" \
  --config "$config" --output "$scratch/absent-api-report.json"
cp "$repo_root/testdata/wbs-smoke/find-alpha.yaml" "$scratch/find-alpha-stale.yaml"
printf '\nexpected_commit: %040d\n' 0 >> "$scratch/find-alpha-stale.yaml"
if "$repo_root/bin/cks" eval --scenarios "$scratch/find-alpha-stale.yaml" \
  --config "$config" --output "$scratch/find-alpha-stale-report.json" \
  > "$scratch/find-alpha-stale.log" 2>&1; then
  echo "eval accepted a conflicting indexed commit" >&2
  exit 1
fi
python3 - "$scratch/find-alpha-report.json" "$scratch/absent-api-report.json" "$scratch/semantic-build.json" "$scratch/semantic-review.json" "$scratch/semantic-promote.json" "$scratch/semantic-code-review.json" "$scratch/semantic.db" "$scratch/semantic-trace.json" "$scratch/semantic-reviewed-trace.json" "$scratch/semantic-reviewed-plan.json" "$scratch/test-pass.json" "$scratch/test-fail.json" "$scratch/test-unlinked.json" "$scratch/base-pack.json" "$scratch/annotated-pack.json" "$scratch/find-alpha-stale-report.json" "$scratch/test-exact.json" <<'PY'
import json
import sqlite3
import sys
found = json.load(open(sys.argv[1], encoding='utf-8'))['results'][0]
absent = json.load(open(sys.argv[2], encoding='utf-8'))['results'][0]
assert found['metrics']['file_recall'] == 1 and found['metrics']['citation_count'] > 0, found
assert absent['citation_abstention_passed'] is True, absent
assert found['retrieval_state'] == 'pass' and found['snapshot_state'] == 'current', found
assert absent['abstention_state'] == 'pass' and absent['snapshot_state'] == 'current', absent
stale = json.load(open(sys.argv[16], encoding='utf-8'))['results'][0]
assert stale['snapshot_state'] == 'conflict' and stale['retrieval_state'] == 'miss', stale
semantic = json.load(open(sys.argv[3], encoding='utf-8'))
review = json.load(open(sys.argv[4], encoding='utf-8'))
assert semantic['activated'] is True and semantic['sections'] > 0 and semantic['chunk_linked_sections'] == semantic['sections'] and semantic['concepts'] == 2 and semantic['requirements'] == 1 and semantic['evidence'] > 0, semantic
assert review['project_id'] == 'ks-fixture' and review['reviewed_precision'] is None and review['concept_proposed'] == 2 and len(review['concept_sample']) == 2 and review['requirement_proposed'] == 1, review
promotion = json.load(open(sys.argv[5], encoding='utf-8'))
code_review = json.load(open(sys.argv[6], encoding='utf-8'))
assert promotion['activated'] is True and promotion['assertions'] == 1, promotion
assert code_review['assertion_proposed'] == 1 and len(code_review['assertion_sample']) == 1, code_review
active = sqlite3.connect(sys.argv[7]).execute("SELECT dataset_id FROM semantic_current WHERE project_id='ks-fixture'").fetchone()
assert active == ('smoke-reviewed',), active
trace = json.load(open(sys.argv[8], encoding='utf-8'))
assert trace['requirements'][0]['state'] == 'spec_unapproved', trace
reviewed = json.load(open(sys.argv[9], encoding='utf-8'))
assert reviewed['requirements'][0]['state'] == 'linked' and len(reviewed['requirements'][0]['paths']) == 1, reviewed
plan = json.load(open(sys.argv[10], encoding='utf-8'))
assert plan['steps'][0]['action'] == 'execute_acceptance_test' and plan['steps'][0]['unconfirmed'] is True, plan
passed = json.load(open(sys.argv[11], encoding='utf-8'))
failed = json.load(open(sys.argv[12], encoding='utf-8'))
assert passed['command_passed'] is True and passed['snapshot_consistent'] is True and passed['criterion_id'] == 'ac-alpha', passed
assert passed['test_canonical_id'] == reviewed['requirements'][0]['paths'][0]['test_canonical_id'], passed
assert passed['tested_by_assertions'] == ['reviewed:tested'] and passed['checked_by_assertions'] == ['reviewed:checked'], passed
assert 'accepted_by_assertions' not in passed, passed
exact = json.load(open(sys.argv[17], encoding='utf-8'))
assert exact['framework'] == 'go-test-json' and exact['test_name'] == 'TestAlpha' and exact['test_observed'] is True and exact['test_passed'] is True, exact
assert exact['test_canonical_id'] == passed['test_canonical_id'] and exact['snapshot'] == passed['snapshot'], exact
assert failed['command_passed'] is False and failed['snapshot_consistent'] is True and failed['exit_code'] != 0, failed
from pathlib import Path
assert not Path(sys.argv[13]).exists(), 'unlinked criterion wrote a result'
base = json.load(open(sys.argv[14], encoding='utf-8'))
annotated = json.load(open(sys.argv[15], encoding='utf-8'))
assert base['metadata']['integrity_hash'] == annotated['metadata']['integrity_hash'], annotated
assert annotated['semantic']['links'][0]['requirement_id'] == 'req-alpha' and annotated['semantic']['links'][0]['unconfirmed'] is True, annotated
PY
pin_dir="$scratch/pinned-server"
python3 "$repo_root/scripts/wbs-mcp-pin-probe.py" "$repo_root/bin/cks" "$config" "$pin_dir" \
  > "$scratch/pinned-server.log" 2>&1 &
pin_pid=$!
trap 'kill "$pin_pid" 2>/dev/null || true' EXIT
for attempt in $(seq 1 100); do
  test -f "$pin_dir/ready.json" && break
  kill -0 "$pin_pid" 2>/dev/null || { cat "$scratch/pinned-server.log" >&2; exit 1; }
  sleep 0.1
done
test -f "$pin_dir/ready.json"
printf '\nA second committed snapshot.\n' >> "$src/README.md"
git -C "$src" add README.md
git -C "$src" -c commit.gpgsign=false -c user.name=Codex \
  -c user.email=codex@example.com commit -qm next-snapshot
"$repo_root/bin/cks" setup --src "$src" --out "$dataset" \
  --embedder mock --version smoke-next --gate-min-canonical 0.4 --progress text \
  > "$scratch/reindex-next.log" 2>&1
test "$(readlink "$dataset/current")" = smoke-next
if "$repo_root/bin/cks" semantic trace --diagnose-stale --project-id ks-fixture \
  --repo "$src" --graph "$dataset/current/graph" --vector "$dataset/current/vector" \
  --store "$scratch/semantic.db" > "$scratch/stale-trace.json" 2> "$scratch/stale-trace-diagnostic.log"; then
  echo "stale semantic trace returned success" >&2
  exit 1
fi
python3 - "$scratch/stale-trace.json" <<'PY'
import json, sys
report = json.load(open(sys.argv[1], encoding='utf-8'))
assert report['state'] == 'stale' and report['requirements'] == [], report
assert 'source commit' in report['reason'], report
PY
if "$repo_root/bin/cks" semantic trace --project-id ks-fixture --repo "$src" \
  --graph "$dataset/current/graph" --vector "$dataset/current/vector" \
  --store "$scratch/semantic.db" > "$scratch/stale-trace.log" 2>&1; then
  echo "semantic trace accepted a different graph/vector snapshot" >&2
  exit 1
fi
if "$repo_root/bin/cks" setup --src "$src" --out "$dataset" \
  --embedder mock --version smoke-test-rejected --gate-min-canonical 0.4 \
  --gate-test-bin go --gate-test-arg invalid-command --progress text \
  > "$scratch/reindex-test-rejected.log" 2>&1; then
  echo "failed candidate test was promoted" >&2
  exit 1
fi
test "$(readlink "$dataset/current")" = smoke-next
"$repo_root/bin/cks" setup --src "$src" --out "$dataset" \
  --embedder mock --version smoke-tested --gate-min-canonical 0.4 \
  --gate-test-bin go --gate-test-arg test --gate-test-arg ./... --progress text \
  > "$scratch/reindex-tested.log" 2>&1
test "$(readlink "$dataset/current")" = smoke-tested
touch "$pin_dir/next"
wait "$pin_pid"
trap - EXIT
fresh_dir="$scratch/fresh-server"
mkdir -p "$fresh_dir"
touch "$fresh_dir/next"
python3 "$repo_root/scripts/wbs-mcp-pin-probe.py" "$repo_root/bin/cks" "$config" "$fresh_dir"
python3 - "$pin_dir" "$fresh_dir" "$src" <<'PY'
import json, pathlib, subprocess, sys
pinned, fresh = pathlib.Path(sys.argv[1]), pathlib.Path(sys.argv[2])
old_before = json.loads((pinned / 'ready.json').read_text())
old_after = json.loads((pinned / 'after.json').read_text())
new = json.loads((fresh / 'ready.json').read_text())
head = subprocess.check_output(['git', '-C', sys.argv[3], 'rev-parse', 'HEAD'], text=True).strip()
assert old_before['serviceable'] and old_after['serviceable'] and new['serviceable']
assert old_before['commit'] == old_after['commit'] != head, (old_before, old_after)
assert old_before['citation_commits'] == old_after['citation_commits'] == [old_before['commit']]
assert new['commit'] == head and new['citation_commits'] == [head], new
assert 'main.go' in new['citation_files'], new
PY
python3 - "$dataset" "$src" <<'PY'
import json, pathlib, subprocess, sys
root = pathlib.Path(sys.argv[1])
commit = subprocess.check_output(['git', '-C', sys.argv[2], 'rev-parse', 'HEAD'], text=True).strip()
failed = json.loads((root / 'smoke-test-rejected' / 'test-gate.json').read_text())
passed = json.loads((root / 'smoke-tested' / 'test-gate.json').read_text())
assert failed['source_commit'] == commit and failed['command_passed'] is False, failed
assert passed['source_commit'] == commit and passed['command_passed'] is True and passed['snapshot_consistent'] is True, passed
PY
if "$repo_root/bin/cks" setup --src "$src" --out "$dataset" \
  --embedder mock --version smoke-rejected --gate-min-canonical 1.01 --progress text \
  > "$scratch/reindex-rejected.log" 2>&1; then
  echo "candidate below the requested canonical coverage was promoted" >&2
  exit 1
fi
test "$(readlink "$dataset/current")" = smoke-tested
"$repo_root/bin/cks" setup --out "$dataset" --rollback smoke > "$scratch/rollback.log" 2>&1
test "$(readlink "$dataset/current")" = smoke
rolled_dir="$scratch/rolled-server"
mkdir -p "$rolled_dir"
touch "$rolled_dir/next"
python3 "$repo_root/scripts/wbs-mcp-pin-probe.py" "$repo_root/bin/cks" "$config" "$rolled_dir"
python3 - "$pin_dir" "$rolled_dir" <<'PY'
import json, pathlib, sys
old = json.loads((pathlib.Path(sys.argv[1]) / 'ready.json').read_text())
rolled = json.loads((pathlib.Path(sys.argv[2]) / 'ready.json').read_text())
assert rolled['commit'] == old['commit'] and rolled['citation_commits'] == old['citation_commits'], rolled
PY
"$repo_root/bin/cks" semantic trace --project-id ks-fixture --repo "$src" \
  --graph "$dataset/current/graph" --vector "$dataset/current/vector" \
  --store "$scratch/semantic.db" > "$scratch/rollback-trace.json"
python3 - "$scratch/rollback-trace.json" <<'PY'
import json, sys
report = json.load(open(sys.argv[1], encoding='utf-8'))
assert report['requirements'][0]['state'] == 'linked', report
PY
printf 'Structural smoke reports: %s\n' "$scratch"
