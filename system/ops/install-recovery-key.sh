#!/bin/bash
# Authorize one remote operator to trigger recovery on this host — and nothing
# else.
#
# The public key is added to authorized_keys pinned to remote-recover.sh: the
# forced command runs instead of whatever the client asks for, and the
# restrictions strip the rest of what an SSH session normally grants (a tty,
# port forwarding, agent forwarding, the user's own rc files). A leaked key
# therefore buys an attacker one thing — restarting a server that is supposed to
# be running — and not a login on this machine.
#
# Usage:
#   system/ops/install-recovery-key.sh <path-to-public-key> [--from <cidr>]
#
#   --from  additionally restrict which addresses may use the key, e.g.
#           --from 10.0.0.0/8 for an office LAN. Recommended.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
forced_command="$repo_root/system/ops/remote-recover.sh"
authorized_keys="$HOME/.ssh/authorized_keys"

usage() {
    echo "usage: $0 <path-to-public-key> [--from <cidr>]" >&2
    exit 64
}

[ $# -ge 1 ] || usage
key_file="$1"
shift
from_cidr=""
while [ $# -gt 0 ]; do
    case "$1" in
        --from)
            [ $# -ge 2 ] || usage
            from_cidr="$2"
            shift 2
            ;;
        *) usage ;;
    esac
done

[ -r "$key_file" ] || { echo "cannot read public key: $key_file" >&2; exit 66; }
key_line="$(tr -d '\r' <"$key_file" | grep -v '^[[:space:]]*$' | head -1)"
case "$key_line" in
    ssh-ed25519\ *|ssh-rsa\ *|ecdsa-sha2-*\ *|sk-ssh-*\ *) ;;
    *) echo "$key_file does not look like an SSH public key (a private key must never be installed here)" >&2; exit 65 ;;
esac
[ -x "$forced_command" ] || { echo "forced command is not executable: $forced_command" >&2; exit 69; }

options="command=\"$forced_command\",no-agent-forwarding,no-port-forwarding,no-pty,no-user-rc,no-X11-forwarding"
if [ -n "$from_cidr" ]; then
    options="from=\"$from_cidr\",$options"
fi

mkdir -p "$HOME/.ssh"
chmod 700 "$HOME/.ssh"
touch "$authorized_keys"
chmod 600 "$authorized_keys"

# Match on the key material itself, so re-running with changed options replaces
# the entry instead of authorizing the same key twice under different rules.
key_material="$(echo "$key_line" | awk '{print $2}')"
if grep -qF "$key_material" "$authorized_keys"; then
    tmp="$(mktemp)"
    grep -vF "$key_material" "$authorized_keys" >"$tmp"
    cat "$tmp" >"$authorized_keys"
    rm -f "$tmp"
    echo "replaced the existing entry for this key"
fi
printf '%s %s\n' "$options" "$key_line" >>"$authorized_keys"

echo "authorized recovery key in $authorized_keys"
echo "  forced command: $forced_command"
if [ -n "$from_cidr" ]; then
    echo "  restricted to: $from_cidr"
fi
echo
echo "Remaining step — Remote Login must be on for the key to be usable:"
echo "  sudo systemsetup -setremotelogin on"
echo "Then, from the operator's machine:"
echo "  ssh -i <private key> $USER@$(ipconfig getifaddr en0 2>/dev/null || hostname) status"
