"""The setup pipeline: preflight -> build -> init -> index -> server config -> verify.

Every external action is one of the commands documented in the repository
README; this module only sequences them, streams their output into a Run and
stops at the first failure.
"""
from __future__ import annotations

import json
import os
import re
import time
import subprocess
import threading
from pathlib import Path
from typing import Callable, Dict, List, Optional, TextIO

from . import events, mcp_check, pack as packs, preflight, profile
from .model import Run, Status, Step
from .options import Options, choose_version, repo_root


class StepFailed(Exception):
    pass


def new_run(opts: Options) -> Run:
    return Run([
        Step("preflight", "Check prerequisites"),
        Step("build", "Build ckg / ckv / cks"),
        Step("init", "Write the project setup config"),
        Step("index", "Build the knowledge dataset"),
        Step("serve", "Write the MCP server config"),
        Step("verify", "Start the server and check health"),
    ])


# --- command builders (pure, unit-tested) ---------------------------------

def init_argv(opts: Options) -> List[str]:
    return [str(opts.binary("cks")), "init", "--src", str(opts.src), "--dataset", str(opts.out),
            "--embedder", opts.embedder, "--model-name", opts.model, "--config-out", str(opts.setup_config)]


def load_pack(opts: Options) -> Optional[packs.Pack]:
    if opts.pack_dir is None:
        return None
    pack = packs.load(opts.pack_dir, opts.src)
    for key in opts.skip_pack_inputs:
        pack.inputs.pop(key, None)
    return pack


def setup_argv(opts: Options, version: Optional[str], pack: Optional[packs.Pack] = None) -> List[str]:
    cks = str(opts.binary("cks"))
    common = ["--progress", "json", "--build-timeout", opts.build_timeout]
    pack_flags = pack.setup_flags(opts.graph_only, opts.derived_dir) if pack else []
    if opts.graph_only:
        return ([cks, "setup", "--src", str(opts.src), "--out", str(opts.out), "--skip-vector"] + common
                + pack_flags + opts.extra_setup_args)
    argv = [cks, "setup", "--config", str(opts.setup_config), "--version", str(version),
            "--source-mode", opts.source_mode, "--embed-dim", str(opts.embed_dim)] + common
    if opts.embedder == "ollama":
        argv += ["--ollama-url", opts.ollama_url, "--query-prefix-policy", "registry"]
    if opts.min_canonical is not None:
        argv += ["--gate-min-canonical", f"{opts.min_canonical:g}"]
    return argv + pack_flags + opts.extra_setup_args


def setup_env(opts: Options) -> Dict[str, str]:
    env = dict(os.environ)
    if opts.strict_embeddings and opts.embedder == "ollama" and not opts.graph_only:
        env["CKV_REQUIRE_COMPLETE_EMBEDDINGS"] = "1"
    return env


def gen_config_argv(opts: Options, out: Optional[Path] = None, pack: Optional[packs.Pack] = None) -> List[str]:
    # gen-config never replaces an existing file (the caller writes to a fresh
    # name and swaps it in); the ruleset is pinned to this checkout's baseline.
    argv = [str(opts.binary("cks")), "mcp", "gen-config", "--dataset-dir", str(opts.serving_dir),
            "--name", opts.name, "--port", str(opts.port), "--out", str(out or opts.server_config),
            "--sanitize-rules", str(repo_root() / "system" / "policies" / "sanitization_rules.yaml"),
            "--source-root", str(opts.src), "--graph-binary", str(opts.binary("ckg")),
            "--vector-binary", str(opts.binary("ckv"))]
    if opts.embedder == "ollama":
        argv += ["--ollama-url", opts.ollama_url, "--embed-model", opts.model, "--embed-dim", str(opts.embed_dim)]
    if pack:
        argv += pack.server_flags(opts.graph_only, opts.derived_dir)
    return argv


def config_src(path: Path) -> Optional[str]:
    """The `src:` value of a cks init config (flat, machine-written YAML)."""
    m = re.search(r"(?m)^src:\s*(.+?)\s*$", path.read_text())
    return m.group(1).strip("'\"") if m else None


# --- execution -------------------------------------------------------------

