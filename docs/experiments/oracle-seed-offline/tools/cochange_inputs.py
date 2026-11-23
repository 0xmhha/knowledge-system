"""Write the question list of the co-change experiment (DESIGN.md section 10).

For the stage 1 fixes and the buildable stage 2 candidates, list the hunks of
the fix diff on existing non-test Go files as base-side line ranges. Hunks whose
changed lines are all blank or // comments are dropped. Only hashes, paths and
line numbers are written. The runner maps the lines to declarations.

Usage:
    python3 -I cochange_inputs.py --repo <go-stablenet> --data <data dir> --out <json>
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
from pathlib import Path

GIT_TIMEOUT_SECONDS = 60
MIN_CHANGED_LINES = 3
HUNK_RE = re.compile(r"^@@ -(\d+)(?:,(\d+))? \+\d+(?:,\d+)? @@")


def git(repo: Path, *args: str) -> str:
    """Run git in repo and return stdout."""
    done = subprocess.run(
        ["git", "-C", str(repo), *args], check=True, capture_output=True, text=True, timeout=GIT_TIMEOUT_SECONDS
    )
    return done.stdout


def buildable(repo: Path, base: str) -> bool:
    """Report whether base has the paths the frozen scope profile needs."""
    files = git(repo, "ls-tree", "-r", "--name-only", base).splitlines()
    return any(f.startswith("systemcontracts/") and f.endswith(".sol") for f in files) and any(
        f.startswith("cmd/gstable/") for f in files
    )


def trivial(lines: list[str]) -> bool:
    """Report whether every changed line is blank or a // comment."""
    return all(not t or t.startswith("//") for t in (line[1:].strip() for line in lines))


def hunks(repo: Path, base: str, fix: str) -> list[dict[str, int | str | bool]]:
    """Return the non-trivial base-side ranges of the fix on modified Go files."""
    names = git(repo, "diff", "--no-renames", "--diff-filter=M", "--name-only", base, fix, "--", "*.go", ":!*_test.go")
    out: list[dict[str, int | str | bool]] = []
    for path in names.splitlines():
        head: tuple[int, int] | None = None
        body: list[str] = []
        for line in [*git(repo, "diff", "--no-renames", "-U0", base, fix, "--", path).splitlines(), "@@ end"]:
            if line.startswith("@@"):
                if head and not trivial(body):
                    start, count = head
                    out.append({"file": path, "start": start, "end": max(start, start + count - 1), "insert": count == 0})
                m = HUNK_RE.match(line)
                head = (int(m.group(1)), int(m.group(2)) if m.group(2) is not None else 1) if m else None
                body = []
            elif head and line[:1] in "+-":
                body.append(line)
    return out


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    for name in ("repo", "data", "out"):
        ap.add_argument(f"--{name}", type=Path, required=True)
    args = ap.parse_args()
    bases = json.loads((args.data / "base-commits.json").read_text(encoding="utf-8"))["bases"]
    cands = json.loads((args.data / "stage2-candidates.json").read_text(encoding="utf-8"))["candidates"]
    rows = [{"id": b["id"], "stage1": True, "fix": b["fix"], "base": b["base"]} for b in bases]
    picked = [c for c in cands if c["eligible"] and c["changed_lines"] >= MIN_CHANGED_LINES]
    picked = [c for c in picked if buildable(args.repo, c["base"])]
    rows += [
        {"id": f"cc-{i:02d}", "stage1": False, "fix": c["fix"], "base": c["base"]}
        for i, c in enumerate(sorted(picked, key=lambda c: (c["date"], c["fix"])), 1)
    ]
    for row in rows:
        row["hunks"] = hunks(args.repo, row["base"], row["fix"])
    doc = {"source": "tools/cochange_inputs.py, DESIGN.md section 10", "questions": rows}
    args.out.write_text(json.dumps(doc, indent=1) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
