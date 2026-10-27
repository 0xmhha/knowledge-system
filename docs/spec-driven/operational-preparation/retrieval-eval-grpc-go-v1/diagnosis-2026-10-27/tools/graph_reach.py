"""For never-retrieved scenarios: can the graph reach an answer node from the Stage 2 seeds, and by which edges?

Seeds = Stage 2 final citations (top 10, as Stage 3 expands). A citation maps to the graph nodes whose span
contains or overlaps it (function/method/type level). BFS up to 2 hops over code edges, both directions.
Also checks the "same file" shortcut (an answer node in a seed's file).
"""
import json, sqlite3, sys, collections
from pathlib import Path

W = Path('/Users/kevin/work/github/0xmhha/auto-coding/knowledge-data/grpc-go-retrieval-eval')
SKIP = {'changed_in', 'blame', 'has_hunk', 'adjacent', 'contains'}
KINDS = ('Function', 'Method', 'Struct', 'Interface', 'TypeAlias', 'Constant', 'Variable', 'Field')


def trace_record(path, seq):
    for line in open(path):
        r = json.loads(line)
        if r['seq'] == seq:
            return r


def nodes_for(con, file, start, end):
    q = ('SELECT id, type, qualified_name FROM nodes WHERE file_path = ? AND start_line <= ? AND end_line >= ? '
         'AND type IN (%s)' % ','.join('?' * len(KINDS)))
    rows = con.execute(q, (file, end, start, *KINDS)).fetchall()
    # keep the innermost-ish: all overlapping symbol nodes are fine for reachability
    return rows


def main(model):
    found = json.load(open(W / 'found-head9e4a' / f'found-{model}.json'))['scenarios']
    report = json.load(open(W / 'eval-head9e4a' / f'debug-report-{model}.json'))['scenarios']
    by = {r['name']: r for r in report}
    out = collections.Counter()
    rel_hop1 = collections.Counter()
    details = []
    for s in found:
        if s['loss'] != 'never_retrieved':
            continue
        row = by[s['name']]
        rec = trace_record(row['trace'], row['seq'])
        final = next(e['data']['citations'] for e in reversed(rec['events'])
                     if e['stage'] == 'stage2' and e['step'] == 'rank_citations' and e['phase'] == 'after')[:10]
        db = W / f"chain-{s['snapshot']}" / 'graph' / 'graph.db'
        con = sqlite3.connect(f'file:{db}?mode=ro', uri=True)
        ans = set()
        for a in s['index']:
            ans |= {n[0] for n in nodes_for(con, a['file'], a['start_line'], a['end_line'])}
        seeds = set()
        for c in final:
            seeds |= {n[0] for n in nodes_for(con, c['file'], c['start'], c['end'])}
        seed_files = {c['file'] for c in final}
        same_file = any(a['file'] in seed_files for a in s['index'])
        # BFS
        frontier = {n: () for n in seeds}
        seen = dict(frontier)
        hit = None
        for hop in (1, 2):
            nxt = {}
            for n, path in frontier.items():
                for src, dst, typ in con.execute('SELECT src, dst, type FROM edges WHERE src = ? OR dst = ?', (n, n)):
                    if typ in SKIP:
                        continue
                    m = dst if src == n else src
                    d = typ if src == n else typ + '^-1'
                    if m not in seen:
                        seen[m] = path + (d,)
                        nxt[m] = path + (d,)
            for m in nxt:
                if m in ans:
                    hit = (hop, nxt[m])
                    break
            if hit:
                break
            frontier = nxt
            if len(frontier) > 20000:
                break
        key = f'hop{hit[0]}' if hit else 'unreachable<=2'
        out[key] += 1
        if hit:
            rel_hop1['/'.join(hit[1])] += 1
        out['same_file_as_seed'] += same_file
        out['answer_nodes_found'] += bool(ans)
        out['seed_nodes_found'] += bool(seeds)
        details.append({'name': s['name'], 'reach': key, 'path': hit[1] if hit else None, 'same_file': same_file,
                        'seeds': len(seeds), 'answers': len(ans)})
    print(model, dict(out))
    print('  paths', rel_hop1.most_common(15))
    return details


if __name__ == '__main__':
    res = {m: main(m) for m in ('qwen3-embedding-4b', 'bge-m3')}
    Path(sys.argv[1]).write_text(json.dumps(res, indent=1))
