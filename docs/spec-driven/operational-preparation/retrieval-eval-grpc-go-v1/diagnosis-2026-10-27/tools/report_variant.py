"""Summarize one eval-<tag> run: any-commit metrics, per-scenario pack diff against a base, debug-trace report.

  summary-cmp.json          scripts/retrieval_eval_summary.py over eval-<tag>
  pack-diff-<slug>.json     scenarios whose recorded pack citations differ from eval-<base>
  debug-report-<slug>.json  scripts/retrieval_debug_trace_report.py over every snapshot's debug-trace.jsonl

usage: report_variant.py --tag dbg [--base .] [--model qwen3-embedding:4b --model bge-m3]
       (--base . means the original eval/ directory)
"""
import argparse
import json
import subprocess
import sys
from pathlib import Path

K = Path('/Users/kevin/work/github/0xmhha/auto-coding/knowledge/knowledge-system')
sys.path.insert(0, str(K / 'scripts'))
from retrieval_eval_summary import first_hit_any_commit, pack_citations  # noqa: E402
from retrieval_snapshot_chain import model_slug  # noqa: E402
import yaml  # noqa: E402

W = Path(__file__).resolve().parent
SCEN = K / 'docs/spec-driven/operational-preparation/retrieval-eval-grpc-go-v1/inputs/scenarios'

a = argparse.ArgumentParser()
a.add_argument('--tag', required=True)
a.add_argument('--base', default='.')
a.add_argument('--model', action='append')
args = a.parse_args()
models = args.model or ['qwen3-embedding:4b', 'bge-m3']
out = W / f'eval-{args.tag}'
base = W if args.base == '.' else W / f'eval-{args.base}'
snaps = json.loads((W / 'snapshots.json').read_text())

cmd = [sys.executable, str(K / 'scripts/retrieval_eval_summary.py'), '--work', str(out), '--repo', str(W / 'grpc-go'),
       '--scenarios', str(SCEN), '--snapshots', str(W / 'snapshots.json'), '--out', str(out / 'summary-cmp.json')]
for m in models:
    cmd += ['--model', m]
subprocess.run(cmd, check=True, cwd=K / 'scripts', stdout=subprocess.DEVNULL)
summary = {s['model']: s['methods']['cks_any_commit'] for s in json.loads((out / 'summary-cmp.json').read_text())}

for m in models:
    slug = model_slug(m, 0)
    diffs, same = [], 0
    for snap in snaps:
        cur = {r['name']: r for r in json.loads((out / 'eval' / slug / snap / 'cks.json').read_text())['results']}
        ref = {r['name']: r for r in json.loads((base / 'eval' / slug / snap / 'cks.json').read_text())['results']}
        for name, r in cur.items():
            sc = yaml.safe_load((SCEN / snap / f'{name}.yaml').read_text())
            exp = sc['expected_citations']
            a_c = [(c['file'], c['start_line'], c['end_line']) for c in pack_citations(ref[name])]
            b_c = [(c['file'], c['start_line'], c['end_line']) for c in pack_citations(r)]
            if a_c == b_c:
                same += 1
                continue
            diffs.append({'name': name, 'snapshot': snap,
                          'base_rank': first_hit_any_commit(exp, pack_citations(ref[name])),
                          'rank': first_hit_any_commit(exp, pack_citations(r))})
    (out / f'pack-diff-{slug}.json').write_text(json.dumps({'identical': same, 'changed': diffs}, indent=1))
    traces = sorted((out / 'eval' / slug).glob('*/debug-trace.jsonl'))
    rep = None
    if traces:
        cmd = [sys.executable, str(K / 'scripts/retrieval_debug_trace_report.py'), '--scenarios', str(SCEN),
               '--out', str(out / f'debug-report-{slug}.json')]
        for t in traces:
            cmd += ['--trace', str(t)]
        subprocess.run(cmd, check=True, cwd=K / 'scripts', stdout=subprocess.DEVNULL)
        rep = json.loads((out / f'debug-report-{slug}.json').read_text())['summary']
    s = summary[m]
    better = sum(1 for d in diffs if d['rank'] and (not d['base_rank'] or d['rank'] < d['base_rank']))
    worse = sum(1 for d in diffs if d['base_rank'] and (not d['rank'] or d['rank'] > d['base_rank']))
    print(f"{m}: any-commit hit@1={s['hit_at_1']:.2f} hit@3={s['hit_at_3']:.2f} mrr={s['mrr']:.3f} | "
          f"pack identical={same} changed={len(diffs)} (better {better}, worse {worse}, "
          f"rank1 {sum(d['base_rank'] != 1 and d['rank'] == 1 for d in diffs)}+/{sum(d['base_rank'] == 1 and d['rank'] != 1 for d in diffs)}-)")
    if rep:
        print('  flags', rep['flags'])
        print('  lost_rank1_at', rep['lost_rank1_at'])
        print('  checkpoints', {k: (v['hit@1'], v['found'], v['mrr']) for k, v in rep['checkpoints'].items()})
        print('  answer_stage4_decision', rep['answer_stage4_decision'])
