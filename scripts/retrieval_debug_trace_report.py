"""Find where each scenario's answer was lost, and flag known failure shapes, from a CKS debug trace.

cks writes one JSON line per compose call to $CKS_DEBUG_TRACE (internal/system/debugtrace).
Each line holds every retrieval algorithm's input and output. This script joins those
lines to the scenarios by prompt and reports, per scenario:

  ranks     the first answer rank at each checkpoint, by the cks eval overlap rule
            without the commit filter (first_hit_any_commit):
              ckv_round<N>    Stage 1 ckv search of that round
              stage1          Stage 1 final hit list (rounds merged, knowledge pass)
              fused           Stage 2 RRF scores before demotion and cap
              stage2          Stage 2 citations after demotion and cap
              stage3          Stage 2 citations followed by Stage 3 neighbors
              stage4          Stage 4 selected bodies, in selection order
  lost_rank1_at   the first checkpoint where a rank-1 answer stopped being rank 1
  lost_found_at   the first checkpoint where a found answer disappeared
  flags     failure shapes seen in this call (see FLAGS)

The summary counts each flag and each loss point, so a change to an algorithm can be
checked by rerunning the evaluation and diffing two reports.

usage: retrieval_debug_trace_report.py --trace <jsonl> [--trace <jsonl> ...]
       --scenarios <dir> [--scenarios <dir> ...] [--out <json>] [--all-calls]
"""
from __future__ import annotations

import argparse
import json
import re
from collections import Counter
from pathlib import Path

import yaml

from retrieval_eval_summary import first_hit_any_commit

CHECKPOINTS = ('stage1', 'fused', 'stage2', 'stage3', 'stage4')

FLAGS = {
    'intent_wrong': 'classified intent differs from expected_intent',
    'intent_threshold': 'nearest intent was the expected one, but it fell below the threshold',
    'list_duplicate_credit': 'one ranked list credited the same citation more than once',
    'duplicate_credit_top1': 'the Stage 2 top citation got its score from repeated credits of one list',
    'symbol_fanout': 'a dotted symbol lost its leading segments and resolved to several nodes',
    'keyword_only_top1': 'the Stage 2 top citation has no ckv evidence',
    'hit_keyword_promoted_top1': 'a wrong Stage 2 top citation won on keywords mined from ckv hits',
    'score_outlier_top1': 'the Stage 2 top score is more than twice the second',
    'answer_demoted': 'demotion moved the answer down between the fused and final Stage 2 lists',
    'answer_cut_stage4': 'the answer reached Stage 4 but was not selected',
}

SOURCE_RE = re.compile(r'^(?P<list>[a-z_]+:.*?)@rank=(?P<rank>\d+)\(\+(?P<c>[0-9.]+)\)$')
HIT_ORIGINS = {'hit_symbol', 'hit_file'}


def cites(rows: list[dict] | None) -> list[dict]:
    return [{'file': r['file'], 'start_line': r['start'], 'end_line': r['end']} for r in rows or []]


def rank(expected: list[dict], rows: list[dict] | None) -> int:
    return first_hit_any_commit(expected, cites(rows))


def load_scenarios(dirs: list[Path]) -> dict[str, dict]:
    by_prompt: dict[str, dict] = {}
    for d in dirs:
        for p in sorted(d.rglob('*.yaml')):
            sc = yaml.safe_load(p.read_text())
            if not isinstance(sc, dict) or 'prompt' not in sc or 'expected_citations' not in sc:
                continue
            sc = {**sc, '_path': str(p)}
            by_prompt.setdefault(sc['prompt'], sc)
    return by_prompt


def events(record: dict, stage: str, step: str, phase: str) -> list[dict]:
    return [e for e in record['events'] if e['stage'] == stage and e['step'] == step and e['phase'] == phase]


def last(record: dict, stage: str, step: str, phase: str) -> dict:
    ev = events(record, stage, step, phase)
    return ev[-1]['data'] if ev else {}


def list_credits(sources: list[str]) -> Counter:
    """How many times each ranked list credited one citation."""
    n = Counter()
    for s in sources or []:
        m = SOURCE_RE.match(s)
        if m:
            n[m.group('list')] += 1
    return n


