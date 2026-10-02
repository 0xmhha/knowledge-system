#!/usr/bin/env bash
# Verify strict mode preserves long passages through bounded child chunks.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
scratch="${KS_B0_STRICT_SMOKE_DIR:-$(mktemp -d)}"
src="$scratch/src"
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
assert identity['chunk_budget_bytes'] == 6144, identity
PY

# Two copies exceed the BGE-M3 context as one input. The chunker must retain
# both copies as independently embeddable source windows.
git -C "$repo_root" show 71cb71cd55960833e930269e272f7a4a060be3aa:cmd/cks/knowledgecli/knowledge.go \
  >> "$src/README.md"
git -C "$src" add README.md
git -C "$src" -c commit.gpgsign=false -c user.name=Codex \
  -c user.email=codex@example.com commit -qm oversized-fixture

CKV_REQUIRE_COMPLETE_EMBEDDINGS=1 "$repo_root/bin/cks" setup \
  --src "$src" --out "$scratch/dataset" --embedder ollama --model-name bge-m3 \
  --ollama-url http://127.0.0.1:11434 --version strict-smoke --progress text \
  > "$scratch/setup.log" 2>&1
python3 - "$scratch/dataset" <<'PY'
import json
from pathlib import Path
import sys
dataset = Path(sys.argv[1])
assert (dataset / 'current').exists(), 'long passage was not published'
manifest = json.loads((dataset / 'strict-smoke/vector/manifest.json').read_text())
assert manifest['embedding_identity_v2']['chunk_budget_bytes'] == 6144
assert manifest['chunk_count'] > 4, manifest['chunk_count']
PY
printf '{"status":"strict_accepted_both_long_passages","quality_metrics":null,"artifacts":"%s"}\n' "$scratch"
