"""Keep Go sources under build/ directories in the vector index.

The vector indexer skips every directory named build/ by default, at any
depth, and admits a Go file there only when it is listed exactly in a
vector_build_sources file. The graph loads Go packages with `go list`
semantics and has no such rule, so a package such as internal/build ends up
in the graph but not in the vector index. This module lists the Go files the
graph would load under build/ directories (same build constraints, same
module boundaries) so the two indexes cover the same Go sources, and checks
the built dataset for any Go file that is still only in the graph.
"""
from __future__ import annotations

import json
import subprocess
from pathlib import Path
from typing import Dict, Iterable, List, Optional

# Directory names the Go tool never loads with ./... and the vector indexer
# never walks; a candidate under one of them is not a graph source either.
_SKIP_SEGMENTS = {"vendor", "node_modules", ".git", "testdata"}

_GO_LIST_FORMAT = ("{{.Dir}}\t{{join .GoFiles \",\"}}\t{{join .CgoFiles \",\"}}"
                   "\t{{join .TestGoFiles \",\"}}\t{{join .XTestGoFiles \",\"}}")


def in_build_dir(rel: str) -> bool:
    """True when a directory segment of rel is build (the indexer default)."""
    return "build" in rel.split("/")[:-1]


def _admissible(rel: str) -> bool:
    dirs = rel.split("/")[:-1]
    return (rel.endswith(".go") and in_build_dir(rel)
            and not any(d in _SKIP_SEGMENTS or d.startswith((".", "_")) for d in dirs))


def tracked_files(src: Path, committed: bool) -> List[str]:
    """Paths the build will see: HEAD's tree, or the working tree's files."""
    argv = (["git", "-C", str(src), "ls-tree", "-r", "--name-only", "HEAD"] if committed
            else ["git", "-C", str(src), "ls-files", "--cached", "--others", "--exclude-standard"])
    out = subprocess.run(argv, capture_output=True, text=True, check=True, timeout=60)
    return [line for line in out.stdout.splitlines() if line]


def _module_root(src: Path, rel_dir: str) -> Optional[Path]:
    d = src / rel_dir
    while True:
        if (d / "go.mod").is_file():
            return d
        if d == src or src not in d.parents:
            return None
        d = d.parent


def go_list_files(src: Path, rel_dirs: Iterable[str]) -> List[str]:
    """Go files `go list` loads for the given package directories, as paths
    relative to src. Build constraints apply as they do for the graph, so a
    `//go:build none` script stays out. Errors in a package are tolerated (-e);
    -find skips dependency resolution, so no module download is needed."""
    src = src.resolve()
    by_module: Dict[Path, List[str]] = {}
    for rel_dir in sorted(set(rel_dirs)):
        root = _module_root(src, rel_dir)
        if root is not None:
            # A ./relative pattern: an absolute one fails when the module path
            # and the given path differ by a symlink (/var vs /private/var).
            rel = (src / rel_dir).relative_to(root).as_posix()
            by_module.setdefault(root, []).append("." if rel == "." else "./" + rel)
    found: List[str] = []
    for root, dirs in by_module.items():
        argv = ["go", "list", "-e", "-find", "-f", _GO_LIST_FORMAT] + dirs
        out = subprocess.run(argv, cwd=root, capture_output=True, text=True, timeout=120, check=False)
        if out.returncode != 0:
            raise RuntimeError(f"go list in {root} exited {out.returncode}: {out.stderr.strip()[:200]}")
        for line in out.stdout.splitlines():
            pkg_dir, *groups = line.split("\t")
            for name in ",".join(groups).split(","):
                if name:
                    found.append((Path(pkg_dir) / name).resolve().relative_to(src).as_posix())
    return sorted(set(found))


def discover(src: Path, committed: bool) -> List[str]:
    """Go files under build/ directories that the graph indexes."""
    candidates = [p for p in tracked_files(src, committed) if _admissible(p)]
    if not candidates:
        return []
    keep = set(candidates)
    loaded = go_list_files(src, {str(Path(p).parent) for p in candidates})
    return [p for p in loaded if p in keep]


def write(path: Path, paths: List[str]) -> None:
    """The vector_build_sources file: schema 1 and exact relative Go paths."""
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps({"schema_version": 1, "paths": paths}, indent=1) + "\n")


def go_scope_gap(serving_dir: Path) -> List[str]:
    """Go files the graph indexed that the vector index did not read.

    Only Go is compared: the vector indexer has no reader for some graph
    languages (proto), so a gap there is expected."""
    root = serving_dir.resolve()
    g, v = root / "graph" / "manifest.json", root / "vector" / "manifest.json"
    if not (g.is_file() and v.is_file()):
        return []
    graph_files = json.loads(g.read_text()).get("files") or []
    vector_files = json.loads(v.read_text()).get("input_files")
    if not isinstance(vector_files, list):
        return []  # older manifest without an input inventory
    read = {f.get("path") for f in vector_files if isinstance(f, dict)}
    go = {f.get("path") for f in graph_files if isinstance(f, dict) and f.get("language") == "go"}
    return sorted(p for p in go - read if p)
