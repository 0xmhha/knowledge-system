#!/usr/bin/env python3
"""Check that the documentation teaches commands the binaries accept.

The CLI surface moves; prose does not follow on its own. Twice now a manual
audit found live documents instructing operators to run retired binaries, pass
flags that no longer parse, or open paths that no longer exist. This script is
that audit, mechanised, so the next drift is caught when it lands.

Ground truth comes from the binaries themselves — a recursive `--help` walk of
ckg/ckv/cks yields the command tree and every flag — plus the Makefiles and the
working tree. Documentation is then parsed for command invocations, `make`
targets and backticked repository paths, and each is validated against that
truth.

The documentation catalog covers every tracked prose file. Current references
are checked even when their names or prose describe a date or handoff. Only
recorded evaluation/source data, project domain inputs and fixtures are exempt;
rewriting their captured bytes would change the evidence. Existing non-prose
script checks retain their historical detection rules.

Usage:  scripts/check-docs.py [--verbose]
Exit:   0 clean, 1 findings, 2 could not establish ground truth
"""

from __future__ import annotations

import glob
import json
import os
from pathlib import Path
import re
import shlex
import subprocess
import sys

from documentation_contract import (
    CURRENT_ROLES, PROSE_SUFFIXES, data_role, validate_catalog, validate_links,
)

BINARIES = ("ckg", "ckv", "cks")

# Names the consolidation retired. Their appearance as a command head in a live
# document is an instruction that cannot be followed.
RETIRED = {
    "knowledge-setup", "filelist-gen", "system-mcp", "graph-mcp",
    "vector-mcp", "cks-mcp", "cks-agent", "cks-eval", "cks-domain-export",
    "cks-domain-sync", "cks-glossary-gen", "cks-entry-verify",
    "cks-inventory-check", "cks-anchor-refresh", "cks-promotion-worksheet",
}
RETIRED_SUBCOMMANDS = {("ckg", "serve"), ("ckg", "viewer")}

# Deliberate keeps: (path suffix, substring). These are not instructions —
# a version string, a flag's own name, filenames owned by another repository.
ALLOWED = (
    ("system/Makefile", "CKS_VERSION"),
    ("system/Makefile", "--cks-mcp"),
    ("system/docs/coding-agent-mcp-mapping.md", "phase3-cks-mcp-ckv.md"),
    ("system/docs/coding-agent-mcp-mapping.md", "phase4-cks-mcp-ckg.md"),
    ("system/docs/SETUP.md", "→"),      # shell→Go mapping table: old on the left
    ("docs/design/cli-consolidation.md", ""),   # the migration's own narrative
    ("docs/downstream-sync.md", "make build-mcp"),
    ("docs/dev/README.md", "archive/"),   # its "Dangling references" section
    ("repro-since-injection.sh", "--since \"--output="),  # the injected value is the point
)

# Paths that legitimately do not resolve in this tree.
PATH_EXEMPT = (
    "cmd/gstable", "cmd/genesis_generator",    # the indexed project's tree
    "internal/testlog", "systemcontracts",
    "tools/list", "tools/call",                # MCP method names, not paths
    "docs/domain-knowledge",                   # the coding-agent repo's layout
    "pkg/store", "pkg/smartctx", "pkg/impact",  # named in VISION as planned
    "pkg/composer",                            # planned, named in a design doc
    "internal/ethapi", "projects/go-stablenet",  # the indexed project's tree
    "tools/check_corpus.py", "tools/build_corpus.py",  # pack-local, relative
    "internal/graph/eval",                     # eval harness moved; see EVAL.md
)

HISTORICAL_MARKERS = (
    "historical design record", "역사 문서", "supersedes", "superseded",
    "handoff", "이 문서는 기록", "archive",
    "reflect the layout at implementation time",   # the design-record banner
)


def is_historical(path: str, head: str) -> bool:
    """A document that records the past rather than instructing the present."""
    if re.search(r"/archive/|/superpowers/|consolidation-session|/adr/", path):
        return True
    if re.search(r"projects/[^/]+/(domain-knowledge|eval)/", path):
        return True        # describes the indexed project, not this repository
    if re.search(r"\d{4}-\d{2}-\d{2}|(^|[-_])\d{4}-\d{2}", os.path.basename(path)):
        return True
    if re.search(r"HANDOFF|remaining-work|verification-worksheet|REMAINING-WORK",
                 os.path.basename(path)):
        return True
    low = head.lower()
    return any(m in low for m in HISTORICAL_MARKERS)


