#!/usr/bin/env python3
"""Count the v2 knowledge context state, declared-unknown IDs and citation
count per question and arm in the preserved part B captures. Run from the
repository root; writes state-tally.json."""
import collections, gzip, json
from pathlib import Path
B = Path("system/eval/b0-knowledge-system/refactoring-final2-b-2026-10-10")
out = {}
for run in ("dev-run", "final-run"):
    c = collections.Counter()
    with gzip.open(B / run / "capture/rows.jsonl.gz", "rt") as f:
        for line in f:
            r = json.loads(line); sc = r["call"]["response"]["structuredContent"]
            k = (sc.get("semantic") or {}).get("knowledge_context") or {}
            c["|".join([r["request_id"], r["arm"], str(k.get("state")), ",".join(d["id"] for d in k.get("declared_unknowns") or []), str(len(sc["citations"]))])] += 1
    out[run] = [dict(zip(("question", "arm", "knowledge_state", "declared_unknowns", "citations"), k.split("|")), rows=v) for k, v in sorted(c.items())]
(B / "state-tally.json").write_text(json.dumps(out, ensure_ascii=False, indent=1) + "\n")
