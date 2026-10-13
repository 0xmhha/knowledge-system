"""Terminal views of a Run: a live rich dashboard and a plain line printer."""
from __future__ import annotations

import threading
import time
from typing import Callable, Dict, List, Optional

from .model import Run, Status, Step
from .options import Options

ICONS = {
    Status.PENDING: ("·", "dim"),
    Status.RUNNING: ("", "cyan"),  # replaced by a spinner
    Status.DONE: ("✓", "green"),
    Status.WARNING: ("!", "yellow"),
    Status.FAILED: ("✗", "red"),
    Status.SKIPPED: ("-", "dim"),
}


def fmt_elapsed(seconds: Optional[float]) -> str:
    if seconds is None:
        return ""
    seconds = int(seconds)
    if seconds < 60:
        return f"{seconds}s"
    m, s = divmod(seconds, 60)
    if m < 60:
        return f"{m}m{s:02d}s"
    h, m = divmod(m, 60)
    return f"{h}h{m:02d}m"


def fmt_value(value) -> str:
    if value is None:
        return "-"
    if isinstance(value, int) and not isinstance(value, bool):
        return f"{value:,}"
    text = str(value)
    return text[:12] if len(text) == 40 and all(c in "0123456789abcdef" for c in text) else text


def header_rows(opts: Options) -> List[tuple]:
    mode = "graph only" if opts.graph_only else f"graph + vector ({opts.embedder}: {opts.model}, {opts.embed_dim}d)"
    rows = [("Source", str(opts.src)), ("Dataset", str(opts.out)), ("Index", mode),
            ("Source mode", opts.source_mode)]
    if opts.pack_dir:
        skipped = f" (without {', '.join(opts.skip_pack_inputs)})" if opts.skip_pack_inputs else ""
        rows.append(("Project pack", f"{opts.pack_dir}{skipped}"))
    rows.append(("Server", f"{opts.name} @ 127.0.0.1:{opts.port}"))
    return rows


def run_worker(work: Callable[[], bool], on_interrupt: Callable[[], None], tick: Callable[[], None],
               interval: float) -> bool:
    """Run `work` on a thread, calling `tick` until it finishes.

    Completion is signalled through an Event rather than Thread.join: a
    KeyboardInterrupt that lands inside join() can leave the thread looking
    finished while it is still unwinding. The first Ctrl-C cancels the
    pipeline; further ones are absorbed until it has stopped.
    """
    result: Dict[str, bool] = {}
    finished = threading.Event()

    def target() -> None:
        try:
            result["ok"] = work()
        finally:
            finished.set()

    threading.Thread(target=target, daemon=True).start()
    interrupted = False
    while not finished.is_set():
        try:
            finished.wait(interval)
            tick()
        except KeyboardInterrupt:
            if not interrupted:
                interrupted = True
                on_interrupt()
    tick()
    return result.get("ok", False)


