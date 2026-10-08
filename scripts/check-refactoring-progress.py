#!/usr/bin/env python3
"""Refuse completion without committed evidence, checks and a verified source binding.

Checks documentary/evidence consistency, not the truth of human judgments.
Historical evidence is checked at its recorded revision, never rewritten to
pretend it was produced by the current code.
"""
import argparse
import functools
import hashlib
import json
from pathlib import Path
import re
import subprocess
import sys

ROOT = Path(__file__).resolve().parents[1]
SPEC = "docs/spec-driven/REFACTORING-EXECUTION-SPEC.md"
LIST = "docs/spec-driven/REFACTORING-REMAINING-WORKLIST.md"


def require(condition, message):
    if not condition:
        raise ValueError(message)


def sha(data):
    return hashlib.sha256(data).hexdigest()


@functools.lru_cache(maxsize=None)
def recorded(revision, path):
    require(not Path(path).is_absolute() and ".." not in Path(path).parts,
            f"invalid evidence path: {path}")
    return subprocess.check_output(["git", "show", f"{revision}:{path}"], cwd=ROOT,
                                   stderr=subprocess.DEVNULL)


def verify_manifest(task):
    revision, path = task.get("evidence_revision"), task.get("manifest")
    require(revision and path, f"{task['id']}: no committed completion evidence")
    manifest = json.loads(recorded(revision, path))
    parent = str(Path(path).parent)
    if task["id"] == "N-01":
        bindings = [(row["path"], row["sha256"]) for row in manifest["reports"]]
        bindings += [(f"{parent}/{row['path']}", row["sha256"]) for row in manifest["files"]]
        trace = json.loads(recorded(revision, f"{parent}/recovered-source-and-trace.json"))
        require(trace["N01_document_recovery_complete"], "N-01: recovery not complete")
    else:
        require(manifest["status"] == task["id"].replace("-", "") + "_acceptance_verified",
                f"{task['id']}: acceptance not verified")
        bindings = list(manifest["reports_sha256"].items())
        bindings += [(f"{parent}/{p}", digest) for p, digest in manifest["evidence_sha256"].items()]
        bindings += list(manifest["changed_source_sha256"].items())
        # Preserve the full acceptance set of the completed specification.
        historic = recorded(revision, SPEC).decode()
        prefix = task["id"].replace("-", "")
        required = set(re.findall(r"\[[ x]\] (" + prefix + r"-[A-Z])\b", historic))
        require(required and required == set(task.get("required_checks", [])),
                f"{task['id']}: required checks omitted from progress record")
        live = (ROOT / SPEC).read_text()
        present = set(re.findall(r"\[[ x]\] (" + prefix + r"-[A-Z])\b", live))
        checked = set(re.findall(r"\[x\] (" + prefix + r"-[A-Z])\b", live))
        require(present == required and checked == required,
                f"{task['id']}: acceptance checkbox is incomplete or changed")
    for artifact, digest in bindings:
        require(sha(recorded(revision, artifact)) == digest,
                f"{task['id']}: committed artifact missing or changed: {artifact}")


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--ledger", type=Path, default=ROOT / "docs/spec-driven/REFACTORING-PROGRESS.json")
    args = parser.parse_args()
    ledger = json.loads(args.ledger.read_text())
    tasks = ledger["tasks"]
    require([t["id"] for t in tasks] == [f"N-{i:02d}" for i in range(1, 19)], "missing, reordered or duplicate task")
    allowed = {"pending", "design", "reproduced", "implemented", "verifying", "waiting", "conditional", "complete"}
    require(all(t["state"] in allowed for t in tasks), "unrecognized state")
    done = {t["id"] for t in tasks if t["state"] == "complete"}
    worklist = (ROOT / LIST).read_text()
    rows = re.findall(r"^\| (N-\d\d) \| ([^|]+) \|", worklist, re.M)
    checked = {ident for ident, status in rows if status.strip().lstrip("*").startswith("완료")}
    require(len(rows) == 18 and checked == done, "worklist completion disagrees with evidence ledger")
    for task in tasks:
        if task["state"] == "complete":
            verify_manifest(task)
    active = ledger["active_source_binding"]
    manifest = json.loads(recorded(active["revision"], active["manifest"]))
    sources = manifest["source_sha256"]
    for path, digest in {**sources, **manifest["approved_input_sha256"]}.items():
        require((ROOT / path).is_file() and sha((ROOT / path).read_bytes()) == digest,
                f"unverified current source/input change: {path}")
    inventory = subprocess.check_output(["git", "ls-files", "--cached", "--others", "--exclude-standard", "*.go"], cwd=ROOT, text=True)
    live = {p for p in inventory.splitlines() if p.startswith(("cmd/", "internal/", "pkg/"))}
    require(live == {p for p in sources if p.endswith(".go")}, "source inventory changed without new binding")
    remaining = [t["id"] for t in tasks if t["state"] != "complete"]
    require(ledger["next_task"] in remaining, "next task is already marked complete")
    require(f"{len(done)}/18" in worklist and f"{len(remaining)}개" in worklist, "progress counts drifted")
    print(f"refactoring progress: PASS {len(done)}/18 complete; {len(remaining)} remaining; next {ledger['next_task']}; {len(sources)} source/dependency bindings")


if __name__ == "__main__":
    try:
        main()
    except (ValueError, KeyError, OSError, subprocess.CalledProcessError) as error:
        print(f"refactoring progress: FAIL: {error}", file=sys.stderr)
        sys.exit(1)
