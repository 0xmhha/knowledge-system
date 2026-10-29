"""Explain, per scenario, whether CKS handed the LLM an answer span and, if not, where and why it was lost.

The question that matters for the product is whether the final pack holds code that leads to the fix.
This script joins a CKS debug trace (internal/system/debugtrace) to the scenarios and adds two probes
the trace cannot answer on its own:

  index         does the vector index hold a chunk overlapping each answer span at all
  vector_rank   where the best answer chunk ranks in a deep, pure vector query of the same prompt
                (ckv query, no BM25 rerank, no threshold); 0 means not within --deep-k

Each scenario gets one loss class:
  found            an answer span is in the Stage 4 pack
  cut_stage4       an answer span reached Stage 2/3 but Stage 4 did not select it
  cut_stage2       an answer span was fused in Stage 2 but fell off the Stage 2 cap or demotion
  never_retrieved  no candidate list (ckv rounds, Stage 1, Stage 2 fusion) held an answer span

never_retrieved is split further by the probes:
  not_indexed      no chunk in the index overlaps any answer span
  vector_deep      an answer chunk ranks beyond the Stage 1 depth (--stage1-k) in pure vector order
  dropped_stage1   an answer chunk is within the Stage 1 depth in pure vector order, yet not kept
  vector_absent    indexed, but not within --deep-k

usage: retrieval_found_diagnosis.py --trace <jsonl> [--trace ...] --scenarios <dir> [--scenarios ...]
       --model <ollama model> --work <datasets dir> --bin <dir with ckv> --repo <clone> --src <worktree>
       [--deep-k 1000] [--stage1-k 20] [--out <json>] [--no-probe]
"""
from __future__ import annotations

import argparse
import json
import sqlite3
import subprocess
from collections import Counter
from pathlib import Path

import yaml

from retrieval_debug_trace_report import build_report

LOSSES = ('found', 'cut_stage4', 'cut_stage2', 'never_retrieved')
CAUSES = ('not_indexed', 'vector_deep', 'dropped_stage1', 'vector_absent')


def overlaps(a: dict, file: str, start: int, end: int) -> bool:
    return a['file'] == file and a['start_line'] <= end and start <= a['end_line']


def loss_class(ranks: dict) -> str:
    if ranks.get('stage4', 0) > 0:
        return 'found'
    if ranks.get('stage2', 0) > 0 or ranks.get('stage3', 0) > 0:
        return 'cut_stage4'
    if ranks.get('fused', 0) > 0:
        return 'cut_stage2'
    return 'never_retrieved'


def cause(indexed: bool, vector_rank: int, stage1_k: int) -> str:
    if not indexed:
        return 'not_indexed'
    if vector_rank == 0:
        return 'vector_absent'
    return 'dropped_stage1' if vector_rank <= stage1_k else 'vector_deep'


def rank_bucket(r: int) -> str:
    if r == 0:
        return 'absent'
    for edge in (1, 3, 10, 20, 50, 100, 200, 500):
        if r <= edge:
            return f'<={edge}'
    return '>500'


def index_coverage(db: Path, answers: list[dict]) -> list[dict]:
    """Chunks overlapping each answer span (any commit; a reindexed index keeps unchanged files' chunks)."""
    out = []
    with sqlite3.connect(f'file:{db}?mode=ro', uri=True) as con:
        for a in answers:
            rows = con.execute('SELECT chunk_kind, is_test, symbol_name FROM chunks WHERE file = ? AND start_line <= ? '
                               'AND end_line >= ?', (a['file'], a['end_line'], a['start_line'])).fetchall()
            out.append({'file': a['file'], 'start_line': a['start_line'], 'end_line': a['end_line'],
                        'chunks': len(rows), 'kinds': sorted({r[0] for r in rows}),
                        'symbols': sorted({r[2] for r in rows if r[2]})[:5], 'is_test': any(r[1] for r in rows)})
    return out


def vector_probe(ckv: Path, vector_dir: Path, model: str, src: Path, prompt: str, k: int) -> list[dict]:
    args = [str(ckv), 'query', '--out', str(vector_dir), '--embedder=ollama', f'--model-name={model}', '--src', str(src),
            '-k', str(k), '--threshold', '-1', '--examples', '0', '--json', '--no-footprint', prompt]
    data = json.loads(subprocess.run(args, capture_output=True, text=True, check=True, timeout=600).stdout)
    return [h['citation'] for h in data.get('hits') or []]


def first_answer_rank(answers: list[dict], cites: list[dict]) -> int:
    for i, c in enumerate(cites, 1):
        if any(overlaps(a, c['file'], c['start_line'], c['end_line']) for a in answers):
            return i
    return 0


def trace_record(path: str, seq: int) -> dict:
    with open(path) as f:
        for line in f:
            rec = json.loads(line)
            if rec['seq'] == seq:
                return rec
    raise KeyError(f'{path}: seq {seq} not found')


