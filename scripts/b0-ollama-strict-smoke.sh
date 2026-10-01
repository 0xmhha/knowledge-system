#!/usr/bin/env bash
# Verify a real Ollama context rejection cannot publish an incomplete CKV index.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
scratch="${KS_B0_STRICT_SMOKE_DIR:-$(mktemp -d)}"
src="$scratch/src"
dataset="$scratch/dataset"
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
printf '{"status":"strict_rejected_partial_input","quality_metrics":null,"artifacts":"%s"}\n' "$scratch"
