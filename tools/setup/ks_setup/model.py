"""State shared by the pipeline and the UI.

The pipeline only mutates a Run; the UI only reads it. Keeping the two apart
lets the same run drive the rich dashboard, the plain line printer and tests.
"""
from __future__ import annotations

import enum
import threading
import time
from collections import deque
from dataclasses import dataclass, field
from typing import Deque, List, Optional


class Status(str, enum.Enum):
    PENDING = "pending"
    RUNNING = "running"
    DONE = "done"
    WARNING = "warning"
    FAILED = "failed"
    SKIPPED = "skipped"


@dataclass
class Step:
    """One row of the progress view. Substeps are the engine's own stages
    (graph-build, vector-build, ...) reported while `cks setup` runs."""

    key: str
    title: str
    status: Status = Status.PENDING
    detail: str = ""
    note: str = ""                      # persistent context shown before the detail
    started: Optional[float] = None
    finished: Optional[float] = None
    substeps: List["Step"] = field(default_factory=list)

    def start(self, detail: str = "") -> None:
        self.status = Status.RUNNING
        self.started = time.monotonic()
        if detail:
            self.detail = detail

    def finish(self, status: Status, detail: str = "") -> None:
        self.status = status
        self.finished = time.monotonic()
        if self.started is None:
            self.started = self.finished
        if detail:
            self.detail = detail

    def elapsed(self) -> Optional[float]:
        if self.started is None:
            return None
        return (self.finished or time.monotonic()) - self.started

    def substep(self, key: str, title: str = "") -> "Step":
        for s in self.substeps:
            if s.key == key:
                return s
        s = Step(key=key, title=title or key)
        self.substeps.append(s)
        return s


class Run:
    """Thread-safe container for the steps, the log tail and the outcome."""

    def __init__(self, steps: List[Step], log_lines: int = 12) -> None:
        self.steps = steps
        self.lock = threading.RLock()
        self.log: Deque[str] = deque(maxlen=log_lines)
        self.warnings: List[str] = []
        self.summary: List[str] = []
        self.error: Optional[str] = None
        self.started = time.monotonic()

    def step(self, key: str) -> Step:
        for s in self.steps:
            if s.key == key:
                return s
        raise KeyError(key)

    def add_log(self, line: str) -> None:
        line = line.rstrip()
        if line:
            with self.lock:
                self.log.append(line)

    def warn(self, message: str) -> None:
        with self.lock:
            self.warnings.append(message)
