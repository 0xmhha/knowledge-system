#!/usr/bin/env bash
# cks-watchdog.sh — one health probe of an installed instance, run on a timer
# by the <label>.watchdog LaunchAgent.
#
# KeepAlive already restarts a server that exits. This covers the other
# failure: the process is up, the port answers, and /healthz says it cannot
# serve — a backend gone, a dataset swapped underneath it, a wedged handle.
#
# Two failure classes, deliberately not treated alike:
#
#   unreachable (000)   nothing answered. A restart is the right response, so
#                       the threshold is low.
#   unserviceable (503) the server answered that it is not ready. A restart
#                       may not fix it (a stopped Ollama, a half-built
#                       dataset), so it takes more consecutive failures — and
#                       the reason is logged either way, which is what the
#                       operator actually needs.
#
# Restarts are rate-limited by a cooldown so a permanently broken instance
# writes a slow log rather than a restart loop.
#
# Usage: cks-watchdog.sh [--label <label>]
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=system/scripts/cks-ops-common.sh
. "$SCRIPT_DIR/cks-ops-common.sh"

label="$CKS_OPS_DEFAULT_LABEL"
while [ $# -gt 0 ]; do
    case "$1" in
        --label) label="$2"; shift 2 ;;
        *)       ops_die "unknown option: $1" ;;
    esac
done

UNREACHABLE_THRESHOLD="${CKS_WATCHDOG_UNREACHABLE_THRESHOLD:-2}"
UNSERVICEABLE_THRESHOLD="${CKS_WATCHDOG_UNSERVICEABLE_THRESHOLD:-5}"
RESTART_COOLDOWN="${CKS_WATCHDOG_COOLDOWN:-300}"
RECOVER_DEADLINE="${CKS_WATCHDOG_RECOVER_DEADLINE:-90}"

ops_load_env "$label"
state_file="$(ops_state_file "$label")"

fails=0
last_restart=0
if [ -f "$state_file" ]; then
    # shellcheck disable=SC1090
    . "$state_file"
fi

write_state() {
    cat >"$state_file" <<EOF
fails=$1
last_restart=$2
EOF
}

probe="$(ops_probe_health "$CKS_HEALTH_URL" 10)"
code="${probe%% *}"
body="${probe#* }"

if [ "$code" = "200" ]; then
    [ "${fails:-0}" -gt 0 ] && ops_log "recovered: $CKS_HEALTH_URL returns 200 after $fails failed probe(s)"
    write_state 0 "${last_restart:-0}"
    exit 0
fi

# Recovery ladder, dependency first. A 503 whose cause is the embedding daemon
# is the failure this host actually produces: the daemon does not survive a
# system sleep, and cks — still up, still serving graph tools — reports
# "degraded" because serviceable() requires model reachability. Restarting cks
# cannot fix that; starting the daemon can, and cks recovers on its own once it
# answers, so no bounce is needed afterwards.
if [ -n "${CKS_OLLAMA_ENDPOINT:-}" ] && ! ops_probe_embedder "$CKS_OLLAMA_ENDPOINT"; then
    ops_log "embedder down at $CKS_OLLAMA_ENDPOINT — starting it"
    if ops_start_embedder && ops_wait_embedder "$CKS_OLLAMA_ENDPOINT" 60; then
        if ops_wait_healthy "$CKS_HEALTH_URL" "$RECOVER_DEADLINE"; then
            ops_log "recovered by restoring the embedder; cks was never bounced"
            write_state 0 "${last_restart:-0}"
            exit 0
        fi
        ops_log "embedder is back but the instance is still not serviceable"
    else
        ops_log "ERROR: could not bring the embedder back at $CKS_OLLAMA_ENDPOINT"
    fi
fi

fails=$((${fails:-0} + 1))
if [ "$code" = "503" ]; then
    threshold="$UNSERVICEABLE_THRESHOLD"
    ops_log "unserviceable ($fails/$threshold): $body"
else
    threshold="$UNREACHABLE_THRESHOLD"
    ops_log "unreachable ($fails/$threshold): http_code=$code"
fi

if [ "$fails" -lt "$threshold" ]; then
    write_state "$fails" "${last_restart:-0}"
    exit 0
fi

now="$(date +%s)"
since=$((now - ${last_restart:-0}))
if [ "$since" -lt "$RESTART_COOLDOWN" ]; then
    ops_log "restart suppressed: last restart was ${since}s ago (cooldown ${RESTART_COOLDOWN}s)"
    write_state "$fails" "${last_restart:-0}"
    exit 0
fi

domain="$(ops_domain)"
ops_log "restarting $domain/$label"
# kickstart -k terminates the running job and starts it again, which is what
# a KeepAlive'd agent needs: plain `kickstart` on a live job is a no-op.
if ! launchctl kickstart -k "$domain/$label"; then
    ops_log "ERROR: launchctl kickstart failed for $domain/$label"
    write_state "$fails" "$now"
    exit 1
fi

if ops_wait_healthy "$CKS_HEALTH_URL" "$RECOVER_DEADLINE"; then
    ops_log "restart recovered the instance"
    write_state 0 "$now"
    exit 0
fi

ops_log "ERROR: still not serviceable ${RECOVER_DEADLINE}s after restart — see $CKS_SERVER_LOG"
write_state "$fails" "$now"
exit 1
