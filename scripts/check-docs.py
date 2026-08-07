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

Historical records are exempt: a file under an archive directory, with a date
in its name, or carrying a superseded/historical banner near the top documents
what was true then, and rewriting it would be a lie. Everything else is live
and must hold.

Usage:  scripts/check-docs.py [--verbose]
Exit:   0 clean, 1 findings, 2 could not establish ground truth
"""

from __future__ import annotations

import glob
import os
import re
import subprocess
import sys

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
    node: dict = {"flags": {m.group(1) for m in FLAG_LINE.finditer(text)}, "subs": {}}
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


def check_invocation(path: str, lineno: int, line: str, grammar: dict,
                     findings: list) -> None:
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
    binary = m.group(1)
    node = grammar[binary]
    allowed_flags = set(node["flags"])
    chain = [binary]
    i += 1
    while i < len(tokens):
        tok = tokens[i].strip("`\"',;)")
        if tok in node["subs"]:
            node = node["subs"][tok]
            allowed_flags |= node["flags"]
            chain.append(tok)
            i += 1
        else:
            break
    if len(chain) > 1 and (binary, chain[1]) in RETIRED_SUBCOMMANDS:
        findings.append((path, lineno, f"retired subcommand `{' '.join(chain)}`"))
        return
    for tok in tokens[i:]:
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

    for path in documents():
        text = open(path, encoding="utf-8", errors="replace").read()
        head = "\n".join(text.splitlines()[:25])
        if is_historical(path, head):
            continue
        scanned += 1
        engine = engine_of(path)
        in_fence = False
        for lineno, line in enumerate(text.splitlines(), 1):
            if path.endswith(".md") and line.lstrip().startswith("```"):
                in_fence = not in_fence
                continue
            if allowed(path, line) or line.lstrip().startswith(">"):
                continue

            # command invocations + retired names anywhere on the line
            if re.search(r"(?:^|[ `(/])(?:ckg|ckv|cks)\b", line) or \
               any(r in line for r in RETIRED):
                before = len(findings)
                check_invocation(path, lineno, line, grammar, findings)
                already = any(f[0] == path and f[1] == lineno for f in findings[before:])
                for retired in (() if already else RETIRED):
                    if re.search(rf"(?:^|[ `(]){re.escape(retired)}(?:[ `,)]|$)", line):
                        f = (path, lineno, f"retired name `{retired}`")
                        if f not in findings:
                            findings.append(f)

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
                if re.search(r"신규|\bnew file\b|\bto be added\b|\bplanned\b", line):
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
        print("\nFix the document, or — if it records the past — give it a "
              "historical banner (see scripts/check-docs.py).", file=sys.stderr)
        return 1
    print(f"documentation: OK ({scanned} live documents)")
    return 0


if __name__ == "__main__":
    sys.exit(main())
