#!/usr/bin/env bash
# cks-ops-common.sh — helpers shared by the availability scripts
# (install-launchd.sh, cks-watchdog.sh, cks-recover.sh).
#
# Sourced, never executed. Everything the three entry points agree on lives
# here — the launchd domain, the instance env file, the health probe — so a
# change lands in one place instead of three.

# Where per-instance operator state lives: the rendered env file, the
# watchdog's counters, and the logs launchd writes. Deliberately not the
# dataset directory: this is host state, and it must survive a dataset swap.
CKS_OPS_HOME="${CKS_OPS_HOME:-$HOME/.knowledge-system}"
CKS_OPS_LOG_DIR="$CKS_OPS_HOME/logs"

# Default instance label. A downstream distribution serving several packs
# gives each one its own label (--label) so their agents never collide.
CKS_OPS_DEFAULT_LABEL="com.knowledge-system.cks-mcp"

ops_log() { printf '%s %s\n' "$(date '+%Y-%m-%dT%H:%M:%S%z')" "$*"; }
ops_die() { printf 'ERROR: %s\n' "$*" >&2; exit 1; }

ops_env_file()   { printf '%s/%s.env\n' "$CKS_OPS_HOME" "$1"; }
ops_state_file() { printf '%s/%s.watchdog.state\n' "$CKS_OPS_HOME" "$1"; }

# ops_domain echoes the launchctl domain target to use for this user.
#
# A LaunchAgent belongs to the GUI session when the user is logged in at the
# console, and to the per-user domain otherwise. An SSH caller cannot assume
# which, so probe: gui/<uid> when launchd answers for it, user/<uid> when not.
# Getting this wrong is the usual reason a remote `launchctl kickstart` reports
# "Could not find service".
ops_domain() {
    local uid
    uid="$(id -u)"
    if launchctl print "gui/$uid" >/dev/null 2>&1; then
        printf 'gui/%s\n' "$uid"
    else
        printf 'user/%s\n' "$uid"
    fi
}

# ops_load_env reads the env file the installer rendered for a label and
# exports its keys. Fails when the instance was never installed, because every
# caller needs CKS_HEALTH_URL and the log paths from it.
ops_load_env() {
    local label="$1" file
    file="$(ops_env_file "$label")"
    [ -f "$file" ] || ops_die "no installed instance named $label (missing $file) — run install-launchd.sh install first"
    # shellcheck disable=SC1090
    . "$file"
    [ -n "${CKS_HEALTH_URL:-}" ] || ops_die "$file does not define CKS_HEALTH_URL"
}

# ops_probe_health prints "<http-code> <body>" for one /healthz request.
# Code 000 means the listener did not answer at all (process down, port
# closed, host unreachable) — a different failure from a 503, which is the
# server answering that it cannot serve. The watchdog treats them differently.
ops_probe_health() {
    local url="$1" timeout="${2:-5}" body code
    body="$(curl -sS --max-time "$timeout" -w '\n%{http_code}' "$url" 2>/dev/null)" || body=$'\n000'
    code="${body##*$'\n'}"
    body="${body%$'\n'*}"
    printf '%s %s\n' "${code:-000}" "$(printf '%s' "$body" | tr -d '\n')"
}

# ops_probe_embedder reports whether the embedding daemon answers. The vector
# engine is the only backend that depends on a second process, and that process
# is the one that does not survive a host sleep: cks stays up and keeps serving
# graph tools while every semantic call fails, so health reads "degraded"
# rather than "down". Probing it separately is what tells those apart.
ops_probe_embedder() {
    curl -fsS --max-time "${2:-5}" "${1%/}/api/version" >/dev/null 2>&1
}

# ops_start_embedder brings the embedding daemon back. The documented install
# is the Ollama app cask (the brew formula ships without llama-server on Apple
# Silicon), so prefer launching the app; fall back to a bare serve when only
# the CLI is present.
ops_start_embedder() {
    if [ -d /Applications/Ollama.app ]; then
        open -ga Ollama
        return $?
    fi
    command -v ollama >/dev/null 2>&1 || return 1
    nohup ollama serve >>"$CKS_OPS_LOG_DIR/ollama.log" 2>&1 &
    return 0
}

# ops_wait_embedder polls until the embedder answers or the deadline passes.
ops_wait_embedder() {
    local url="$1" deadline="${2:-60}" waited=0
    while [ "$waited" -lt "$deadline" ]; do
        ops_probe_embedder "$url" 5 && return 0
        sleep 2
        waited=$((waited + 2))
    done
    return 1
}

# ops_wait_healthy polls until /healthz returns 200 or the deadline passes.
# Returns 0 on success so callers can report whether a restart actually
# recovered the instance rather than only that the process respawned.
ops_wait_healthy() {
    local url="$1" deadline="${2:-60}" waited=0 code
    while [ "$waited" -lt "$deadline" ]; do
        code="$(ops_probe_health "$url" 5 | cut -d' ' -f1)"
        [ "$code" = "200" ] && return 0
        sleep 2
        waited=$((waited + 2))
    done
    return 1
}
