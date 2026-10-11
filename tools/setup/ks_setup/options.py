"""User-facing options and the paths derived from them."""
from __future__ import annotations

import os
import re
import time
from dataclasses import dataclass, field
from pathlib import Path
from typing import List, Optional

DEFAULT_MODEL = "bge-m3:latest"
DEFAULT_DIM = 1024
DEFAULT_OLLAMA = "http://127.0.0.1:11434"
DEFAULT_PORT = 8930


def repo_root() -> Path:
    """The knowledge-system checkout this tool ships in (tools/setup/ks_setup)."""
    return Path(__file__).resolve().parents[3]


def slug(name: str) -> str:
    """A safe instance name from a directory name."""
    s = re.sub(r"[^A-Za-z0-9_-]+", "-", name).strip("-").lower()
    return s or "project"


@dataclass
class Options:
    src: Path
    out: Path
    name: str
    embedder: str = "ollama"            # ollama | mock (mock is for trying the flow only)
    model: str = DEFAULT_MODEL
    embed_dim: int = DEFAULT_DIM
    ollama_url: str = DEFAULT_OLLAMA
    port: int = DEFAULT_PORT
    version: Optional[str] = None       # None: derived from time and source commit
    graph_only: bool = False
    source_mode: str = "committed"      # committed | working-tree
    build: bool = True                  # run `make build-bins` first
    bin_dir: Optional[Path] = None      # use prebuilt ckg/ckv/cks from here
    verify: bool = True                 # start the server once and call cks.ops.health
    build_timeout: str = "2h"
    strict_embeddings: bool = True      # fail when any chunk is left without an embedding
    extra_setup_args: List[str] = field(default_factory=list)

    @property
    def bins(self) -> Path:
        return self.bin_dir or (repo_root() / "bin")

    def binary(self, name: str) -> Path:
        return self.bins / name

    @property
    def setup_config(self) -> Path:
        """`cks init` output; it pins the project ID, so it is reused on rebuilds."""
        return self.out.parent / f"{self.out.name}-setup.yaml"

    @property
    def server_config(self) -> Path:
        return self.out.parent / f"{self.out.name}-cks.yaml"

    @property
    def stdio_config(self) -> Path:
        return self.out.parent / f"{self.out.name}-cks-stdio.yaml"

    @property
    def log_file(self) -> Path:
        return self.out.parent / f"{self.out.name}-setup-logs" / time.strftime("%Y%m%d-%H%M%S.log")

    @property
    def versioned(self) -> bool:
        # The engine only supports graph-only builds in the unversioned layout.
        return not self.graph_only

    @property
    def serving_dir(self) -> Path:
        """The directory the server reads: <out>/current for versioned builds."""
        return self.out / "current" if self.versioned else self.out


def choose_version(out: Path, commit: str, now: Optional[float] = None) -> str:
    """A new, sortable version label that does not exist yet under `out`."""
    stamp = time.strftime("%Y%m%d-%H%M%S", time.localtime(now))
    base = f"v{stamp}-{commit[:8]}" if commit else f"v{stamp}"
    label, n = base, 2
    while (out / label).exists():
        label, n = f"{base}-{n}", n + 1
    return label


def expand(path: str) -> Path:
    return Path(os.path.expanduser(path)).resolve()
