#!/bin/bash
# Remote recovery entry point, run as an SSH forced command.
#
# An operator on another machine asks this host to recover its MCP server. The
# key they connect with is pinned to this script in authorized_keys, so the
# connection can do exactly what is allowlisted below and cannot become a shell:
# whatever they typed arrives in SSH_ORIGINAL_COMMAND and is matched against a
# fixed list, never interpreted.
#
# Install the key with install-recovery-key.sh; drive it from the other machine:
#
#   ssh -i ~/.ssh/cks_recovery cks-ops@<host>            # recover if unhealthy
#   ssh -i ~/.ssh/cks_recovery cks-ops@<host> status     # report, change nothing
#   ssh -i ~/.ssh/cks_recovery cks-ops@<host> force      # restart unconditionally
#
# Exit status is the recovery verdict, so the caller can act on it: 0 means the
# instance is serving, non-zero means it is not and the log has to be read.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

# Overridable for a second instance on the same host; the defaults are this
# deployment's.
CKS_BIN="${CKS_BIN:-$repo_root/bin/cks}"
CKS_CONFIG="${CKS_CONFIG:-$repo_root/cks.yaml}"
CKS_RECOVERY_LOG="${CKS_RECOVERY_LOG:-$repo_root/run/remote-recovery.log}"

client="${SSH_CONNECTION%% *}"
[ -n "$client" ] || client="local"

log() {
    mkdir -p "$(dirname "$CKS_RECOVERY_LOG")"
    printf '%s %s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$client" "$*" >>"$CKS_RECOVERY_LOG"
}

# The request is untrusted text from the network: it selects a branch, it is
# never expanded into a command.
request="${SSH_ORIGINAL_COMMAND:-recover}"
case "$request" in
    ""|recover) verb=(service recover) ;;
    force)      verb=(service recover --force) ;;
    status)     verb=(service status) ;;
    *)
        log "REJECTED $request"
        echo "cks remote recovery: unknown request '$request' (allowed: recover, force, status)" >&2
        exit 64
        ;;
esac

if [ ! -x "$CKS_BIN" ]; then
    log "FAILED missing binary $CKS_BIN"
    echo "cks remote recovery: $CKS_BIN is not executable on this host" >&2
    exit 69
fi

log "ACCEPTED $request"
set +e
output="$("$CKS_BIN" mcp "${verb[@]}" --config "$CKS_CONFIG" 2>&1)"
rc=$?
set -e

printf '%s\n' "$output"
log "RESULT $request rc=$rc"
exit "$rc"
