#!/usr/bin/env python3
"""Verify a signed release sidecar and all archive hashes before extraction.

This script never executes an archive binary. Trust the script and public key
through an independent channel before using either with an untrusted archive.
Requires Python 3 and OpenSSL with Ed25519 support.
"""

import argparse
import hashlib
import json
import platform
import posixpath
import subprocess
import tarfile
from pathlib import Path


EXPECTED_KEYS = {
    "schema_version", "signature_algorithm", "scope", "key_id", "archive_name",
    "archive_sha256", "archive_bytes", "host_os", "host_arch", "commit",
    "manifest_sha256", "modules_sha256", "license_sha256", "license_inventory_sha256",
}
MAX_ARCHIVE_BYTES = 512 * 1024 * 1024
MAX_MEMBER_BYTES = 256 * 1024 * 1024


def sha256(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def digest_stream(stream) -> str:
    value = hashlib.sha256()
    for chunk in iter(lambda: stream.read(1024 * 1024), b""):
        value.update(chunk)
    return value.hexdigest()


def canonical_release(data: bytes) -> dict:
    def unique_keys(pairs):
        result = {}
        for key, value in pairs:
            if key in result:
                raise ValueError(f"duplicate signed field: {key}")
            result[key] = value
        return result
    release = json.loads(data, object_pairs_hook=unique_keys)
    if not isinstance(release, dict) or set(release) != EXPECTED_KEYS:
        raise ValueError("release manifest fields differ from the signed contract")
    encoded = (json.dumps(release, sort_keys=True, separators=(",", ":"), ensure_ascii=False) + "\n").encode()
    if encoded != data:
        raise ValueError("release manifest is not canonical JSON")
    if release["schema_version"] != 1 or release["signature_algorithm"] != "Ed25519" or \
       release["scope"] != "test-signed-preview":
        raise ValueError("unsupported release contract or scope")
    for field in ("key_id", "archive_sha256", "manifest_sha256", "modules_sha256", "license_sha256", "license_inventory_sha256"):
        value = release[field]
        if not isinstance(value, str) or len(value) != 64 or any(c not in "0123456789abcdef" for c in value):
            raise ValueError(f"invalid {field}")
    if type(release["archive_bytes"]) is not int or release["archive_bytes"] <= 0 or \
       release["archive_bytes"] > MAX_ARCHIVE_BYTES:
        raise ValueError("archive exceeds release size bound")
    if not isinstance(release["archive_name"], str) or not release["archive_name"].isascii() or \
       not release["archive_name"].endswith(".tar.gz") or "/" in release["archive_name"]:
        raise ValueError("unsafe archive name")
    if release["host_os"] not in ("darwin", "linux") or release["host_arch"] not in ("arm64", "amd64"):
        raise ValueError("unsupported target")
    if not isinstance(release["commit"], str) or len(release["commit"]) != 40 or \
       any(c not in "0123456789abcdef" for c in release["commit"]):
        raise ValueError("invalid source commit")
    return release


def verify(public_key: Path, release_path: Path, signature_path: Path, archive: Path,
           target_os: str, target_arch: str) -> dict:
    if signature_path.stat().st_size != 64:
        raise ValueError("Ed25519 signature must be 64 bytes")
    result = subprocess.run(
        ["openssl", "pkeyutl", "-verify", "-pubin", "-inkey", str(public_key),
         "-rawin", "-in", str(release_path), "-sigfile", str(signature_path)],
        capture_output=True,
    )
    if result.returncode != 0:
        raise ValueError("release signature verification failed")
    release = canonical_release(release_path.read_bytes())
    public_der = subprocess.check_output(
        ["openssl", "pkey", "-pubin", "-in", str(public_key), "-outform", "DER"],
        stderr=subprocess.DEVNULL,
    )
    if sha256(public_der) != release["key_id"]:
        raise ValueError("trusted public key differs from release key ID")
    if archive.name != release["archive_name"] or archive.stat().st_size != release["archive_bytes"]:
        raise ValueError("archive name or size differs from signed release")
    if release["host_os"] != target_os or release["host_arch"] != target_arch:
        raise ValueError("archive target differs from requested runtime")
    with archive.open("rb") as stream:
        if digest_stream(stream) != release["archive_sha256"]:
            raise ValueError("archive bytes differ from signed release")
    with tarfile.open(archive, "r:gz") as tar:
        members = tar.getmembers()
        if not members or len(members) > 100 or sum(item.size for item in members) > MAX_ARCHIVE_BYTES:
            raise ValueError("archive member count or total size exceeds bound")
        names = set()
        roots = set()
        for item in members:
            name = item.name
            if not name or name.startswith("/") or "\\" in name or posixpath.normpath(name) != name or \
               ".." in name.split("/") or item.size > MAX_MEMBER_BYTES or \
               not (item.isfile() or item.isdir()) or name in names:
                raise ValueError("unsafe or duplicate archive member")
            names.add(name)
            roots.add(name.split("/")[0])
        if len(roots) != 1:
            raise ValueError("archive has multiple roots")
        root = roots.pop()
        def member(relative: str) -> bytes:
            item = tar.getmember(root + "/" + relative)
            if not item.isfile():
                raise ValueError(f"{relative} is not a regular archive file")
            stream = tar.extractfile(item)
            assert stream is not None
            return stream.read()
        manifest_bytes = member("manifest.json")
        if sha256(manifest_bytes) != release["manifest_sha256"] or \
           sha256(member("modules.txt")) != release["modules_sha256"] or \
           sha256(member("LICENSE")) != release["license_sha256"] or \
           sha256(member("third-party-licenses.json")) != release["license_inventory_sha256"]:
            raise ValueError("internal release metadata differs from signed hashes")
        manifest = json.loads(manifest_bytes)
        if manifest.get("commit") != release["commit"] or manifest.get("host_os") != target_os or \
           manifest.get("host_arch") != target_arch or manifest.get("scope") != "host-preview":
            raise ValueError("internal package manifest differs from signed target")
        for binary in ("cks", "ckg", "ckv"):
            item = tar.getmember(root + "/" + binary)
            if not item.isfile() or not item.mode & 0o111:
                raise ValueError(f"{binary} is missing or not executable")
            if sha256(member(binary)) != manifest["binaries"][binary]["sha256"]:
                raise ValueError(f"{binary} differs from internal package manifest")
        for required in ("INSTALLATION.md", "policies/sanitization_rules.yaml"):
            member(required)
        inventory = json.loads(member("third-party-licenses.json"))
        if inventory.get("schema_version") != 1 or inventory.get("review_status") != "pending" or \
           not isinstance(inventory.get("modules"), list) or \
           not isinstance(inventory.get("vendored_assets", []), list):
            raise ValueError("invalid license inventory")
        if manifest.get("third_party_license_inventory", {}).get("sha256") != release["license_inventory_sha256"]:
            raise ValueError("package license inventory digest differs")
        for entry in inventory["modules"] + inventory.get("vendored_assets", []):
            for license_file in entry["license_files"]:
                if sha256(member(license_file["path"])) != license_file["sha256"]:
                    raise ValueError("third-party license file differs from inventory")
    return {"status": "verified", "scope": release["scope"], "archive": str(archive),
            "target": target_os + "/" + target_arch, "commit": release["commit"],
            "key_id": release["key_id"]}


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--public-key", type=Path, required=True)
    parser.add_argument("--release", type=Path, required=True)
    parser.add_argument("--signature", type=Path, required=True)
    parser.add_argument("--archive", type=Path, required=True)
    parser.add_argument("--target-os", default=platform.system().lower())
    parser.add_argument("--target-arch", default={"aarch64": "arm64", "x86_64": "amd64"}.get(platform.machine().lower(), platform.machine().lower()))
    args = parser.parse_args()
    print(json.dumps(verify(args.public_key, args.release, args.signature, args.archive,
                            args.target_os, args.target_arch), sort_keys=True))


if __name__ == "__main__":
    main()