class RichView:
    """Live dashboard: header, step table with nested engine stages, warnings
    and the tail of the command output."""

    def __init__(self, opts: Options, run: Run, log_path: str) -> None:
        from rich.console import Console
        self.console = Console()
        self.opts, self.run, self.log_path = opts, run, log_path

    def _status_cell(self, step: Step):
        from rich.spinner import Spinner
        from rich.text import Text
        if step.status == Status.RUNNING:
            return Spinner("dots", style="cyan")
        icon, style = ICONS[step.status]
        return Text(icon, style=style)

    def render(self):
        from rich.console import Group
        from rich.panel import Panel
        from rich.table import Table
        from rich.text import Text

        head = Table.grid(padding=(0, 2))
        head.add_column(style="bold")
        head.add_column(overflow="fold")
        for k, v in header_rows(self.opts):
            head.add_row(k, v)

        steps = Table(expand=True, show_edge=False, pad_edge=False, header_style="bold")
        steps.add_column("", width=2, no_wrap=True)
        steps.add_column("Step", min_width=38, no_wrap=True)
        steps.add_column("Time", width=8, justify="right", no_wrap=True)
        steps.add_column("Detail", ratio=1, overflow="ellipsis", no_wrap=True)
        with self.run.lock:
            done = sum(1 for s in self.run.steps if s.status in (Status.DONE, Status.WARNING, Status.SKIPPED))
            for i, s in enumerate(self.run.steps, 1):
                style = "bold" if s.status == Status.RUNNING else ("dim" if s.status in (Status.PENDING, Status.SKIPPED) else "")
                steps.add_row(self._status_cell(s), Text(f"{i}. {s.title}", style=style),
                              fmt_elapsed(s.elapsed()), Text(" · ".join(x for x in (s.note, s.detail) if x), style="dim" if s.status != Status.FAILED else "red"))
                for sub in s.substeps:
                    steps.add_row(self._status_cell(sub), Text(f"   {sub.title}", style="dim" if sub.status == Status.PENDING else ""),
                                  fmt_elapsed(sub.elapsed()), Text(sub.detail, style="dim"))
            log = "\n".join(self.run.log) or " "
            warnings = list(self.run.warnings)
        total = fmt_elapsed(time.monotonic() - self.run.started)
        parts = [Panel(head, title="[bold]knowledge-system setup", title_align="left", border_style="blue"),
                 Panel(steps, title=f"Progress {done}/{len(self.run.steps)}  ·  {total}", title_align="left")]
        if warnings:
            parts.append(Panel(Text("\n".join(warnings[-5:]), style="yellow"), title="Warnings", title_align="left",
                               border_style="yellow"))
        parts.append(Panel(Text(log, style="dim", overflow="ellipsis", no_wrap=True), title="Output",
                           subtitle=f"full log: {self.log_path}", subtitle_align="right", title_align="left",
                           border_style="grey37", height=min(len(self.run.log), 12) + 2))
        return Group(*parts)

    def live(self, work: Callable[[], bool], on_interrupt: Callable[[], None]) -> bool:
        from rich.live import Live
        with Live(self.render(), console=self.console, refresh_per_second=8, transient=False) as live:
            ok = run_worker(work, on_interrupt, lambda: live.update(self.render()), 0.125)
        self.finish(ok)
        return ok

    def finish(self, ok: bool) -> None:
        from rich.panel import Panel
        from rich.text import Text
        if ok:
            body = Text("\n".join(self.run.summary))
            self.console.print(Panel(body, title="[bold green]Dataset ready", title_align="left", border_style="green"))
            if self.run.comparison:
                from rich.table import Table
                table = Table(title=f"Compared with {self.run.compare_label}", title_justify="left")
                table.add_column("Metric")
                table.add_column("This build", justify="right")
                table.add_column("Compared", justify="right")
                for metric, a, b in self.run.comparison:
                    style = "" if a == b else "yellow"
                    table.add_row(metric, fmt_value(a), fmt_value(b), style=style)
                self.console.print(table)
        else:
            body = Text(f"{self.run.error}\n\nFull log: {self.log_path}")
            self.console.print(Panel(body, title="[bold red]Setup failed", title_align="left", border_style="red"))


class PlainView:
    """Line-per-change output for pipes, CI logs and terminals without rich."""

    def __init__(self, opts: Options, run: Run, log_path: str,
                 write: Callable[[str], None] = lambda line: print(line, flush=True)) -> None:
        self.opts, self.run, self.log_path, self.write = opts, run, log_path, write
        self.seen: Dict[str, tuple] = {}

    def _emit_changes(self) -> None:
        with self.run.lock:
            for i, s in enumerate(self.run.steps, 1):
                self._emit(f"{i}. {s.title}", s)
                for sub in s.substeps:
                    self._emit(f"{i}. {s.title} / {sub.title}", sub)

    def _emit(self, label: str, step: Step) -> None:
        state = (step.status, step.detail if step.status != Status.RUNNING else "")
        if self.seen.get(label) == state or step.status == Status.PENDING:
            return
        self.seen[label] = state
        tail = f" ({fmt_elapsed(step.elapsed())})" if step.status not in (Status.RUNNING, Status.SKIPPED) else ""
        detail = f" - {step.detail}" if step.detail else ""
        self.write(f"[{step.status.value:>7}] {label}{tail}{detail}")

    def live(self, work: Callable[[], bool], on_interrupt: Callable[[], None]) -> bool:
        for k, v in header_rows(self.opts):
            self.write(f"{k:>12}: {v}")
        ok = run_worker(work, on_interrupt, self._emit_changes, 0.5)
        if ok:
            self.write("Dataset ready.")
            for line in self.run.summary:
                self.write("  " + line)
            if self.run.comparison:
                self.write(f"Compared with {self.run.compare_label}:")
                for metric, a, b in self.run.comparison:
                    mark = " " if a == b else "*"
                    self.write(f" {mark} {metric:<34} {fmt_value(a):>14}  {fmt_value(b):>14}")
        else:
            self.write(f"Setup failed: {self.run.error}")
            self.write(f"Full log: {self.log_path}")
        return ok
