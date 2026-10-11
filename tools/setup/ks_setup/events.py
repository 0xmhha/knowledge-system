"""Translate `cks setup --progress json` events into substep state.

Each stdout line is one JSON object: {"time", "step", "type", "message"?,
"resources"?}. Types are start | output | warning | done | error | status
(internal/setup/runner.go). Anything that is not JSON is treated as output.
"""
from __future__ import annotations

import json
import re
from dataclasses import dataclass
from typing import Optional

from .model import Run, Status, Step

# Engine stage ids -> labels shown in the dashboard. Unknown ids are shown
# as-is so a new engine stage still appears.
STAGE_TITLES = {
    "domain-derive": "Derive domain artifacts",
    "filelist": "Derive the build file list",
    "vector-preflight": "Embedding service check",
    "graph-build": "Graph index (ckg)",
    "vector-build": "Vector index (ckv)",
    "verify-align": "Graph/vector alignment",
    "reindex-gate": "Promotion gate",
    "reindex-review": "Hold for review",
    "reindex-promote": "Promote to current",
    "domain-export": "Render the domain corpus",
    "policy-fresh": "Governance policy matches entries",
    "glossary-fresh": "Glossary matches entries",
    "filelist-derive": "Derive the build scope",
    "verify-content": "Domain and flow content reached the index",
}

# The promotion gate's ckg audit line for files the Go build compiles but the
# graph did not index.
_AUDIT_GAP = re.compile(r"in build only \(missing from DB\):\s*(\d+)")

# Status-only stages carry no start/done pair; they are folded into the log.
STATUS_ONLY = {"reindex-resources", "toolchain"}


@dataclass
class Event:
    step: str
    type: str
    message: str = ""
    resources: Optional[dict] = None


def parse(line: str) -> Optional[Event]:
    line = line.strip()
    if not line:
        return None
    try:
        obj = json.loads(line)
    except ValueError:
        return Event(step="", type="output", message=line)
    if not isinstance(obj, dict):
        return Event(step="", type="output", message=line)
    return Event(step=str(obj.get("step", "")), type=str(obj.get("type", "output")),
                 message=str(obj.get("message", "") or ""), resources=obj.get("resources"))


def _short(message: str, width: int = 90) -> str:
    message = " ".join(message.split())
    return message if len(message) <= width else message[: width - 1] + "…"


def apply(run: Run, parent: Step, event: Event) -> None:
    """Fold one engine event into `parent`'s substeps and the run log."""
    with run.lock:
        if event.type == "status" or event.step in STATUS_ONLY:
            if event.resources:
                res = event.resources
                files, size = res.get("source_files"), res.get("source_bytes")
                if files is not None:
                    parent.note = f"{files} source files, {_human_bytes(size or 0)}"
            if event.message:
                run.add_log(f"[{event.step}] {event.message}")
            return
        if not event.step:
            run.add_log(event.message)
            return
        sub = parent.substep(event.step, STAGE_TITLES.get(event.step, event.step))
        if event.type == "start":
            sub.start(_short(event.message))
            parent.detail = sub.title
        elif event.type == "output":
            run.add_log(f"[{event.step}] {event.message}")
            gap = _AUDIT_GAP.search(event.message)
            if gap and int(gap.group(1)) > 0:
                run.warn(f"graph is missing {gap.group(1)} source files the project build compiles "
                         "(a pack file list narrows the scope; --skip-pack-input filelist indexes them)")
            if sub.status == Status.PENDING:
                sub.start()
            if sub.status != Status.WARNING:  # keep the warning text visible
                sub.detail = _short(event.message)
        elif event.type == "warning":
            run.warn(f"{event.step}: {event.message}")
            run.add_log(f"[{event.step}] warning: {event.message}")
            if sub.status in (Status.PENDING, Status.RUNNING):
                sub.finish(Status.WARNING, _short(event.message))
        elif event.type == "done":
            sub.finish(Status.DONE, _short(event.message) if event.message else "")
        elif event.type == "error":
            sub.finish(Status.FAILED, _short(event.message))
            run.add_log(f"[{event.step}] error: {event.message}")


def _human_bytes(n: int) -> str:
    value = float(n)
    for unit in ("B", "KiB", "MiB", "GiB"):
        if value < 1024 or unit == "GiB":
            return f"{value:.0f} {unit}" if unit == "B" else f"{value:.1f} {unit}"
        value /= 1024
    return f"{n} B"
