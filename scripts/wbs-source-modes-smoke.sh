#!/usr/bin/env bash
# A4 structural smoke with mock embeddings; no model-quality assertion.
set -euo pipefail
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
scratch="${KS_SOURCE_SMOKE_DIR:-$(mktemp -d)}"
src="$scratch/git-src"
data="$scratch/git-data"
plain="$scratch/plain-src"
mkdir -p "$src" "$plain"
cp "$repo_root/testdata/wbs-smoke/go.mod" "$repo_root/testdata/wbs-smoke/main.go" "$repo_root/testdata/wbs-smoke/README.md" "$src/"
cp "$repo_root/testdata/wbs-smoke/go.mod" "$repo_root/testdata/wbs-smoke/main.go" "$repo_root/testdata/wbs-smoke/README.md" "$plain/"
git -C "$src" init -q
git -C "$src" add .
git -C "$src" -c commit.gpgsign=false -c user.name=Test -c user.email=test@example.org commit -qm base
"$repo_root/bin/cks" setup --src "$src" --out "$data" --project-id p --source-mode working-tree --version clean --embedder mock > "$scratch/clean.log" 2>&1
sed 's/Alpha/Beta/g' "$repo_root/testdata/wbs-smoke/main.go" > "$src/main.go"
"$repo_root/bin/cks" setup --src "$src" --out "$data" --project-id p --source-mode working-tree --version modified --embedder mock > "$scratch/modified.log" 2>&1
printf '# New\n' > "$src/new.md"
"$repo_root/bin/cks" setup --src "$src" --out "$data" --project-id p --source-mode working-tree --version added --embedder mock > "$scratch/added.log" 2>&1
sed 's/Alpha/Gamma/g' "$repo_root/testdata/wbs-smoke/main.go" > "$src/main.go"
"$repo_root/bin/cks" setup --src "$src" --out "$data" --project-id p --source-mode working-tree --version remodified --embedder mock > "$scratch/remodified.log" 2>&1
"$repo_root/bin/cks" setup --src "$plain" --out "$scratch/plain-data" --project-id plain --source-mode snapshot-only --version plain --embedder mock > "$scratch/plain.log" 2>&1
"$repo_root/bin/cks" mcp gen-config --dataset-dir "$scratch/plain-data/current" \
  --name ks-plain --source-root "$plain" --sanitize-rules "$repo_root/system/policies/sanitization_rules.yaml" \
  --out "$scratch/plain-mcp.yaml" > "$scratch/plain-mcp-config.log"
python3 - "$scratch/plain-mcp.yaml" <<'PY'
from pathlib import Path
import sys
path = Path(sys.argv[1])
config = path.read_text()
for before, after in (
    ('provider: ""', 'provider: mock'),
    ('embed_model: bge-m3', 'embed_model: mock-feature-hash-v1'),
    ('mcp_stdio: false', 'mcp_stdio: true'),
    ('transport: http', 'transport: stdio'),
):
    assert before in config
    config = config.replace(before, after)
path.write_text(config)
PY
sed 's/Alpha/Zeta/g' "$repo_root/testdata/wbs-smoke/main.go" > "$plain/main.go"
sed 's/Alpha/Zeta/g' "$repo_root/testdata/wbs-smoke/README.md" > "$plain/README.md"
python3 "$repo_root/scripts/wbs-source-mcp-probe.py" "$repo_root/bin/cks" \
  "$scratch/plain-mcp.yaml" "$scratch/plain-mcp.log"
python3 - "$repo_root/bin/cks" "$plain" "$scratch/plain-data/current" "$scratch" <<'PY'
import json, pathlib, subprocess, sys
binary, src, version, scratch = sys.argv[1:]
root = pathlib.Path(version)
identity = json.loads((root / 'dataset-identity.json').read_text())
out = pathlib.Path(scratch) / 'plain-semantic.json'
args = [binary, 'semantic', 'build', '--repo', src, '--project-id', 'plain',
    '--dataset-id', identity['dataset_id'], '--graph', str(root / 'graph'),
    '--vector', str(root / 'vector'), '--store', str(pathlib.Path(scratch) / 'plain-semantic.db'),
    '--out', str(out), '--docs', 'README.md', '--version-dir', version, '--activate']
