#!/usr/bin/env bash
# activate.sh — load cks + jira env into the CURRENT shell ON-DEMAND.
#
#   source ./activate.sh        # (or:  . ./activate.sh)
#   claude                      # launch Claude Code from THIS shell so the
#                               # coding-agent plugin's MCP servers inherit the env
#
# No ~/.zshrc changes: nothing here persists beyond this shell session. Run it
# only when you actually want to use the coding-agent plugin against go-stablenet.
#
# What it sets:
#   CKS_MCP_BIN / CKS_MCP_URL / GO_STABLENET_ROOT / CKV_OLLAMA_ENDPOINT
#     — derived from the cks config YAML via `cks mcp client-config`
#       (the cks.env file is retired; the config is the single source of truth)
#   JIRA_BASE_URL / JIRA_USER_EMAIL / JIRA_API_TOKEN  (from ~/.config/coding-agent/jira.env, if present)

# Resolve this file's dir whether sourced from bash or zsh.
_act_src="${BASH_SOURCE[0]:-${(%):-%N}}"; _act_src="${_act_src:-$0}"
_CKS_HERE="$(cd "$(dirname "$_act_src")" && pwd)"
_KS_ROOT="$(cd "$_CKS_HERE/.." && pwd)"

# Config = gen-config output (machine-local, gitignored under run/).
_CKS_BIN="$_KS_ROOT/bin/cks"
_KS_CONFIG="${KS_CONFIG:-$_KS_ROOT/run/cks.yaml}"

if [ ! -x "$_CKS_BIN" ]; then
  echo "activate: cks not built — run: make -C \"$_KS_ROOT\" build-mcp" >&2
  return 1 2>/dev/null || exit 1
fi
if [ ! -f "$_KS_CONFIG" ]; then
  echo "activate: config missing ($_KS_CONFIG) — generate it first, e.g.:" >&2
  echo "  \"$_CKS_BIN\" mcp gen-config --out \"$_KS_CONFIG\" --dataset-dir <dataset> [--lan]" >&2
  return 1 2>/dev/null || exit 1
fi

# Derive the plugin's ${VAR} placeholders from client-config (URL) and the
# config itself (source_root / ollama endpoint). jq is required for the URL.
if ! command -v jq >/dev/null 2>&1; then
  echo "activate: jq not found — install jq (brew install jq)" >&2
  return 1 2>/dev/null || exit 1
fi
export CKS_MCP_BIN="$_CKS_BIN"
export CKS_MCP_URL="$("$_CKS_BIN" mcp client-config --config "$_KS_CONFIG" | jq -r '.mcpServers[].url // empty')"
# source_root / ollama_url live in the config YAML; keep the exports only for
# consumers that still resolve ${VAR} placeholders (coding-agent .mcp.json).
export GO_STABLENET_ROOT="$(awk '/^  ckg:/{f=1} f&&/source_root:/{gsub(/"/,"",$2); print $2; exit}' "$_KS_CONFIG")"
export CKV_OLLAMA_ENDPOINT="$(awk '/ollama_url:/{gsub(/"/,"",$2); print $2; exit}' "$_KS_CONFIG")"
echo "activate: cks env derived from $_KS_CONFIG (CKS_MCP_URL=${CKS_MCP_URL:-stdio})"

# jira-gateway binary (resolved from the coding-agent sibling repo; .mcp.json reads ${JIRA_GATEWAY_BIN}).
_CODING_AGENT="${CODING_AGENT:-$(cd "$_CKS_HERE/../coding-agent" 2>/dev/null && pwd)}"
if [ -n "$_CODING_AGENT" ]; then
  export JIRA_GATEWAY_BIN="$_CODING_AGENT/tools/jira-gateway-mcp/bin/jira-gateway-mcp"
  [ -x "$JIRA_GATEWAY_BIN" ] && echo "activate: jira-gateway bin OK" \
    || echo "activate: jira-gateway NOT built — ( cd $_CODING_AGENT/tools/jira-gateway-mcp && go build -o bin/jira-gateway-mcp ./cmd/server )"
fi

# chainbench (optional MCP). Installed by chainbench install.sh to ~/.chainbench;
# `chainbench-mcp` resolves via ~/.local/bin on PATH. .mcp.json reads ${CHAINBENCH_DIR}.
if [ -d "$HOME/.chainbench" ]; then
  export CHAINBENCH_DIR="$HOME/.chainbench"
  command -v chainbench-mcp >/dev/null 2>&1 && echo "activate: chainbench-mcp OK" \
    || echo "activate: chainbench-mcp not on PATH (symlink ~/.local/bin/chainbench-mcp → \$CHAINBENCH_DIR/bin/chainbench-mcp)"
fi

# jira-gateway secret env (optional).
_JIRA_ENV="$HOME/.config/coding-agent/jira.env"
if [ -f "$_JIRA_ENV" ]; then
  # shellcheck disable=SC1090
  source "$_JIRA_ENV"
  case "$JIRA_BASE_URL" in
    *CHANGE-ME*|"") echo "activate: jira env loaded but NOT configured (edit $_JIRA_ENV)";;
    *)              echo "activate: jira env loaded ($JIRA_BASE_URL)";;
  esac
else
  echo "activate: jira env absent (optional) — $_JIRA_ENV"
fi

# Friendly nudge: is Ollama up? cks ckv leg needs it for non-degraded health.
if command -v curl >/dev/null 2>&1; then
  curl -fsS "${CKV_OLLAMA_ENDPOINT:-http://localhost:11434}/api/version" >/dev/null 2>&1 \
    && echo "activate: ollama reachable" \
    || echo "activate: WARNING ollama not reachable at ${CKV_OLLAMA_ENDPOINT:-http://localhost:11434} (cks will be degraded). Start: ollama serve"
fi

unset _act_src _CKS_HERE _KS_ROOT _CKS_BIN _KS_CONFIG _JIRA_ENV _CODING_AGENT
