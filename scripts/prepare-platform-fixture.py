#!/usr/bin/env python3
"""Copy current tracked bytes to a new fixture, excluding Git and untracked files."""

import argparse
import os
from pathlib import Path
import shutil
import subprocess


def prepare(source, output):
    source = source.resolve()
    output = output.absolute()
    if output == source or source in output.parents:
        raise ValueError("output must be outside the source repository")
    env = {k: v for k, v in os.environ.items() if not k.startswith("GIT_")}
    env.update(GIT_CONFIG_NOSYSTEM="1", GIT_CONFIG_GLOBAL=os.devnull)
    raw = subprocess.check_output(["git", "-c", f"safe.directory={source}", "-C", str(source),
                                   "ls-files", "-z"], env=env)
    paths = [os.fsdecode(p) for p in raw.split(b"\0") if p]
    if not paths:
        raise ValueError("tracked source is empty")
    output.mkdir(parents=True)  # Exclusive: never alter an existing destination.
    try:
        count = 0
        for relative in paths:
            parts = Path(relative).parts
            if Path(relative).is_absolute() or ".." in parts or ".git" in parts:
                raise ValueError("unsafe tracked path")
            original, target = source / relative, output / relative
            if not original.exists() and not original.is_symlink():
                continue  # Preserve tracked working-tree deletions.
            if original.is_dir():
                raise ValueError("submodules require a separate fixture input policy")
            target.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(original, target, follow_symlinks=False)
            count += 1
        return count
    except BaseException:
        shutil.rmtree(output)
        raise


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source", type=Path, required=True)
    parser.add_argument("--out", type=Path, required=True)
    args = parser.parse_args()
    print(prepare(args.source, args.out))
