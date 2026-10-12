"""ks-setup: build a knowledge dataset for one project with a live progress view.

usage:
  ks-setup --src ~/work/my-project --out ~/knowledge-data/my-project
  ks-setup                       # asks for the source and dataset paths
  ks-setup --src . --out /tmp/kd --graph-only --plain
"""
from __future__ import annotations

import argparse
import sys
from pathlib import Path
from typing import List, Optional

from .pack import PATH_FLAGS
from .options import DEFAULT_DIM, DEFAULT_MODEL, DEFAULT_OLLAMA, DEFAULT_PORT, Options, expand, slug
from .pipeline import Pipeline, new_run
from .ui import PlainView, header_rows


def parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(prog="ks-setup", description=__doc__.split("\n\n")[0],
                                formatter_class=argparse.RawDescriptionHelpFormatter,
                                epilog="Every step runs a documented cks command; the full output is kept in a log file.")
    p.add_argument("--src", help="Git repository to index (asked for when omitted)")
    p.add_argument("--out", help="dataset directory, outside the source tree (default: ../knowledge-data/<name> beside the source)")
    p.add_argument("--name", help="MCP instance name (default: source directory name)")
    p.add_argument("--project-id", help="stable ID matching ontology/spec project_id (kept on rebuild)")
    p.add_argument("--semantic-store", help="aligned semantic SQLite store; never activated by this tool")
    p.add_argument("--ontology-mode", choices=["off", "baseline", "concept_text", "relations", "combined"], default="off")
    p.add_argument("--ontology-budget-ms", type=int, default=1000, help="optional ontology/status budget 1..5000ms")
    p.add_argument("--ontology", help="source-relative ontology YAML; prepare a held candidate without changing current")
    p.add_argument("--spec", help="source-relative spec YAML; requires the same project_id as the dataset")
    p.add_argument("--port", type=int, default=DEFAULT_PORT, help=f"HTTP port written into the server config (default {DEFAULT_PORT})")
    p.add_argument("--graph-only", action="store_true", help="build only the graph index; no Ollama needed")
    p.add_argument("--embedder", choices=["ollama", "mock"], default="ollama",
                   help="embedding backend; mock produces a dataset that cannot answer semantic queries (default ollama)")
    p.add_argument("--model", default=DEFAULT_MODEL, help=f"Ollama embedding model (default {DEFAULT_MODEL})")
    p.add_argument("--embed-dim", type=int, default=DEFAULT_DIM, help=f"embedding dimension (default {DEFAULT_DIM})")
    p.add_argument("--ollama-url", default=DEFAULT_OLLAMA, help=f"Ollama endpoint (default {DEFAULT_OLLAMA})")
    p.add_argument("--source-mode", choices=["committed", "working-tree"], default="committed",
                   help="index the committed HEAD (reproducible) or include uncommitted edits (default committed)")
    p.add_argument("--version", dest="version_label", help="version label (default v<date>-<time>-<commit8>)")
    p.add_argument("--build-timeout", default="2h", help="total build deadline passed to cks setup (default 2h)")
    p.add_argument("--no-build", action="store_true", help="skip `make build-bins` and use existing binaries")
    p.add_argument("--bin-dir", help="directory holding prebuilt ckg/ckv/cks (implies --no-build)")
    p.add_argument("--no-verify", action="store_true", help="skip starting the server for a health check")
    p.add_argument("--allow-missing-embeddings", action="store_true",
                   help="do not fail when some chunks could not be embedded")
    p.add_argument("--pack", help="project pack directory (e.g. projects/stablenet) whose setup.yaml inputs the build uses: "
                   "graph policy, vector policy, domain knowledge, glossary, flow corpus, file list")
    p.add_argument("--skip-pack-input", action="append", default=[], choices=sorted(PATH_FLAGS), metavar="KEY",
                   help="leave one pack input out (repeatable), e.g. filelist to index the whole repository")
    p.add_argument("--min-canonical", type=float, metavar="RATIO",
                   help="promotion gate: minimum canonical_id coverage of symbol chunks (e.g. 0.9 for Go); "
                        "without it the engine skips the join-density check and warns")
    p.add_argument("--no-build-sources", action="store_true",
                   help="do not list Go sources under build/ directories for the vector index; by default the "
                        "Go files the graph loads there (e.g. internal/build) are indexed by both engines")
    p.add_argument("--compare", help="another dataset directory to profile side by side after the build")
    p.add_argument("--plain", action="store_true", help="print one line per change instead of the live dashboard")
    p.add_argument("-y", "--yes", action="store_true", help="start without the confirmation prompt")
    return p


