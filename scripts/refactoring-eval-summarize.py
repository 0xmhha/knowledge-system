#!/usr/bin/env python3
"""Score a guarded N11 matrix capture against the frozen question book.

Retrieval metrics, denominators, paired bootstrap and latency come from the
B1 matrix summarizer (scripts/b1-summarize-matrix.py) unchanged. This adapter only maps the frozen
question book to its case format and adds a mechanical citation check for ABS
items. No generated answer exists, so answer-level abstention stays null.
Groups below the protocol's independent-unit floor are inconclusive.
"""
import argparse
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import sys

ROOT = Path(__file__).resolve().parent.parent
B1 = Path(__file__).with_name('b1-summarize-matrix.py')
_spec = importlib.util.spec_from_file_location('b1_summarize_matrix', B1)
M = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(M)


def require(ok, message):
    if not ok:
        raise ValueError(message)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def cases_from_book(book, split, source_commit):
    """Map frozen question-book rows of one split to B1 summarizer cases."""
    cases = {}
    for q in book['questions']:
        if q['split'] != split:
            continue
        require(q['review_state'] == 'approved', 'unapproved question: ' + q['id'])
        require(q['source_commit'] == source_commit, 'question source differs from capture: ' + q['id'])
        abstain = q['expected_behavior'] == 'abstain'
        require(abstain == (q['group'] == 'ABS'), 'ABS/abstain mismatch: ' + q['id'])
        cases[q['id']] = {
            'group': q['group'],
            # Bootstrap units are counted per group (protocol cluster_policy); a
            # reviewed CODE/WHY pair shares one fact cluster across two groups.
            'independent_unit_id': q['group'] + '/' + q['cluster_id'],
            'cluster_id': q['cluster_id'],
            'expected_commit': source_commit,
            # ABS rows have no answer span; the strict flag keeps the frozen per-item rule.
            'expected_citations': [{'file': e['path'], 'start_line': e['first'], 'end_line': e['last']}
                                   for e in q['evidence']],
            'expect_no_citations': abstain and q.get('strict_zero_citations') is True,
            'abstention_basis_sources': q.get('abstention_basis_sources', []) if abstain else None,
        }
    require(cases, 'no questions for split ' + split)
    return cases


def overlaps(span, citation):
    return span['path'] == citation['file'] and span['first'] <= citation['end_line'] and citation['start_line'] <= span['last']


def abstention_rows(rows, cases, arms):
    """Mechanical ABS citation check per arm on retrieval rows.

    off_basis citations are the frozen rule's "citing any source as the answer";
    citations of the recorded null/unassigned basis are allowed. This is not an
    answer abstention verdict.
    """
    result = {}
    for arm in arms:
        selected = [r for r in rows if r['arm'] == arm and r['phase'] == 'retrieval' and cases[r['request_id']]['group'] == 'ABS']
        per = []
        for r in selected:
            response = r['call'].get('response') or {}
            cites = (response.get('structuredContent') or {}).get('citations', [])[:10]
            basis = cases[r['request_id']]['abstention_basis_sources'] or []
            off = sum(not any(overlaps(b, c) for b in basis) for c in cites)
            per.append({'request_id': r['request_id'], 'citations': len(cites), 'basis_citations': len(cites) - off,
                        'off_basis_citations': off, 'error': bool(r.get('error') or response.get('isError'))})
        result[arm] = {'retrieval_rows': len(per), 'error_rows': sum(p['error'] for p in per),
                       'zero_citation_rows': sum(p['citations'] == 0 for p in per),
                       'basis_only_rows': sum(p['citations'] > 0 and p['off_basis_citations'] == 0 for p in per),
                       'rows_with_off_basis_citation': sum(p['off_basis_citations'] > 0 for p in per),
                       'off_basis_citation_rate': (sum(p['off_basis_citations'] > 0 for p in per) / len(per)) if per else None}
    base = result.get('baseline_off', {}).get('off_basis_citation_rate')
    for value in result.values():
        rate = value['off_basis_citation_rate']
        value['off_basis_rate_delta_to_baseline_off'] = None if rate is None or base is None else rate - base
    return result


