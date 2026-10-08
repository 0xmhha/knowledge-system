#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
scratch="${KS_RELEASE_SMOKE_DIR:-$(mktemp -d)}"
mkdir -p "$scratch/dist" "$scratch/signed" "$scratch/changed" "$scratch/unpacked"
python3 "$repo_root/scripts/package-host.py" --out-dir "$scratch/dist" --allow-dirty > "$scratch/package.json"
archive="$(python3 - "$scratch/package.json" <<'PY'
import json, sys
print(json.load(open(sys.argv[1]))["archive"])
PY
)"
openssl genpkey -algorithm ED25519 -out "$scratch/private.pem" >/dev/null 2>&1
openssl pkey -in "$scratch/private.pem" -pubout -out "$scratch/public.pem" >/dev/null 2>&1
openssl genpkey -algorithm ED25519 -out "$scratch/other-private.pem" >/dev/null 2>&1
openssl pkey -in "$scratch/other-private.pem" -pubout -out "$scratch/other-public.pem" >/dev/null 2>&1
python3 "$repo_root/scripts/release-sidecar.py" --archive "$archive" \
  --private-key "$scratch/private.pem" --public-key "$scratch/public.pem" \
  --out-dir "$scratch/signed" > "$scratch/sign-result.json"
release="$scratch/signed/release.json"
signature="$scratch/signed/release.json.sig"
os="$(python3 -c 'import platform; print(platform.system().lower())')"
arch="$(python3 -c 'import platform; print({"aarch64":"arm64","x86_64":"amd64"}.get(platform.machine().lower(), platform.machine().lower()))')"
verify() {
  python3 "$repo_root/scripts/verify-release.py" --public-key "$1" --release "$2" \
    --signature "$signature" --archive "$3" --target-os "$4" --target-arch "$arch" > /dev/null 2>&1
}
must_reject() {
  if verify "$@"; then echo "expected release verification rejection" >&2; exit 1; fi
}
verify "$scratch/public.pem" "$release" "$archive" "$os"
must_reject "$scratch/other-public.pem" "$release" "$archive" "$os"
cp "$release" "$scratch/changed/release.json"
python3 - "$scratch/changed/release.json" <<'PY'
from pathlib import Path
import sys
p = Path(sys.argv[1]); p.write_bytes(p.read_bytes().replace(b'test-signed-preview', b'test-signed-previEW'))
PY
must_reject "$scratch/public.pem" "$scratch/changed/release.json" "$archive" "$os"
cp "$archive" "$scratch/changed/$(basename "$archive")"
python3 - "$scratch/changed/$(basename "$archive")" <<'PY'
from pathlib import Path
import sys
p = Path(sys.argv[1]); data = bytearray(p.read_bytes()); data[-8] ^= 1; p.write_bytes(data)
PY
must_reject "$scratch/public.pem" "$release" "$scratch/changed/$(basename "$archive")" "$os"
cp "$archive" "$scratch/changed/another.tar.gz"
must_reject "$scratch/public.pem" "$release" "$scratch/changed/another.tar.gz" "$os"
other_os=linux; if [[ "$os" == linux ]]; then other_os=darwin; fi
must_reject "$scratch/public.pem" "$release" "$archive" "$other_os"
# Only the independently verified archive may now be unpacked and execute cks.
tar -xzf "$archive" -C "$scratch/unpacked"
stage="$scratch/unpacked/$(basename "$archive" .tar.gz)"
"$stage/cks" package verify --public-key "$scratch/public.pem" --release "$release" \
  --signature "$signature" --archive "$archive" --target-os "$os" --target-arch "$arch" \
  > "$scratch/after-install.json"
if "$stage/cks" package verify --public-key "$scratch/other-public.pem" --release "$release" \
  --signature "$signature" --archive "$archive" --target-os "$os" --target-arch "$arch" \
  > /dev/null 2>&1; then
  echo "installed verifier accepted an unrelated public key" >&2; exit 1
fi
if "$stage/cks" package verify --public-key "$scratch/public.pem" --release "$release" \
  --signature "$signature" --archive "$scratch/changed/$(basename "$archive")" \
  --target-os "$os" --target-arch "$arch" > /dev/null 2>&1; then
  echo "installed verifier accepted a modified archive" >&2; exit 1
fi
python3 - "$scratch/after-install.json" <<'PY'
import json, sys
assert json.load(open(sys.argv[1]))["status"] == "verified"
PY
printf 'Signed host preview verification passed: %s\n' "$archive"
