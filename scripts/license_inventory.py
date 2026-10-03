#!/usr/bin/env python3
"""Collect linked module root and explicitly selected vendored asset notices.

This is an evidence inventory, not an automated license compatibility ruling.
Missing license files remain explicit review items in the manifest.
"""

import hashlib
import json
from pathlib import Path


PREFIXES = ("license", "licence", "copying", "notice")


def digest(data: bytes) -> str:
    return hashlib.sha256(data).hexdigest()


def module_cache_name(module: str) -> str:
    return "".join("!" + c.lower() if c.isupper() else c for c in module)


def collect(module_info: str, module_cache: Path, stage: Path, vendored_assets=()) -> dict:
    modules = set()
    for line in module_info.splitlines():
        parts = line.strip().split()
        if len(parts) >= 3 and parts[0] == "dep":
            modules.add((parts[1], parts[2]))
    if not modules:
        raise ValueError("no linked third-party Go modules in package build info")
    entries = []
    for module, version in sorted(modules):
        root = module_cache / f"{module_cache_name(module)}@{version}"
        files = []
        if root.is_dir():
            candidates = sorted(p for p in root.iterdir() if p.is_file() and p.name.lower().startswith(PREFIXES))
            for source in candidates:
                data = source.read_bytes()
                if not data or len(data) > 2 * 1024 * 1024:
                    raise ValueError(f"unexpected license file size: {module}@{version}/{source.name}")
                key = digest(f"{module}@{version}".encode())[:16]
                target = stage / "third-party-licenses" / key / source.name
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_bytes(data)
                files.append({"path": str(target.relative_to(stage)), "sha256": digest(data)})
        entries.append({"module": module, "version": version, "license_files": files,
                        "review_status": "collected" if files else "missing_source_license"})
    assets = []
    seen = set()
    for name, root in vendored_assets:
        if name in seen:
            raise ValueError(f"duplicate vendored asset: {name}")
        seen.add(name)
        root = Path(root)
        files = []
        sources = []
        for source in sorted(root.rglob("*")):
            if not source.is_file():
                continue
            if source.name.lower().startswith(PREFIXES):
                data = source.read_bytes()
                if not data or len(data) > 2 * 1024 * 1024:
                    raise ValueError(f"unexpected vendored license file size: {name}/{source.name}")
                target = stage / "third-party-licenses" / ("vendored-" + digest(name.encode())[:16]) / source.relative_to(root)
                target.parent.mkdir(parents=True, exist_ok=True)
                target.write_bytes(data)
                files.append({"path": str(target.relative_to(stage)), "sha256": digest(data)})
            if source.suffix in (".c", ".h", ".go"):
                sources.append({"path": str(source.relative_to(root)), "sha256": digest(source.read_bytes())})
        assets.append({"asset": name, "license_files": files, "source_files": sources,
                       "review_status": "collected" if files else "missing_source_license"})
    inventory = {"schema_version": 1, "review_status": "pending", "modules": entries,
                 "vendored_assets": assets}
    data = (json.dumps(inventory, sort_keys=True, indent=2) + "\n").encode()
    (stage / "third-party-licenses.json").write_bytes(data)
    return {"sha256": digest(data), "module_count": len(entries),
            "missing_license_count": sum(not entry["license_files"] for entry in entries),
            "vendored_asset_count": len(assets),
            "missing_vendored_license_count": sum(not asset["license_files"] for asset in assets)}
