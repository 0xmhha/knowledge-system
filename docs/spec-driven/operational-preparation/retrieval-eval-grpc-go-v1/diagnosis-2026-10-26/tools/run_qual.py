"""Rerun only `cks eval` with bin-<tag>/cks on the existing indexes (qual: skip receiver-qualified hit symbols;
rrf-v1: keyword lists left out of RRF; rrf-v2: keyword list weights halved; trace: HEAD plus a per-stage citation dump to stage-trace.jsonl).

Same config, scenarios, flags and indexes as scripts/retrieval_eval_run.py; only the cks binary differs.
Reports go to eval-qual/eval/<slug>/<snap>/ next to a symlink to the original baselines.json, so
scripts/retrieval_eval_summary.py can summarize eval-qual the same way.

usage: run_qual.py --model qwen3-embedding:4b [--tag rrf-v1]
"""
import argparse
import os
import json
import subprocess
import sys
from pathlib import Path

K = Path('/Users/kevin/work/github/0xmhha/auto-coding/knowledge/knowledge-system')
sys.path.insert(0, str(K / 'scripts'))
from retrieval_baselines import index_for  # noqa: E402
from retrieval_eval_run import checkout, cks_yaml  # noqa: E402
from retrieval_snapshot_chain import model_slug  # noqa: E402

W = Path(__file__).resolve().parent
INPUTS = K / 'docs/spec-driven/operational-preparation/retrieval-eval-grpc-go-v1/inputs'

a = argparse.ArgumentParser()
a.add_argument('--model', required=True)
a.add_argument('--tag', default='qual', help='bin-<tag>/ in, eval-<tag>/ out')
a.add_argument('--bin', help='bin-<bin>/ instead of bin-<tag>/')
a.add_argument('--env', action='append', default=[], help='KEY=VALUE passed to cks eval')
args = a.parse_args()
BIN = W / f'bin-{args.bin or args.tag}'
EXTRA = dict(e.split('=', 1) for e in args.env)
slug = model_slug(args.model, 0)
src = W / 'src-eval'
for snap in json.loads((W / 'snapshots.json').read_text()):
    out = W / f'eval-{args.tag}' / 'eval' / slug / snap
    if (out / 'cks.json').exists():
        continue
    ix = index_for(W, snap, args.model, 0)
    checkout(W / 'grpc-go', src, snap)
    out.mkdir(parents=True, exist_ok=True)
    base = out / 'baselines.json'
    if not base.exists():
        base.symlink_to(W / 'eval' / slug / snap / 'baselines.json')
    config = out / 'cks.yaml'
    config.write_text(cks_yaml(ix.graph, src, ix.vector, BIN, args.model, 0, out / 'telemetry'))
    with (out / 'run.log').open('a') as log:
        r = subprocess.run([str(BIN / 'cks'), 'eval', '--config', str(config), '--scenarios', str(INPUTS / 'scenarios' / snap),
                            '--verify-anchors', str(src), '--record-responses', '--output', str(out / 'cks.json')],
                           stdout=log, stderr=subprocess.STDOUT, check=False, timeout=3 * 3600,
                           env={**os.environ, 'CKS_STAGE_TRACE': str(out / 'stage-trace.jsonl'),
                                'CKS_DEBUG_TRACE': str(out / 'debug-trace.jsonl'), **EXTRA})
    (out / 'cks-exit.txt').write_text(f'{r.returncode}\n')
    (out / 'run-env.json').write_text(json.dumps({'bin': str(BIN), 'env': EXTRA}))
    if not (out / 'cks.json').exists():
        sys.exit(f'no report for {snap} (exit {r.returncode})')
    print(snap, 'exit', r.returncode, flush=True)
print('all done', flush=True)
