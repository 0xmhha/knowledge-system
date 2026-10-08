#!/usr/bin/env python3
"""Sign an existing host archive's exact bytes with an out-of-repo Ed25519 key."""

import argparse
import hashlib
import json
import subprocess
import tarfile
from pathlib import Path


def sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def digest_file(path: Path) -> str:
    digest = hashlib.sha256()
    with path.open("rb") as stream:
        for chunk in iter(lambda: stream.read(1024 * 1024), b""):
            digest.update(chunk)
    return digest.hexdigest()


def member_bytes(tar: tarfile.TarFile, suffix: str) -> bytes:
    matches = [item for item in tar.getmembers() if "/" in item.name and item.name.split("/", 1)[1] == suffix]
    if len(matches) != 1 or not matches[0].isfile():
        raise ValueError(f"archive needs one regular {suffix}")
    stream = tar.extractfile(matches[0])
    assert stream is not None
    return stream.read()


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--archive", type=Path, required=True)
    parser.add_argument("--private-key", type=Path, required=True)
    parser.add_argument("--public-key", type=Path, required=True)
    parser.add_argument("--out-dir", type=Path, required=True)
    args = parser.parse_args()
    archive = args.archive.resolve(strict=True)
    if not archive.name.isascii() or not archive.name.endswith(".tar.gz"):
        raise ValueError("archive name must be ASCII .tar.gz")
    with tarfile.open(archive, "r:gz") as tar:
        package_manifest = member_bytes(tar, "manifest.json")
        modules = member_bytes(tar, "modules.txt")
        license_text = member_bytes(tar, "LICENSE")
        license_inventory = member_bytes(tar, "third-party-licenses.json")
    package = json.loads(package_manifest)
    if package.get("scope") != "host-preview":
        raise ValueError("this signer accepts only a clearly marked host preview")
    public_der = subprocess.check_output(
        ["openssl", "pkey", "-pubin", "-in", str(args.public_key), "-outform", "DER"],
        stderr=subprocess.DEVNULL,
    )
    derived_der = subprocess.check_output(
        ["openssl", "pkey", "-in", str(args.private_key), "-pubout", "-outform", "DER"],
        stderr=subprocess.DEVNULL,
    )
    if derived_der != public_der:
        raise ValueError("private and public signing keys do not match")
    release = {
        "schema_version": 1,
        "signature_algorithm": "Ed25519",
        "scope": "test-signed-preview",
        "key_id": sha256(public_der),
        "archive_name": archive.name,
        "archive_sha256": digest_file(archive),
        "archive_bytes": archive.stat().st_size,
        "host_os": package["host_os"],
        "host_arch": package["host_arch"],
        "commit": package["commit"],
        "manifest_sha256": sha256(package_manifest),
        "modules_sha256": sha256(modules),
        "license_sha256": sha256(license_text),
        "license_inventory_sha256": sha256(license_inventory),
    }
    data = (json.dumps(release, sort_keys=True, separators=(",", ":"), ensure_ascii=False) + "\n").encode()
    args.out_dir.mkdir(parents=True, exist_ok=True)
    release_path = args.out_dir / "release.json"
    signature_path = args.out_dir / "release.json.sig"
    with release_path.open("xb") as stream:
        stream.write(data)
    try:
        subprocess.run(
            ["openssl", "pkeyutl", "-sign", "-inkey", str(args.private_key), "-rawin",
             "-in", str(release_path), "-out", str(signature_path)],
            check=True, capture_output=True,
        )
    except Exception:
        release_path.unlink(missing_ok=True)
        signature_path.unlink(missing_ok=True)
        raise
    print(json.dumps({"release": str(release_path), "signature": str(signature_path),
                      "archive": str(archive), "key_id": release["key_id"]}))


if __name__ == "__main__":
    main()
