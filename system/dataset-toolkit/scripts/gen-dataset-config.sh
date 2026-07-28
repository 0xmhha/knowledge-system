#!/usr/bin/env bash
# gen-dataset-config.sh — generate the cks runtime config (yaml) for a
# self-contained dataset directory by DELEGATING to `system-mcp gen-config`
# (the single config generator; this script only resolves dataset-toolkit
# paths and passes flags). Re-runnable: moving the repos just means
# re-running this.
#
# Generalized from knowledge-data/pr-{14,58}/gen-pr*-config.sh. One dataset
# dir holds its own graph-db/ + vector-db/ + _src/ + logs/ + cks-<name>.yaml;
# the config is INDEPENDENT (never references another dataset's DB). The
# pre-consolidation graph-db/vector-db layout is passed through gen-config's
# --graph-path/--vector-path overrides.
#
# The env-file half (cks-<NAME>.env) is RETIRED with the cks.env chain:
# register a client from the config instead —
#   system-mcp print-mcp-config --config <dataset>/cks-<NAME>.yaml
#
# Usage:
#   DATASET=/abs/knowledge-data/pr-14 NAME=pr14 ./gen-dataset-config.sh
#
# Optional overrides:
#   SRC_ROOT       source_root for ckg (default: $DATASET/_src)
#   KS_ROOT        knowledge-system checkout (default: via this script's location)
#   EMBED_MODEL    embedding model name (default: bge-m3)
#   CKV_OLLAMA_ENDPOINT  ollama url (default: http://localhost:11434)
#   HTTP_ADDR      listen address (default: 127.0.0.1:8080)
#   DOMAIN_PROJECT_DIR / DOMAIN_CORPUS_DIR / GLOSSARY_PATH
#                  set the first two together to wire a cks domain project
#                  (optional; omit for datasets whose domain knowledge lives
#                  in ckv directly)
set -euo pipefail

abs() { ( cd "$1" 2>/dev/null && pwd ) || { echo "ERROR: path not found: $1" >&2; exit 1; }; }

DATASET="$(abs "${DATASET:?set DATASET=/abs/path/to/dataset/dir}")"
NAME="${NAME:?set NAME=short dataset name (used in filenames, e.g. pr14)}"
SRC_ROOT="$(abs "${SRC_ROOT:-$DATASET/_src}")"

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KS_ROOT="$(abs "${KS_ROOT:-$SCRIPT_DIR/../../..}")"
OLLAMA_URL="${CKV_OLLAMA_ENDPOINT:-http://localhost:11434}"
EMBED_MODEL="${EMBED_MODEL:-bge-m3}"
HTTP_ADDR="${HTTP_ADDR:-127.0.0.1:8080}"

SYSTEM_MCP="$KS_ROOT/bin/system-mcp"
[ -x "$SYSTEM_MCP" ] || {
  echo "ERROR: system-mcp not built ($SYSTEM_MCP) — run: make -C \"$KS_ROOT\" build-mcp" >&2
  exit 1
}

CONFIG="$DATASET/cks-$NAME.yaml"
mkdir -p "$DATASET/logs/footprint" "$DATASET/logs/audit"

# Optional domain wiring: both dirs must be set together (mirrors config
# semantics — the pair enables channel 2); the glossary rides along.
domain_flags=()
if [ -n "${DOMAIN_PROJECT_DIR:-}" ] && [ -n "${DOMAIN_CORPUS_DIR:-}" ]; then
  domain_flags+=(--domain-project-dir "$(abs "$DOMAIN_PROJECT_DIR")"
                 --domain-corpus-dir  "$(abs "$DOMAIN_CORPUS_DIR")")
  [ -n "${GLOSSARY_PATH:-}" ] && domain_flags+=(--glossary "$GLOSSARY_PATH")
fi

"$SYSTEM_MCP" gen-config \
  --out "$CONFIG" \
  --name "$NAME" \
  --description "dataset $NAME ($EMBED_MODEL)" \
  --graph-path  "$DATASET/graph-db/graph.db" \
  --vector-path "$DATASET/vector-db" \
  --source-root "$SRC_ROOT" \
  --graph-binary  "$KS_ROOT/bin/ckg" \
  --vector-binary "$KS_ROOT/bin/ckv" \
  --embed-model "$EMBED_MODEL" \
  --ollama-url  "$OLLAMA_URL" \
  --http-addr   "$HTTP_ADDR" \
  --sanitize-rules "$KS_ROOT/system/policies/sanitization_rules.yaml" \
  --footprint-dir "$DATASET/logs/footprint" \
  --audit-dir     "$DATASET/logs/audit" \
  "${domain_flags[@]+"${domain_flags[@]}"}"

[ -x "$KS_ROOT/bin/ckg" ] || echo "  WARN: ckg not built ($KS_ROOT/bin/ckg) — go build -o bin/ckg ./cmd/graph"
[ -x "$KS_ROOT/bin/ckv" ] || echo "  WARN: ckv not built ($KS_ROOT/bin/ckv) — go build -o bin/ckv ./cmd/vector"
[ -f "$DATASET/graph-db/graph.db" ] || echo "  WARN: graph.db missing ($DATASET/graph-db/graph.db)"
[ -d "$DATASET/vector-db" ]         || echo "  WARN: vector index missing ($DATASET/vector-db)"
echo ""
echo "serve:     \"$SYSTEM_MCP\" -config \"$CONFIG\""
echo "register:  \"$SYSTEM_MCP\" print-mcp-config --config \"$CONFIG\"   # .mcp.json entry (env file retired)"
