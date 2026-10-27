"""Offline: with the neighbor slots each final pack actually gave neighbors, would the answer neighbor get a slot
under a different neighbor order? Seeds are fixed as in the final pack; only neighbor order changes."""
import json, sys, yaml
from pathlib import Path
W = Path('/Users/kevin/work/github/0xmhha/auto-coding/knowledge-data/grpc-go-retrieval-eval')
SCEN = Path('/Users/kevin/work/github/0xmhha/auto-coding/knowledge/knowledge-system/docs/spec-driven/operational-preparation/retrieval-eval-grpc-go-v1/inputs/scenarios')
def ov(a, f, s, e): return a['file'] == f and a['start_line'] <= e and s <= a['end_line']
span = lambda n: n['end'] - n['start']
ORDERS = {
 'stage4_now': lambda n: (-n['score'], n['file'], n['start']),
 'd4_stage3': lambda n: (-n['score'], -span(n), n['file'], n['start']),
 'rel': lambda n: (-n['rel'], -n['score'], -span(n)),
 'score_x_rel': lambda n: (-n['score'] * n['rel'], -span(n)),
 'score_x_rel2': lambda n: (-n['score'] * n['rel'] ** 2, -span(n)),
 'tie_rel': lambda n: (-round(n['score'], 6), -n['rel'], -span(n)),
}
for slug in ['qwen3-embedding-4b', 'bge-m3']:
    rp = json.load(open(W / 'eval-final' / f'relprobe-{slug}.json'))
    found = {r['name']: r for r in json.load(open(W / 'eval-final' / f'found-{slug}.json'))['scenarios']}
    rep = {r['name']: r for r in json.load(open(W / 'eval-final' / f'debug-report-{slug}.json'))['scenarios']}
    res = {o: {'hit': 0, 'gain': [], 'lose': []} for o in ORDERS}
    for name, d in rp.items():
        f = found[name]; nbs = d['neighbors']
        k = f['selected_by_origin'].get('neighbor', 0)
        ans = yaml.safe_load((SCEN / f['snapshot'] / f'{name}.yaml').read_text())['expected_citations']
        row = rep[name]
        r = next(json.loads(l) for l in open(row['trace']) if json.loads(l)['seq'] == row['seq'])
        dec = [e for e in r['events'] if e['stage'] == 'stage4' and e['step'] == 'allocate' and e['phase'] == 'after'][0]['data']['decisions']
        # answer in a selected seed: neighbor order cannot lose it (seeds stay as in the final pack)
        seed_found = any(d['origin'] == 'seed' and d['decision'].startswith('selected') and ov(a, d['file'], d['start'], d['end'])
                         for a in ans for d in dec)
        for o, key in ORDERS.items():
            top = sorted(nbs, key=key)[:k]
            hit = seed_found or any(ov(a, n['file'], n['start'], n['end']) for a in ans for n in top)
            res[o]['hit'] += hit
            if hit and f['loss'] != 'found': res[o]['gain'].append(name)
            if not hit and f['loss'] == 'found': res[o]['lose'].append(name)
    print('==', slug, 'final found', sum(r['loss'] == 'found' for r in found.values()))
    for o, r in res.items():
        print(f"  {o:13s} found {r['hit']:3d}  gain {len(r['gain'])} {r['gain']}  lose {len(r['lose'])} {r['lose']}")
