#!/usr/bin/env python3
"""Validate a review proposal for static v2 scopes; never export final prompts."""
import argparse
import datetime as dt
import json
import os
from pathlib import Path
import re
import subprocess
import sys

sys.path.insert(0, str(Path(__file__).resolve().parent))
import importlib.util

ROOT = Path(__file__).resolve().parent.parent
DATA = ROOT / 'system/eval/b0-knowledge-system'
spec = importlib.util.spec_from_file_location('static_inputs', Path(__file__).with_name('b0-prepare-static-inputs.py'))
INPUTS = importlib.util.module_from_spec(spec)
spec.loader.exec_module(INPUTS)


def inspect(scope_raw, questions_raw, protocol_raw, fixtures_raw, review_raw, commit_time):
    definition, _ = INPUTS.inspect(questions_raw, protocol_raw, fixtures_raw, review_raw)
    scope = INPUTS.decode(scope_raw)
    book = INPUTS.decode(questions_raw)
    protocol = INPUTS.decode(protocol_raw)
    review = INPUTS.decode(review_raw)
    if scope.get('schema_version') != 1 or scope.get('status') not in {'draft', 'approved'}:
        raise ValueError('unsupported scope proposal schema/status')
    for field, expected in [('question_set_sha256', INPUTS.digest(questions_raw)),
                            ('protocol_sha256', INPUTS.digest(protocol_raw)),
                            ('corpus_commit', definition['corpus_commit']),
                            ('corpus_tree', definition['corpus_tree'])]:
        if scope.get(field) != expected:
            raise ValueError(field + ' differs from current input bytes/identity')
    if scope.get('commit_time_evidence') != commit_time:
        raise ValueError('commit time evidence differs from actual Git metadata')
    instant = dt.datetime.fromisoformat(commit_time)
    if instant.utcoffset() is None:
        raise ValueError('Git commit time must contain a timezone')
    if scope.get('date_basis') != 'calendar date in the recorded commit timezone; proposed query date, not policy approval':
        raise ValueError('date basis must distinguish a query date from policy approval')
    patch = scope.get('runtime_config_patch')
    if (patch != {'retrieval': {'recall_k': protocol['retrieval_k']}}
            or type(patch['retrieval']['recall_k']) is not int):
        raise ValueError('runtime K patch differs from protocol; default K20 cannot substitute for K10')
    rows = scope.get('question_scopes')
    if not isinstance(rows, list) or any(not isinstance(row, dict) for row in rows):
        raise ValueError('question scope inventory missing')
    actual = INPUTS.identifiers([row.get('id') for row in rows], 'scope question IDs')
    expected = {q['id'] for q in book['questions']}
    if actual != expected:
        raise ValueError('scope IDs must cover all static questions exactly once')
    for row in rows:
        if set(row) != {'id', 'partition', 'knowledge_as_of', 'knowledge_subsystem'}:
            raise ValueError('scope rows must contain only ID/partition/date/subsystem; no prompts or gold')
        partition = 'development' if row['id'] in protocol['development_questions'] else 'final'
        if row['partition'] != partition:
            raise ValueError('scope partition differs from reviewed protocol split')
        value = row['knowledge_as_of']
        if not isinstance(value, str) or not re.fullmatch(r'[0-9]{4}-[0-9]{2}-[0-9]{2}', value):
            raise ValueError('knowledge_as_of must be YYYY-MM-DD')
        try:
            date = dt.date.fromisoformat(value)
        except ValueError as exc:
            raise ValueError('knowledge_as_of must be a valid calendar date') from exc
        if date != instant.date():
            raise ValueError('this proposal must use the declared commit-calendar-date basis')
        if row['knowledge_subsystem'] != book['corpus_project']:
            raise ValueError('this proposal must explicitly use the corpus project as the proposed subsystem')
    if scope.get('results') is not None:
        raise ValueError('scope proposal cannot contain measurement results')
    decisions = [d for d in review['decisions'] if d.get('scope') == 'static_v2_query_scopes']
    approved = (scope['status'] == 'approved' and INPUTS.reviewed(scope)
                and len(decisions) == 1
                and decisions[0].get('scope_sha256_after') == INPUTS.digest(scope_raw))
    reasons = list(definition['pending_reasons'])
    if not approved:
        reasons.append('static_v2_scope_review_pending')
    return {'schema_version': 1, 'gate': 'B0-static-v2-scope-proposal',
            'status': 'pending' if reasons else 'approved_scope_definition',
            'scope_sha256': INPUTS.digest(scope_raw),
            'question_set_sha256': definition['question_set_sha256'],
            'protocol_sha256': definition['protocol_sha256'],
            'human_review_sha256': definition['human_review_sha256'],
            'question_count': len(rows), 'scope_review_record_bound': approved,
            'knowledge_as_of': instant.date().isoformat(),
            'proposed_subsystem': book['corpus_project'],
            'runtime_config_patch': scope['runtime_config_patch'],
            'pending_reasons': reasons, 'official_execution_ready': False,
            'v2_matrix_ready': False, 'prompts_exported': False, 'metrics': None,
            'remaining': ['corpus coverage and semantic-fact decisions',
                          'reviewed source-bound pack/semantic inventories and dataset locks',
                          'fresh strict dataset build and actual runtime K/config binding',
                          'actual model/environment/binary preflight and final independent execution']}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--scopes', type=Path, default=DATA / 'static-v2-scopes-m2max-draft.json')
    parser.add_argument('--questions', type=Path, default=DATA / 'questions.json')
    parser.add_argument('--protocol', type=Path, default=DATA / 'protocol-m2max-draft.json')
    parser.add_argument('--fixtures', type=Path, default=DATA / 'dynamic-fixtures-m2max-draft.json')
    parser.add_argument('--human-review', type=Path, default=DATA / 'human-review-m2max-2026-10-03.json')
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    inputs = [p.read_bytes() for p in (args.scopes, args.questions, args.protocol, args.fixtures, args.human_review)]
    commit = INPUTS.decode(inputs[1])['corpus_commit']
    commit_time = subprocess.run(['git', 'show', '-s', '--format=%cI', commit], cwd=ROOT,
                                 check=True, capture_output=True, text=True).stdout.strip()
    result = inspect(*inputs, commit_time)
    fd = os.open(args.output, os.O_WRONLY | os.O_CREAT | os.O_EXCL, 0o600)
    with os.fdopen(fd, 'w', encoding='utf-8') as stream:
        stream.write(json.dumps(result, ensure_ascii=False, indent=2) + '\n')
    return 2 if result['pending_reasons'] else 0


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (ValueError, OSError, subprocess.CalledProcessError) as exc:
        print('b0-check-static-v2-scopes: ' + str(exc), file=sys.stderr)
        sys.exit(1)
