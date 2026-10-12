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
    project_id: Optional[str] = None    # explicit identity for project ontology/spec inputs
    semantic_store: Optional[Path] = None
    ontology_mode: str = "off"
    ontology_budget_ms: int = 1000
    ontology: Optional[str] = None       # source-relative YAML, captured in the candidate
    spec: Optional[str] = None
    semantic_candidate_dir: Optional[Path] = None  # held candidate; never the current pointer
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
    pack_dir: Optional[Path] = None     # project pack (projects/<name>) whose inputs the build uses
    skip_pack_inputs: List[str] = field(default_factory=list)  # setup.yaml keys to leave out
    min_canonical: Optional[float] = None  # promotion gate: minimum canonical_id coverage
    compare_with: Optional[Path] = None  # another dataset to profile side by side after the build
    build_sources: bool = True          # list Go sources under build/ dirs for the vector index

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
        suffix = f"-candidate-{self.semantic_candidate_dir.name}" if self.semantic_candidate_dir else ""
        return self.out.parent / f"{self.out.name}{suffix}-cks.yaml"

    @property
    def stdio_config(self) -> Path:
        suffix = f"-candidate-{self.semantic_candidate_dir.name}" if self.semantic_candidate_dir else ""
        return self.out.parent / f"{self.out.name}{suffix}-cks-stdio.yaml"

    @property
    def report_file(self) -> Path:
        return self.out.parent / f"{self.out.name}-setup-report.json"

    @property
    def log_file(self) -> Path:
        return self.out.parent / f"{self.out.name}-setup-logs" / time.strftime("%Y%m%d-%H%M%S.log")

    @property
    def derived_dir(self) -> Path:
        """Where the pack's domain artifacts are rendered for this dataset."""
        return self.out.parent / f"{self.out.name}-derived"

    @property
    def build_sources_file(self) -> Path:
        """The generated vector_build_sources list (see buildsrc)."""
        return self.derived_dir / "vector-build-sources.json"

    @property
    def versioned(self) -> bool:
        # The engine only supports graph-only builds in the unversioned layout.
        return not self.graph_only

    @property
    def serving_dir(self) -> Path:
        """The directory the server reads: <out>/current for versioned builds."""
        return self.semantic_candidate_dir or (self.out / "current" if self.versioned else self.out)

    @property
    def prepares_semantic(self) -> bool:
        return bool(self.ontology or self.spec)

    @property
    def meaning_store(self) -> Optional[Path]:
        if self.semantic_store:
            return self.semantic_store
        if self.prepares_semantic:
            return self.out.parent / f"{self.out.name}-semantic.sqlite"
        return None

    def validate_semantic(self) -> None:
        if self.ontology_mode not in {"off", "baseline", "concept_text", "relations", "combined"}:
            raise ValueError("unsupported ontology mode")
        if not 1 <= self.ontology_budget_ms <= 5000:
            raise ValueError("ontology budget must be 1..5000ms")
        if self.graph_only and (self.meaning_store or self.prepares_semantic or self.ontology_mode not in {"off", "baseline"}):
            raise ValueError("semantic inputs need a versioned graph+vector dataset")
        if self.ontology_mode not in {"off", "baseline"} and not self.meaning_store:
            raise ValueError("an enabled ontology mode requires --semantic-store or captured semantic inputs")
        if self.meaning_store:
            store = self.meaning_store.resolve()
            if store == self.src.resolve() or self.src.resolve() in store.parents:
                raise ValueError("semantic store must be outside the source tree")
        for name in (self.ontology, self.spec):
            if not name:
                continue
            file = Path(name)
            if file.is_absolute() or ".." in file.parts:
                raise ValueError("ontology/spec paths must be source-relative without ..")
            resolved = (self.src / file).resolve()
            if self.src.resolve() not in resolved.parents or not resolved.is_file():
                raise ValueError("ontology/spec input is missing or outside the source tree")
        if self.prepares_semantic and not (self.project_id or self.setup_config.exists()):
            raise ValueError("--project-id is required for new ontology/spec preparation; use the same ID in the YAML")


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