def verdicts(summary, protocol, cases):
    """Apply frozen thresholds descriptively; small groups stay inconclusive."""
    t = protocol['thresholds']
    units = {}
    for c in cases.values():
        units.setdefault(c['group'], set()).add(c['independent_unit_id'])
    groups = {}
    for group, ids in sorted(units.items()):
        groups[group] = {'independent_units': len(ids),
                         'verdict': 'inconclusive' if len(ids) < 10 else 'requires_full_gate_review'}
    p95 = {arm: v['warm_p95_ratio_to_baseline_off'] for arm, v in summary['arm_summaries'].items()}
    return {'groups': groups,
            'warm_p95_ratio_to_baseline_off': p95,
            'warm_p95_ratio_max': t['warm_p95_ratio_max'],
            'arms_over_p95_ratio': sorted(a for a, r in p95.items() if r is not None and r > t['warm_p95_ratio_max']),
            'overall': 'inconclusive',
            'default_on': False,
            'release_eligible': False,
            'reason': 'every important group has fewer than 10 independent units (frozen small-group rule)'}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--capture-dir', type=Path, required=True)
    parser.add_argument('--questions', type=Path, required=True)
    parser.add_argument('--protocol', type=Path, required=True)
    parser.add_argument('--source-root', type=Path, required=True)
    parser.add_argument('--split', choices=['DEV', 'FINAL'], required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    require(not (args.output.exists() or args.output.is_symlink()), 'output already exists')
    d = args.capture_dir
    raw = {name: (d / name).read_bytes() for name in ('report.json', 'rows.jsonl', 'requests.json')}
    raw_book, raw_protocol = args.questions.read_bytes(), args.protocol.read_bytes()
    report, requests = M.decode(raw['report.json']), M.decode(raw['requests.json'])
    book, protocol = M.decode(raw_book), M.decode(raw_protocol)
    require(sha(raw['rows.jsonl']) == report['rows_sha256'] and sha(raw['requests.json']) == report['request_sha256'],
            'rows/requests differ from capture report')
    commit = report['dataset_identity']['source']['source_commit']
    require(protocol['source_commit'] == commit and book['source_commit'] == commit, 'protocol/book source differs from capture')
    require(protocol['retrieval_k'] == 10 and protocol['uncertainty']['ci'] == .95, 'unsupported K or interval')
    cases = cases_from_book(book, args.split, commit)
    rows = [M.decode(line) for line in raw['rows.jsonl'].splitlines()]
    ledgers, footprints = {}, {}
    for arm in M.ARMS:
        ledgers[arm], footprints[arm] = M.backend_scopes(d, arm)
    configs = M.verify_retained_configs(d, report, protocol['retrieval_k'])
    sources = M.referenced_sources(rows, cases, args.source_root)
    u = protocol['uncertainty']
    summary = M.summarize(report, requests, rows, cases, args.source_root, ledgers, u['bootstrap_seed'], u['resamples'])
    summary['split'] = args.split
    summary['independent_units_scope'] = 'group-scoped units (group/cluster); see distinct_fact_clusters for clusters across groups'
    summary['distinct_fact_clusters'] = len({c['cluster_id'] for c in cases.values()})
    summary['abstention_mechanical'] = abstention_rows(rows, cases, M.ARMS)
    summary['answer_level_abstention'] = None
    summary['frozen_threshold_application'] = verdicts(summary, protocol, cases)
    summary['reviewer_kind'] = protocol.get('reviewer_kind')
    summary['scorer_sha256'] = {'adapter': sha(Path(__file__).read_bytes()), 'b1_summarizer': sha(B1.read_bytes())}
    summary['input_sha256'] = {'report': sha(raw['report.json']), 'rows': sha(raw['rows.jsonl']),
                               'requests': sha(raw['requests.json']), 'questions': sha(raw_book),
                               'protocol': sha(raw_protocol), 'footprints': footprints, 'arm_configurations': configs}
    summary['referenced_source_files_sha256'] = sources
    require(M.referenced_sources(rows, cases, args.source_root) == sources, 'source changed during scoring')
    fd = os.open(args.output, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, 'w', encoding='utf-8') as stream:
        stream.write(json.dumps(summary, ensure_ascii=False, indent=2) + '\n')
    return 0


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (ValueError, OSError, KeyError, TypeError) as exc:
        print('refactoring-eval-summarize: ' + str(exc), file=sys.stderr)
        sys.exit(1)
