#!/usr/bin/env bash
# setup-all.sh — one-click setup of the go-stablenet knowledge stack on a
# fresh machine: prereqs → Ollama+bge-m3 → binaries → dataset → config →
# jira-gateway → activation instructions.
#
# The three former engine repos are consolidated into this single module;
# only these SIBLINGS of knowledge-system are still expected:
#   <parent>/knowledge-system        (this repo — script lives in system/scripts/)
#   <parent>/go-stablenet
#   <parent>/coding-agent
#   <parent>/chainbench              (optional)
#
# Usage:
#   ./system/scripts/setup-all.sh               # full (includes the long ckv embed)
#   SKIP_CKV=1 ./system/scripts/setup-all.sh    # everything except the multi-hour ckv embed
#   SKIP_OLLAMA=1 ./system/scripts/setup-all.sh # assume Ollama+bge-m3 already present
#
# Idempotent: re-running rebuilds/refreshes each artifact. Safe to interrupt and resume.
set -euo pipefail

CKS_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"   # system/
KS_ROOT="$(cd "$CKS_ROOT/.." && pwd)"                          # module root
cd "$KS_ROOT"
PARENT="$(cd "$KS_ROOT/.." && pwd)"

CODING_AGENT="${CODING_AGENT:-$PARENT/coding-agent}"
GO_STABLENET_ROOT="${GO_STABLENET_ROOT:-$PARENT/go-stablenet}"
DATASET_OUT="${CKS_DATASET_DIR:-$PARENT/knowledge-data/stablenet}"
KS_CONFIG="${KS_CONFIG:-$KS_ROOT/run/cks.yaml}"
EMBED_MODEL="${EMBED_MODEL:-bge-m3}"
OLLAMA_URL="${CKV_OLLAMA_ENDPOINT:-http://localhost:11434}"
SKIP_CKV="${SKIP_CKV:-0}"
SKIP_OLLAMA="${SKIP_OLLAMA:-0}"

log()  { printf '\n\033[1;36m==> %s\033[0m\n' "$*"; }
warn() { printf '\033[1;33m! %s\033[0m\n' "$*"; }
die()  { printf '\033[1;31mERROR: %s\033[0m\n' "$*" >&2; exit 1; }

log "0. prerequisites"
command -v go    >/dev/null || die "Go not found (need 1.23+). https://go.dev/dl/"
command -v cc    >/dev/null || warn "C toolchain (cc) not found — cks links sqlite-vec (CGO). Install Xcode CLT."
command -v git   >/dev/null || die "git not found"
[ -d "$GO_STABLENET_ROOT" ] || die "missing sibling repo: $GO_STABLENET_ROOT"
echo "go: $(go version)"

# ── 1. Ollama (app cask, NOT brew formula) + bge-m3 ─────────────────────────
if [ "$SKIP_OLLAMA" != "1" ]; then
  log "1. Ollama + $EMBED_MODEL"
  if ! command -v ollama >/dev/null 2>&1; then
    command -v brew >/dev/null || die "Homebrew required to install Ollama (or set SKIP_OLLAMA=1 and install manually)"
    warn "Installing Ollama via APP CASK (the brew *formula* lacks llama-server on Apple Silicon)"
    brew install --cask ollama-app
  fi
  curl -fsS "$OLLAMA_URL/api/version" >/dev/null 2>&1 || { warn "starting 'ollama serve' in background"; nohup ollama serve >/tmp/ollama-serve.log 2>&1 & sleep 5; }
  ollama list 2>/dev/null | grep -q "$EMBED_MODEL" || ollama pull "$EMBED_MODEL"
  # sanity: bge-m3 must return 1024-dim
  dim=$(curl -fsS "$OLLAMA_URL/api/embed" -d "{\"model\":\"$EMBED_MODEL\",\"input\":\"x\"}" 2>/dev/null \
        | python3 -c 'import sys,json;e=json.load(sys.stdin).get("embeddings",[[]]);print(len(e[0]) if e and e[0] else 0)' 2>/dev/null || echo 0)
  [ "$dim" = "1024" ] && echo "ollama $EMBED_MODEL OK (dim=1024)" || warn "embedding sanity failed (dim=$dim) — check 'ollama serve'"
else
  log "1. Ollama — SKIPPED (SKIP_OLLAMA=1)"
fi

