#!/usr/bin/env bash
# install-launchd.sh — keep one MCP instance serving on a macOS host.
#
# Installs two LaunchAgents from the templates in system/scripts/launchd/:
#
#   <label>            the server itself, wrapped in caffeinate (the host does
#                      not sleep while it serves) and KeepAlive'd (a process
#                      that exits comes back).
#   <label>.watchdog   a timer that probes /healthz and restarts the server
#                      when it is up but cannot serve — the failure KeepAlive
#                      cannot see.
#
# Usage:
#   system/scripts/install-launchd.sh install --config <cks.yaml> [options]
#   system/scripts/install-launchd.sh uninstall [--label <label>]
#   system/scripts/install-launchd.sh status    [--label <label>]
#
# install options:
#   --config <path>       runtime config the instance serves (required)
#   --bin <path>          cks binary            (default: <repo>/bin/cks)
#   --label <label>       launchd label         (default: com.knowledge-system.cks-mcp)
#   --health-url <url>    override the probe URL derived from the config
#   --interval <seconds>  watchdog period       (default: 60)
#   --ollama <url>        embedder endpoint     (default: $CKV_OLLAMA_ENDPOINT
#                                                or http://localhost:11434)
#
# Re-running install re-renders and reloads both agents, so it doubles as the
# upgrade path after a rebuild or a config change.
set -euo pipefail

KS_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SCRIPT_DIR="$KS_ROOT/system/scripts"
# shellcheck source=system/scripts/cks-ops-common.sh
. "$SCRIPT_DIR/cks-ops-common.sh"

TEMPLATE_DIR="$SCRIPT_DIR/launchd"
AGENT_DIR="$HOME/Library/LaunchAgents"

label="$CKS_OPS_DEFAULT_LABEL"
config=""
cks_bin="$KS_ROOT/bin/cks"
health_url=""
interval=60
ollama="${CKV_OLLAMA_ENDPOINT:-http://localhost:11434}"

action="${1:-}"
[ -n "$action" ] || ops_die "usage: install-launchd.sh <install|uninstall|status> [options]"
shift || true

while [ $# -gt 0 ]; do
    case "$1" in
        --config)     config="$2"; shift 2 ;;
        --bin)        cks_bin="$2"; shift 2 ;;
        --label)      label="$2"; shift 2 ;;
        --health-url) health_url="$2"; shift 2 ;;
        --interval)   interval="$2"; shift 2 ;;
        --ollama)     ollama="$2"; shift 2 ;;
        *)            ops_die "unknown option: $1" ;;
    esac
done

# config_value echoes one scalar from the listen block of a cks config. The
# block is flat YAML written by gen-config, so a scoped awk read is enough and
# keeps the installer free of a YAML dependency.
config_value() {
    awk -v key="$2:" '
        /^[^[:space:]]/  { inlisten = ($1 == "listen:") }
        inlisten && $1 == key { print $2; exit }
    ' "$1"
}

# derive_health_url turns the configured bind address into a URL this host can
# probe. A wildcard bind answers on loopback; a LAN bind answers on its own
# address, and probing that address is what a remote caller would do too.
derive_health_url() {
    local cfg="$1" addr host port
    addr="$(config_value "$cfg" http_addr)"
    [ -n "$addr" ] || return 1
    host="${addr%:*}"
    port="${addr##*:}"
    case "$host" in
        ""|"0.0.0.0"|"::"|"[::]") host="127.0.0.1" ;;
    esac
    printf 'http://%s:%s/healthz\n' "$host" "$port"
}

render() {
    local template="$1" out="$2"
    sed -e "s|__LABEL__|$label|g" \
        -e "s|__CKS_BIN__|$cks_bin|g" \
        -e "s|__CONFIG__|$config|g" \
        -e "s|__WORKDIR__|$KS_ROOT|g" \
        -e "s|__LOG_DIR__|$CKS_OPS_LOG_DIR|g" \
        -e "s|__PATH__|$PATH|g" \
        -e "s|__OLLAMA_ENDPOINT__|$ollama|g" \
        -e "s|__WATCHDOG_SH__|$SCRIPT_DIR/cks-watchdog.sh|g" \
        -e "s|__INTERVAL__|$interval|g" \
        "$template" >"$out"
}

