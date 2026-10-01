#!/usr/bin/env bash
# Build, sign, independently verify, unpack and exercise a Linux host preview.
# The fixture uses a synthetic Git commit so dirty development sources can be
# tested without claiming a release from the original repository commit.
set -euo pipefail

arch="${1:?usage: wbs-linux-package-smoke.sh arm64|amd64}"
if [[ "$arch" != arm64 && "$arch" != amd64 ]]; then echo "unsupported arch: $arch" >&2; exit 2; fi
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
scratch="${KS_LINUX_SMOKE_DIR:-$(mktemp -d)}"
mkdir -p "$scratch/dist" "$scratch/signature" "$scratch/runtime"
build_image="knowledge-system-linux-build:go1.25.13-$arch"
runtime_image="knowledge-system-runtime-smoke:bookworm-$arch"
docker build --platform "linux/$arch" -t "$build_image" -f "$repo_root/scripts/Dockerfile.linux-build" "$repo_root" > "$scratch/build-image.log"
docker build --platform "linux/$arch" -t "$runtime_image" -f "$repo_root/scripts/Dockerfile.linux-runtime-smoke" "$repo_root" > "$scratch/runtime-image.log"
cache_args=()
module_env=''
if [[ -n "${KS_GO_MOD_CACHE:-}" ]]; then
  cache_args=(-v "$KS_GO_MOD_CACHE:/go/pkg/mod:ro")
  module_env='export GOMODCACHE=/go/pkg/mod GOPROXY=off GOSUMDB=off;'
fi
docker run --rm --platform "linux/$arch" -v "$repo_root:/source:ro" -v "$scratch/dist:/out" \
  ${cache_args[@]+"${cache_args[@]}"} "$build_image" bash -lc "set -euo pipefail; export PATH=/usr/local/go/bin:\$PATH; $module_env cp -a /source/. /worktree; cd /worktree; rm -f .git; git init -q; git config user.name Fixture; git config user.email fixture@example.invalid; git config commit.gpgsign false; git add -A; git commit -qm 'linux package fixture'; python3 scripts/package-host.py --out-dir /out" \
  > "$scratch/package.json"
archive="$scratch/dist/$(python3 - "$scratch/package.json" <<'PY'
import json, sys
from pathlib import Path
print(Path(json.load(open(sys.argv[1]))['archive']).name)
PY
)"
python3 - "$archive" <<'PY'
import json
from pathlib import Path
import sys
import tarfile
with tarfile.open(sys.argv[1], 'r:gz') as tar:
    manifest_member = next(member for member in tar if member.name.endswith('/manifest.json'))
    inventory_member = next(member for member in tar if member.name.endswith('/third-party-licenses.json'))
    manifest = json.load(tar.extractfile(manifest_member))
    inventory = json.load(tar.extractfile(inventory_member))
assert manifest['third_party_license_inventory']['missing_license_count'] == 0
assert manifest['third_party_license_inventory']['module_count'] == len(inventory['modules'])
assert all(module['license_files'] for module in inventory['modules'])
PY
openssl genpkey -algorithm ED25519 -out "$scratch/signature/private.pem" >/dev/null 2>&1
openssl pkey -in "$scratch/signature/private.pem" -pubout -out "$scratch/signature/public.pem" >/dev/null 2>&1
python3 "$repo_root/scripts/release-sidecar.py" --archive "$archive" \
  --private-key "$scratch/signature/private.pem" --public-key "$scratch/signature/public.pem" \
  --out-dir "$scratch/signature/sidecar" > "$scratch/sign-result.json"
python3 "$repo_root/scripts/verify-release.py" --public-key "$scratch/signature/public.pem" \
  --release "$scratch/signature/sidecar/release.json" \
  --signature "$scratch/signature/sidecar/release.json.sig" \
  --archive "$archive" --target-os linux --target-arch "$arch" > "$scratch/preinstall-verify.json"
archive_name="$(basename "$archive")"
stage_name="${archive_name%.tar.gz}"
docker run --rm --platform "linux/$arch" -v "$scratch:/dist:ro" -v "$scratch/runtime:/logs" \
  -v "$repo_root/scripts:/scripts:ro" "$runtime_image" bash -lc \
  "set -euo pipefail; if command -v go >/dev/null; then echo unexpected-go >&2; exit 1; fi; mkdir -p /tmp/unpacked; tar -xzf '/dist/dist/$archive_name' -C /tmp/unpacked; stage='/tmp/unpacked/$stage_name'; \"\$stage/cks\" package verify --public-key /dist/signature/public.pem --release /dist/signature/sidecar/release.json --signature /dist/signature/sidecar/release.json.sig --archive '/dist/dist/$archive_name' --target-os linux --target-arch '$arch'; KS_BIN_DIR=\"\$stage\" KS_SANITIZE_RULES=\"\$stage/policies/sanitization_rules.yaml\" KS_INSTALL_SMOKE_DIR=/logs /scripts/wbs-install-smoke.sh" \
  > "$scratch/runtime.log"
printf 'Linux %s signed package smoke passed: %s\n' "$arch" "$scratch"