subprocess.run(args, check=True, capture_output=True, text=True)
projection = json.loads(out.read_text())
assert projection['snapshot']['source_mode'] == 'snapshot-only'
assert projection['snapshot']['commit'] == ''
assert any(section['heading'] == 'Alpha' for section in projection['sections'])
assert all(section['heading'] != 'Zeta' for section in projection['sections'])
check = subprocess.run([binary, 'semantic', 'lookup-term', '--project-id', 'plain',
    '--repo', src, '--graph', str(root / 'graph'), '--vector', str(root / 'vector'),
    '--store', str(pathlib.Path(scratch) / 'plain-semantic.db'), '--lang', 'en',
    '--term', 'Alpha'], check=True, capture_output=True, text=True)
assert json.loads(check.stdout)['snapshot']['snapshot_id'] == identity['source']['snapshot_id']
PY
cat > "$scratch/mutate.sh" <<EOF
#!/bin/sh
printf 'changed during gate\n' > '$src/new.md'
EOF
chmod +x "$scratch/mutate.sh"
if "$repo_root/bin/cks" setup --src "$src" --out "$data" --project-id p --source-mode working-tree --version mutated --embedder mock --gate-test-bin "$scratch/mutate.sh" > "$scratch/mutated.log" 2>&1; then
  echo "build-time source change was promoted" >&2; exit 1
fi
"$repo_root/bin/cks" doctor --src "$src" --dataset "$data" > "$scratch/git-doctor.json"
"$repo_root/bin/cks" doctor --src "$plain" --dataset "$scratch/plain-data" > "$scratch/plain-doctor.json"
python3 - "$data" "$scratch/plain-data" "$src" "$scratch/git-doctor.json" "$scratch/plain-doctor.json" <<'PY'
import hashlib, json, pathlib, sqlite3, sys
data, plain, source, git_doctor_path, plain_doctor_path = map(pathlib.Path, sys.argv[1:])
ids = []
for version in ('clean', 'modified', 'added', 'remodified'):
    root = data / version
    identity = json.loads((root / 'dataset-identity.json').read_text())
    ids.append(identity['source']['snapshot_id'])
    assert identity['source']['source_mode'] == 'working-tree'
    assert len(identity['source']['source_commit']) == 40
    assert identity['dataset_id']
    inventory = json.loads((root / 'sources' / 'manifest.json').read_text())
    assert inventory['identity'] == identity['source']
    captured = {item['path']: item['sha256'] for item in inventory['files']}
    for item in inventory['files']:
        blob = (root / 'sources' / 'blobs' / item['sha256']).read_bytes()
        assert hashlib.sha256(blob).hexdigest() == item['sha256']
    for side, filename in (('graph','graph.db'), ('vector','vector.db')):
        manifest = json.loads((root / side / 'manifest.json').read_text())
        assert manifest['src_root'] == str(source)
        assert manifest['snapshot_id'] == identity['source']['snapshot_id']
        if side == 'vector':
            assert manifest['input_files']
            for item in manifest['input_files']:
                assert item['origin_id'] == 'repo'
                assert captured[item['path']] == item['sha256']
        with sqlite3.connect(root / side / filename) as db:
            native = dict(db.execute('SELECT key, value FROM manifest'))
        assert native['snapshot_id'] == manifest['snapshot_id']
assert len(set(ids)) == 4, ids
assert (data / 'current').resolve() == (data / 'remodified').resolve()
non_git = json.loads((plain / 'current' / 'dataset-identity.json').read_text())
assert non_git['source']['source_mode'] == 'snapshot-only'
assert non_git['source']['source_commit'] == ''
for side in ('graph', 'vector'):
    manifest = json.loads((plain / 'current' / side / 'manifest.json').read_text())
    assert manifest.get('src_commit', '') == ''
git_doctor = json.loads(git_doctor_path.read_text())
plain_doctor = json.loads(plain_doctor_path.read_text())
assert git_doctor['status'] == 'ready' and git_doctor['source_mode'] == 'working-tree'
assert git_doctor['source_drift'] is True and git_doctor['history_status'] == 'base_history_only'
assert plain_doctor['status'] == 'ready' and plain_doctor['source_mode'] == 'snapshot-only'
assert plain_doctor['history_status'] == 'history_unavailable'
PY
echo "Source modes and mutation rejection smoke passed: $scratch"
