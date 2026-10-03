#!/usr/bin/env python3
"""Create a B0 source checkout with its own Git objects and reflogs.

CKG intentionally indexes locally abandoned commits for recovery. A Git
worktree shares those inputs with its parent, so it cannot pin a benchmark.
"""

import argparse
import json
import os
from pathlib import Path
import re
import shutil
import subprocess
import sys
import tempfile


SHA40 = re.compile(r"^[0-9a-f]{40}$")


def git(repo: Path, *args: str, check: bool = True) -> str:
    completed = subprocess.run(
        ["git", "-C", str(repo), *args], text=True, capture_output=True,
        check=False,
    )
    if check and completed.returncode:
        raise ValueError(f"git {' '.join(args)}: {completed.stderr.strip()}")
    return completed.stdout.strip()


def create(source: Path, out: Path, commit: str, tree: str) -> dict:
    if not SHA40.fullmatch(commit) or not SHA40.fullmatch(tree):
        raise ValueError("commit and tree must be full lowercase Git SHA-1 IDs")
    source = source.resolve(strict=True)
    if not source.is_dir() or git(source, "rev-parse", "--is-inside-work-tree") != "true":
        raise ValueError("source must be a Git worktree")
    if out.exists() or out.is_symlink():
        raise ValueError(f"output already exists: {out}")
    out.parent.mkdir(parents=True, exist_ok=True)
    stage = Path(tempfile.mkdtemp(prefix=out.name + "-stage-", dir=out.parent))
    try:
        git(stage, "init", "--quiet")
        # Fetch only this commit and its ancestry. No remote ref, alternate,
        # or shared worktree metadata is installed in the new repository.
        git(stage, "-c", "protocol.file.allow=always", "fetch", "--no-tags",
            "--no-write-fetch-head", str(source), commit)
        git(stage, "checkout", "--quiet", "--detach", commit)
        actual_commit = git(stage, "rev-parse", "HEAD")
        actual_tree = git(stage, "rev-parse", "HEAD^{tree}")
        if (actual_commit, actual_tree) != (commit, tree):
            raise ValueError("fetched commit/tree differs from the pinned B0 input")
        if git(stage, "status", "--porcelain"):
            raise ValueError("isolated checkout is dirty")
        common_dir = Path(git(stage, "rev-parse", "--path-format=absolute", "--git-common-dir"))
        if common_dir.resolve() != (stage / ".git").resolve():
            raise ValueError("Git object store is not independent")
        if (stage / ".git/objects/info/alternates").exists():
            raise ValueError("Git alternates would share source objects")
        # A fixed HEAD may have ancestors in its reflog; only commits not
        # reachable from HEAD are forbidden in the isolated evaluation input.
        fsck = subprocess.run(
            ["git", "-C", str(stage), "fsck", "--no-reflogs", "--unreachable"],
            text=True, capture_output=True, check=False,
        )
        if fsck.returncode:
            raise ValueError(f"isolated Git object check failed: {fsck.stderr.strip()}")
        unreachable = fsck.stdout + "\n" + fsck.stderr
        if any(line.startswith(("unreachable commit ", "dangling commit "))
               for line in unreachable.splitlines()):
            raise ValueError("isolated repository contains unreachable commits")
        if out.exists() or out.is_symlink():
            raise ValueError(f"output appeared during build: {out}")
        os.rename(stage, out)
        return {"status": "isolated", "path": str(out.resolve()),
                "commit": commit, "tree": tree, "unreachable_commits": 0}
    finally:
        if stage.exists():
            shutil.rmtree(stage)


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=Path, required=True,
                        help="Git repository containing the pinned commit")
    parser.add_argument("--out", type=Path, required=True,
                        help="new, nonexistent directory for the isolated checkout")
    parser.add_argument("--commit", required=True, help="full pinned commit SHA")
    parser.add_argument("--tree", required=True, help="full expected tree SHA")
    args = parser.parse_args()
    try:
        print(json.dumps(create(args.source, args.out, args.commit, args.tree)))
        return 0
    except (OSError, ValueError) as exc:
        print(f"b0-isolate-corpus: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    sys.exit(main())
