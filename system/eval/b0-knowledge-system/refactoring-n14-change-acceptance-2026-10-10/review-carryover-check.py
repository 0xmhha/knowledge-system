"""Compare the N12 reviewed projection with the N14 result projection by content.
Evidence and section IDs embed the snapshot, so each is replaced by its span
content key (kind, path, lines, content SHA, canonical). CKV chunk IDs are
dataset-derived and reported separately, not compared as review content.
Usage: review-carryover-check.py <n12 projection> <n14 projection>"""
import json, sys
a, b = (json.load(open(p)) for p in sys.argv[1:3])
def norm(p):
    key = {e['id']: [e['kind'], e['path'], e['start_line'], e['end_line'], e['content_sha256'], e.get('canonical_id', '')] for e in p['evidence']}
    for s in p.get('sections') or []:
        key[s['id']] = ['section'] + key[s['evidence_id']]
    def walk(x, field=None):
        if isinstance(x, dict):
            return {k: walk(v, k) for k, v in x.items() if k not in ('snapshot', 'chunk_ids')}
        if isinstance(x, list):
            return [walk(v) for v in x]
        if isinstance(x, str) and x in key:
            return key[x]
        return x
    out = {f: sorted(json.dumps(walk(v), sort_keys=True) for v in p.get(f) or []) for f in ('concepts', 'requirements', 'assertions', 'claims', 'sections')}
    out['evidence'] = sorted(json.dumps(v) for v in key.values() if v[0] != 'section')
    return out, sum(len(s.get('chunk_ids') or []) for s in p.get('sections') or [])
(na, ca), (nb, cb) = norm(a), norm(b)
res = {f: na[f] == nb[f] for f in na}
res['counts'] = {f: len(nb[f]) for f in nb}
res['section_chunk_links'] = {'n12': ca, 'n14': cb}
res['all_identical_by_content'] = all(res[f] for f in na)
print(json.dumps(res))
