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
  "$repo_root/testdata/wbs-smoke/README.md" "$src/"
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
  --out "$scratch/projection.json" --docs README.md \
  --min-canonical-ratio 0.4 --activate \
  > "$scratch/semantic-build.json"
"$repo_root/bin/cks" semantic review --input "$scratch/projection.json" \
  --repo "$src" --sample 10 > "$scratch/semantic-review.json"
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
"$repo_root/bin/cks" eval --scenarios "$repo_root/testdata/wbs-smoke/find-alpha.yaml" \
  --config "$config" --verify-anchors "$src" --output "$scratch/find-alpha-report.json"
"$repo_root/bin/cks" eval --scenarios "$repo_root/testdata/wbs-smoke/absent-api.yaml" \
  --config "$config" --output "$scratch/absent-api-report.json"
python3 - "$scratch/find-alpha-report.json" "$scratch/absent-api-report.json" "$scratch/semantic-build.json" "$scratch/semantic-review.json" <<'PY'
import json
import sys
found = json.load(open(sys.argv[1], encoding='utf-8'))['results'][0]
absent = json.load(open(sys.argv[2], encoding='utf-8'))['results'][0]
assert found['metrics']['file_recall'] == 1 and found['metrics']['citation_count'] > 0, found
assert absent['citation_abstention_passed'] is True, absent
semantic = json.load(open(sys.argv[3], encoding='utf-8'))
review = json.load(open(sys.argv[4], encoding='utf-8'))
assert semantic['activated'] is True and semantic['sections'] > 0 and semantic['evidence'] > 0, semantic
assert review['project_id'] == 'ks-fixture' and review['reviewed_precision'] is None, review
PY
printf 'Structural smoke reports: %s\n' "$scratch"
