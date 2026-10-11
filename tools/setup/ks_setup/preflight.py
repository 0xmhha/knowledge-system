"""Checks run before anything is built or written.

Each check returns a Check; `fatal` checks stop the run, the rest are shown
as warnings. Checks never modify the source tree or the dataset.
"""
from __future__ import annotations

import json
import shutil
import subprocess
import urllib.error
import urllib.request
from dataclasses import dataclass
from pathlib import Path
from typing import Callable, List, Optional

from .options import Options, repo_root


@dataclass
class Check:
    name: str
    ok: bool
    detail: str
    fatal: bool = True


def _run(argv: List[str], cwd: Optional[Path] = None) -> subprocess.CompletedProcess:
    return subprocess.run(argv, cwd=cwd, capture_output=True, text=True)


def check_tool(name: str, version_args: List[str], required: bool = True) -> Check:
    path = shutil.which(name)
    if not path:
        return Check(name, False, "not found in PATH", fatal=required)
    proc = _run([path] + version_args)
    first = (proc.stdout or proc.stderr).strip().splitlines()
    return Check(name, True, first[0] if first else path)


def check_repo() -> Check:
    root = repo_root()
    ok = (root / "go.mod").is_file() and (root / "cmd" / "cks").is_dir()
    return Check("knowledge-system", ok, str(root) if ok else f"{root} is not a knowledge-system checkout")


def check_binaries(opts: Options) -> Check:
    missing = [n for n in ("ckg", "ckv", "cks") if not opts.binary(n).is_file()]
    if missing:
        return Check("binaries", False, f"missing in {opts.bins}: {', '.join(missing)}")
    return Check("binaries", True, str(opts.bins))


def source_commit(src: Path) -> str:
    proc = _run(["git", "-C", str(src), "rev-parse", "--verify", "--quiet", "HEAD^{commit}"])
    return proc.stdout.strip() if proc.returncode == 0 else ""


def check_source(opts: Options) -> List[Check]:
    src = opts.src
    if not src.is_dir():
        return [Check("source", False, f"{src} does not exist")]
    top = _run(["git", "-C", str(src), "rev-parse", "--show-toplevel"])
    if top.returncode != 0:
        return [Check("source", False, f"{src} is not a Git repository")]
    commit = source_commit(src)
    if not commit:
        return [Check("source", False, f"{src} has no commit yet")]
    checks = [Check("source", True, f"{src} @ {commit[:12]}")]
    if Path(top.stdout.strip()).resolve() != src.resolve():
        checks.append(Check("source root", False,
                            f"{src} is inside {top.stdout.strip()}; index the repository root", fatal=False))
    dirty = _run(["git", "-C", str(src), "status", "--porcelain", "--untracked-files=no"]).stdout.strip()
    if dirty and opts.source_mode == "committed":
        n = len(dirty.splitlines())
        checks.append(Check("uncommitted changes", False,
                            f"{n} modified file(s) are not indexed in committed mode "
                            "(commit them or use --source-mode working-tree)", fatal=False))
    return checks


def check_output(opts: Options) -> List[Check]:
    out, src = opts.out.resolve(), opts.src.resolve()
    if out == src or src in out.parents:
        return [Check("dataset", False, f"{out} is inside the source tree; choose a directory outside {src}")]
    parent = out if out.exists() else out.parent
    while not parent.exists():
        parent = parent.parent
    usage = shutil.disk_usage(parent)
    checks = [Check("dataset", True, f"{out} ({usage.free / 2**30:.1f} GiB free)")]
    if usage.free < 2 * 2**30:
        checks.append(Check("disk space", False, "less than 2 GiB free on the dataset filesystem", fatal=False))
    if opts.versioned and out.exists() and (out / "graph").is_dir() and not (out / "current").exists():
        checks.append(Check("dataset layout", False,
                            f"{out} holds an unversioned graph-only build; use another --out for a full build"))
    return checks


def _get_json(url: str, timeout: float = 5.0) -> dict:
    with urllib.request.urlopen(url, timeout=timeout) as resp:  # noqa: S310 - local endpoint chosen by the user
        return json.load(resp)


def check_ollama(opts: Options, get_json: Callable[[str], dict] = _get_json) -> List[Check]:
    if opts.graph_only or opts.embedder != "ollama":
        return []
    base = opts.ollama_url.rstrip("/")
    try:
        version = get_json(base + "/api/version").get("version", "?")
        tags = get_json(base + "/api/tags")
    except (urllib.error.URLError, OSError, ValueError) as err:
        return [Check("ollama", False, f"{base} is not answering ({err}); start it with `ollama serve`")]
    names = {m.get("name", "") for m in tags.get("models", [])}
    wanted = opts.model if ":" in opts.model else opts.model + ":latest"
    checks = [Check("ollama", True, f"{base} (version {version})")]
    if wanted not in names:
        checks.append(Check("embedding model", False, f"{opts.model} is not pulled; run `ollama pull {opts.model}`"))
    else:
        checks.append(Check("embedding model", True, wanted))
    return checks


def run_all(opts: Options) -> List[Check]:
    checks = [check_repo(), check_tool("git", ["--version"])]
    if opts.build:
        checks += [check_tool("go", ["version"]), check_tool("make", ["--version"])]
    else:
        # The graph build still shells out to the Go toolchain to load packages.
        checks.append(check_tool("go", ["version"], required=False))
        checks.append(check_binaries(opts))
    checks += check_source(opts)
    checks += check_output(opts)
    checks += check_ollama(opts)
    return checks
