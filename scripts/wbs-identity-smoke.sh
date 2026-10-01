#!/usr/bin/env bash
# A3 structural proof: one pinned CKV+CKG build and a semantic projection
# share project, snapshot and dataset coordinates. No real-model claim.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
scratch="${KS_IDENTITY_SMOKE_DIR:-$(mktemp -d)}"
src="$scratch/src"
dataset="$scratch/dataset"
mkdir -p "$src"
cp "$repo_root/testdata/wbs-smoke/go.mod" "$repo_root/testdata/wbs-smoke/main.go" \
  "$repo_root/testdata/wbs-smoke/README.md" "$src/"
git -C "$src" init -q
git -C "$src" add .
git -C "$src" -c commit.gpgsign=false -c user.name=Codex \
  -c user.email=codex@example.com commit -qm fixture
"$repo_root/bin/cks" init --src "$src" --dataset "$dataset" \
  --config-out "$scratch/setup.yaml" --embedder mock > "$scratch/init.json"
"$repo_root/bin/cks" setup --config "$scratch/setup.yaml" --version pinned \
  --progress text > "$scratch/setup.log" 2>&1
python3 - "$dataset/current/dataset-identity.json" "$scratch/identity.env" <<'PY'
import json, pathlib, shlex, sys
identity = json.loads(pathlib.Path(sys.argv[1]).read_text())
source = identity['source']
pathlib.Path(sys.argv[2]).write_text(
    'project=' + shlex.quote(source['project_id']) + '\n' +
    'dataset_id=' + shlex.quote(identity['dataset_id']) + '\n'
)
PY
source "$scratch/identity.env"
"$repo_root/bin/cks" semantic build --repo "$src" --project-id "$project" \
  --dataset-id "$dataset_id" --graph "$dataset/current/graph" \
  --vector "$dataset/current/vector" --store "$scratch/semantic.db" \
  --out "$scratch/projection.json" --docs README.md --extract-only \
  > "$scratch/semantic.json"
"$repo_root/bin/cks" doctor --src "$src" --dataset "$dataset" > "$scratch/doctor.json"
python3 - "$dataset/current/dataset-identity.json" "$scratch/projection.json" "$dataset/current" "$scratch/doctor.json" "$src" <<'PY'
import hashlib, json, pathlib, sqlite3, sys
identity = json.loads(pathlib.Path(sys.argv[1]).read_text())
projection = json.loads(pathlib.Path(sys.argv[2]).read_text())
root = pathlib.Path(sys.argv[3])
doctor = json.loads(pathlib.Path(sys.argv[4]).read_text())
logical_source = str(pathlib.Path(sys.argv[5]).resolve())
assert not (root / '.build-source').exists()
captured = json.loads((root / 'sources' / 'manifest.json').read_text())
assert captured['identity'] == identity['source']
assert captured['files']
for entry in captured['files']:
    blob = (root / 'sources' / 'blobs' / entry['sha256']).read_bytes()
    assert len(blob) == entry['size'] and hashlib.sha256(blob).hexdigest() == entry['sha256']
assert doctor['identity_status'] == 'pinned' and doctor.get('reindex_required', False) is False
assert doctor['project_id'] == identity['source']['project_id']
assert doctor['snapshot_id'] == identity['source']['snapshot_id']
assert doctor['dataset_id'] == identity['dataset_id']
assert projection['snapshot']['project_id'] == identity['source']['project_id']
assert projection['snapshot']['dataset_id'] == identity['dataset_id']
assert projection['snapshot']['snapshot_id'] == identity['source']['snapshot_id']
for engine in ('graph', 'vector'):
    manifest = json.loads((root / engine / 'manifest.json').read_text())
    assert manifest['src_root'] == logical_source
    assert '.build-source' not in json.dumps(manifest)
    db_name = 'graph.db' if engine == 'graph' else 'vector.db'
    with sqlite3.connect(root / engine / db_name) as db:
        native = dict(db.execute('SELECT key, value FROM manifest'))
    for key, want in (
        ('src_root', logical_source),
        ('project_id', identity['source']['project_id']),
        ('snapshot_id', identity['source']['snapshot_id']),
        ('dataset_id', identity['dataset_id']),
        ('file_manifest_digest', identity['source']['file_manifest_digest']),
        ('capture_policy_digest', identity['source']['capture_policy_digest']),
    ):
        assert manifest[key] == native[key] == want, (engine, key, manifest.get(key), native.get(key), want)
PY
first_blob="$(find "$dataset/current/sources/blobs" -type f | LC_ALL=C sort | head -n 1)"
chmod u+w "$first_blob"
printf 'tampered retained source\n' > "$first_blob"
if "$repo_root/bin/cks" setup --out "$dataset" --rollback pinned > "$scratch/tamper.log" 2>&1; then
  echo "rollback accepted a corrupted retained source blob" >&2
  exit 1
fi
if "$repo_root/bin/cks" doctor --src "$src" --dataset "$dataset" > "$scratch/tampered-doctor.json"; then
  python3 - "$scratch/tampered-doctor.json" <<'PY'
import json, pathlib, sys
report = json.loads(pathlib.Path(sys.argv[1]).read_text())
assert report['status'] == 'degraded'
assert report['identity_status'] == 'invalid' and report['reindex_required'] is True
assert report['issues'] and all('tampered retained source' not in issue for issue in report['issues'])
PY
fi
echo "Pinned three-layer identity smoke passed: $scratch"