def pack_shape(record: dict) -> dict:
    """What the Stage 4 pack spent its slots on, and the Stage 1 files the answer competed with."""
    dec = next((e['data'].get('decisions') or [] for e in reversed(record['events'])
                if e['stage'] == 'stage4' and e['step'] == 'allocate' and e['phase'] == 'after'), [])
    sel = [d for d in dec if d['decision'].startswith('selected')]
    hits = next((e['data'].get('hits') or [] for e in record['events']
                 if e['stage'] == 'stage1' and e['step'] == 'ckv_search' and e['phase'] == 'after' and e['key'] == '1'), [])
    return {'selected_by_origin': dict(Counter(d.get('origin', '') for d in sel)), 'selected': len(sel),
            'stage1_top3': [f"{h['file']}:{h['start']}" for h in hits[:3]],
            'stage1_files': sorted({h['file'] for h in hits})}


def snapshot_of(record_path: str) -> str:
    return Path(record_path).parent.name


def diagnose(row: dict, scenario: dict, record: dict, probe: dict | None, stage1_k: int) -> dict:
    answers = scenario['expected_citations']
    shape = pack_shape(record)
    out = {'name': row['name'], 'snapshot': snapshot_of(row['trace']), 'prompt': scenario['prompt'],
           'expected_intent': scenario.get('expected_intent'), 'intent': row.get('intent'),
           'loss': loss_class(row['ranks']), 'ranks': row['ranks'], 'answer_stage4_decision': row.get('answer_stage4_decision'),
           'answer_files_in_stage1': sorted({a['file'] for a in answers} & set(shape['stage1_files'])),
           'answer_tests_only': all(a['file'].endswith('_test.go') for a in answers), **shape}
    out.pop('stage1_files')
    if probe is not None:
        out['index'] = probe['index']
        out['indexed'] = any(c['chunks'] for c in probe['index'])
        out['vector_rank'] = probe['vector_rank']
        out['vector_rank_bucket'] = rank_bucket(probe['vector_rank'])
        if out['loss'] == 'never_retrieved':
            out['cause'] = cause(out['indexed'], probe['vector_rank'], stage1_k)
    return out


def summarize(rows: list[dict]) -> dict:
    lost = [r for r in rows if r['loss'] == 'never_retrieved']
    return {
        'scenarios': len(rows),
        'loss': {k: sum(r['loss'] == k for r in rows) for k in LOSSES},
        'never_retrieved_cause': {k: sum(r.get('cause') == k for r in lost) for k in CAUSES},
        'never_retrieved_answer_file_in_stage1': sum(bool(r['answer_files_in_stage1']) for r in lost),
        'never_retrieved_tests_only': sum(r['answer_tests_only'] for r in lost),
        'vector_rank_bucket': {loss: dict(Counter(r.get('vector_rank_bucket', 'unprobed') for r in rows if r['loss'] == loss))
                               for loss in LOSSES},
        'cut_stage4_answer_decision': dict(Counter(r['answer_stage4_decision'] for r in rows if r['loss'] == 'cut_stage4')),
    }


def main() -> None:
    a = argparse.ArgumentParser(description=__doc__.split('\n', 1)[0])
    a.add_argument('--trace', action='append', required=True, type=Path)
    a.add_argument('--scenarios', action='append', required=True, type=Path)
    a.add_argument('--model', required=True)
    a.add_argument('--work', type=Path, help='datasets dir holding vec/<model slug>/<snapshot>')
    a.add_argument('--bin', type=Path, help='directory holding the ckv binary')
    a.add_argument('--repo', type=Path, help='grpc-go clone (source of the --src worktree)')
    a.add_argument('--src', type=Path, help='worktree checked out at each snapshot before probing')
    a.add_argument('--deep-k', type=int, default=1000)
    a.add_argument('--stage1-k', type=int, default=20)
    a.add_argument('--no-probe', action='store_true', help='trace only; skip the index and vector probes')
    a.add_argument('--out', type=Path)
    args = a.parse_args()

    report = build_report(args.trace, args.scenarios)
    by_path = {str(p): yaml.safe_load(p.read_text()) for d in args.scenarios for p in d.rglob('*.yaml')}
    rows = []
    if not args.no_probe:
        from retrieval_eval_run import checkout
        from retrieval_snapshot_chain import model_slug
        slug = model_slug(args.model, 0)
    current = None
    for row in sorted(report['scenarios'], key=lambda r: snapshot_of(r['trace'])):
        sc = by_path[row['scenario']]
        rec = trace_record(row['trace'], row['seq'])
        probe = None
        if not args.no_probe:
            snap = snapshot_of(row['trace'])
            if snap != current:
                checkout(args.repo, args.src, snap)
                current = snap
            vdir = args.work / 'vec' / slug / snap
            cites = vector_probe(args.bin / 'ckv', vdir, args.model, args.src, sc['prompt'], args.deep_k)
            probe = {'index': index_coverage(vdir / 'vector.db', sc['expected_citations']),
                     'vector_rank': first_answer_rank(sc['expected_citations'], cites)}
        rows.append(diagnose(row, sc, rec, probe, args.stage1_k))
        print(f"{row['name']} {rows[-1]['loss']} {rows[-1].get('cause', '')} vector_rank={rows[-1].get('vector_rank')}",
              flush=True)
    out = {'model': args.model, 'deep_k': args.deep_k, 'stage1_k': args.stage1_k, 'summary': summarize(rows),
           'scenarios': rows}
    if args.out:
        args.out.write_text(json.dumps(out, ensure_ascii=False, indent=1))
    print(json.dumps(out['summary'], ensure_ascii=False, indent=1))


if __name__ == '__main__':
    main()