def keyword_of(list_label: str) -> str:
    kind, _, kw = list_label.partition(':')
    return kw.split('@', 1)[0] if kind != 'ckv' else ''


def analyze(record: dict, sc: dict) -> dict:
    exp = sc['expected_citations']
    row: dict = {'name': Path(sc['_path']).stem, 'scenario': sc['_path'], 'seq': record['seq'],
                 'expected_intent': sc.get('expected_intent'), 'flags': [], 'ranks': {}}
    flags = row['flags']

    # Intent.
    cls = last(record, 'intent', 'classify', 'after')
    override = last(record, 'intent', 'caller_override', 'note')
    row['intent'] = override.get('intent') if override else cls.get('intent', '')
    if cls:
        row['intent_confidence'] = cls.get('confidence')
        row['nearest_intent'] = cls.get('nearest_intent')
        order = [s['intent'] for s in cls.get('scores') or []]
        exp_intent = sc.get('expected_intent')
        row['expected_intent_rank'] = order.index(exp_intent) + 1 if exp_intent in order else 0
    if sc.get('expected_intent') and row['intent'] != sc['expected_intent']:
        flags.append('intent_wrong')
        if cls.get('below_threshold') and cls.get('nearest_intent') == sc['expected_intent']:
            flags.append('intent_threshold')

    # Ranks at each checkpoint.
    for e in events(record, 'stage1', 'ckv_search', 'after'):
        row['ranks'][f"ckv_round{e['key']}"] = rank(exp, e['data'].get('hits'))
    row['ranks']['stage1'] = rank(exp, last(record, 'stage1', 'keyword_cap', 'after').get('hits'))
    fused = last(record, 'stage2', 'rank_citations', 'before').get('citations') or []
    final = last(record, 'stage2', 'rank_citations', 'after').get('citations') or []
    neighbors = last(record, 'stage3', 'expand', 'after').get('neighbors') or []
    decisions = last(record, 'stage4', 'allocate', 'after').get('decisions') or []
    selected = [d for d in decisions if d['decision'].startswith('selected')]
    row['ranks']['fused'] = rank(exp, fused)
    row['ranks']['stage2'] = rank(exp, final)
    row['ranks']['stage3'] = rank(exp, final + neighbors)
    row['ranks']['stage4'] = rank(exp, selected)

    order = [k for k in row['ranks'] if k.startswith('ckv_round')][:1] + list(CHECKPOINTS)
    row['lost_rank1_at'] = row['lost_found_at'] = None
    for prev, cur in zip(order, order[1:]):
        if row['lost_rank1_at'] is None and row['ranks'][prev] == 1 and row['ranks'][cur] != 1:
            row['lost_rank1_at'] = f'{prev}->{cur}'
        if row['lost_found_at'] is None and row['ranks'][prev] > 0 and row['ranks'][cur] == 0:
            row['lost_found_at'] = f'{prev}->{cur}'

    # Stage 2 fusion shapes.
    origins = {}
    for e in events(record, 'stage1', 'extract_keywords', 'after'):
        for c in e['data'].get('candidates') or []:
            origins.setdefault(c['keyword'], c['origin'])
    max_dup = 0
    for c in fused:
        dup = max(list_credits(c.get('sources')).values(), default=0)
        max_dup = max(max_dup, dup)
    if max_dup > 1:
        flags.append('list_duplicate_credit')
    row['max_list_credits'] = max_dup
    if final:
        top = final[0]
        credits = list_credits(top.get('sources'))
        top_is_answer = rank(exp, [top]) == 1
        row['top1'] = {'file': top['file'], 'start': top['start'], 'score': top['score'], 'answer': top_is_answer}
        if max(credits.values(), default=0) > 1:
            flags.append('duplicate_credit_top1')
        if not any(lbl.startswith('ckv:') for lbl in credits):
            flags.append('keyword_only_top1')
        kws = {keyword_of(lbl) for lbl in credits} - {''}
        if not top_is_answer and kws and all(origins.get(k) in HIT_ORIGINS for k in kws):
            flags.append('hit_keyword_promoted_top1')
        if len(final) > 1 and final[1]['score'] > 0 and top['score'] > 2 * final[1]['score']:
            flags.append('score_outlier_top1')
    if 0 < row['ranks']['fused'] < row['ranks']['stage2'] or (row['ranks']['fused'] and not row['ranks']['stage2']):
        flags.append('answer_demoted')

    # Symbol resolution.
    fanout = []
    for e in events(record, 'ckgclient', 'resolve_symbol', 'after'):
        att = e['data'].get('attempts') or []
        if '.' in e['key'] and att and att[-1]['via'] == 'symbol_drop_leading' and att[-1]['nodes'] > 1:
            fanout.append({'name': e['key'], 'resolved_as': att[-1]['name'], 'nodes': att[-1]['nodes']})
    if fanout:
        flags.append('symbol_fanout')
    row['symbol_fanout'] = fanout

    # Stage 4 outcome of the answer.
    for d in decisions:
        if rank(exp, [d]) == 1:
            row['answer_stage4_decision'] = d['decision']
            if not d['decision'].startswith('selected'):
                flags.append('answer_cut_stage4')
            break
    return row