def _ask(prompt: str, default: Optional[str] = None) -> str:
    from rich.prompt import Prompt
    return Prompt.ask(prompt, default=default) if default is not None else Prompt.ask(prompt)


def build_options(ns: argparse.Namespace, interactive: bool) -> Options:
    src_arg = ns.src
    if not src_arg:
        if not interactive:
            raise SystemExit("ks-setup: --src is required when not running in a terminal")
        src_arg = _ask("Project to index (Git repository path)", str(Path.cwd()))
    src = expand(src_arg)
    name = ns.name or slug(src.name)
    out_arg = ns.out
    default_out = str(src.parent / "knowledge-data" / name)
    if not out_arg:
        out_arg = _ask("Dataset directory", default_out) if interactive else default_out
    bin_dir = expand(ns.bin_dir) if ns.bin_dir else None
    return Options(src=src, out=expand(out_arg), name=name,
                   project_id=ns.project_id, semantic_store=expand(ns.semantic_store) if ns.semantic_store else None,
                   ontology_mode=ns.ontology_mode, ontology_budget_ms=ns.ontology_budget_ms, ontology=ns.ontology, spec=ns.spec, embedder=ns.embedder, model=ns.model,
                   embed_dim=ns.embed_dim, ollama_url=ns.ollama_url, port=ns.port, version=ns.version_label,
                   graph_only=ns.graph_only, source_mode=ns.source_mode, build=not (ns.no_build or bin_dir),
                   bin_dir=bin_dir, verify=not ns.no_verify, build_timeout=ns.build_timeout,
                   strict_embeddings=not ns.allow_missing_embeddings,
                   pack_dir=expand(ns.pack) if ns.pack else None, skip_pack_inputs=list(ns.skip_pack_input),
                   compare_with=expand(ns.compare) if ns.compare else None, min_canonical=ns.min_canonical,
                   build_sources=not ns.no_build_sources)


def _rich_available() -> bool:
    try:
        import rich  # noqa: F401
    except ImportError:
        return False
    return True


def main(argv: Optional[List[str]] = None) -> int:
    ns = parser().parse_args(argv)
    tty = sys.stdin.isatty() and sys.stdout.isatty()
    use_rich = tty and not ns.plain and _rich_available()
    opts = build_options(ns, interactive=use_rich)
    if opts.graph_only and ns.version_label:
        print("ks-setup: --version applies to full builds; graph-only builds are unversioned", file=sys.stderr)
        return 2

    if use_rich and not ns.yes:
        from rich.console import Console
        from rich.prompt import Confirm
        from rich.table import Table
        t = Table.grid(padding=(0, 2))
        for k, v in header_rows(opts):
            t.add_row(f"[bold]{k}", v)
        Console().print(t)
        if not Confirm.ask("Start?", default=True):
            return 1

    opts.log_file.parent.mkdir(parents=True, exist_ok=True)
    log_path = opts.log_file
    run = new_run(opts)
    with open(log_path, "w") as log:
        pipe = Pipeline(opts, run, log)
        if use_rich:
            from .ui import RichView
            view = RichView(opts, run, str(log_path))
        else:
            view = PlainView(opts, run, str(log_path))
        ok = view.live(pipe.execute, pipe.cancel)
    if pipe.cancelled.is_set():
        return 130
    return 0 if ok else 1


if __name__ == "__main__":
    sys.exit(main())
