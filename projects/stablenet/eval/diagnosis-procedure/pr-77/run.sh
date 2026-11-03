#!/usr/bin/env bash
# Run /core-dev:diagnose on the PR-77 problem text for one arm.
# usage: run.sh A0|A1   (the arm's cks instance must already listen on its port, see cks-<arm>.yaml)
set -euo pipefail
arm=$1
M=$(cd "$(dirname "$0")" && pwd)
case $arm in A0) port=18777 ;; A1) port=18778 ;; *) echo "arm must be A0 or A1" >&2; exit 2 ;; esac
problem=$(cat "$M/problem.txt")
cd "$M/work/$arm"
start=$(date -u +%s)
STABLENET_KNOWLEDGE_MCP_URL="http://127.0.0.1:$port/mcp" claude -p "/core-dev:diagnose \"$problem\"" \
  --restricted --tools "Read,Write,Edit,Bash,Grep,Glob,Agent,Skill,ToolSearch" \
  --plugin-dir "$M/plugins/$arm/plugins/core-dev" \
  --permission-mode acceptEdits --permission-prompts none \
  --allowedTools "mcp__plugin_core-dev_stablenet-knowledge__*" \
    "Bash(git rev-parse:*)" "Bash(git log:*)" "Bash(git show:*)" "Bash(git diff:*)" "Bash(git blame:*)" \
    "Bash(git status:*)" "Bash(go test:*)" "Bash(go build:*)" "Bash(go vet:*)" "Bash(go doc:*)" \
    "Bash(mkdir:*)" "Bash(date:*)" "Bash(ls:*)" "Bash(cat:*)" "Bash(grep:*)" "Bash(rg:*)" "Bash(find:*)" \
    "Bash(sed -n:*)" "Bash(head:*)" "Bash(tail:*)" "Bash(wc:*)" \
  --max-budget-usd 30 --output-format stream-json --verbose \
  > "$M/runs/$arm/stream.jsonl" 2> "$M/runs/$arm/stderr.log"
echo "wall_seconds=$(( $(date -u +%s) - start ))" > "$M/runs/$arm/wall.txt"
cp .stablenet-expert/diagnoses/*/diagnosis.md "$M/runs/$arm/diagnosis.md" 2>/dev/null || echo "no diagnosis.md" >&2