def metrics(rows: list[dict], key: str) -> dict:
    rk = [r['ranks'].get(key, 0) for r in rows]
    n = len(rk) or 1
    return {'hit@1': round(sum(x == 1 for x in rk) / n, 3), 'hit@3': round(sum(1 <= x <= 3 for x in rk) / n, 3),
            'found': round(sum(x > 0 for x in rk) / n, 3), 'mrr': round(sum(1 / x for x in rk if x) / n, 3)}


def summarize(rows: list[dict]) -> dict:
    keys = sorted({k for r in rows for k in r['ranks'] if k.startswith('ckv_round')}) + list(CHECKPOINTS)
    return {
        'scenarios': len(rows),
        'checkpoints': {k: metrics(rows, k) for k in keys},
        'lost_rank1_at': dict(Counter(r['lost_rank1_at'] for r in rows if r['lost_rank1_at']).most_common()),
        'lost_found_at': dict(Counter(r['lost_found_at'] for r in rows if r['lost_found_at']).most_common()),
        'flags': {f: sum(f in r['flags'] for r in rows) for f in FLAGS},
        'answer_stage4_decision': dict(Counter(r.get('answer_stage4_decision', 'not_reached') for r in rows)),
        'intent': {'correct': sum('intent_wrong' not in r['flags'] for r in rows),
                   'classified_as': dict(Counter(r['intent'] or 'unknown' for r in rows))},
    }


def build_report(trace_paths: list[Path], scenario_dirs: list[Path], all_calls: bool = False) -> dict:
    scenarios = load_scenarios(scenario_dirs)
    rows, unmatched, seen = [], 0, set()
    for path in trace_paths:
        for line in path.read_text().splitlines():
            record = json.loads(line)
            sc = scenarios.get(record['prompt'])
            if sc is None:
                unmatched += 1
                continue
            if not all_calls and sc['_path'] in seen:
                continue  # first compose call per scenario, like cks eval's recorded response
            seen.add(sc['_path'])
            rows.append({**analyze(record, sc), 'trace': str(path)})
    return {'flag_definitions': FLAGS, 'summary': summarize(rows), 'unmatched_records': unmatched, 'scenarios': rows}


def main() -> None:
    a = argparse.ArgumentParser(description=__doc__.split('\n', 1)[0])
    a.add_argument('--trace', action='append', required=True, type=Path)
    a.add_argument('--scenarios', action='append', required=True, type=Path)
    a.add_argument('--out', type=Path)
    a.add_argument('--all-calls', action='store_true', help='analyze every compose call, not only the first per scenario')
    args = a.parse_args()
    report = build_report(args.trace, args.scenarios, args.all_calls)
    if args.out:
        args.out.write_text(json.dumps(report, ensure_ascii=False, indent=1))
    print(json.dumps(report['summary'], ensure_ascii=False, indent=1))


if __name__ == '__main__':
    main()
