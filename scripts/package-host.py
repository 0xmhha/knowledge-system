#!/usr/bin/env python3
"""Build a host preview archive of ckg, ckv, and cks with integrity metadata.

This is one-host packaging, not a cross-platform release certification. Native
runtime dependencies and third-party licenses still need target-host review.
"""

import argparse
import gzip
import hashlib
import json
import os
import platform
import shutil
import subprocess
import sys
import tarfile
import tempfile
from pathlib import Path

# Packaging must not create __pycache__ inside the source tree before its
# clean-check, including the local license inventory helper import below.
sys.dont_write_bytecode = True
from license_inventory import collect as collect_licenses


ROOT = Path(__file__).resolve().parent.parent
BINARIES = ("ckg", "ckv", "cks")


def run(*args: str) -> str:
    return subprocess.check_output(args, cwd=ROOT, text=True).strip()


def digest(path: Path) -> str:
    sha = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(1024 * 1024), b""):
            sha.update(block)
    return sha.hexdigest()


def native_dependencies(binary: Path) -> list[str]:
    command = ["otool", "-L", str(binary)] if platform.system() == "Darwin" else ["ldd", str(binary)]
    if not shutil.which(command[0]):
        return [f"dependency inspector {command[0]} unavailable"]
    try:
        return subprocess.check_output(command, text=True, stderr=subprocess.STDOUT).splitlines()[1:]
    except subprocess.CalledProcessError as exc:
        return [f"dependency inspection failed: {exc.output.strip()}"]


def package(out_dir: Path, allow_dirty: bool) -> Path:
    dirty = bool(run("git", "status", "--porcelain"))
    if dirty and not allow_dirty:
        raise RuntimeError("working tree is dirty; commit changes before packaging or pass --allow-dirty for a preview")
    # Keep stdout a single JSON object for callers that automate packaging.
    # The build transcript is shown on failure without corrupting that contract.
    build = subprocess.run(["make", "build-bins"], cwd=ROOT, text=True, capture_output=True)
    if build.returncode != 0:
        raise RuntimeError(f"build-bins failed:\n{build.stdout}{build.stderr}")
    commit = run("git", "rev-parse", "HEAD")
    system = platform.system().lower()
    machine = {"aarch64": "arm64", "x86_64": "amd64"}.get(platform.machine().lower(), platform.machine().lower())
    name = f"knowledge-system-{system}-{machine}-{commit[:12]}"
    if dirty:
        name += "-dirty-preview"
    out_dir.mkdir(parents=True, exist_ok=True)
    archive = out_dir / f"{name}.tar.gz"
    if archive.exists():
        raise FileExistsError(f"archive already exists: {archive}")
    with tempfile.TemporaryDirectory(prefix="ks-package-", dir=out_dir) as temp:
        stage = Path(temp) / name
        stage.mkdir()
        metadata = {
            "format_version": 1,
            "scope": "host-preview",
            "commit": commit,
            "dirty": dirty,
            "host_os": system,
            "host_arch": machine,
            "go_version": run("go", "version"),
            "binaries": {},
            "native_dependencies": {},
            "third_party_license_review": "pending",
        }
        for binary_name in BINARIES:
            source = ROOT / "bin" / binary_name
            if not source.is_file():
                raise FileNotFoundError(source)
            target = stage / binary_name
            shutil.copy2(source, target)
            metadata["binaries"][binary_name] = {"sha256": digest(target), "bytes": target.stat().st_size}
            metadata["native_dependencies"][binary_name] = native_dependencies(target)
        shutil.copy2(ROOT / "LICENSE", stage / "LICENSE")
        shutil.copy2(ROOT / "docs" / "spec-driven" / "INSTALLATION-PILOT.md", stage / "INSTALLATION.md")
        policy = stage / "policies" / "sanitization_rules.yaml"
        policy.parent.mkdir()
        shutil.copy2(ROOT / "system" / "policies" / "sanitization_rules.yaml", policy)
        # Built-in Go build info lists modules actually linked into each
        # binary and works offline even when unused go.sum modules are absent.
        # Go prints its argument as the build-info heading. A random staging
        # directory here would make identical package bytes hash differently.
        # Relative names preserve the actual linked build info without that path.
        module_info = "\n".join(subprocess.check_output(
            ["go", "version", "-m", name], cwd=stage, text=True).strip()
            for name in BINARIES)
        (stage / "modules.txt").write_text(module_info + "\n")
        metadata["third_party_license_inventory"] = collect_licenses(
            module_info, Path(run("go", "env", "GOMODCACHE")), stage,
            [(name, ROOT / name) for name in (
                "internal/graph/parse/solidity/binding",
                "internal/vector/parse/solidity/binding")])
        (stage / "manifest.json").write_text(json.dumps(metadata, indent=2, sort_keys=True) + "\n")
        with archive.open("wb") as raw:
            with gzip.GzipFile(filename="", mode="wb", fileobj=raw, mtime=0) as zipped:
                with tarfile.open(fileobj=zipped, mode="w", format=tarfile.PAX_FORMAT) as tar:
                    for path in sorted(stage.rglob("*")):
                        info = tar.gettarinfo(str(path), arcname=str(Path(name) / path.relative_to(stage)))
                        info.uid = info.gid = 0
                        info.uname = info.gname = ""
                        info.mtime = 0
                        if path.is_file():
                            with path.open("rb") as stream:
                                tar.addfile(info, stream)
                        else:
                            tar.addfile(info)
    return archive


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--out-dir", type=Path, required=True)
    parser.add_argument("--allow-dirty", action="store_true", help="mark the artifact as a dirty development preview")
    args = parser.parse_args()
    archive = package(args.out_dir.resolve(), args.allow_dirty)
    print(json.dumps({"archive": str(archive), "sha256": digest(archive)}))


if __name__ == "__main__":
    main()
