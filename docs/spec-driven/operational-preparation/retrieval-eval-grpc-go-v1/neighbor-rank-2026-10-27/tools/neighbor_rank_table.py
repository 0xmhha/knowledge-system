"""Tabulate NEIGHBOR-RANK-PLAN.md variants against eval-final (adds the go-stablenet Korean arm ko-<tag>): answer in pack, any-commit metrics, tokens, go-stablenet.

Run report_variant.py --tag <variant> --base head9e4a for each grpc-go variant first, and
retrieval_found_diagnosis.py --no-probe into eval-<variant>/found-<slug>.json.

usage: found_fix_table.py fix-d1 fix-d2 fix-d12 [...]
"""
import json
import subprocess
import sys
from collections import Counter
from pathlib import Path

W = Path(__file__).resolve().parent
E = Path('/Users/kevin/work/github/0xmhha/auto-coding/knowledge-data/go-stablenet-pr-eval-20261025')
MODELS = {'qwen3-embedding:4b': 'qwen3-embedding-4b', 'bge-m3': 'bge-m3'}
BASE = 'final'


def found(tag, slug):
    p = W / f'eval-{tag}' / f'found-{slug}.json'
    if False:
        p = W / 'found-head9e4a' / f'found-{slug}.json'
    return {r['name']: r for r in json.loads(p.read_text())['scenarios']}


def grpc(tag):
    s = {x['model']: x for x in json.loads((W / f'eval-{tag}' / 'summary-cmp.json').read_text())}
    out = {}
    for m, slug in MODELS.items():
        a = s[m]['methods']['cks_any_commit']
        f = found(tag, slug)
        out[m] = {'hit1': a['hit_at_1'], 'mrr': a['mrr'], 'tokens': s[m]['methods']['cks']['used_tokens_mean'],
                  'found': sum(r['loss'] == 'found' for r in f.values()),
                  'loss': dict(Counter(r['loss'] for r in f.values()))}
    return out


def stablenet(tag):
    line = subprocess.run([sys.executable, 'score_requery.py', tag], cwd=E / 'scripts', capture_output=True, text=True,
                          check=True).stdout.splitlines()[0]
    kv = dict(p.split('=') for p in line.split()[1:] if '=' in p)
    return {k: float(kv[k]) for k in ('r@5', 'r@10', 'mrr')}


def transitions(tag, slug):
    b, v = found(BASE, slug), found(tag, slug)
    gained = sorted(n for n in v if v[n]['loss'] == 'found' and b[n]['loss'] != 'found')
    lost = sorted(n for n in v if v[n]['loss'] != 'found' and b[n]['loss'] == 'found')
    return {'gained': [(n, b[n]['loss'], b[n]['ranks']['stage2'], b[n]['answer_stage4_decision'], v[n]['ranks']['stage4'])
                       for n in gained],
            'lost': [(n, v[n]['loss'], v[n]['ranks']['stage2'], v[n]['answer_stage4_decision'], b[n]['ranks']['stage4'])
                     for n in lost]}


tags = [BASE] + sys.argv[1:]
rows = {t: {'grpc': grpc(t), 'sn': stablenet(t), 'ko': stablenet('ko-' + t)} for t in tags}
base = rows[BASE]
print('| variant | qwen3 found / hit@1 / MRR / tokens | bge-m3 found / hit@1 / MRR / tokens | go-stablenet PR body r@5 / r@10 / MRR | go-stablenet Korean r@5 / r@10 / MRR |')
print('|---|---|---|---|---|')
for t, r in rows.items():
    cells = []
    for m in MODELS:
        g, bg = r['grpc'][m], base['grpc'][m]
        cells.append(f"{g['found']} ({g['found'] - bg['found']:+d}) / {g['hit1']:.2f} / {g['mrr']:.3f} ({g['mrr'] - bg['mrr']:+.3f}) "
                     f"/ {g['tokens']:.0f} ({(g['tokens'] / bg['tokens'] - 1) * 100:+.0f}%)")
    sn, bs = r['sn'], base['sn']
    print(f"| {t} | {cells[0]} | {cells[1]} | {sn['r@5']:.2f} / {sn['r@10']:.2f} ({sn['r@10'] - bs['r@10']:+.2f}) "
          f"/ {sn['mrr']:.2f} ({sn['mrr'] - bs['mrr']:+.2f}) | {r['ko']['r@5']:.2f} / {r['ko']['r@10']:.2f} ({r['ko']['r@10'] - base['ko']['r@10']:+.2f}) "
          f"/ {r['ko']['mrr']:.2f} ({r['ko']['mrr'] - base['ko']['mrr']:+.2f}) |")
detail = {}
for t in tags[1:]:
    detail[t] = {m: transitions(t, slug) for m, slug in MODELS.items()}
    for m in MODELS:
        d = detail[t][m]
        print(f"\n{t} {m}: gained {len(d['gained'])}, lost {len(d['lost'])}")
        for x in d['gained']:
            print('  +', x)
        for x in d['lost']:
            print('  -', x)
(W / 'neighbor-rank-results.json').write_text(json.dumps({'rows': rows, 'transitions': detail}, indent=1))
