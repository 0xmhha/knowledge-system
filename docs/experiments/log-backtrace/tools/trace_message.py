"""Steps S1 and S2 of the log back-trace study (DESIGN.md).

For every question with a failure message, find where the message is written in
the base source, map the line to its declaration, and walk the base graph from
there to the core declarations. The messages and labels stay outside the repo;
the output file holds ids, paths of steps and counts only.

Usage:
    python3 -I trace_message.py --runs <runs dir> --sources <sources dir> --out <json> [--show]
"""

from __future__ import annotations

import argparse
import json
import re
import sqlite3
from collections import defaultdict
from pathlib import Path

BATCHES = ("20261123-oracle-seed-stage2-batch1", "20261123-oracle-seed-stage2-pilot2")
MAX_HOPS = 4
DECLS = ("Function", "Method")
VALUES = ("Variable", "Constant")
CALLS = ("calls", "invokes")
FIELDS = ("reads_field", "writes_field")
MIXED = (*CALLS, "references", *FIELDS)
# A wrapped error prints as outer: inner; each part is written somewhere else.
WRAP = ": "
VERB = re.compile(r"%[-+# 0-9.*\[\]]*[a-zA-Z]")
GO_LITERAL = re.compile(r'"(?:[^"\\]|\\.)*"|`[^`]*`')


def fragments(message: str) -> list[str]:
    """Return the fixed parts of a message: one per wrapped part, longest piece of each."""
    out = []
    for part in message.split(WRAP):
        pieces = [p.strip() for p in VERB.split(part) if p.strip()]
        if pieces:
            out.append(max(pieces, key=len))
    return out


def literal_lines(source: Path, fragment: str) -> list[tuple[str, int]]:
    """Return (file, line) of non-test Go string literals holding the fragment."""
    hits = []
    for path in sorted(source.rglob("*.go")):
        rel = path.relative_to(source).as_posix()
        if rel.endswith("_test.go"):
            continue
        for number, line in enumerate(path.read_text(encoding="utf-8", errors="replace").splitlines(), 1):
            if fragment in line and any(fragment in lit for lit in GO_LITERAL.findall(line)):
                hits.append((rel, number))
    return hits


class Graph:
    """The base graph's declarations and the edges the walks use."""

    def __init__(self, db: Path) -> None:
        con = sqlite3.connect(f"file:{db}?mode=ro&immutable=1", uri=True)
        self.nodes = {
            r[0]: r
            for r in con.execute(
                "SELECT id, type, qualified_name, file_path, start_line, end_line, coalesce(canonical_id,'') FROM nodes"
            )
        }
        self.out: dict[str, list[tuple[str, str]]] = defaultdict(list)
        self.back: dict[str, list[tuple[str, str]]] = defaultdict(list)
        marks = ",".join("?" * len(MIXED))
        for kind, src, dst in con.execute(f"SELECT type, src, dst FROM edges WHERE type IN ({marks})", MIXED):
            self.out[src].append((kind, dst))
            self.back[dst].append((kind, src))
        con.close()
        self.by_file: dict[str, list[tuple]] = defaultdict(list)
        for n in self.nodes.values():
            self.by_file[n[3]].append(n)

    def enclosing(self, file: str, line: int, kinds: tuple[str, ...]) -> tuple | None:
        """Return the smallest node of the kinds around a line, canonical first."""
        found = [n for n in self.by_file[file] if n[1] in kinds and n[4] <= line <= n[5]]
        return min(found, key=lambda n: (n[5] - n[4], n[6] == "", n[0]), default=None)

    def steps(self, node: str, walk: str) -> list[str]:
        """Return the neighbours one hop of the walk away."""
        if walk == "callers":
            return [s for k, s in self.back[node] if k in CALLS]
        if walk == "callees":
            return [d for k, d in self.out[node] if k in CALLS]
        return [d for _, d in self.out[node]] + [s for _, s in self.back[node]]

    def reach(self, starts: set[str], targets: set[str], walk: str) -> dict[str, int | None]:
        """Return the hop that first reaches a target and the declarations reached by then."""
        seen, level = set(starts), set(starts)
        for hop in range(MAX_HOPS + 1):
            if seen & targets:
                return {"hop": hop, "declarations": self.count(seen - starts)}
            level = {n for x in level for n in self.steps(x, walk)} - seen
            seen |= level
        return {"hop": None, "declarations": self.count(seen - starts)}

    def count(self, ids: set[str]) -> int:
        """Count reached non-test Go functions, methods, variables and constants."""
        return sum(
            1
            for i in ids
            if i in self.nodes
            and self.nodes[i][1] in DECLS + VALUES
            and self.nodes[i][3].endswith(".go")
            and not self.nodes[i][3].endswith("_test.go")
        )


def study(question: dict, db: Path, source: Path, show: bool) -> dict:
    """Run S1 and S2 on one question."""
    graph = Graph(db)
    core = set()
    for c in question["citations"]:
        if c["core"]:
            name = c["symbol"].rsplit(".", 1)[-1]
            core |= {
                n[0]
                for n in graph.by_file[c["file"]]
                if n[2].rsplit(".", 1)[-1] == name and n[4] <= c["end_line"] and n[5] >= c["start_line"]
            }
    parts = []
    for fragment in fragments(question["log_text"]):
        hits = literal_lines(source, fragment)
        sites, values = set(), set()
        for file, line in hits:
            fn = graph.enclosing(file, line, DECLS)
            if fn:
                sites.add(fn[0])
                continue
            value = graph.enclosing(file, line, VALUES)
            if value:
                values.add(value[0])
        users = {s for v in values for _, s in graph.back[v] if graph.nodes[s][1] in DECLS}
        starts = sites | users
        part = {
            "matched_lines": len(hits),
            "emit_functions": len(sites),
            "message_values": len(values),
            "functions_using_those_values": len(users),
            "emit_is_core": bool(starts & core),
            "walks": {w: graph.reach(starts, core, w) for w in ("callers", "callees", "mixed")} if starts else {},
        }
        parts.append(part)
        if show:
            print(question["id"], repr(fragment))
            for i in sorted(sites):
                print("   emit", graph.nodes[i][2], f"{graph.nodes[i][3]}:{graph.nodes[i][4]}")
            for i in sorted(values):
                print("   value", graph.nodes[i][2], f"{graph.nodes[i][3]}:{graph.nodes[i][4]}")
            for i in sorted(users):
                print("   used by", graph.nodes[i][2], f"{graph.nodes[i][3]}:{graph.nodes[i][4]}")
            print("   core", sorted(graph.nodes[i][2] for i in core), part["walks"])
    return {"id": question["id"], "core_nodes": len(core), "parts": parts}


def main() -> None:
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    for name in ("runs", "sources", "out"):
        ap.add_argument(f"--{name}", type=Path, required=True)
    ap.add_argument("--show", action="store_true")
    args = ap.parse_args()
    results, skipped = [], []
    for batch in BATCHES:
        for path in sorted((args.runs / batch / "author").glob("*.json")):
            question = json.loads(path.read_text(encoding="utf-8"))
            if not question.get("log_text"):
                continue
            db = args.runs / batch / "graphs" / question["base"] / "graph.db"
            if not db.exists():
                skipped.append(question["id"])
                continue
            results.append(study(question, db, args.sources / question["base"], args.show))
    doc = {"design": "../DESIGN.md", "max_hops": MAX_HOPS, "no_graph": skipped, "questions": results}
    args.out.write_text(json.dumps(doc, indent=1) + "\n", encoding="utf-8")


if __name__ == "__main__":
    main()
