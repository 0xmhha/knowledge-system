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
#   GSN_SRC   go-stablenet checkout (required; also the code_root the
#             project's authoritative_docs are copied from)
#   OUT       dataset output dir     (required)
#   SKIP_CKV  1 = pass --skip-vector (default 0)
#   CODE_ROOT checkout holding the authoritative_docs (default: $GSN_SRC)
set -euo pipefail

KS_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
: "${GSN_SRC:?set GSN_SRC=/path/to/go-stablenet}"
: "${OUT:?set OUT=/path/to/knowledge-data/<name>}"
SKIP_CKV="${SKIP_CKV:-0}"
# The project's authoritative_docs (CLAUDE.md, .claude/docs/*) are copied from
# the checkout, not the pack. Normally that is the tree being indexed; set
# CODE_ROOT when indexing a copy that was stripped of them.
CODE_ROOT="${CODE_ROOT:-$GSN_SRC}"

# knowledge-setup defaults --graph-bin/--vector-bin to "ckg"/"ckv" on PATH;
# pass the in-repo builds explicitly so no PATH setup is needed.
for b in knowledge-setup ckg ckv filelist-gen cks-domain-export cks-domain-sync cks-glossary-gen; do
  [ -x "$KS_ROOT/bin/$b" ] || {
    echo "ERROR: $KS_ROOT/bin/$b not built — run:" >&2
    echo "  (cd \"$KS_ROOT\" && make build-dataset-bins)" >&2
    exit 1
  }
done

extra=()
[ "$SKIP_CKV" = "1" ] && extra+=(--skip-vector)

exec "$KS_ROOT/bin/knowledge-setup" \
  --config "$KS_ROOT/projects/stablenet/setup.yaml" \
  --src "$GSN_SRC" --out "$OUT" \
  --graph-bin "$KS_ROOT/bin/ckg" --vector-bin "$KS_ROOT/bin/ckv" \
  --filelist-bin "$KS_ROOT/bin/filelist-gen" \
  --domain-export-bin "$KS_ROOT/bin/cks-domain-export" \
  --domain-sync-bin "$KS_ROOT/bin/cks-domain-sync" \
  --glossary-gen-bin "$KS_ROOT/bin/cks-glossary-gen" \
  --code-root "$CODE_ROOT" \
  "${extra[@]}" "$@"
