"""Offline: demote doc (x f) and file_header (x f) seeds also under Unknown intent; seed slot count and selected
neighbors fixed per pack. Baseline (f=1) must reproduce the variant's found count."""
import json, sys, yaml
from pathlib import Path
SCEN = Path('/Users/kevin/work/github/0xmhha/auto-coding/knowledge/knowledge-system/docs/spec-driven/operational-preparation/retrieval-eval-grpc-go-v1/inputs/scenarios')
tag = sys.argv[1]
def ov(a, f, s, e): return a['file'] == f and a['start_line'] <= e and s <= a['end_line']
def isdoc(c): return c.get('kind', '') == 'doc' or c['file'].lower().endswith(('.md', '.markdown'))
for slug in ['qwen3-embedding-4b', 'bge-m3']:
    found = {r['name']: r for r in json.load(open(f'eval-{tag}/found-{slug}.json'))['scenarios']}
    rep = {r['name']: r for r in json.load(open(f'eval-{tag}/debug-report-{slug}.json'))['scenarios']}
    for fac in (1.0, 0.5, 0.25):
        hit = 0; gain = []; lose = []; docslots = 0
        for name, f in found.items():
            row = rep[name]
            r = next(json.loads(l) for l in open(row['trace']) if json.loads(l)['seq'] == row['seq'])
            s2 = [e for e in r['events'] if e['stage'] == 'stage2' and e['step'] == 'rank_citations' and e['phase'] == 'after'][0]['data']
            dec = [e for e in r['events'] if e['stage'] == 'stage4' and e['step'] == 'allocate' and e['phase'] == 'after'][0]['data']['decisions']
            ans = yaml.safe_load((SCEN / f['snapshot'] / f'{name}.yaml').read_text())['expected_citations']
            sel = [d for d in dec if d['decision'].startswith('selected')]
            nseed = sum(d['origin'] == 'seed' for d in sel)
            nb_found = any(d['origin'] == 'neighbor' and ov(a, d['file'], d['start'], d['end']) for a in ans for d in sel)
            cites = s2['citations']
            apply = not s2['demote_docs']
            def sc(c):
                if apply and (isdoc(c) or c.get('kind', '') == 'file_header'): return c['score'] * fac
                return c['score']
            top = sorted(cites, key=lambda c: -sc(c))[:nseed]
            docslots += sum(isdoc(c) for c in top)
            h = nb_found or any(ov(a, c['file'], c['start'], c['end']) for a in ans for c in top)
            hit += h
            if h and f['loss'] != 'found': gain.append(name)
            if not h and f['loss'] == 'found': lose.append(name)
        print(slug, f'factor {fac}: found {hit} (actual {sum(r["loss"]=="found" for r in found.values())}) doc seed slots {docslots} gain {gain} lose {lose}')
