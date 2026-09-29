#!/usr/bin/env bash
# Multi-project installation capability smoke. Mock embeddings verify wiring,
# not semantic answer quality.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
bin_dir="${KS_BIN_DIR:-$repo_root/bin}"
sanitize_rules="${KS_SANITIZE_RULES:-$repo_root/system/policies/sanitization_rules.yaml}"
scratch="${KS_INSTALL_SMOKE_DIR:-$(mktemp -d)}"
for kind in empty-go typescript unsupported-python; do
  src="$scratch/$kind/src"
  dataset="$scratch/$kind/dataset"
  config="$scratch/$kind/setup.yaml"
  mkdir -p "$src"
  printf '# Guide\nA committed guide for %s.\n' "$kind" > "$src/README.md"
  case "$kind" in
    empty-go) printf 'module example.com/empty\n\ngo 1.25\n' > "$src/go.mod" ;;
    typescript)
      printf 'export function greet(): string { return "hello"; }\n' > "$src/main.ts"
      printf 'node_modules/\n' > "$src/.gitignore"
      mkdir -p "$src/web/node_modules"
      printf 'export const stale = "must never be indexed";\n' > "$src/web/node_modules/noise.ts"
      ;;
    unsupported-python) printf 'def greet():\n    return "hello"\n' > "$src/main.py" ;;
  esac
  git -C "$src" init -q
  git -C "$src" add .
  git -C "$src" -c commit.gpgsign=false -c user.name=Codex \
    -c user.email=codex@example.com commit -qm fixture
  "$bin_dir/cks" init --src "$src" --dataset "$dataset" \
    --config-out "$config" --embedder mock > "$scratch/$kind/init.json"
  "$bin_dir/cks" setup --config "$config" --version auto \
    --progress text > "$scratch/$kind/setup.log" 2>&1
  "$bin_dir/cks" doctor --src "$src" --dataset "$dataset" \
    > "$scratch/$kind/doctor.json"
  "$bin_dir/ckv" --embedder mock query "committed guide" \
    --out "$dataset/current/vector" --threshold -1 --json \
    > "$scratch/$kind/query.json" 2> "$scratch/$kind/query.log"
  mcp_config="$scratch/$kind/mcp.yaml"
  "$bin_dir/cks" mcp gen-config --dataset-dir "$dataset/current" \
    --name "ks-$kind" --source-root "$src" --sanitize-rules "$sanitize_rules" \
    --out "$mcp_config" \
    > "$scratch/$kind/mcp-config.log"
  python3 - "$mcp_config" <<'PY'
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
  python3 "$repo_root/scripts/wbs-mcp-pin-probe.py" "$bin_dir/cks" \
    "$mcp_config" --once "Where is the committed guide for $kind?" \
    > "$scratch/$kind/mcp.json"
  python3 "$repo_root/scripts/wbs-mcp-pin-probe.py" "$bin_dir/cks" \
    "$mcp_config" --once "Where is the committed guide for $kind?" \
    > "$scratch/$kind/mcp-restarted.json"
  if [[ "$kind" == typescript ]]; then
    python3 "$repo_root/scripts/wbs-mcp-pin-probe.py" "$bin_dir/cks" \
      "$mcp_config" --once "Where is greet function implemented?" \
      > "$scratch/$kind/mcp-code.json"
  fi
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
    mcp = json.loads((base / 'mcp.json').read_text())
    restarted = json.loads((base / 'mcp-restarted.json').read_text())
    assert Path(init['source_root']).resolve() == (base / 'src').resolve(), init
    assert doctor['dataset_version'] and doctor['commit'], doctor
    assert query['hits'] and all(hit['citation']['commit_hash'] == doctor['commit'] for hit in query['hits']), (kind, query)
    assert mcp['serviceable'] and mcp['commit'] == doctor['commit'], (kind, mcp, doctor)
    assert mcp['citation_commits'] == [doctor['commit']], (kind, mcp)
    assert mcp['citation_files'] and set(mcp['citation_files']) <= {'README.md', 'main.ts'}, (kind, mcp)
    assert restarted == mcp, (kind, mcp, restarted)
    if kind == 'typescript':
        assert doctor['shared_code_files'] == 1 and doctor['status'] == 'ready', doctor
        assert any(hit['citation']['file'] == 'main.ts' for hit in query['hits']), query
        assert all('node_modules' not in hit['citation']['file'] for hit in query['hits']), query
        code = json.loads((base / 'mcp-code.json').read_text())
        assert code['commit'] == doctor['commit'] and code['citation_commits'] == [doctor['commit']], code
        assert 'main.ts' in code['citation_files'], code
    else:
        assert doctor['shared_code_files'] == 0 and doctor['status'] == 'degraded', doctor
        assert all(hit['citation']['file'] == 'README.md' for hit in query['hits']), query
print(f'Installation capability smoke reports: {root}')
PY
