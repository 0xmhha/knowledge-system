#!/usr/bin/env bash
# Verify strict mode indexes a full passage and rejects a truly oversized one.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
scratch="${KS_B0_STRICT_SMOKE_DIR:-$(mktemp -d)}"
src="$scratch/src"
dataset="$scratch/dataset"
accepted_dataset="$scratch/accepted-dataset"
mkdir -p "$src"
cat > "$src/go.mod" <<'EOF'
module example.com/ks-b0-strict-smoke

go 1.25
EOF
cat > "$src/main.go" <<'EOF'
package smoke

func Alpha() string { return "alpha" }
EOF
printf '# Long source excerpt\n\n' > "$src/README.md"
git -C "$repo_root" show 71cb71cd55960833e930269e272f7a4a060be3aa:cmd/cks/knowledgecli/knowledge.go \
  >> "$src/README.md"
git -C "$src" init -q
git -C "$src" add .
git -C "$src" -c commit.gpgsign=false -c user.name=Codex \
  -c user.email=codex@example.com commit -qm fixture

CKV_REQUIRE_COMPLETE_EMBEDDINGS=1 "$repo_root/bin/cks" setup \
  --src "$src" --out "$accepted_dataset" --embedder ollama --model-name bge-m3 \
  --ollama-url http://127.0.0.1:11434 --version strict-accepted --progress text \
  > "$scratch/accepted-setup.log" 2>&1
python3 - "$accepted_dataset" <<'PY'
import json
from pathlib import Path
import sys
dataset = Path(sys.argv[1])
assert (dataset / 'current').exists(), 'accepted dataset was not published'
manifest = json.loads((dataset / 'strict-accepted/vector/manifest.json').read_text())
identity = manifest['embedding_identity_v2']
assert identity['runtime_context_tokens'] == 8192, identity
assert identity['runtime_batch_tokens'] == 8192, identity
PY

# Two copies exceed the 8192-token BGE-M3 context while remaining a single
# Markdown chunk under CKV's 32768-byte conservative source cap.
git -C "$repo_root" show 71cb71cd55960833e930269e272f7a4a060be3aa:cmd/cks/knowledgecli/knowledge.go \
  >> "$src/README.md"
git -C "$src" add README.md
git -C "$src" -c commit.gpgsign=false -c user.name=Codex \
  -c user.email=codex@example.com commit -qm oversized-fixture

if CKV_REQUIRE_COMPLETE_EMBEDDINGS=1 "$repo_root/bin/cks" setup \
    --src "$src" --out "$dataset" --embedder ollama --model-name bge-m3 \
    --ollama-url http://127.0.0.1:11434 --version strict-smoke --progress text \
    > "$scratch/setup.log" 2>&1; then
  echo "strict setup unexpectedly published an oversized embedding input" >&2
  exit 1
fi
if ! grep -q 'embedding input incomplete' "$scratch/setup.log"; then
  echo "strict setup failed for a reason other than incomplete embedding input" >&2
  exit 1
fi
if [[ -e "$dataset/current" || -e "$dataset/strict-smoke/vector/manifest.json" ]]; then
  echo "strict setup exposed a rejected candidate as serviceable" >&2
  exit 1
fi
printf '{"status":"strict_accepted_full_then_rejected_oversized","quality_metrics":null,"artifacts":"%s"}\n' "$scratch"
