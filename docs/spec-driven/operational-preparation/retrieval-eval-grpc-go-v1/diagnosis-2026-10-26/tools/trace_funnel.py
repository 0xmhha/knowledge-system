"""Follow each scenario's first answer through the CKS stages recorded by bin-trace (stage-trace.jsonl).

Ranks use the cks eval overlap rule without the commit filter (first_hit_any_commit):
  vector     ckv query -k 10 baseline (baselines.json)
  vector_bm  ckv query -k 10 --bm25-rerank baseline
  s1         Stage 1 ckv hit list, the list fused into RRF (rounds merged, knowledge pass appended)
  s2         Stage 2 RRF citations
  s3         Stage 2 seeds followed by Stage 3 neighbors
  s4         Stage 4 budget selection
  pack       final pack (cks.json recorded response)
Also counts intent classification against expected_intent.

usage: trace_funnel.py --model qwen3-embedding:4b [--out funnel-qwen3.json]
"""
import argparse
import json
import sys
from collections import Counter
from pathlib import Path

import yaml

K = Path('/Users/kevin/work/github/0xmhha/auto-coding/knowledge/knowledge-system')
sys.path.insert(0, str(K / 'scripts'))
from retrieval_eval_summary import first_hit_any_commit, pack_citations  # noqa: E402
from retrieval_snapshot_chain import model_slug  # noqa: E402

W = Path(__file__).resolve().parent
SCEN = K / 'docs/spec-driven/operational-preparation/retrieval-eval-grpc-go-v1/inputs/scenarios'
STAGES = ['vector', 'vector_bm', 's1', 's2', 's3', 's4', 'pack']


def cites(rows):
    return [{'file': r['file'], 'start_line': r['start'], 'end_line': r['end']} for r in rows or []]


a = argparse.ArgumentParser()
a.add_argument('--model', required=True)
a.add_argument('--out')
args = a.parse_args()
slug = model_slug(args.model, 0)
rows = []
for snap in json.loads((W / 'snapshots.json').read_text()):
    d = W / 'eval-trace' / 'eval' / slug / snap
    traces = {}
    for line in (d / 'stage-trace.jsonl').read_text().splitlines():
        t = json.loads(line)
        traces.setdefault(t['prompt'], t)  # first compose call per prompt
    base = {b['name']: b for b in json.loads((W / 'eval' / slug / snap / 'baselines.json').read_text())['results']}
    for res in json.loads((d / 'cks.json').read_text())['results']:
        sc = yaml.safe_load((SCEN / snap / f"{res['name']}.yaml").read_text())
        t = traces.get(sc['prompt'])
        if t is None:
            sys.exit(f"no trace for {res['name']}")
        exp = sc['expected_citations']
        r = {'name': res['name'], 'snapshot': snap, 'expected_intent': sc['expected_intent'], 'intent': t['intent'],
             'keywords': t['keywords'], 'rounds': t['rounds'],
             'vector': base[res['name']]['ckv_vector']['first_hit_rank'],
             'vector_bm': base[res['name']]['ckv_vector_bm25']['first_hit_rank'],
             's1': first_hit_any_commit(exp, cites(t['stage1_hits'])),
             's2': first_hit_any_commit(exp, cites(t['stage2_citations'])),
             's3': first_hit_any_commit(exp, cites(t['stage3'])),
             's4': first_hit_any_commit(exp, cites(t['stage4_selected'])),
             'pack': first_hit_any_commit(exp, pack_citations(res)),
             'answer_skipped_by_budget': first_hit_any_commit(exp, t.get('stage4_skipped') or []) > 0,
             'stage1_len': len(t['stage1_hits'] or []), 'stage2_len': len(t['stage2_citations'] or []),
             'stage4_len': len(t['stage4_selected'] or [])}
        rows.append(r)
n = len(rows)
print(f'{args.model}: n={n}')
for s in STAGES:
    rk = [r[s] for r in rows]
    print(f"  {s:9} hit@1={sum(x == 1 for x in rk) / n:.2f} hit@3={sum(1 <= x <= 3 for x in rk) / n:.2f} "
          f"found={sum(x > 0 for x in rk) / n:.2f} mrr={sum(1 / x for x in rk if x) / n:.3f}")
top = [r for r in rows if r['vector'] == 1]
print(f'  vector rank 1: {len(top)} scenarios; where the rank-1 answer is first lost:')
lost = Counter()
for r in top:
    prev = 'vector'
    for s in STAGES[2:]:
        if r[s] != 1:
            lost[f'{prev}->{s}' + (' (dropped)' if r[s] == 0 else '')] += 1
            break
        prev = s
    else:
        lost['kept'] += 1
for k, v in lost.most_common():
    print(f'    {k}: {v}')
print('  intent:', dict(Counter(r['intent'] for r in rows)), 'correct', sum(r['intent'] == r['expected_intent'] for r in rows))
print('  budget skipped the answer:', sum(r['answer_skipped_by_budget'] for r in rows))
if args.out:
    Path(args.out).write_text(json.dumps(rows, ensure_ascii=False, indent=1))
