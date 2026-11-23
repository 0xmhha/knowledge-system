"""List stage 2 question candidates: fix commits of go-stablenet and their shape.

For every commit since SINCE whose subject starts with "fix", the script counts
the Go files the fix changes, the new files it adds (a new file cannot be an
oracle location at the base commit), whether it touches tests (a regression
test exists), and whether its removed or added lines carry a log or error
message (input for the later log back-trace experiment). Stage 1 fixes are
marked so they are not reused.

Two outputs: a JSON with hashes and counts only (safe for the public repo), and
a Markdown view with the commit subjects (kept outside the repo).

Usage:
    python3 -I stage2_candidates.py --repo <go-stablenet> --data <experiment data dir> \
        --json <out.json> --view <out.md>
"""

from __future__ import annotations

import argparse
import json
import re
import subprocess
from dataclasses import asdict, dataclass
from pathlib import Path

SINCE = "2025-01-01"
GIT_TIMEOUT_SECONDS = 60
FIX_PREFIX = "fix"
# A log call or an error constructor with a literal message.
MESSAGE_RE = re.compile(r'\b(log\.(Error|Warn|Info|Debug|Trace|Crit)|errors\.New|fmt\.Errorf)\("')
# Upper bound on changed non-test Go files for a question to stay reviewable.
MAX_SOURCE_FILES = 5


@dataclass(frozen=True, slots=True)
class Candidate:
    fix: str
    base: str
    date: str
    source_go_files: int
    test_go_files: int
    added_go_files: int
    changed_lines: int
    message_lines: int
    stage1: bool
    eligible: bool


def git(repo: Path, *args: str) -> str:
    """Run git in repo and return stdout."""
    out = subprocess.run(
        ["git", "-C", str(repo), *args],
        check=True,
        capture_output=True,
        text=True,
        timeout=GIT_TIMEOUT_SECONDS,
    )
    return out.stdout


def fix_commits(repo: Path) -> list[tuple[str, str, str]]:
    """Return (hash, date, subject) of fix commits since SINCE, newest first."""
    log = git(repo, "log", f"--since={SINCE}", "--no-merges", "--format=%H%x09%ad%x09%s", "--date=short")
    rows = []
    for line in log.splitlines():
        h, date, subject = line.split("\t", 2)
        if subject.lower().startswith(FIX_PREFIX):
            rows.append((h, date, subject))
    return rows


def shape(repo: Path, fix: str, date: str, stage1: set[str]) -> Candidate:
    """Measure one fix commit against its first parent."""
    base = git(repo, "rev-parse", f"{fix}^").strip()
    status = git(repo, "diff", "--name-status", base, fix, "--", "*.go")
    source = test = added = 0
    for line in status.splitlines():
        kind, path = line.split("\t", 1)
        if path.endswith("_test.go"):
            test += 1
            continue
        source += 1
        if kind.startswith("A"):
            added += 1
    diff = git(repo, "diff", "-U0", base, fix, "--", "*.go", ":!*_test.go")
    changed = message = 0
    for line in diff.splitlines():
        if line.startswith(("+++", "---")) or not line.startswith(("+", "-")):
            continue
        changed += 1
        if MESSAGE_RE.search(line):
            message += 1
    in_stage1 = fix in stage1
    eligible = not in_stage1 and 0 < source <= MAX_SOURCE_FILES and added < source
    return Candidate(fix, base, date, source, test, added, changed, message, in_stage1, eligible)


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--repo", type=Path, required=True)
    ap.add_argument("--data", type=Path, required=True)
    ap.add_argument("--json", type=Path, required=True)
    ap.add_argument("--view", type=Path, required=True)
    args = ap.parse_args()

    bases = json.loads((args.data / "base-commits.json").read_text(encoding="utf-8"))["bases"]
    stage1 = {b["fix"] for b in bases}

    cands: list[Candidate] = []
    view = [
        "# stage 2 candidates",
        "",
        "| # | fix | date | go src | test | added | lines | msg | stage1 | eligible | subject |",
        "|---|---|---|---|---|---|---|---|---|---|---|",
    ]
    for i, (h, date, subject) in enumerate(fix_commits(args.repo), 1):
        c = shape(args.repo, h, date, stage1)
        cands.append(c)
        view.append(
            f"| {i} | `{h[:12]}` | {date} | {c.source_go_files} | {c.test_go_files} | {c.added_go_files} | "
            f"{c.changed_lines} | {c.message_lines} | {c.stage1} | {c.eligible} | {subject} |"
        )

    doc = {
        "source": f"go-stablenet fix commits since {SINCE}; tools/stage2_candidates.py",
        "eligible_rule": f"not a stage 1 fix, 1..{MAX_SOURCE_FILES} non-test Go files, at least one of them not new",
        "candidates": [asdict(c) for c in cands],
    }
    args.json.write_text(json.dumps(doc, indent=1) + "\n", encoding="utf-8")
    args.view.parent.mkdir(parents=True, exist_ok=True)
    args.view.write_text("\n".join(view) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
