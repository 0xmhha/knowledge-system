#!/usr/bin/env bash
# Exercise the current-host archive's extracted binaries and policy on three
# independent repositories. Requires the host's Go/Git toolchain; not an OS
# compatibility or third-party-license certification.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
scratch="${KS_PACKAGE_SMOKE_DIR:-$(mktemp -d)}"
mkdir -p "$scratch"
args=()
if [[ "${KS_PACKAGE_ALLOW_DIRTY:-0}" == 1 ]]; then
  args+=(--allow-dirty)
fi
python3 "$repo_root/scripts/package-host.py" --out-dir "$scratch/dist" "${args[@]}" \
  > "$scratch/package.json"
stage="$(python3 - "$scratch/package.json" "$scratch/unpacked" <<'PY'
import hashlib
import json
from pathlib import Path
import sys
import tarfile
package = json.loads(Path(sys.argv[1]).read_text())
archive = Path(package['archive'])
assert hashlib.sha256(archive.read_bytes()).hexdigest() == package['sha256']
root = Path(sys.argv[2])
root.mkdir(parents=True)
with tarfile.open(archive, 'r:gz') as tar:
    names = tar.getnames()
    assert names and len({Path(name).parts[0] for name in names}) == 1, names
    assert all('..' not in Path(name).parts for name in names), names
    tar.extractall(root)
stage = root / Path(names[0]).parts[0]
manifest = json.loads((stage / 'manifest.json').read_text())
assert manifest['scope'] == 'host-preview' and manifest['third_party_license_review'] == 'pending'
for name in ('ckg', 'ckv', 'cks'):
    binary = stage / name
    assert binary.is_file() and binary.stat().st_mode & 0o111, name
    assert hashlib.sha256(binary.read_bytes()).hexdigest() == manifest['binaries'][name]['sha256'], name
for name in ('LICENSE', 'INSTALLATION.md', 'modules.txt', 'policies/sanitization_rules.yaml'):
    assert (stage / name).is_file(), name
print(stage)
PY
)"
KS_BIN_DIR="$stage" KS_SANITIZE_RULES="$stage/policies/sanitization_rules.yaml" \
  KS_INSTALL_SMOKE_DIR="$scratch/installs" "$repo_root/scripts/wbs-install-smoke.sh"
printf 'Current-host archive smoke: %s\n' "$stage"
