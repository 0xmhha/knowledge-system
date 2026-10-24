"""Set sources.ckg.src_commit of each reindexed snapshot index to its graph's commit.

ckv reindex before 9c7231e8 re-aligned to the snapshot graph and recorded its
digest but kept the first snapshot's commit. A reindex with the fixed ckv of
snapshot 1->2 (bge-m3) produced the same vector.db (db_sha256) and differed
only in this field. Each change is checked first and the original manifest is
kept as manifest.json.pre-ledger-fix.
"""
import json, sqlite3, sys
from pathlib import Path

work = Path(sys.argv[1])
snaps = json.loads((work / 'snapshots.json').read_text())
rows = []
for slug in ('qwen3-embedding-4b', 'bge-m3'):
    for snap in snaps[1:]:
        vec = work / 'vec' / slug / snap
        man_path = vec / 'manifest.json'
        man = json.loads(man_path.read_text())
        ckg = man['sources']['ckg']
        graph = work / f'chain-{snap}' / 'graph'
        db = sqlite3.connect(f'file:{graph / "graph.db"}?mode=ro', uri=True)
        g = dict(db.execute("select key, value from manifest where key in ('src_commit','graph_digest')"))
        db.close()
        assert ckg['path'] == str(graph), (slug, snap, ckg['path'])
        assert ckg['graph_digest'] == g['graph_digest'], (slug, snap)
        assert g['src_commit'] == snap == man['src_commit'] == man['indexed_head'], (slug, snap)
        old = ckg['src_commit']
        if old == snap:
            rows.append({'model': slug, 'snapshot': snap, 'status': 'already-correct'})
            continue
        assert old == snaps[0], (slug, snap, old)
        backup = vec / 'manifest.json.pre-ledger-fix'
        if not backup.exists():
            backup.write_text(man_path.read_text())
        ckg['src_commit'] = snap
        man_path.write_text(json.dumps(man, indent=2) + '\n')
        rows.append({'model': slug, 'snapshot': snap, 'status': 'fixed', 'old_src_commit': old})
(work / 'fix-ckg-ledger.json').write_text(json.dumps(rows, indent=2) + '\n')
print(len(rows), {s: sum(r['status'] == s for r in rows) for s in ('fixed', 'already-correct')})
