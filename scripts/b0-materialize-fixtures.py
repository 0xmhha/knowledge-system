#!/usr/bin/env python3
"""Materialize frozen B0 fixture sources; never score or approve their facts."""

import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import subprocess
import sys
import tempfile


ROOT = Path(__file__).resolve().parent.parent
DEFAULT = ROOT / "system/eval/b0-knowledge-system/dynamic-fixtures-m2max-draft.json"
DATE = "2026-10-03T00:00:00+00:00"


def digest(raw):
    return hashlib.sha256(raw).hexdigest()


def safe_path(value):
    if not isinstance(value, str) or not value or "\\" in value:
        raise ValueError("invalid source path")
    path = PurePosixPath(value)
    if path.is_absolute() or any(p in {"", ".", "..", ".git"} for p in value.split("/")):
        raise ValueError("source path must be normalized, relative and outside .git")
    return path


def validate(book, partition, allow_draft):
    if book.get("schema_version") != 1:
        raise ValueError("unsupported fixture manifest schema")
    fixtures = book.get("fixtures")
    if not isinstance(fixtures, list) or not fixtures:
        raise ValueError("fixture manifest is empty")
    selected, ids = [], set()
    for fixture in fixtures:
        fid = fixture.get("id", "")
        if not re.fullmatch(r"F-0[1-6]-(DEV|FINAL)", fid) or fid in ids:
            raise ValueError("invalid or duplicate fixture ID")
        ids.add(fid)
        expected_partition = "development" if fid.endswith("DEV") else "final"
        if fixture.get("evaluation_partition") != expected_partition or fixture.get("family") != fid[:4]:
            raise ValueError("fixture family/partition mismatch")
        if partition != "all" and expected_partition != partition:
            continue
        approved = (fixture.get("review_state") == "approved" and
                    bool(fixture.get("reviewer")) and bool(fixture.get("reviewed_at")))
        if not approved and not allow_draft:
            raise ValueError("fixture review is pending; --allow-draft prepares diagnostic inputs only")
        sources, hashes = fixture.get("sources"), fixture.get("source_sha256")
        if not isinstance(sources, dict) or not sources or not isinstance(hashes, dict) or sources.keys() != hashes.keys():
            raise ValueError("source/hash inventory mismatch")
        for path, content in sources.items():
            safe_path(path)
            if not isinstance(content, str) or digest(content.encode("utf-8")) != hashes[path]:
                raise ValueError(f"source hash mismatch: {fid}/{path}")
        if fixture["family"] in {"F-04", "F-05"}:
            prefixes = {"state-old", "state-new"} if fixture["family"] == "F-04" else {"project-a", "project-b"}
            if "go.mod" not in sources or any(p != "go.mod" and p.split("/")[0] not in prefixes for p in sources):
                raise ValueError("state/project source layout mismatch")
            if any(not any(p.startswith(prefix + "/") for p in sources) for prefix in prefixes):
                raise ValueError("missing fixture state/project")
        selected.append(fixture)
    if not selected:
        raise ValueError("partition contains no fixtures")
    return selected


def git(repo, *args, raw=False):
    # Never inherit alternate object directories, user hooks, signing keys or
    # Git config that could make the fixture depend on this workstation.
    env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
    env.update(GIT_CONFIG_NOSYSTEM="1", GIT_CONFIG_GLOBAL=os.devnull,
               GIT_AUTHOR_NAME="B0 synthetic fixture", GIT_AUTHOR_EMAIL="b0@example.invalid",
               GIT_COMMITTER_NAME="B0 synthetic fixture", GIT_COMMITTER_EMAIL="b0@example.invalid",
               GIT_AUTHOR_DATE=DATE, GIT_COMMITTER_DATE=DATE, LC_ALL="C")
    command = ["git", "-c", "core.hooksPath=" + os.devnull, "-c", "commit.gpgsign=false",
               "-c", "core.autocrlf=false", *args]
    result = subprocess.run(command, cwd=repo, env=env, check=True,
                            capture_output=True, text=not raw).stdout
    return result if raw else result.strip()


