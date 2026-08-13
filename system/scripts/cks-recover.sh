#!/usr/bin/env bash
# cks-recover.sh — the recovery entry point a remote operator drives over SSH.
#
#   ssh <host> '<repo>/system/scripts/cks-recover.sh status'
#   ssh <host> '<repo>/system/scripts/cks-recover.sh restart'
#
# The watchdog handles the failures it can classify on its own; this is the
# escalation path for the ones it cannot — an operator who sees a client
# failing and wants the instance looked at or bounced from another machine.
#
# SSH is the transport on purpose: it authenticates the caller and needs no
# new listener. The MCP port itself is network-scope filtered, not
# authenticated, so exposing process control there would let anyone on the
# LAN restart the server.
#
# Actions:
#   status    launchd state + embedder + health + served identity (read-only)
#   health    the raw /healthz body and status code (read-only)
#   logs      tail the server and watchdog logs (read-only)
#   recover   the same ladder the watchdog walks — restore the embedder first,
#             bounce the server only if that was not the fault
#   restart   bounce the server agent unconditionally
#
# Prefer `recover` when a client reports failures and you do not know why:
# bouncing the server does not fix an embedder that a host sleep took down.
#
# Usage: cks-recover.sh <status|health|logs|recover|restart> [--label <label>] [--lines <n>]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=system/scripts/cks-ops-common.sh
. "$SCRIPT_DIR/cks-ops-common.sh"

label="$CKS_OPS_DEFAULT_LABEL"
lines=40
deadline="${CKS_RECOVER_DEADLINE:-90}"

action="${1:-}"
[ -n "$action" ] || ops_die "usage: cks-recover.sh <status|health|logs|recover|restart> [--label <label>] [--lines <n>]"
shift || true
while [ $# -gt 0 ]; do
    case "$1" in
        --label) label="$2"; shift 2 ;;
        --lines) lines="$2"; shift 2 ;;
        *)       ops_die "unknown option: $1" ;;
    esac
done

ops_load_env "$label"
domain="$(ops_domain)"

agent_state() {
    local name="$1"
    if ! launchctl print "$domain/$name" >/dev/null 2>&1; then
        printf '  %-28s not loaded\n' "$name"
        return
    fi
    launchctl print "$domain/$name" \
        | awk -v n="$name" '
            /^[[:space:]]*state =/          { s = $3 }
            /^[[:space:]]*pid =/            { p = $3 }
            /^[[:space:]]*last exit code =/ { e = $5 }
            END { printf "  %-28s state=%s pid=%s last_exit=%s\n", n, (s?s:"-"), (p?p:"-"), (e?e:"-") }
        '
}

case "$action" in
status)
    ops_log "instance $label in $domain"
    agent_state "$label"
    agent_state "$label.watchdog"
    if [ -n "${CKS_OLLAMA_ENDPOINT:-}" ]; then
        if ops_probe_embedder "$CKS_OLLAMA_ENDPOINT"; then
            printf '  %-28s up (%s)\n' "embedder" "$CKS_OLLAMA_ENDPOINT"
        else
            printf '  %-28s DOWN (%s) — semantic tools will fail\n' "embedder" "$CKS_OLLAMA_ENDPOINT"
        fi
    fi
    probe="$(ops_probe_health "$CKS_HEALTH_URL" 10)"
    printf '  %-28s %s\n' "health" "$probe"
    # The served dataset is the identity question a remote caller usually has
    # next: a healthy server on last week's index is still the wrong answer.
    if command -v python3 >/dev/null 2>&1; then
        printf '%s' "${probe#* }" | python3 -c '
import json, sys
try:
    d = json.load(sys.stdin)
except Exception:
    sys.exit(0)
for k in ("name", "serviceable", "reason"):
    if k in d:
        print(f"  {k:<28} {json.dumps(d[k])}")
' || true
    fi
    ;;

health)
    ops_probe_health "$CKS_HEALTH_URL" 10
    ;;

logs)
    for f in "$CKS_SERVER_LOG" "$CKS_WATCHDOG_LOG"; do
        printf '\n==> %s (last %s lines)\n' "$f" "$lines"
        [ -f "$f" ] && tail -n "$lines" "$f" || printf '(no such file yet)\n'
    done
    ;;

recover)
    if [ "$(ops_probe_health "$CKS_HEALTH_URL" 10 | cut -d' ' -f1)" = "200" ]; then
        ops_log "nothing to do: $CKS_HEALTH_URL already returns 200"
        exit 0
    fi
    if [ -n "${CKS_OLLAMA_ENDPOINT:-}" ] && ! ops_probe_embedder "$CKS_OLLAMA_ENDPOINT"; then
        ops_log "embedder down at $CKS_OLLAMA_ENDPOINT — starting it"
        if ops_start_embedder && ops_wait_embedder "$CKS_OLLAMA_ENDPOINT" 60 \
           && ops_wait_healthy "$CKS_HEALTH_URL" "$deadline"; then
            ops_log "recovered by restoring the embedder; the server was not bounced"
            exit 0
        fi
        ops_log "embedder recovery did not restore service — bouncing the server"
    fi
    ops_log "restarting $domain/$label"
    launchctl kickstart -k "$domain/$label"
    if ops_wait_healthy "$CKS_HEALTH_URL" "$deadline"; then
        ops_log "recovered: $CKS_HEALTH_URL returns 200"
    else
        ops_log "FAILED: not serviceable ${deadline}s after restart"
        printf '\n==> %s (last %s lines)\n' "$CKS_SERVER_LOG" "$lines"
        tail -n "$lines" "$CKS_SERVER_LOG" 2>/dev/null || true
        exit 1
    fi
    ;;

restart)
    ops_log "restarting $domain/$label"
    launchctl kickstart -k "$domain/$label"
    if ops_wait_healthy "$CKS_HEALTH_URL" "$deadline"; then
        ops_log "recovered: $CKS_HEALTH_URL returns 200"
    else
        ops_log "FAILED: not serviceable ${deadline}s after restart"
        printf '\n==> %s (last %s lines)\n' "$CKS_SERVER_LOG" "$lines"
        tail -n "$lines" "$CKS_SERVER_LOG" 2>/dev/null || true
        exit 1
    fi
    ;;

*)
    ops_die "unknown action: $action (expected status, health, logs, recover or restart)"
    ;;
esac