def allowed(path: str, line: str) -> bool:
    return any(path.endswith(suffix) and (sub == "" or sub in line)
               for suffix, sub in ALLOWED)


# --- ground truth ---------------------------------------------------------

FLAG_LINE = re.compile(r"^[ \t]+(?:-\w, )?--([\w-]+)(?:[ \t]+([^\n]*?))?[ \t]{2,}\S", re.M)


def walk(argv: list[str], depth: int = 0) -> dict:
    """Recursive --help walk: the command tree as the binary defines it."""
    proc = subprocess.run(argv + ["--help"], capture_output=True, text=True, timeout=60)
    text = proc.stdout + proc.stderr
    node: dict = {"flags": {m.group(1) for m in FLAG_LINE.finditer(text)},
                  "value_flags": {m.group(1) for m in FLAG_LINE.finditer(text)
                                  if (m.group(2) or "").strip().split(" ")[0] in
                                  {"string", "strings", "stringArray", "int", "int64", "int32",
                                   "uint", "uint32", "uint64", "float64", "duration"}}, "subs": {}}
    in_commands = False
    for line in text.splitlines():
        if re.match(r"^(Available Commands|Commands):", line):
            in_commands = True
            continue
        if not in_commands:
            continue
        if not line.startswith("  ") or not line.strip():
            in_commands = False
            continue
        name = line.split()[0]
        if name in ("help", "completion") or depth >= 3:
            continue
        if re.match(r"^[a-z][\w-]*$", name):
            node["subs"][name] = walk(argv + [name], depth + 1)
    return node


def binary_path(name: str) -> str | None:
    p = os.path.join("bin", name)
    return p if os.access(p, os.X_OK) else None


def make_targets() -> dict[str, set[str]]:
    out: dict[str, set[str]] = {}
    for makefile, label in (("Makefile", "root"), ("graph/Makefile", "graph"),
                            ("vector/Makefile", "vector"), ("system/Makefile", "system")):
        if not os.path.isfile(makefile):
            continue
        targets = set()
        for line in open(makefile, encoding="utf-8", errors="replace"):
            m = re.match(r"^([a-zA-Z][\w./-]*)\s*:(?!=)", line)
            if m:
                targets.add(m.group(1))
        out[label] = targets
    return out


# --- document parsing -----------------------------------------------------

def git_ignored() -> set[str]:
    try:
        out = subprocess.run(["git", "ls-files", "--others", "--ignored",
                              "--exclude-standard"], capture_output=True,
                             text=True, timeout=60)
        return set(out.stdout.split())
    except Exception:
        return set()


def documents() -> list[str]:
    patterns = ("*.md", "docs/**/*.md", "graph/*.md", "vector/**/*.md",
                "system/**/*.md", "projects/**/*.md", "tools/**/*.md",
                "cmd/**/*.md", "**/*.sh", "*/Makefile", "Makefile",
                "projects/**/*.yaml", "*.yaml")
    ignored = git_ignored()
    seen: list[str] = []
    for pat in patterns:
        for f in glob.glob(pat, recursive=True):
            if os.path.isfile(f) and "node_modules" not in f and f not in seen \
                    and f not in ignored:
                seen.append(f)
    return sorted(seen)


def engine_of(path: str) -> str | None:
    for engine in ("graph", "vector", "system"):
        if path.startswith(f"{engine}/") or path.startswith(f"docs/{engine}/"):
            return engine
    return None


def command_context(path: str, line: str, in_fence: bool) -> bool:
    """True when the line is something a reader would run, not prose about it."""
    if path.endswith((".sh", "Makefile")) or "/Makefile" in path:
        return not line.lstrip().startswith("#")
    return in_fence or bool(re.search(r"`make(?: -C \w+)? [a-z][\w-]*`", line))


def should_check(path: str, head: str, catalog: dict) -> bool:
    if path in catalog:
        return catalog[path]["role"] in CURRENT_ROLES
    if data_role(path) is not None:
        return False
    return not is_historical(path, head)


