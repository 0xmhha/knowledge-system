#!/usr/bin/env bash
# Engine-boundary check.
#
# Engine internals live at internal/<engine> so that root-level commands can
# import them (Go's internal-visibility rule scopes internal/ to its parent
# directory). That relocation means the compiler no longer enforces isolation
# BETWEEN engines — this script does, as part of `make lint`:
#
#   - an engine's code (internal/<e>, <e>/, cmd/<e>) may import its own
#     internals and any root pkg/;
#   - it must NOT import another engine's internal/<other> packages;
#   - cross-engine consumption goes through the public pkg/ surfaces only.
set -euo pipefail
cd "$(dirname "$0")/.."

MOD="github.com/0xmhha/knowledge-system"
fail=0

check() { # <dir> <forbidden-engines-regex>
  local dir=$1 pat=$2 hits
  [ -d "$dir" ] || return 0
  hits=$(grep -rn --include='*.go' -E "\"$MOD/internal/($pat)/" "$dir" || true)
  if [ -n "$hits" ]; then
    echo "boundary violation — $dir must not import internal/{$pat}:"
    echo "$hits"
    fail=1
  fi
}

# internal/setup orchestrates the engines strictly through their CLIs
# (subprocess) and manifest files — it must not import any engine internals.
check internal/setup   "graph|vector|system"
check internal/graph   "vector|system"
check internal/vector  "graph|system"
check internal/system  "graph|vector"
check graph            "vector|system"
check vector           "graph|system"
check system           "graph|vector"
check cmd/graph        "vector|system"
check cmd/graph-mcp    "vector|system"
check cmd/vector       "graph|system"
check cmd/vector-mcp   "graph|system"
check cmd/cks          "graph|vector"
# The filelist derivation is engine-free by design (R8): go list/git
# subprocesses only. It lives inside the cks tree but keeps the stronger
# guarantee; same for the eval-gate comparator, which reads JSON files.
check cmd/cks/filelistcli "graph|vector|system"
check cmd/cks/evalgatecli "graph|vector|system"

# Engines and shared code must not hardcode a specific project pack in
# string literals — pack data reaches the engines through flags/config only.
# (The generic "projects/" marker string is allowed; "projects/<name>" is not.)
pack_hits=$(grep -rn --include='*.go' -E '"projects/[a-z]' cmd internal graph vector system pkg 2>/dev/null || true)
if [ -n "$pack_hits" ]; then
  echo "boundary violation — engine/shared code references a specific project pack:"
  echo "$pack_hits"
  fail=1
fi

if [ "$fail" -eq 0 ]; then
  echo "engine boundaries: OK"
fi
exit $fail