# reload_agent replaces a job definition atomically enough for an operator:
# bootout an existing job (ignoring "not loaded"), then bootstrap the new one.
reload_agent() {
    local domain="$1" plist="$2" name="$3"
    launchctl bootout "$domain/$name" >/dev/null 2>&1 || true
    launchctl bootstrap "$domain" "$plist"
    launchctl enable "$domain/$name"
}

case "$action" in
install)
    [ -n "$config" ] || ops_die "--config is required"
    [ -f "$config" ] || ops_die "config not found: $config"
    config="$(cd "$(dirname "$config")" && pwd)/$(basename "$config")"
    [ -x "$cks_bin" ] || ops_die "cks binary not executable: $cks_bin (run: make build-bins)"
    cks_bin="$(cd "$(dirname "$cks_bin")" && pwd)/$(basename "$cks_bin")"
    [ -d "$TEMPLATE_DIR" ] || ops_die "missing templates: $TEMPLATE_DIR"
    command -v caffeinate >/dev/null || ops_die "caffeinate not found — this installer is macOS-only"

    transport="$(config_value "$config" transport)"
    [ "$transport" = "http" ] || ops_die "listen.transport is '${transport:-unset}', not http — a stdio instance has no /healthz to supervise. Regenerate with: cks mcp gen-config --port <port>"

    [ -n "$health_url" ] || health_url="$(derive_health_url "$config")" \
        || ops_die "could not derive a health URL from $config — pass --health-url"

    mkdir -p "$AGENT_DIR" "$CKS_OPS_HOME" "$CKS_OPS_LOG_DIR"

    server_plist="$AGENT_DIR/$label.plist"
    watchdog_plist="$AGENT_DIR/$label.watchdog.plist"
    render "$TEMPLATE_DIR/server.plist.template"   "$server_plist"
    render "$TEMPLATE_DIR/watchdog.plist.template" "$watchdog_plist"

    # The env file is the contract the watchdog and the recovery script read;
    # it is written before the agents load so their first run already sees it.
    cat >"$(ops_env_file "$label")" <<EOF
# Written by install-launchd.sh — one installed instance's operator contract.
CKS_LABEL="$label"
CKS_CONFIG="$config"
CKS_BIN="$cks_bin"
CKS_HEALTH_URL="$health_url"
CKS_OLLAMA_ENDPOINT="$ollama"
CKS_SERVER_LOG="$CKS_OPS_LOG_DIR/$label.err.log"
CKS_WATCHDOG_LOG="$CKS_OPS_LOG_DIR/$label.watchdog.log"
EOF

    domain="$(ops_domain)"
    reload_agent "$domain" "$server_plist"   "$label"
    reload_agent "$domain" "$watchdog_plist" "$label.watchdog"

    ops_log "installed $label in $domain (watchdog every ${interval}s)"
    if ops_wait_healthy "$health_url" 60; then
        ops_log "serving: $health_url returns 200"
    else
        ops_log "WARNING: $health_url did not return 200 within 60s — check $CKS_OPS_LOG_DIR/$label.err.log"
        exit 1
    fi
    ;;

uninstall)
    domain="$(ops_domain)"
    for name in "$label.watchdog" "$label"; do
        launchctl bootout "$domain/$name" >/dev/null 2>&1 || true
        rm -f "$AGENT_DIR/$name.plist"
    done
    rm -f "$(ops_env_file "$label")" "$(ops_state_file "$label")"
    ops_log "uninstalled $label from $domain (logs kept in $CKS_OPS_LOG_DIR)"
    ;;

status)
    domain="$(ops_domain)"
    ops_log "domain: $domain"
    for name in "$label" "$label.watchdog"; do
        if launchctl print "$domain/$name" >/dev/null 2>&1; then
            launchctl print "$domain/$name" | awk '
                /^[[:space:]]*(state|pid|last exit code) =/ { printf "  %s %s\n", n, $0 }
            ' n="$name"
        else
            printf '  %s not loaded\n' "$name"
        fi
    done
    if [ -f "$(ops_env_file "$label")" ]; then
        ops_load_env "$label"
        ops_log "health: $(ops_probe_health "$CKS_HEALTH_URL")"
    fi
    ;;

*)
    ops_die "unknown action: $action (expected install, uninstall or status)"
    ;;
esac
