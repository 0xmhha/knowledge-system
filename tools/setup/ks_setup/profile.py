"""Measure a built dataset so two builds can be compared side by side.

Reads the engine manifests and runs a few read-only SQLite counts. Every
database is opened with mode=ro&immutable=1, so profiling never writes
sidecar files into a served version.
"""
from __future__ import annotations

import json
import sqlite3
from pathlib import Path
from typing import Any, Dict, Optional


def _ro(db: Path) -> sqlite3.Connection:
    return sqlite3.connect(f"file:{db}?mode=ro&immutable=1", uri=True)


def _one(conn: sqlite3.Connection, sql: str) -> Optional[int]:
    try:
        row = conn.execute(sql).fetchone()
    except sqlite3.Error:
        return None  # table absent in an older schema
    return row[0] if row else None


def _grouped(conn: sqlite3.Connection, sql: str) -> Dict[str, int]:
    try:
        return {str(k or ""): v for k, v in conn.execute(sql).fetchall()}
    except sqlite3.Error:
        return {}


def graph_profile(graph_dir: Path) -> Dict[str, Any]:
    out: Dict[str, Any] = {}
    manifest = graph_dir / "manifest.json"
    if manifest.is_file():
        m = json.loads(manifest.read_text())
        out.update(schema_version=m.get("schema_version"), src_commit=m.get("src_commit"),
                   languages=m.get("languages"), files=len(m.get("files") or []),
                   nodes=(m.get("stats") or {}).get("nodes"), edges=(m.get("stats") or {}).get("edges"))
    db = graph_dir / "graph.db"
    if db.is_file():
        conn = _ro(db)
        try:
            types = _grouped(conn, "SELECT type, count(*) FROM nodes GROUP BY type")
            out.update(
                functions=types.get("Function", 0) + types.get("Method", 0),
                commits=types.get("Commit", 0),
                policy_nodes=types.get("Policy", 0),
                security_pattern_nodes=types.get("SecurityPattern", 0),
                governed_by_edges=_one(conn, "SELECT count(*) FROM edges WHERE +type = 'governed_by'"),
                node_pr_links=_one(conn, "SELECT count(*) FROM node_prs"),
                test_nodes=_one(conn, "SELECT count(*) FROM nodes WHERE is_test = 1"),
                review_code_edges=_one(conn, "SELECT count(*) FROM code_edges"),
            )
        finally:
            conn.close()
        out["db_bytes"] = db.stat().st_size
    return out


def vector_profile(vector_dir: Path) -> Dict[str, Any]:
    out: Dict[str, Any] = {}
    manifest = vector_dir / "manifest.json"
    if manifest.is_file():
        m = json.loads(manifest.read_text())
        out.update(chunks=m.get("chunk_count"), languages=m.get("languages"),
                   model=m.get("embedding_model"), dim=m.get("embedding_dim"),
                   policy=((m.get("sources") or {}).get("policy") or {}).get("path"))
    db = vector_dir / "vector.db"
    if db.is_file():
        conn = _ro(db)
        try:
            out.update(
                chunk_kinds=_grouped(conn, "SELECT chunk_kind, count(*) FROM chunks GROUP BY chunk_kind"),
                categorized=_one(conn, "SELECT count(*) FROM chunks WHERE category IS NOT NULL AND category != ''"),
                with_guidance=_one(conn, "SELECT count(*) FROM chunks WHERE guidance IS NOT NULL AND guidance != ''"),
                with_canonical_id=_one(conn, "SELECT count(*) FROM chunks WHERE canonical_id IS NOT NULL AND canonical_id != ''"),
                domain_chunks=_one(conn, "SELECT count(*) FROM chunks WHERE category = 'domain'"),
                flow_chunks=_one(conn, "SELECT count(*) FROM chunks WHERE flow_meta IS NOT NULL AND flow_meta != ''"),
                test_chunks=_one(conn, "SELECT count(*) FROM chunks WHERE is_test = 1"),
            )
        finally:
            conn.close()
        out["db_bytes"] = db.stat().st_size
    return out


def dataset_profile(serving_dir: Path) -> Dict[str, Any]:
    """Profile <serving_dir>/graph and <serving_dir>/vector (resolving `current`)."""
    root = serving_dir.resolve()
    profile: Dict[str, Any] = {"path": str(root), "graph": graph_profile(root / "graph")}
    if (root / "vector").exists():
        profile["vector"] = vector_profile(root / "vector")
    return profile


def summary_lines(profile: Dict[str, Any]) -> list:
    g, v = profile.get("graph") or {}, profile.get("vector") or {}
    lines = [f"Graph:          {g.get('files')} files, {g.get('nodes')} nodes, {g.get('edges')} edges, "
             f"{g.get('functions')} functions, {g.get('commits')} commits, "
             f"{g.get('policy_nodes')} policies / {g.get('governed_by_edges')} governed_by"]
    if v:
        lines.append(f"Vector:         {v.get('chunks')} chunks ({v.get('model')}, {v.get('dim')}d), "
                     f"{v.get('categorized')} categorized, {v.get('with_guidance')} with guidance, "
                     f"{v.get('domain_chunks')} domain, {v.get('flow_chunks')} flow")
    return lines


# Metrics worth comparing between two builds of the same project, as
# (label, section, key). Nested dicts (languages, chunk kinds) are flattened.
COMPARE_KEYS = [
    ("graph schema", "graph", "schema_version"), ("source commit", "graph", "src_commit"),
    ("graph files", "graph", "files"), ("graph nodes", "graph", "nodes"), ("graph edges", "graph", "edges"),
    ("functions + methods", "graph", "functions"), ("commits", "graph", "commits"),
    ("policy nodes", "graph", "policy_nodes"), ("governed_by edges", "graph", "governed_by_edges"),
    ("node-PR links", "graph", "node_pr_links"), ("vector chunks", "vector", "chunks"),
    ("categorized chunks", "vector", "categorized"), ("chunks with guidance", "vector", "with_guidance"),
    ("chunks with canonical_id", "vector", "with_canonical_id"), ("domain chunks", "vector", "domain_chunks"),
    ("flow chunks", "vector", "flow_chunks"), ("test chunks", "vector", "test_chunks"),
]


def compare(this: Dict[str, Any], other: Dict[str, Any]) -> list:
    """Rows of (metric, this value, other value) for the shared metrics."""
    rows = []
    for label, section, key in COMPARE_KEYS:
        a, b = (this.get(section) or {}).get(key), (other.get(section) or {}).get(key)
        if a is None and b is None:
            continue
        rows.append((label, a, b))
    for section, key in (("graph", "languages"), ("vector", "languages"), ("vector", "chunk_kinds")):
        a, b = (this.get(section) or {}).get(key) or {}, (other.get(section) or {}).get(key) or {}
        for name in sorted(set(a) | set(b)):
            rows.append((f"{section} {key[:-1] if key.endswith('s') else key}: {name}", a.get(name), b.get(name)))
    return rows


def retained_versions(out: Path) -> Dict[str, Any]:
    """Version directories under a versioned dataset root and their disk use."""
    versions = sorted(p for p in out.iterdir() if p.is_dir() and not p.is_symlink() and p.name.startswith("v")) \
        if out.is_dir() else []
    total = 0
    for v in versions:
        for f in v.rglob("*"):
            try:
                if f.is_file() and not f.is_symlink():
                    total += f.stat().st_size
            except OSError:
                continue
    return {"count": len(versions), "bytes": total, "labels": [v.name for v in versions]}