class Pipeline:
    def __init__(self, opts: Options, run: Run, log: TextIO) -> None:
        self.opts, self.run, self.log = opts, run, log
        self.version: Optional[str] = None
        self.commit = ""
        self.proc: Optional[subprocess.Popen] = None
        self.cancelled = threading.Event()
        self._last_stderr = ""
        self.pack: Optional[packs.Pack] = None
        self.profile: Optional[dict] = None
        self.comparison: List[tuple] = []

    def _log(self, line: str) -> None:
        self.log.write(line.rstrip("\n") + "\n")
        self.log.flush()

    def _exec(self, argv: List[str], on_line: Callable[[str], None], cwd: Optional[Path] = None,
              env: Optional[Dict[str, str]] = None, merge_stderr: bool = True) -> int:
        self._log("$ " + " ".join(argv))
        self.proc = subprocess.Popen(argv, cwd=cwd, env=env, stdout=subprocess.PIPE,
                                     stderr=subprocess.STDOUT if merge_stderr else subprocess.PIPE,
                                     text=True, bufsize=1)
        stderr_lines: List[str] = []
        if not merge_stderr:
            def drain() -> None:
                assert self.proc is not None and self.proc.stderr is not None
                for line in self.proc.stderr:
                    stderr_lines.append(line)
                    self._log("stderr: " + line)
                    self.run.add_log(line)
            t = threading.Thread(target=drain, daemon=True)
            t.start()
        assert self.proc.stdout is not None
        for line in self.proc.stdout:
            self._log(line)
            on_line(line)
        code = self.proc.wait()
        if not merge_stderr:
            t.join(timeout=5)
        self._last_stderr = "".join(stderr_lines[-3:]).strip()
        self.proc = None
        if self.cancelled.is_set():
            raise StepFailed("cancelled")
        return code

    def cancel(self) -> None:
        self.cancelled.set()
        proc = self.proc
        if proc and proc.poll() is None:
            proc.terminate()

    def _step(self, key: str, fn: Callable[[Step], None]) -> None:
        step = self.run.step(key)
        if self.cancelled.is_set():
            raise StepFailed("cancelled")
        step.start()
        try:
            fn(step)
        except StepFailed as err:
            step.finish(Status.FAILED, str(err))
            raise
        except Exception as err:  # unexpected: show it on the row instead of a traceback
            step.finish(Status.FAILED, f"{type(err).__name__}: {err}")
            raise StepFailed(str(err)) from err
        if step.status == Status.RUNNING:
            step.finish(Status.DONE)

    def execute(self) -> bool:
        order = [("preflight", self.do_preflight), ("build", self.do_build), ("init", self.do_init),
                 ("index", self.do_index), ("serve", self.do_serve), ("verify", self.do_verify)]
        try:
            for key, fn in order:
                self._step(key, fn)
        except StepFailed as err:
            self.run.error = str(err)
            for s in self.run.steps:
                if s.status == Status.PENDING:
                    s.status = Status.SKIPPED
            return False
        self._measure()
        self._summarize()
        return True

    # --- steps ---------------------------------------------------------------

    def do_preflight(self, step: Step) -> None:
        checks = preflight.run_all(self.opts)
        for c in checks:
            sub = step.substep(c.name)
            sub.finish(Status.DONE if c.ok else (Status.FAILED if c.fatal else Status.WARNING), c.detail)
            self._log(f"preflight {c.name}: {'ok' if c.ok else 'FAIL' if c.fatal else 'warn'} {c.detail}")
            if not c.ok and not c.fatal:
                self.run.warn(f"{c.name}: {c.detail}")
        fatal = [c for c in checks if not c.ok and c.fatal]
        if fatal:
            raise StepFailed("; ".join(f"{c.name}: {c.detail}" for c in fatal))
        self.commit = preflight.source_commit(self.opts.src)
        self.pack = load_pack(self.opts)
        warned = sum(1 for c in checks if not c.ok)
        step.finish(Status.WARNING if warned else Status.DONE,
                    f"{len(checks)} checks, {warned} warning(s)" if warned else f"{len(checks)} checks passed")

    def do_build(self, step: Step) -> None:
        if not self.opts.build:
            step.finish(Status.SKIPPED, f"using {self.opts.bins}")
            return
        step.detail = "make build-bins"

        def on_line(line: str) -> None:
            self.run.add_log(line)
            m = re.search(r"-o bin/(\w+)", line)
            if m:
                step.detail = f"go build {m.group(1)}"
        code = self._exec(["make", "build-bins"], on_line, cwd=repo_root())
        if code != 0:
            raise StepFailed(f"make build-bins exited {code}")
        step.finish(Status.DONE, str(self.opts.bins))

    def do_init(self, step: Step) -> None:
        if self.opts.graph_only:
            step.finish(Status.SKIPPED, "graph-only build needs no project config")
            return
        cfg = self.opts.setup_config
        if cfg.exists():
            existing = config_src(cfg)
            if existing and Path(existing).resolve() != self.opts.src.resolve():
                raise StepFailed(f"{cfg} was written for {existing}; choose another --out")
            step.finish(Status.DONE, f"reusing {cfg.name} (project ID kept)")
            return
        output: List[str] = []
        code = self._exec(init_argv(self.opts), lambda l: (output.append(l), self.run.add_log(l)))
        if code != 0:
            raise StepFailed(f"cks init exited {code}: {''.join(output[-2:]).strip()}")
        step.finish(Status.DONE, str(cfg))

    def do_index(self, step: Step) -> None:
        if self.opts.versioned:
            self.version = self.opts.version or choose_version(self.opts.out, self.commit)
            if (self.opts.out / self.version).exists():
                raise StepFailed(f"version {self.version} already exists under {self.opts.out}")
            step.detail = f"version {self.version}"
        argv = setup_argv(self.opts, self.version, self.pack)
        code = self._exec(argv, lambda l: events.apply(self.run, step, events.parse(l)) if l.strip() else None,
                          env=setup_env(self.opts), merge_stderr=False)
        failed = [s for s in step.substeps if s.status == Status.FAILED]
        if code != 0:
            reason = failed[0].detail if failed else (self._last_stderr or f"cks setup exited {code}")
            raise StepFailed(reason)
        for s in step.substeps:
            if s.status == Status.RUNNING:
                s.finish(Status.DONE)
        where = f"{self.opts.out}/{self.version}" if self.version else str(self.opts.out)
        step.finish(Status.WARNING if any(s.status == Status.WARNING for s in step.substeps) else Status.DONE, where)

    def do_serve(self, step: Step) -> None:
        target = self.opts.server_config
        fresh = target.with_name(target.name + ".new")
        if fresh.exists():
            fresh.unlink()
        output: List[str] = []
        code = self._exec(gen_config_argv(self.opts, fresh, self.pack), lambda l: (output.append(l), self.run.add_log(l)))
        if code != 0:
            raise StepFailed(f"cks mcp gen-config exited {code}: {''.join(output[-2:]).strip()}")
        stdio = mcp_check.stdio_variant(fresh.read_text())
        os.replace(fresh, target)  # replace the previous config only after a successful write
        self.opts.stdio_config.write_text(stdio)
        step.finish(Status.DONE, f"{self.opts.server_config.name}, {self.opts.stdio_config.name}")

    def do_verify(self, step: Step) -> None:
        if not self.opts.verify:
            step.finish(Status.SKIPPED, "--no-verify")
            return
        h = mcp_check.health(self.opts.binary("cks"), self.opts.stdio_config)
        self._log(f"health: {h}")
        step.substep("graph").finish(Status.DONE if h["graph_reachable"] else Status.FAILED,
                                     f"schema {h['graph_schema']}, head {str(h['indexed_head'])[:12]}")
        if not h["graph_reachable"]:
            raise StepFailed("graph index is not reachable")
        vec = step.substep("vector")
        if self.opts.graph_only:
            vec.finish(Status.SKIPPED, "graph-only build")
        elif h["vector_reachable"]:
            vec.finish(Status.DONE, "reachable")
        else:
            vec.finish(Status.WARNING if self.opts.embedder == "mock" else Status.FAILED,
                       (h.get("vector_reason") or "unreachable")[:120])
            if self.opts.embedder != "mock":
                raise StepFailed("vector index is not reachable: " + str(h.get("vector_reason")))
        self.health = h
        symbol = mcp_check.busiest_function(self.opts.serving_dir / "graph" / "graph.db")
        smoke = mcp_check.smoke_queries(self.opts.binary("cks"), self.opts.stdio_config, symbol,
                                        with_vector=not self.opts.graph_only and self.opts.embedder != "mock")
        self.health["queries"] = smoke
        failed = []
        rv = smoke.get("review_function")
        if rv:
            sub = step.substep("review_function", "Query: review_function")
            sub.finish(Status.DONE if rv["ok"] else Status.FAILED,
                       f"{rv['symbol']}: {rv['callers']} callers, {rv['commits']} commits, {rv['ms']} ms")
            failed += [] if rv["ok"] else ["review_function"]
        gt = smoke.get("get_for_task_v2")
        if gt:
            ok = gt["ok"] and gt["citations"] > 0
            step.substep("get_for_task_v2", "Query: get_for_task_v2").finish(
                Status.DONE if ok else Status.FAILED, f"{gt['citations']} citations, {gt['ms']} ms")
            failed += [] if ok else ["get_for_task_v2"]
        if failed:
            raise StepFailed("the server answered health but not " + ", ".join(failed))
        detail = f"{h['tools']} MCP tools, status {h['status']}"
        step.finish(Status.DONE if h["serviceable"] else Status.WARNING, detail)

    def _measure(self) -> None:
        """Profile the served dataset (and the comparison one) and write the report."""
        try:
            self.profile = profile.dataset_profile(self.opts.serving_dir)
            other = profile.dataset_profile(self.opts.compare_with) if self.opts.compare_with else None
        except (OSError, ValueError) as err:  # measuring is informative only
            self.run.warn(f"profile: {err}")
            return
        if other:
            self.comparison = profile.compare(self.profile, other)
            self.run.comparison = self.comparison
            self.run.compare_label = str(self.opts.compare_with)
        report = {
            "generated_at": time.strftime("%Y-%m-%dT%H:%M:%S%z"),
            "source": str(self.opts.src), "source_commit": self.commit, "dataset": str(self.opts.out),
            "version": self.version, "pack": str(self.opts.pack_dir) if self.opts.pack_dir else None,
            "pack_inputs": sorted(self.pack.inputs) if self.pack else [],
            "embedder": None if self.opts.graph_only else {"backend": self.opts.embedder, "model": self.opts.model,
                                                            "dim": self.opts.embed_dim},
            "steps": [{"key": s.key, "status": s.status.value, "seconds": round(s.elapsed() or 0, 1),
                       "substeps": [{"key": x.key, "status": x.status.value, "seconds": round(x.elapsed() or 0, 1)}
                                    for x in s.substeps]} for s in self.run.steps],
            "health": getattr(self, "health", None),
            "profile": self.profile,
            "compared_with": str(self.opts.compare_with) if self.opts.compare_with else None,
            "comparison": [{"metric": m, "this": a, "other": b} for m, a, b in self.comparison],
        }
        self.opts.report_file.write_text(json.dumps(report, indent=1, ensure_ascii=False) + "\n")

    def _summarize(self) -> None:
        o = self.opts
        lines = [f"Dataset:        {o.serving_dir}" + (f"  (version {self.version})" if self.version else "")]
        lines.append(f"Server config:  {o.server_config}  (HTTP, 127.0.0.1:{o.port})")
        lines.append(f"Stdio config:   {o.stdio_config}")
        lines.append(f"Serve:          {o.binary('cks')} mcp --config {o.server_config}")
        lines.append(f"Client entry:   {o.binary('cks')} mcp client-config --config {o.server_config}")
        if self.version:
            kept = profile.retained_versions(o.out)
            lines.append(f"Versions:       {kept['count']} retained, {kept['bytes'] / 2**30:.1f} GiB; review a cleanup with "
                         f"{o.binary('cks')} gc --out {o.out} --dry-run")
            lines.append("Rebuild:        rerun this tool; a new version is promoted only after its gates pass")
            lines.append(f"Roll back:      {o.binary('cks')} setup --out {o.out} --src {o.src} --rollback <version>")
        if self.profile:
            lines += profile.summary_lines(self.profile)
            lines.append(f"Report:         {o.report_file}")
        lines.append("Running servers keep the dataset they opened; restart them after a rebuild.")
        self.run.summary = lines
