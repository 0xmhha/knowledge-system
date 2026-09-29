#!/usr/bin/env python3
"""Read-only CKV/CKG alignment report for one promoted dataset version."""

import argparse
from contextlib import closing
import json
import sqlite3
import sys
from pathlib import Path


def read_db(path: Path) -> sqlite3.Connection:
    return sqlite3.connect(path.as_uri() + "?mode=ro", uri=True)


def scalar(db: sqlite3.Connection, sql: str) -> int:
    return int(db.execute(sql).fetchone()[0])


def main() -> None:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("dataset", type=Path, help="dataset version directory containing graph/ and vector/")
    args = parser.parse_args()
    root = args.dataset.resolve()
    manifest = json.loads((root / "vector" / "manifest.json").read_text(encoding="utf-8"))
    with closing(read_db(root / "vector" / "vector.db")) as vector, closing(read_db(root / "graph" / "graph.db")) as graph:
        symbol_count = scalar(vector, "SELECT COUNT(*) FROM chunks WHERE chunk_kind IN ('symbol', 'function_split') AND start_line > 0")
        canonical_count = scalar(vector, "SELECT COUNT(*) FROM chunks WHERE chunk_kind IN ('symbol', 'function_split') AND start_line > 0 AND canonical_id <> ''")
        graph_ids = {row[0] for row in graph.execute("SELECT canonical_id FROM nodes WHERE canonical_id IS NOT NULL AND canonical_id <> ''")}
        orphan_ids = sum(
            canonical_id not in graph_ids
            for (canonical_id,) in vector.execute("SELECT canonical_id FROM chunks WHERE canonical_id <> ''")
        )
        commits = [row[0] for row in vector.execute("SELECT DISTINCT commit_hash FROM chunks WHERE commit_hash <> '' ORDER BY commit_hash")]
        graph_files = scalar(graph, "SELECT COUNT(*) FROM nodes WHERE type = 'File'")
        report = {
            "dataset": str(root),
            "source_root": manifest.get("src_root", ""),
            "source_commit": manifest.get("src_commit", ""),
            "graph_source_commit": (manifest.get("sources") or {}).get("ckg", {}).get("src_commit", ""),
            "vector_commits": commits,
            "snapshot_aligned": bool(manifest.get("src_commit"))
            and manifest.get("src_commit") == (manifest.get("sources") or {}).get("ckg", {}).get("src_commit")
            and commits == [manifest.get("src_commit")],
            "graph_file_nodes": graph_files,
            "graph_canonical_nodes": len(graph_ids),
            "vector_chunks": scalar(vector, "SELECT COUNT(*) FROM chunks"),
            "manifest_chunk_count": manifest.get("chunk_count", 0),
            "manifest_symbol_count": manifest.get("symbol_count", 0),
            "manifest_canonical_count": manifest.get("canonical_count", 0),
            "symbol_chunks": symbol_count,
            "canonical_symbol_chunks": canonical_count,
            "canonical_ratio": round(canonical_count / symbol_count, 6) if symbol_count else None,
            "orphan_canonical_chunks": orphan_ids,
            "split_document_children": scalar(vector, "SELECT COUNT(*) FROM chunks WHERE parent_id <> ''"),
            "languages": dict(vector.execute("SELECT language, COUNT(*) FROM chunks GROUP BY language ORDER BY language")),
            "embedder": manifest.get("embedding_model", ""),
            "embedding_dim": manifest.get("embedding_dim", 0),
            "semantic_quality_measured": False,
        }
    report["manifest_counts_match_db"] = (
        report["manifest_chunk_count"] == report["vector_chunks"]
        and report["manifest_symbol_count"] == report["symbol_chunks"]
        and report["manifest_canonical_count"] == report["canonical_symbol_chunks"]
    )
    print(json.dumps(report, ensure_ascii=False, indent=2, sort_keys=True))
    if not report["manifest_counts_match_db"] or not report["snapshot_aligned"] or report["orphan_canonical_chunks"]:
        sys.exit(1)


if __name__ == "__main__":
    main()
