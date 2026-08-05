#!/usr/bin/env bash
#
# build-vector-stablenet.sh — reproducible CKV vector index for the go-stablenet
# (pr-77-gstable) dataset, with the knowledge bridge layers pinned to their
# single tracked source home.
#
# Why this exists: a bare `ckv build` repeatedly dropped a bridge layer (docs /
# flow-corpus), producing inconsistent search quality. This pins every input so
# the same dataset in -> the same vector.db out, no re-derivation.
#
# Single source of truth (after the repo consolidation):
#   - Flow corpus (steps/invariants/edges) lives ONLY at:
#       projects/stablenet/domain-knowledge/flow-corpus/
#   - Domain corpus markdown is a REGENERATED artifact of the tracked YAML
#     (projects/stablenet/domain-knowledge/entries/*.yaml) via the
#     cks-domain-export tool; we embed the generated/ cache (gitignored under
#     projects/*/generated/). No make target regenerates it — unlike graph.yaml
#     and glossary.yaml, which `make sync-domain-artifacts` owns — so run it by
#     hand first if the YAML changed:
#       make build          # builds bin/cks-domain-export
#       bin/cks-domain-export --project projects/stablenet/domain-knowledge \
#         --out projects/stablenet/generated/domain-corpus \
#         --code-root /abs/path/to/go-stablenet
#     --code-root is REQUIRED: project.yaml's authoritative_docs are CLAUDE.md
#     and .claude/docs/*.md, which live in the go-stablenet checkout, not here.
#     Without it the tool fails closed (exit 1, "7 of 7 authoritative_docs could
#     not be resolved") after writing the entries but no docs. Use
#     --allow-missing-docs only when shipping a corpus without them is intended.
#
# Usage:
#   scripts/build-vector-stablenet.sh            # build
#   DRY_RUN=1 scripts/build-vector-stablenet.sh  # print the command + check paths only
#
# Env overrides (all optional; defaults target this workspace layout):
#   WORKSPACE, SRC, DATASET, POLICY, DOMAIN_DOCS, FLOW_DOCS, FLOW_CORPUS,
#   LANGS, EMBEDDER, MODEL, OLLAMA, CKV_BIN, DRY_RUN
#
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
KS_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"                 # knowledge-system module root
WORKSPACE="${WORKSPACE:-$(cd "$KS_ROOT/.." && pwd)}"       # sibling checkouts live here

CKV_BIN="${CKV_BIN:-$KS_ROOT/bin/ckv}"                     # go build -o bin/ckv ./cmd/vector
SRC="${SRC:-$WORKSPACE/go-stablenet/pr/pr-77-problem}"
DATASET="${DATASET:-$WORKSPACE/knowledge-data/pr-77-gstable}"   # holds graph-db/, vector-db/, files.json
POLICY="${POLICY:-$KS_ROOT/projects/stablenet/policies/vector.yaml}"

# Bridge-layer sources (single tracked home + regenerable domain cache),
# all inside the stablenet project pack since the consolidation.
FLOW_DOCS="${FLOW_DOCS:-$KS_ROOT/projects/stablenet/domain-knowledge/flow-corpus}"
FLOW_CORPUS="${FLOW_CORPUS:-$FLOW_DOCS/corpus.jsonl}"
DOMAIN_DOCS="${DOMAIN_DOCS:-$KS_ROOT/projects/stablenet/generated/domain-corpus/entries}"

LANGS="${LANGS:-go,solidity}"
EMBEDDER="${EMBEDDER:-ollama}"
MODEL="${MODEL:-bge-m3}"
OLLAMA="${OLLAMA:-http://127.0.0.1:11434}"
DRY_RUN="${DRY_RUN:-0}"

GRAPH_DB="$DATASET/graph-db"
FILTER="$DATASET/files.json"
OUT="$DATASET/vector-db"

# ---- preflight: every pinned input must exist -------------------------------
fail=0
check() { [ -e "$2" ] || { echo "MISSING $1: $2" >&2; fail=1; }; }
check "ckv binary"   "$CKV_BIN"
check "src"          "$SRC"
check "graph-db"     "$GRAPH_DB"
check "files-from"   "$FILTER"
check "policy"       "$POLICY"
check "domain docs"  "$DOMAIN_DOCS"
check "flow docs"    "$FLOW_DOCS"
check "flow corpus"  "$FLOW_CORPUS"
[ "$fail" -eq 0 ] || { echo "preflight failed" >&2; exit 1; }

args=(build
  --src="$SRC"
  --ckg="$GRAPH_DB"
  --files-from="$FILTER"
  --lang="$LANGS"
  --policy="$POLICY"
  --docs="$DOMAIN_DOCS"
  --docs="$FLOW_DOCS"
  --flow-corpus="$FLOW_CORPUS"
  --out="$OUT"
  --embedder="$EMBEDDER"
  --model-name="$MODEL"
)

echo "== ckv $("$CKV_BIN" version 2>/dev/null || echo '?') =="
echo "+ CKV_OLLAMA_ENDPOINT=$OLLAMA $CKV_BIN ${args[*]}"
if [ "$DRY_RUN" = "1" ]; then
  echo "(DRY_RUN: preflight OK, not building)"
  exit 0
fi

CKV_OLLAMA_ENDPOINT="$OLLAMA" "$CKV_BIN" "${args[@]}"

# ---- post-build summary: prove the bridge layers landed ---------------------
DB="$OUT/vector.db"
if command -v sqlite3 >/dev/null 2>&1 && [ -f "$DB" ]; then
  echo ""
  echo "== summary: $OUT =="
  sqlite3 "$DB" "SELECT chunk_kind, COUNT(*) FROM chunks GROUP BY chunk_kind ORDER BY 2 DESC;"
  echo "  (expect doc/flow_step/flow_spine > 0 — the bridge layers)"
fi