def write_sources(repo, sources):
    for relative, content in sources.items():
        target = repo.joinpath(*safe_path(relative).parts)
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_bytes(content.encode("utf-8"))
        target.chmod(0o644)


def commit(repo, sources, message):
    write_sources(repo, sources)
    git(repo, "add", "--all")
    git(repo, "commit", "--quiet", "-m", message)
    if git(repo, "status", "--porcelain"):
        raise ValueError("fixture worktree is dirty after commit")
    if (repo / ".git/objects/info/alternates").exists():
        raise ValueError("fixture must have independent Git objects")
    return {"commit": git(repo, "rev-parse", "HEAD"), "tree": git(repo, "rev-parse", "HEAD^{tree}"),
            "source_sha256": {p: digest(c.encode("utf-8")) for p, c in sorted(sources.items())}}


def init_repo(path):
    path.mkdir(parents=True)
    git(path, "init", "--quiet", "-b", "main")


def split_sources(sources, prefix):
    return {"go.mod": sources["go.mod"], **{p[len(prefix) + 1:]: c for p, c in sources.items() if p.startswith(prefix + "/")}}


def materialize(manifest_path, output, partition="development", allow_draft=False):
    raw = manifest_path.read_bytes()
    selected = validate(json.loads(raw), partition, allow_draft)
    if output.exists() or output.is_symlink():
        raise ValueError("output already exists; fixture inputs are immutable")
    output.parent.mkdir(parents=True, exist_ok=True)
    # Reserve the destination exclusively; readers use materialization.json
    # as the publication marker. Never rename over another creator's directory.
    output.mkdir()
    temp = Path(tempfile.mkdtemp(prefix=".staging-", dir=output))
    records = []
    try:
        for fixture in selected:
            fid, family, sources = fixture["id"], fixture["family"], fixture["sources"]
            record = {"id": fid, "family": family, "evaluation_partition": fixture["evaluation_partition"],
                      "review_state": fixture["review_state"], "reviewer": fixture.get("reviewer"),
                      "reviewed_at": fixture.get("reviewed_at"), "states": []}
            states = [("old", "state-old"), ("new", "state-new")] if family == "F-04" else (
                [("a", "project-a"), ("b", "project-b")] if family == "F-05" else [("current", None)])
            for name, prefix in states:
                relative = f"{fid}/repo" if family != "F-05" else f"{fid}/project-{name}"
                repo = temp / relative
                if not repo.exists():
                    init_repo(repo)
                content = split_sources(sources, prefix) if prefix else sources
                info = commit(repo, content, f"{fid} frozen {name}")
                project = fid.lower() + (f"-{name}" if family == "F-05" else "")
                record["states"].append({"name": name, "repository": relative, "project_id": project, **info})
            records.append(record)
        report = {"schema_version": 1, "gate": "B0-fixture-materialization",
                  "input_manifest_sha256": digest(raw), "partition": partition,
                  "diagnostic_only": any(f["review_state"] != "approved" or not f.get("reviewer") or
                                         not f.get("reviewed_at") for f in selected),
                  "quality_metrics": None, "fixtures": records}
        (temp / "materialization.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        for child in sorted(temp.iterdir(), key=lambda p: p.name == "materialization.json"):
            os.rename(child, output / child.name)
        temp.rmdir()
        return report
    except BaseException:
        shutil.rmtree(output, ignore_errors=True)
        raise


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--manifest", type=Path, default=DEFAULT)
    parser.add_argument("--out-dir", type=Path, required=True)
    parser.add_argument("--partition", choices=["development", "final", "all"], default="development")
    parser.add_argument("--allow-draft", action="store_true", help="prepare unapproved diagnostic inputs; does not authorize scoring")
    args = parser.parse_args()
    result = materialize(args.manifest, args.out_dir, args.partition, args.allow_draft)
    print(json.dumps({"fixture_count": len(result["fixtures"]), "diagnostic_only": result["diagnostic_only"],
                      "input_manifest_sha256": result["input_manifest_sha256"]}))


if __name__ == "__main__":
    try:
        main()
    except (OSError, ValueError, subprocess.CalledProcessError) as exc:
        print(f"b0-materialize-fixtures: {exc}", file=sys.stderr)
        sys.exit(1)