# ── 2. binaries (single consolidated module) ────────────────────────────────
log "2. binaries (go build + make build-mcp)"
go build -o bin/ckg  ./cmd/graph
go build -o bin/ckv  ./cmd/vector
go build -o bin/knowledge-setup ./cmd/knowledge-setup
make build-mcp

# ── 3. dataset (Go pipeline via the stablenet pack wrapper) ─────────────────
log "3. dataset → $DATASET_OUT (knowledge-setup)"
GSN_SRC="$GO_STABLENET_ROOT" OUT="$DATASET_OUT" SKIP_CKV="$SKIP_CKV" \
  "$KS_ROOT/projects/stablenet/scripts/build-dataset.sh"

# ── 4. config (cks mcp gen-config; replaces the retired gen-cks-config.sh) ──
log "4. config → $KS_CONFIG (cks mcp gen-config)"
mkdir -p "$(dirname "$KS_CONFIG")"
"$KS_ROOT/bin/cks" mcp gen-config --out "$KS_CONFIG" \
  --dataset-dir "$DATASET_OUT" --source-root "$GO_STABLENET_ROOT" \
  --graph-binary "$KS_ROOT/bin/ckg" --vector-binary "$KS_ROOT/bin/ckv" \
  --embed-model "$EMBED_MODEL" --ollama-url "$OLLAMA_URL"
"$KS_ROOT/bin/cks" mcp client-config --config "$KS_CONFIG"

# ── 5. jira-gateway (coding-agent in-tree MCP) ──────────────────────────────
if [ -d "$CODING_AGENT/tools/jira-gateway-mcp" ]; then
  log "5. jira-gateway build"
  ( cd "$CODING_AGENT/tools/jira-gateway-mcp" && go build -o bin/jira-gateway-mcp ./cmd/server )
  echo "built: $CODING_AGENT/tools/jira-gateway-mcp/bin/jira-gateway-mcp"
else
  warn "5. jira-gateway — coding-agent repo not found at $CODING_AGENT (skipping)"
fi

# ── 6. jira.env scaffold (secret, outside any repo) ─────────────────────────
JENV="$HOME/.config/coding-agent/jira.env"
if [ ! -f "$JENV" ]; then
  log "6. jira.env scaffold"
  mkdir -p "$(dirname "$JENV")"; ( umask 077; cat > "$JENV" <<'EOF'
# jira-gateway credentials (SECRET — never commit). chmod 600.
# JIRA_BASE_URL: your Atlassian site. JIRA_API_TOKEN: id.atlassian.com/manage-profile/security/api-tokens
export JIRA_BASE_URL="https://CHANGE-ME.atlassian.net"
export JIRA_USER_EMAIL="CHANGE-ME@example.com"
export JIRA_API_TOKEN="CHANGE-ME"
EOF
)
  chmod 600 "$JENV"; echo "created $JENV (fill in before using jira_* tools)"
else
  echo "6. jira.env exists ($JENV) — left as-is"
fi

# ── 7. Claude Code settings.json env (launch-method-independent) ────────────
log "7. apply env to ~/.claude/settings.json (so MCP works from GUI or terminal)"
"$CKS_ROOT/scripts/apply-cc-settings.sh" || warn "apply-cc-settings failed (run it manually)"

# ── 8. autonomous (no-prompt) execution for the go-stablenet project ────────
log "8. enable autopilot (bypassPermissions) for go-stablenet"
GO_STABLENET_ROOT="$GO_STABLENET_ROOT" "$CKS_ROOT/scripts/enable-autopilot.sh" \
  || warn "enable-autopilot failed (run it manually)"

# ── done ────────────────────────────────────────────────────────────────────
log "SETUP COMPLETE — next steps"
cat <<EOF
  1) Install the plugin (in Claude Code):
       /plugin marketplace add 0xmhha/coding-agent
       /plugin install coding-agent@coding-agent
  2) Launch the autonomous pipeline via the launcher (ensures bypassPermissions
     is in place, then opens Claude Code in go-stablenet):
       "$CKS_ROOT/scripts/coding-agent.sh"                      # interactive
       "$CKS_ROOT/scripts/coding-agent.sh" /coding-agent:work STABLE-1234
     (MCP env is already global via ~/.claude/settings.json — no per-launch source.)
  3) Verify cks health any time (cks-health.sh is retired):
       - MCP tool: cks.ops.health           # via any connected client
       - HTTP:     GET <CKS_MCP_URL%/mcp>/healthz   # liveness probe
  4) (optional) Jira token: edit $JENV, then re-run system/scripts/apply-cc-settings.sh
EOF
