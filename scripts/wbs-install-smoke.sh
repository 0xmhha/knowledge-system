#!/usr/bin/env bash
# Multi-project installation capability smoke. Mock embeddings verify wiring,
# not semantic answer quality.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
scratch="${KS_INSTALL_SMOKE_DIR:-$(mktemp -d)}"
for kind in empty-go typescript unsupported-python; do
  src="$scratch/$kind/src"
  dataset="$scratch/$kind/dataset"
  config="$scratch/$kind/setup.yaml"
  mkdir -p "$src"
  printf '# Guide\nA committed guide for %s.\n' "$kind" > "$src/README.md"
  case "$kind" in
    empty-go) printf 'module example.com/empty\n\ngo 1.25\n' > "$src/go.mod" ;;
    typescript) printf 'export function greet(): string { return "hello"; }\n' > "$src/main.ts" ;;
    unsupported-python) printf 'def greet():\n    return "hello"\n' > "$src/main.py" ;;
  esac
  git -C "$src" init -q
  git -C "$src" add .
  git -C "$src" -c commit.gpgsign=false -c user.name=Codex \
    -c user.email=codex@example.com commit -qm fixture
  "$repo_root/bin/cks" init --src "$src" --dataset "$dataset" \
    --config-out "$config" --embedder mock > "$scratch/$kind/init.json"
  "$repo_root/bin/cks" setup --config "$config" --version auto \
    --progress text > "$scratch/$kind/setup.log" 2>&1
  "$repo_root/bin/cks" doctor --src "$src" --dataset "$dataset" \
    > "$scratch/$kind/doctor.json"
  "$repo_root/bin/ckv" --embedder mock query "committed guide" \
    --out "$dataset/current/vector" --threshold -1 --json \
    > "$scratch/$kind/query.json" 2> "$scratch/$kind/query.log"
done
python3 - "$scratch" <<'PY'
import json
from pathlib import Path
import sys
root = Path(sys.argv[1])
for kind in ('empty-go', 'typescript', 'unsupported-python'):
    base = root / kind
    init = json.loads((base / 'init.json').read_text())
    doctor = json.loads((base / 'doctor.json').read_text())
    query = json.loads((base / 'query.json').read_text())
    assert Path(init['source_root']).resolve() == (base / 'src').resolve(), init
    assert doctor['dataset_version'] and doctor['commit'], doctor
    assert query['hits'] and all(hit['citation']['commit_hash'] == doctor['commit'] for hit in query['hits']), (kind, query)
    if kind == 'typescript':
        assert doctor['shared_code_files'] == 1 and doctor['status'] == 'ready', doctor
        assert any(hit['citation']['file'] == 'main.ts' for hit in query['hits']), query
    else:
        assert doctor['shared_code_files'] == 0 and doctor['status'] == 'degraded', doctor
        assert all(hit['citation']['file'] == 'README.md' for hit in query['hits']), query
print(f'Installation capability smoke reports: {root}')
PY
