#!/usr/bin/env bash
# Start the (case, arm) cks instance, run /core-dev:diagnose on the case's problem text, stop the instance.
# usage: run.sh <case> <arm> <port>
set -euo pipefail
case=$1 arm=$2 port=$3
A=${AUTO_CODING:?set AUTO_CODING to the auto-coding checkout root}
M=$(cd "$(dirname "$0")" && pwd)
B=$A/knowledge-data/retrieval-eval-v2/bin-main2
R=$M/$case/runs/$arm; mkdir -p "$R" "$M/$case/telemetry/$arm"
cat > "$R/cks.yaml" <<YAML
version: 1
name: $case-$arm
backends:
  ckg:
    path: $M/$case/graph/graph.db
    source_root: $M/$case/src
    binary_path: $B/ckg
  ckv:
    path: $M/$case/vector
    provider: ollama
    binary_path: $B/ckv
    embed_model: bge-m3
listen:
  transport: http
  http_addr: 127.0.0.1:$port
logging:
  level: error
  footprint_dir: $M/$case/telemetry/$arm
sanitize:
  rules_path: $A/knowledge/knowledge-system/system/policies/sanitization_rules.yaml
  default_action: drop
  fail_closed_on_unknown_rule: true
YAML
"$A/test-data/pr-77-procedure/bin/cks" mcp --config "$R/cks.yaml" > "$R/cks.log" 2>&1 &
cks_pid=$!
trap 'kill $cks_pid 2>/dev/null || true' EXIT
for _ in $(seq 1 60); do grep -q "reachable at" "$R/cks.log" && break; sleep 1; done
problem=$(cat "$M/$case.problem.txt")
cd "$M/$case/work/$arm"
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
  > "$R/stream.jsonl" 2> "$R/stderr.log"
echo "wall_seconds=$(( $(date -u +%s) - start ))" > "$R/wall.txt"
cp .stablenet-expert/diagnoses/*/diagnosis.md "$R/diagnosis.md" 2>/dev/null || echo "no diagnosis.md" >&2
