"""Tabulate ABLATION-PLAN.md variants: grpc-go any-commit metrics and go-stablenet file metrics, deltas vs abl-base.

Run report_variant.py --tag <variant> --base abl-base for each grpc-go variant first.
"""
import json
import subprocess
import sys
from pathlib import Path

W = Path(__file__).resolve().parent
E = Path('/Users/kevin/work/github/0xmhha/auto-coding/knowledge-data/go-stablenet-pr-eval-20261025')
VARIANTS = ['abl-base', 'abl-s1norr', 'abl-nohitkw', 'abl-nokw', 'abl-nr0', 'abl-mc20', 'abl-it0', 'abl-notd']
MODELS = {'qwen3-embedding:4b': 'qwen3-embedding-4b', 'bge-m3': 'bge-m3'}


def grpc(tag):
    s = {x['model']: x for x in json.loads((W / f'eval-{tag}' / 'summary-cmp.json').read_text())}
    out = {}
    for m, slug in MODELS.items():
        a = s[m]['methods']['cks_any_commit']
        rep = json.loads((W / f'eval-{tag}' / f'debug-report-{slug}.json').read_text())['summary']
        diff = json.loads((W / f'eval-{tag}' / f'pack-diff-{slug}.json').read_text())
        out[m] = {'hit1': a['hit_at_1'], 'hit3': a['hit_at_3'], 'mrr': a['mrr'],
                  'intent_acc': s[m]['methods']['cks'].get('intent_accuracy'),
                  'found_s4': rep['checkpoints']['stage4']['found'],
                  'changed': len(diff['changed']),
                  'better': sum(1 for d in diff['changed'] if d['rank'] and (not d['base_rank'] or d['rank'] < d['base_rank'])),
                  'worse': sum(1 for d in diff['changed'] if d['base_rank'] and (not d['rank'] or d['rank'] > d['base_rank']))}
    return out


def stablenet(tag):
    line = subprocess.run([sys.executable, 'score_requery.py', tag], cwd=E / 'scripts', capture_output=True, text=True,
                          check=True).stdout.splitlines()[0]
    kv = dict(p.split('=') for p in line.split()[1:] if '=' in p)
    return {k: float(kv[k]) for k in ('r@5', 'r@10', 'mrr')}


rows = {t: {'grpc': grpc(t), 'sn': stablenet(t)} for t in VARIANTS}
base = rows['abl-base']
print('| variant | qwen3 hit@1 / MRR (dMRR) | bge-m3 hit@1 / MRR (dMRR) | qwen3 better/worse | bge better/worse | stablenet r@10 / MRR (dMRR) |')
print('|---|---|---|---|---|---|')
for t, r in rows.items():
    q, b, sn = r['grpc']['qwen3-embedding:4b'], r['grpc']['bge-m3'], r['sn']
    bq, bb, bs = base['grpc']['qwen3-embedding:4b'], base['grpc']['bge-m3'], base['sn']
    print(f"| {t} | {q['hit1']:.2f} / {q['mrr']:.3f} ({q['mrr'] - bq['mrr']:+.3f}) | {b['hit1']:.2f} / {b['mrr']:.3f} ({b['mrr'] - bb['mrr']:+.3f}) "
          f"| {q['better']}/{q['worse']} | {b['better']}/{b['worse']} | {sn['r@10']:.2f} / {sn['mrr']:.2f} ({sn['mrr'] - bs['mrr']:+.2f}) |")
print()
for t, r in rows.items():
    q, b = r['grpc']['qwen3-embedding:4b'], r['grpc']['bge-m3']
    print(f"{t}: intent_acc q={q['intent_acc']} b={b['intent_acc']} found@stage4 q={q['found_s4']} b={b['found_s4']}")
(W / 'ablation-results.json').write_text(json.dumps(rows, indent=1))
