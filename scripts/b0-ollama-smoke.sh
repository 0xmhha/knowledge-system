#!/usr/bin/env bash
# Small real-Ollama integration probe. This does not score B0 gold answers.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
scratch="${KS_B0_SMOKE_DIR:-$(mktemp -d)}"
model="${KS_B0_MODEL:-bge-m3}"
endpoint="${KS_B0_OLLAMA_URL:-http://127.0.0.1:11434}"
src="$scratch/src"
dataset="$scratch/dataset"
mkdir -p "$src"
python3 "$repo_root/scripts/b0-preflight.py" --model "$model" \
  --ollama-url "$endpoint" --output "$scratch/preflight.json"
cat > "$src/go.mod" <<'EOF'
module example.com/ks-b0-ollama-smoke

go 1.25
EOF
cat > "$src/main.go" <<'EOF'
package smoke

// Alpha returns the documented marker.
func Alpha() string { return "alpha-marker" }
EOF
cat > "$src/README.md" <<'EOF'
# Alpha behavior

The Alpha function in main.go returns the literal alpha-marker.
EOF
git -C "$src" init -q
git -C "$src" add .
git -C "$src" -c commit.gpgsign=false -c user.name=Codex \
  -c user.email=codex@example.com commit -qm fixture
commit="$(git -C "$src" rev-parse HEAD)"

"$repo_root/bin/cks" setup --src "$src" --out "$dataset" \
  --embedder ollama --model-name "$model" --ollama-url "$endpoint" \
  --version real-ollama-smoke --progress text > "$scratch/setup.log" 2>&1
"$repo_root/bin/cks" doctor --src "$src" --dataset "$dataset" \
  > "$scratch/doctor.json"
CKV_OLLAMA_ENDPOINT="$endpoint" "$repo_root/bin/ckv" --embedder ollama \
  --model-name "$model" query 'Where is Alpha function implemented?' \
  --out "$dataset/current/vector" --threshold -1 --json \
  > "$scratch/query.json" 2> "$scratch/query.log"
"$repo_root/bin/cks" mcp gen-config --dataset-dir "$dataset/current" \
  --name ks-b0-ollama-smoke --source-root "$src" \
  --sanitize-rules "$repo_root/system/policies/sanitization_rules.yaml" \
  --embed-model "$model" --ollama-url "$endpoint" --out "$scratch/mcp.yaml" \
  > "$scratch/mcp-config.log"
python3 - "$scratch/mcp.yaml" <<'PY'
from pathlib import Path
import sys
p = Path(sys.argv[1])
s = p.read_text()
for before, after in (
    ('provider: ""', 'provider: ollama'),
    ('mcp_stdio: false', 'mcp_stdio: true'),
    ('transport: http', 'transport: stdio'),
):
    if before not in s:
        raise SystemExit(f"missing generated config field: {before}")
    s = s.replace(before, after)
p.write_text(s)
PY
python3 "$repo_root/scripts/wbs-mcp-pin-probe.py" "$repo_root/bin/cks" \
  "$scratch/mcp.yaml" --once 'Where is Alpha function implemented?' \
  > "$scratch/mcp.json"
python3 - "$scratch" "$commit" <<'PY'
import json
from pathlib import Path
import sys
root, commit = Path(sys.argv[1]), sys.argv[2]
doctor = json.loads((root / 'doctor.json').read_text())
query = json.loads((root / 'query.json').read_text())
mcp = json.loads((root / 'mcp.json').read_text())
preflight = json.loads((root / 'preflight.json').read_text())
manifest = json.loads((root / 'dataset/current/vector/manifest.json').read_text())
model = preflight['model']
identity = manifest['embedding_identity_v2']
assert model and identity['Model'] == model['model'], (model, identity)
assert identity['model_digest'] == model['digest'] and identity['Dim'] == model['dimension'], (model, identity)
assert doctor['commit'] == commit and doctor['status'] == 'ready', doctor
assert query['hits'] and all(hit['citation']['commit_hash'] == commit for hit in query['hits']), query
assert mcp['serviceable'] and mcp['commit'] == commit, mcp
assert mcp['citation_commits'] and mcp['citation_commits'] == [commit], mcp
print(json.dumps({'status': 'integration_smoke_passed', 'commit': commit,
                  'dataset_version': doctor['dataset_version'],
                  'model': model['model'], 'model_digest': model['digest'],
                  'ckv_hit_count': len(query['hits']),
                  'cks_citation_files': mcp['citation_files'],
                  'quality_metrics': None, 'artifacts': str(root)}, indent=2))
PY
