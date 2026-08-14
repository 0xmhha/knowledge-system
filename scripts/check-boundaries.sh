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
check cmd/vector       "graph|system"
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

# A pack's NAME must not reach a string a user can see either. The rule above
# catches its path; this catches the name itself in production string literals
# — a launchd label, a flag's example, an instruction sent to an agent. Those
# make a generalized build claim to be one project's tool, and they are how
# branding leaked in before: a distribution's label prefix compiled and linted
# clean while saying the wrong thing at runtime.
#
# Scoped deliberately:
#   - string literals only. Doc comments may cite the reference pack as an
#     example, which the README sanctions.
#   - non-test files only. Fixtures naming a pack are noise, not a claim.
#   - the pack list comes from projects/, so it grows with the packs.
#
# Exceptions are listed rather than silenced, each with the reason it is not
# simply a leak:
#   pkg/mcp/namespace.go   documents the branding mechanism itself; the example
#                          has to name some distribution to be an example.
#   cmd/cks/domaincli/worksheet.go
#                          its promotion catalog is pack-level domain knowledge
#                          living in engine code — a real violation, but one
#                          that needs the catalog extracted into the pack
#                          rather than the name filed off. See
#                          docs/dev/2026-08-14-pack-knowledge-in-engine-code.md.
name_exempt='pkg/mcp/namespace.go|cmd/cks/domaincli/worksheet.go'
for pack in $(ls -1 projects 2>/dev/null); do
  [ -d "projects/$pack" ] || continue
  # A quoted example inside a comment is still a comment, so drop lines whose
  # content begins with //; only code carries a string a user can see.
  #
  # Import paths are dropped too. A distribution may carry the pack's name in
  # its own module path, which would make every import of its own packages a
  # match — the repository naming itself is not the leak this looks for.
  name_hits=$(grep -rn --include='*.go' -iE "\"[^\"]*${pack}[^\"]*\"" cmd internal graph vector system pkg 2>/dev/null \
    | grep -v '_test\.go:' | grep -vE "^[^:]+:[0-9]+:[[:space:]]*//" \
    | grep -vF "\"$MOD/" | grep -vE "^[^:]+:[0-9]+:[[:space:]]*[a-z_]* ?\"$MOD\"$" \
    | grep -vE "^($name_exempt):" || true)
  if [ -n "$name_hits" ]; then
    echo "boundary violation — a user-visible string in engine/shared code names the '$pack' pack:"
    echo "$name_hits"
    fail=1
  fi
done

if [ "$fail" -eq 0 ]; then
  echo "engine boundaries: OK"
fi
exit $fail