def current_reference_paths(catalog: dict) -> set[str]:
    return {path for path, row in catalog.items() if row["role"] in CURRENT_ROLES}


def logical_lines(text: str):
    pending, first = "", 0
    for lineno, line in enumerate(text.splitlines(), 1):
        if not pending:
            first = lineno
        pending += line.rstrip().removesuffix("\\") + " "
        if line.rstrip().endswith("\\"):
            continue
        yield first, pending.rstrip()
        pending = ""
    if pending:
        yield first, pending.rstrip()


def check_invocation(path: str, lineno: int, line: str, grammar: dict,
                     findings: list, strict_subcommands: bool = False) -> None:
    if re.search(r"[─│┌└▼]", line):
        return
    try:
        tokens = shlex.split(line.strip())
    except ValueError:
        tokens = line.strip().split()
    i = 0
    while i < len(tokens) and re.match(r"^[A-Z][A-Z0-9_]*=", tokens[i]):
        i += 1                                    # environment prefix
    if i >= len(tokens):
        return
    head = os.path.basename(tokens[i].strip("`\"',;()"))
    if head in RETIRED:
        findings.append((path, lineno, f"retired command `{head}`"))
        return
    m = re.match(r"^(?:\.?/?(?:bin/)?)?(ckg|ckv|cks)$", head)
    if not m or m.group(1) not in grammar:
        return
    if tokens[i].strip("\"'").startswith(("pkg/", "internal/", "docs/")):
        return
    if re.fullmatch(r"\s*(ckg|ckv|cks)(?:\s+(ckg|ckv|cks))+\s*", line):
        return
    binary = m.group(1)
    node = grammar[binary]
    allowed_flags = set(node["flags"])
    value_flags = set(node.get("value_flags", ()))
    chain = [binary]
    remaining = []
    i += 1
    while i < len(tokens):
        tok = tokens[i].strip("`\"',;)\\")
        if not tok or tok.startswith(("#", ">", "|", "&&", ";")) or re.match(r"^\d*[<>]", tok):
            break
        if tok.startswith("--"):
            remaining.append(tok)
            name = tok[2:].split("=", 1)[0]
            if "=" not in tok and name in value_flags and i + 1 < len(tokens):
                i += 1
            i += 1
            continue
        if tok in node["subs"]:
            node = node["subs"][tok]
            allowed_flags |= node["flags"]
            value_flags |= set(node.get("value_flags", ()))
            chain.append(tok)
            i += 1
            continue
        if tok.endswith(":") or tok.startswith("<"):
            return  # output labels or documented placeholders, not invocations
        if strict_subcommands and node["subs"] and not tok.startswith("-"):
            findings.append((path, lineno,
                             f"`{' '.join(chain)}` has no subcommand `{tok}`"))
            return
        remaining.append(tok)
        i += 1
    if len(chain) > 1 and (binary, chain[1]) in RETIRED_SUBCOMMANDS:
        findings.append((path, lineno, f"retired subcommand `{' '.join(chain)}`"))
        return
    for tok in remaining:
        tok = tok.strip("`\"',;)\\")
        long_flag = re.match(r"^--([\w-]+)(?:=.*)?$", tok)
        if long_flag and "<" not in tok and long_flag.group(1) not in allowed_flags:
            findings.append((path, lineno,
                             f"`{' '.join(chain)}` has no flag `--{long_flag.group(1)}`"))
        short = re.match(r"^-([a-z][\w-]{2,})$", tok)
        if short:
            findings.append((path, lineno,
                             f"`{' '.join(chain)}`: single-dash long flag `-{short.group(1)}`"))


