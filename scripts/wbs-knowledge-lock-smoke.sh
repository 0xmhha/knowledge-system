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
printf 'status: proposed\n' > "$src/.cks/knowledge/policies/BR-17.yaml"
if "$repo_root/bin/cks" setup --src "$src" --out "$data" --project-id knowledge-fixture \
  --source-mode snapshot-only --version stale --embedder mock > "$scratch/stale.log" 2>&1; then
  echo "stale knowledge lock was accepted" >&2; exit 1
fi
"$repo_root/bin/cks" knowledge lock --project-root "$src" > "$scratch/lock-2.json"
"$repo_root/bin/cks" setup --src "$src" --out "$data" --project-id knowledge-fixture \
  --source-mode snapshot-only --version second --embedder mock > "$scratch/second.log" 2>&1
python3 - "$data" "$scratch" <<'PY'
import json, pathlib, sys
data, scratch = map(pathlib.Path, sys.argv[1:])
first = json.loads((data/'first'/'dataset-identity.json').read_text())
second = json.loads((data/'second'/'dataset-identity.json').read_text())
assert first['dataset_id'] != second['dataset_id']
assert first['source']['snapshot_id'] != second['source']['snapshot_id']
assert (data/'current').resolve() == (data/'second').resolve()
assert not (data/'stale').exists()
assert json.loads((scratch/'validate-1.json').read_text())['status'] == 'locked'
assert json.loads((scratch/'lock-1.json').read_text())['lock_digest'] != json.loads((scratch/'lock-2.json').read_text())['lock_digest']
assert 'pack_lock_mismatch' in (scratch/'stale.log').read_text()
PY
echo "Knowledge lock and candidate identity smoke passed: $scratch"
