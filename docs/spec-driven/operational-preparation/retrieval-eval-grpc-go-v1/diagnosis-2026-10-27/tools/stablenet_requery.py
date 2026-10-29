"""Requery CKS on already built indexes with another cks binary or log level.

Stage 4 (debug footprints of the PRs whose old and new CKS ranks differ) and
stage 5 (current code with only faab70ba reverted, bin-revert/cks, on the
current indexes). Indexes are not rebuilt.

usage: requery.py --bin <bin-dir name> --index-arm new|old --tag <name> [--level debug] [--only pr70,...]
"""
import argparse
import json
import subprocess
import time
from pathlib import Path

import run
import run_chain


def main():
    a = argparse.ArgumentParser()
    a.add_argument('--bin', required=True)
    a.add_argument('--index-arm', required=True)
    a.add_argument('--tag', required=True)
    a.add_argument('--level', default='error')
    a.add_argument('--only', default='')
    a.add_argument('--prompts', default='', help='JSON {pr: question} replacing the PR title and body (prompts-ko.json)')
    args = a.parse_args()
    ids = args.only.split(',') if args.only else run_chain.CHAIN
    bindir = run.E / args.bin
    for pid in ids:
        pr = json.loads((run.E / 'prs' / f'{pid}.json').read_text())
        sha = pr['fixture_base_sha']
        idx = 'head-0bf2f4d1' if sha == run_chain.HEAD else pid
        d = run.E / 'idx' / run.SLUG / args.index_arm / idx
        outdir = run.E / 'requery' / args.tag
        outdir.mkdir(parents=True, exist_ok=True)
        cfg = outdir / f'{pid}.yaml'
        cfg.write_text(run.cks_config(args.index_arm, idx).read_text()
                       .replace(f'{run.E}/bin-{args.index_arm}/', f'{bindir}/')
                       .replace('level: error', f'level: {args.level}')
                       .replace(f'footprint_dir: {d}/tel', f'footprint_dir: {outdir}/tel-{pid}'))
        subprocess.run(['git', '-C', str(run.SRC), 'checkout', '-q', '--detach', sha], check=True)
        text = json.loads(Path(args.prompts).read_text())[pid] if args.prompts else run.query_text(pr)
        p = subprocess.Popen([str(bindir / 'cks'), 'mcp', '--config', str(cfg)], stdin=subprocess.PIPE,
                             stdout=subprocess.PIPE, stderr=open(cfg.with_suffix('.stderr'), 'w'), text=True)
        msgs = [{'jsonrpc': '2.0', 'id': 1, 'method': 'initialize',
                 'params': {'protocolVersion': '2025-03-26', 'capabilities': {},
                            'clientInfo': {'name': 'pr-eval', 'version': '0'}}},
                {'jsonrpc': '2.0', 'method': 'notifications/initialized'},
                {'jsonrpc': '2.0', 'id': 2, 'method': 'tools/call',
                 'params': {'name': 'cks.context.get_for_task', 'arguments': {'prompt': text}}}]
        t = time.monotonic()
        for m in msgs:
            p.stdin.write(json.dumps(m) + '\n')
            p.stdin.flush()
        resp = None
        while resp is None:
            line = p.stdout.readline()
            if not line:
                raise RuntimeError('cks closed stdout')
            m = json.loads(line)
            if m.get('id') == 2:
                resp = m
        ms = round((time.monotonic() - t) * 1000)
        p.stdin.close()
        p.wait(timeout=60)
        (outdir / f'{pid}.json').write_text(json.dumps(
            {'id': pid, 'bin': args.bin, 'index_arm': args.index_arm, 'index': idx, 'base_sha': sha, 'query': text,
             'cks': {'latency_ms': ms, 'response': resp}}, ensure_ascii=False, indent=1))
        print(args.tag, pid, 'done', flush=True)


if __name__ == '__main__':
    main()