def main() -> int:
    verbose = "--verbose" in sys.argv
    grammar = {}
    for name in BINARIES:
        p = binary_path(name)
        if p:
            grammar[name] = walk([p])
    if not grammar:
        print("check-docs: no binaries in bin/ — run `make build-bins` first "
              "(command grammar cannot be established)", file=sys.stderr)
        return 2

    targets = make_targets()
    findings: list[tuple[str, int, str]] = []
    scanned = 0
    root = Path.cwd()
    catalog_path = root / "docs/DOCUMENTATION-CATALOG.json"
    try:
        catalog = json.loads(catalog_path.read_text())
        tracked = subprocess.check_output(
            ["git", "ls-files", "--cached", "--others", "--exclude-standard", "-z"],
            text=True,
        ).split("\0")
        inventory = {p for p in tracked if Path(p).suffix.lower() in PROSE_SUFFIXES
                     and Path(p).is_file()}
        findings.extend(validate_catalog(root, catalog, inventory))
    except (OSError, ValueError, subprocess.CalledProcessError) as error:
        print(f"check-docs: cannot establish documentation catalog: {error}", file=sys.stderr)
        return 2
    if findings:
        for path, line, message in findings:
            print(f"  {path}:{line}: {message}", file=sys.stderr)
        return 1
    rows = catalog["documents"]

    for path in sorted(set(documents()) | current_reference_paths(rows)):
        text = open(path, encoding="utf-8", errors="replace").read()
        head = "\n".join(text.splitlines()[:25])
        if not should_check(path, head, rows):
            continue
        if path.endswith(".md"):
            findings.extend(validate_links(root, path, text))
        scanned += 1
        engine = engine_of(path)
        in_fence = False
        for lineno, line in (logical_lines(text) if path.endswith(".md")
                             else enumerate(text.splitlines(), 1)):
            if path.endswith(".md") and line.lstrip().startswith("```"):
                in_fence = not in_fence
                continue
            if allowed(path, line) or line.lstrip().startswith(">"):
                continue

            # command invocations + retired names anywhere on the line
            if re.search(r"(?:^|[ `(/])(?:ckg|ckv|cks)\b", line) or \
               any(r in line for r in RETIRED):
                before = len(findings)
                check_invocation(path, lineno, line, grammar, findings,
                                 strict_subcommands=path.endswith(".md") and command_context(path, line, in_fence))
                already = any(f[0] == path and f[1] == lineno for f in findings[before:])
                for retired in (() if already else RETIRED):
                    if re.search(rf"(?:^|[ `(]){re.escape(retired)}(?:[ `,)]|$)", line):
                        f = (path, lineno, f"retired name `{retired}`")
                        if f not in findings:
                            findings.append(f)

            if path.endswith(".md"):
                for inline in re.findall(r"`([^`]+)`", line):
                    check_invocation(path, lineno, inline, grammar, findings,
                                     strict_subcommands=True)

            # make targets — only where the line is a command, and only when
            # the target exists in no Makefile at all (a wrong -C is noise; the
            # engine Makefiles deliberately share target names).
            if command_context(path, line, in_fence):
                every = set().union(*targets.values()) if targets else set()
                for m in re.finditer(r"\bmake\s+(?:-C\s+(\w+)\s+)?([a-z][\w-]+)", line):
                    target = m.group(2)
                    if target not in every:
                        findings.append((path, lineno, f"`make {target}` is not a target"))

            # repository paths in backticks
            for m in re.finditer(r"`((?:cmd|internal|pkg|docs|projects|system|graph|vector|tools|scripts)/[\w./-]+?)`", line):
                p = m.group(1).rstrip("/")
                if any(p.startswith(x) for x in PATH_EXEMPT):
                    continue
                if re.search(r"\.[A-Z]\w*$", p):
                    continue        # package.Symbol reference, not a path
                if re.search(r"신규|\bnew file\b|\bto be added\b|\bplanned\b|\bgenerated\b", line):
                    continue        # a file the document proposes creating
                if not (os.path.exists(p) or glob.glob(p + "*")):
                    findings.append((path, lineno, f"path `{p}` does not exist"))

    if verbose:
        print(f"check-docs: {scanned} live documents scanned, "
              f"{sum(1 for _ in grammar)} binaries walked")
    if findings:
        print(f"check-docs: {len(findings)} finding(s) — live documentation "
              "disagrees with the code:", file=sys.stderr)
        for path, lineno, message in findings:
            print(f"  {path}:{lineno}: {message}", file=sys.stderr)
        print("\nFix the current reference or catalog classification; current prose is not exempt by a "
              "historical banner.", file=sys.stderr)
        return 1
    print(f"documentation: OK ({scanned} live documents)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
