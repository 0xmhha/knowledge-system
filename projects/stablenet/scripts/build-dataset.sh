#!/usr/bin/env bash
# build-dataset.sh — build the go-stablenet knowledge dataset via the Go
# pipeline (cmd/knowledge-setup). Thin wrapper: existence checks + flag
# pass-through only; orchestration logic lives in internal/setup (graph
# build → aligned vector build → alignment verify). The pre-consolidation
# three-repo shell orchestration is retired.
#
# Usage:
#   GSN_SRC=/path/to/go-stablenet OUT=/path/to/knowledge-data/stablenet \
#     ./projects/stablenet/scripts/build-dataset.sh [extra knowledge-setup flags]
#
#   SKIP_CKV=1 ...   # graph only (skips the multi-hour ollama embed)
#
# Env:
#   GSN_SRC   go-stablenet checkout (required)
#   OUT       dataset output dir     (required)
#   SKIP_CKV  1 = pass --skip-vector (default 0)
set -euo pipefail

KS_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
: "${GSN_SRC:?set GSN_SRC=/path/to/go-stablenet}"
: "${OUT:?set OUT=/path/to/knowledge-data/<name>}"
SKIP_CKV="${SKIP_CKV:-0}"

# knowledge-setup defaults --graph-bin/--vector-bin to "ckg"/"ckv" on PATH;
# pass the in-repo builds explicitly so no PATH setup is needed.
for b in knowledge-setup ckg ckv; do
  [ -x "$KS_ROOT/bin/$b" ] || {
    echo "ERROR: $KS_ROOT/bin/$b not built — run:" >&2
    echo "  (cd \"$KS_ROOT\" && go build -o bin/ckg ./cmd/graph && go build -o bin/ckv ./cmd/vector && go build -o bin/knowledge-setup ./cmd/knowledge-setup)" >&2
    exit 1
  }
done

extra=()
[ "$SKIP_CKV" = "1" ] && extra+=(--skip-vector)

exec "$KS_ROOT/bin/knowledge-setup" \
  --config "$KS_ROOT/projects/stablenet/setup.yaml" \
  --src "$GSN_SRC" --out "$OUT" \
  --graph-bin "$KS_ROOT/bin/ckg" --vector-bin "$KS_ROOT/bin/ckv" \
  "${extra[@]}" "$@"
