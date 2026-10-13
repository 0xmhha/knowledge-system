#!/usr/bin/env python3
"""Check ontology task closure without turning observations into quality approval.

Task closure records implementation, execution and an explicit inconclusive
verdict. It does not grant independent-quality, criterion, release or default-on
acceptance. Frozen inputs, raw pack verification and committed SHA bindings are
checked by their separate evidence receipts.
"""
import argparse
import importlib.util
import json
from pathlib import Path
import sys

S = importlib.util.spec_from_file_location('completion_score', Path(__file__).with_name('ontology-eval-score.py'))
A = importlib.util.module_from_spec(S)
S.loader.exec_module(A)


def check(book, protocol, result, verdict):
    require = A.require
    require(book['scope'] == protocol['scope'] == result['scope'] == 'source_bound_candidate_observation',
            'observation scope changed')
    retrieval = [q for q in book['questions'] if q['kind'] == 'retrieval']
    guards = [q for q in book['questions'] if q['kind'] == 'trace_guard']
    require(len(retrieval) == protocol['planned_retrieval_questions'] == 13 and len(guards) == 2,
            'frozen question inventory changed')
    require([q['id'] for q in retrieval] == protocol['retrieval_question_ids'], 'question IDs changed')
    require(all(q['independent_FINAL_eligible'] is False for q in book['questions']), 'FINAL eligibility invented')
    require(protocol['approved_independent_units'] == result['approved_independent_units'] ==
            verdict['approved_independent_units'] == 0, 'independent units invented')
    require(protocol['counts'] == {'warmup': 2, 'retrieval': 5, 'warm_latency': 20, 'cold_process': 0},
            'frozen repetitions changed')
    calls = len(retrieval) * sum(protocol['counts'].values())
    for arm in ('off', 'on'):
        r = result['arms'][arm]
        require(r['planned_calls'] == r['observed_calls'] == calls and
                r['failed_calls'] == r['missing_calls'] == 0, 'incomplete execution')
    require(result['verified_pairs'] == calls and result['failed_or_missing_pairs'] == 0,
            'paired outputs incomplete')
    require(len(result['pairs']) == calls and len(result['execution_runs']) == 26,
            'comparison or execution receipts missing')
    require(all(r['exit'] == 0 and r['observed'] == r['planned'] == 27 for r in result['execution_runs']),
            'execution receipt failed')
    require(set(result['trace_guards']) == {q['id'] for q in guards} and
            all(g['passed'] for g in result['trace_guards'].values()), 'trace guard missing or failed')
    for key in ('failed_HTTP_attempts', 'pending_calls', 'unbound_request_scopes'):
        require(result['backend'][key] == {'off': 0, 'on': 0}, 'backend measurement incomplete')
    for key in ('paired_execution_inputs_verified', 'model_preflight_digest_verified',
                'held_index_and_semantic_SHA_preserved', 'operator_source_current_config_preserved'):
        require(result[key] is True, 'input or preservation guard failed')
    off, on = result['arms']['off']['warm_p95_ns'], result['arms']['on']['warm_p95_ns']
    require(off and on and abs(result['warm_p95_ratio_on_to_off'] - on / off) < 1e-12,
            'cost ratio not derived from measurements')
    budget = 'within_budget' if on / off <= protocol['proposed_warm_p95_ratio_max'] else 'over_budget'
    require(result['pilot_cost_budget_observation'] == verdict['cost_budget_observation'] == budget,
            'cost verdict contradicts measurements')
    require(result['location_metrics_evaluated'] is True and result['answer_quality_evaluated'] is False,
            'location metrics misrepresented as answers')
    require(result['new_FINAL_samples'] == verdict['new_FINAL_samples'] == 0 and
            verdict['human_independence_review'] is False, 'FINAL or human review invented')
    require(verdict['independent_evaluation_eligibility'] == 'not_established' and
            verdict['independent_quality_accepted'] is False and
            verdict['quality_verdict'] == result['quality_verdict'] and
            verdict['quality_verdict'].startswith('inconclusive:'), 'quality success invented')
    require(verdict['task_completion_basis'] == 'specified_evaluation_and_limitations_recorded',
            'unknown task closure basis')
    require(set(verdict['criteria']) == {'O04-A', 'O04-B', 'O04-C', 'O04-D'} and
            all(isinstance(e, list) and e for e in verdict['criteria'].values()), 'criterion evidence omitted')
    for key in ('default_on', 'activation', 'release_eligible', 'code_relation_approval',
                'WBFT_criterion_acceptance', 'public_transfer'):
        require(verdict[key] is False, 'closure expands acceptance or authority')
    require(verdict['limitations'] and verdict['full_task_complete'] is True,
            'task decision or limitations absent')
    return {'status': 'PASS', 'planned_responses': calls * 2, 'verified_pairs': calls,
            'task_complete': True, 'quality_accepted': False, 'new_FINAL_samples': 0,
            'independent_evaluation_eligibility': 'not_established'}


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--packet', type=Path, required=True)
    args = p.parse_args()
    def read(name): return A.M.decode((args.packet / name).read_bytes())
    result = check(read('question-book-frozen.json'), read('protocol-frozen.json'),
                   read('result-summary.json'), read('final-verdict.json'))
    print(json.dumps(result))


if __name__ == '__main__':
    try:
        main()
    except (ValueError, KeyError, OSError, TypeError) as error:
        print('ontology completion: FAIL: ' + str(error), file=sys.stderr)
        sys.exit(1)
