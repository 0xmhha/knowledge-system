#!/usr/bin/env python3
"""Validate a proposed ontology pilot book without running MCP or changing approval.

Preparation is deliberately separate from evaluation eligibility. Location gold
supports retrieval metrics, not a factual answer or implementation verdict.
"""
import argparse
import hashlib
import json
from pathlib import Path
import re
import sys


def require(ok, message):
    if not ok:
        raise ValueError(message)


def sha(raw):
    return hashlib.sha256(raw).hexdigest()


def decode(raw):
    def unique(pairs):
        result = {}
        for key, value in pairs:
            require(key not in result, 'duplicate JSON key: ' + key)
            result[key] = value
        return result
    return json.loads(raw, object_pairs_hook=unique,
                      parse_constant=lambda value: (_ for _ in ()).throw(ValueError('nonfinite number')))


def safe_source(root, relative):
    require(isinstance(relative, str) and relative and '\\' not in relative,
            'invalid source path')
    path = Path(relative)
    require(not path.is_absolute() and '..' not in path.parts, 'unsafe source path')
    resolved = (root / path).resolve()
    require(resolved.is_relative_to(root.resolve()) and resolved.is_file(),
            'source missing or escaped source root')
    return resolved


def check(book, protocol, packet_root, source_root=None):
    require(book['schema_version'] == protocol['schema_version'] == 1, 'unsupported schema')
    require(book['scope'] == protocol['scope'] == 'proposed_descriptive_DEV_pilot',
            'pilot cannot be relabeled FINAL or independent')
    require(book['review_state'] == protocol['review_state'] == 'proposed',
            'review state changes require a separately retained approval and frozen input')
    coordinates = book['coordinates']
    require(coordinates == protocol['coordinates'], 'book/protocol coordinates differ')
    require(re.fullmatch(r'[0-9a-f]{40}', coordinates['base_commit']) is not None,
            'invalid source commit')
    require(all(isinstance(coordinates.get(k), str) and coordinates[k]
                for k in ['project_id', 'dataset_id', 'snapshot_id']), 'missing coordinates')
    require(protocol['arms'] == {'off': 'baseline', 'on': 'concept_text'}, 'unsupported arms')
    require(protocol['counts'] == {'warmup': 2, 'retrieval': 5, 'warm_latency': 20, 'cold_process': 0},
            'repetition plan changed')
    require(protocol['retrieval_k'] == 10 and protocol['independent_unit_floor'] == 10,
            'K or independent-unit floor changed')
    require(protocol['proposed_warm_p95_ratio_max'] == 1.25, 'proposed cost budget changed')
    require(protocol['activation'] is False and protocol['release_eligible'] is False,
            'pilot must not activate or release')
    require(protocol['default_on'] is False and protocol['code_relation_approval'] is False
            and protocol['frozen_B0_changes'] is False, 'pilot cannot change defaults or existing approvals')
    require(protocol['answer_level_scoring'] is None, 'retrieval is not generated-answer scoring')
    questions = book['questions']
    require(len(questions) == 8 and len({q['id'] for q in questions}) == 8,
            'missing or duplicate question')
    retrieval, guards, groups, files = [], [], {}, {}
    for q in questions:
        require(q['review_state'] == 'proposed' and q['independent_FINAL_eligible'] is False,
                'question approval or independence cannot be inferred')
        require(q['prior_DEV_cluster_overlap'] is True, 'known DEV overlap must remain recorded')
        require(isinstance(q['fact_cluster'], str) and q['fact_cluster'] and q['answer_reference'],
                'fact cluster or answer reference missing')
        require(q['expected_concept'] in {'wbft-quorum', 'wbft-round-change',
                                         'wbft-message-handlers', 'wbft-current-lock'},
                'unknown expected concept')
        require(q['evidence'], 'explicit source evidence required, including guard basis')
        for e in q['evidence']:
            relative = e['file']
            require(isinstance(relative, str) and relative and '\\' not in relative
                    and not Path(relative).is_absolute() and '..' not in Path(relative).parts,
                    'unsafe original source path')
            require(e['source_copy'] == relative.replace('/', '__') + '.txt', 'source copy mapping differs')
            require(e['commit_hash'] == coordinates['base_commit'], 'foreign gold commit')
            safe_source(packet_root / 'sources', e['source_copy'])
            path = safe_source(source_root, e['file']) if source_root else safe_source(packet_root / 'sources', e['source_copy'])
            raw = path.read_bytes()
            lines = raw.splitlines(keepends=True)
            first, last = e['start_line'], e['end_line']
            require(type(first) is int and type(last) is int and 1 <= first <= last <= len(lines),
                    'gold range outside source')
            require(sha(raw) == e['file_sha256'] and
                    sha(b''.join(lines[first - 1:last])) == e['content_sha256'], 'gold source hash mismatch')
            copy_raw = safe_source(packet_root / 'sources', e['source_copy']).read_bytes()
            require(copy_raw == raw, 'archived source differs from retained source')
            files[e['file']] = e['file_sha256']
            require(e['relation_assertion'] is False, 'location gold cannot approve code/test relation')
        if q['kind'] == 'retrieval':
            require(q['tool'] == 'cks.context.get_for_task_v2', 'wrong retrieval tool')
            require(q['arguments'] == {'prompt': q['question'], 'explain_ontology': True},
                    'retrieval arguments differ from question')
            require(q['strict_zero_citations'] is False and q['metric_group'] == 'CODE',
                    'positive retrieval is not strict abstention')
            require(all(e['canonical_id'] and e['chunk_ids'] for e in q['evidence']),
                    'retrieval gold lacks actual index coverage receipt')
            retrieval.append(q['id'])
            groups.setdefault(q['metric_group'], set()).add(q['fact_cluster'])
        else:
            require(q['kind'] == 'trace_guard' and q['tool'] == 'cks.semantic.trace_requirement',
                    'unsupported question kind')
            require(q['metric_group'] is None and q['strict_zero_citations'] is None,
                    'trace guard is not a citation abstention metric')
            require(q['expected_trace_state'] == 'missing_code' and
                    q['expected_acceptance_evaluated'] is False and
                    q['arguments'] == {'requirement_id': 'req-' + q['expected_concept'], 'version': 1},
                    'guard cannot claim implementation or criterion success')
            guards.append(q['id'])
    require(len(retrieval) == 6 and len(guards) == 2, 'pilot inventory differs')
    require(protocol['retrieval_question_ids'] == retrieval and protocol['guard_question_ids'] == guards,
            'protocol/question inventory differs')
    require(protocol['order_per_question'] == {qid: (['off', 'on'] if i % 2 == 0 else ['on', 'off'])
                                               for i, qid in enumerate(retrieval)},
            'counterbalanced arm schedule differs')
    return {'preparation_valid': True, 'evaluation_eligible': False, 'review_state': 'proposed',
            'new_FINAL_samples': 0, 'approved_independent_units': 0,
            'retrieval_questions': len(retrieval), 'trace_guards': len(guards),
            'proposed_fact_clusters': {g: len(ids) for g, ids in groups.items()},
            'source_files_verified': len(files), 'quality_evaluated': False,
            'quality_verdict': 'not_evaluated', 'anticipated_inference': 'inconclusive',
            'reason': 'content/protocol review pending; all four proposed fact clusters overlap earlier DEV; repeats are not independent units',
            'answer_level_abstention': None, 'operating_activation': False}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--book', required=True, type=Path)
    parser.add_argument('--protocol', required=True, type=Path)
    parser.add_argument('--source-root', type=Path)
    parser.add_argument('--output', required=True, type=Path)
    parser.add_argument('--prepare', action='store_true', help='validate preparation only, without evaluation approval')
    args = parser.parse_args()
    raw_book, raw_protocol = args.book.read_bytes(), args.protocol.read_bytes()
    result = check(decode(raw_book), decode(raw_protocol), args.book.parent, args.source_root)
    result['input_sha256'] = {'book': sha(raw_book), 'protocol': sha(raw_protocol),
                              'checker': sha(Path(__file__).read_bytes())}
    with args.output.open('x', encoding='utf-8') as stream:
        stream.write(json.dumps(result, ensure_ascii=False, indent=2) + '\n')
    print(json.dumps({k: result[k] for k in ['preparation_valid', 'evaluation_eligible', 'quality_evaluated', 'new_FINAL_samples']}))
    return 0 if args.prepare else 2


if __name__ == '__main__':
    try:
        sys.exit(main())
    except (ValueError, OSError, KeyError, TypeError) as error:
        print('ontology evaluation input: FAIL: ' + str(error), file=sys.stderr)
        sys.exit(1)
