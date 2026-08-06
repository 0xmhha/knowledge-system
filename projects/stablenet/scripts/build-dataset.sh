#!/usr/bin/env bash
# build-dataset.sh — build the go-stablenet knowledge dataset via the Go
# pipeline (`cks setup`). Thin wrapper: existence checks + flag pass-through
# only; orchestration logic lives in internal/setup (graph build → aligned
# vector build → alignment verify).
#
# Usage:
#   GSN_SRC=/path/to/go-stablenet OUT=/path/to/knowledge-data/stablenet \
#     ./projects/stablenet/scripts/build-dataset.sh [extra cks setup flags]
#
#   SKIP_CKV=1 ...   # graph only (skips the multi-hour ollama embed)
#
# Env:
#   GSN_SRC   go-stablenet checkout (required; the tree being indexed)
#   OUT       dataset output dir    (required)
#   SKIP_CKV  1 = pass --skip-vector (default 0)
#   CODE_ROOT tree the pack's code_root resolves against (default: $GSN_SRC)
set -euo pipefail

KS_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
: "${GSN_SRC:?set GSN_SRC=/path/to/go-stablenet}"
: "${OUT:?set OUT=/path/to/knowledge-data/<name>}"
SKIP_CKV="${SKIP_CKV:-0}"
CODE_ROOT="${CODE_ROOT:-$GSN_SRC}"

# cks setup resolves ckg/ckv beside its own binary and self-execs its
# domain/filelist steps, so the three engine binaries are all it needs.
for b in ckg ckv cks; do
  [ -x "$KS_ROOT/bin/$b" ] || {
    echo "ERROR: $KS_ROOT/bin/$b not built — run:" >&2
    echo "  (cd \"$KS_ROOT\" && make build-dataset-bins)" >&2
    exit 1
  }
done

extra=()
[ "$SKIP_CKV" = "1" ] && extra+=(--skip-vector)

exec "$KS_ROOT/bin/cks" setup \
  --config "$KS_ROOT/projects/stablenet/setup.yaml" \
  --src "$GSN_SRC" --out "$OUT" \
  --code-root "$CODE_ROOT" \
  "${extra[@]}" "$@"
