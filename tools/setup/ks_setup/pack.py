"""Project packs: the per-project inputs under projects/<name>/.

A pack's setup.yaml names the governance policy, the vector chunk policy, the
domain-knowledge entries, the glossary, the flow corpus and the build-scope
file list. `cks setup --config` reads the same file, but a versioned build
needs the project ID that `cks init` pins, so the tool keeps its own init
config and passes the pack inputs as flags, resolved to absolute paths.
"""
from __future__ import annotations

import os
import re
from dataclasses import dataclass, field
from pathlib import Path
from typing import Dict, List, Optional

# setup.yaml key -> `cks setup` flag. Only path-valued inputs are forwarded;
# embedder settings stay with the tool's own options so one source of truth
# decides the embedding identity.
PATH_FLAGS = {
    "policy_file": "--policy-file",
    "security_pattern_file": "--security-pattern-file",
    "vector_policy": "--vector-policy",
    "domain_knowledge": "--domain-knowledge",
    "glossary_file": "--glossary-file",
    "flow_corpus": "--flow-corpus",
    "filelist": "--filelist",
    "code_root": "--code-root",
    "vector_build_sources": "--vector-build-sources",
}

# Inputs that only make sense with a vector index (the glossary and code root
# are checked against the domain entries, which feed the vector corpus).
VECTOR_ONLY = {"vector_policy", "flow_corpus", "domain_knowledge", "vector_build_sources", "glossary_file", "code_root"}

_VAR = re.compile(r"\$\{([A-Za-z_][A-Za-z0-9_]*)\}")


@dataclass
class Pack:
    root: Path
    inputs: Dict[str, Path] = field(default_factory=dict)  # key -> absolute path
    embedder: Optional[str] = None
    model_name: Optional[str] = None

    @property
    def name(self) -> str:
        return self.root.name

    def setup_flags(self, graph_only: bool, derived_dir: Path) -> List[str]:
        argv: List[str] = []
        for key, flag in PATH_FLAGS.items():
            if key in self.inputs and not (graph_only and key in VECTOR_ONLY):
                argv += [flag, str(self.inputs[key])]
        if "domain_knowledge" in self.inputs and not graph_only:
            argv += ["--derived-dir", str(derived_dir)]
        return argv

    def server_flags(self, graph_only: bool, derived_dir: Path) -> List[str]:
        """gen-config flags so the server uses what the build consumed."""
        argv: List[str] = []
        if "policy_file" in self.inputs:
            argv += ["--policy-file", str(self.inputs["policy_file"])]
        if "glossary_file" in self.inputs:
            argv += ["--glossary", str(self.inputs["glossary_file"])]
        if "domain_knowledge" in self.inputs and not graph_only:
            argv += ["--domain-project-dir", str(self.inputs["domain_knowledge"]),
                     "--domain-corpus-dir", str(derived_dir / "domain-corpus")]
        return argv

    def missing(self) -> List[str]:
        return [f"{k}: {p}" for k, p in self.inputs.items() if not p.exists()]


def _expand(value: str, env: Dict[str, str]) -> str:
    def sub(m: "re.Match[str]") -> str:
        name = m.group(1)
        if name not in env:
            raise ValueError(f"${{{name}}} is not set")
        return env[name]
    return _VAR.sub(sub, value)


def load(root: Path, src: Path) -> Pack:
    """Read <root>/setup.yaml. ${CKS_CODE_ROOT} expands to the indexed source,
    the variable the pack documents for its code_root."""
    import yaml  # declared in pyproject; the system python usually ships it

    path = root / "setup.yaml"
    if not path.is_file():
        raise ValueError(f"{root} has no setup.yaml; not a project pack")
    data = yaml.safe_load(path.read_text()) or {}
    if not isinstance(data, dict):
        raise ValueError(f"{path} is not a mapping")
    env = dict(os.environ)
    env["CKS_CODE_ROOT"] = str(src)
    pack = Pack(root=root.resolve(), embedder=data.get("embedder"), model_name=data.get("model_name"))
    for key in PATH_FLAGS:
        value = data.get(key)
        if not value:
            continue
        expanded = Path(_expand(str(value), env)).expanduser()
        pack.inputs[key] = (expanded if expanded.is_absolute() else pack.root / expanded).resolve()
    return pack
