#!/usr/bin/env python3
"""Score descriptive ontology DEV rows; repetitions never become independent units."""
import importlib.util
from pathlib import Path
import statistics

_spec = importlib.util.spec_from_file_location('ontology_base_metrics', Path(__file__).with_name('b1-summarize-matrix.py'))
M = importlib.util.module_from_spec(_spec)
_spec.loader.exec_module(M)


def require(ok, message):
    if not ok:
        raise ValueError(message)


def score(book, protocol, rows, verify_pack):
    """Caller supplies retained-source verification; errors/missing slots score zero.

    This is not answer scoring, independence review, or a release decision.
    Duplicate/unplanned rows or unverified returned evidence stop scoring.
    """
    questions = {q['id']: q for q in book['questions'] if q['kind'] == 'retrieval'}
    require(len(questions) == 6 and list(questions) == protocol['retrieval_question_ids'], 'question inventory differs')
    require(protocol['activation'] is False and protocol['release_eligible'] is False, 'pilot cannot release')
    require(all(q['independent_FINAL_eligible'] is False for q in questions.values()), 'DEV cannot become independent FINAL')
    counts = protocol['counts']
    require(counts == {'warmup': 2, 'retrieval': 5, 'warm_latency': 20, 'cold_process': 0}, 'counts differ')
    slots = {(a, qid, phase, i) for a in ['off', 'on'] for qid in questions
             for phase, n in counts.items() for i in range(1, n + 1)}
    actual = {}
    for row in rows:
        require(type(row.get('iteration')) is int, 'invalid iteration')
        key = (row['arm'], row['request_id'], row['phase'], row['iteration'])
        require(key in slots and key not in actual, 'duplicate or unplanned row')
        call = row.get('call') or {}
        require(call.get('tool') == 'cks.context.get_for_task_v2' and
                call.get('arguments') == questions[row['request_id']]['arguments'], 'wrong tool or request arguments')
        elapsed = call.get('elapsed_ns')
        require(type(elapsed) is int and elapsed >= 0, 'invalid elapsed time')
        response = call.get('response') or {}
        failed = bool(row.get('error') or call.get('transport_error') or response.get('isError') or not response.get('structuredContent'))
        if not failed:
            pack = response['structuredContent']
            verify_pack(pack)
            require(pack['query'] == questions[row['request_id']]['question'], 'response query differs')
        actual[key] = (row, failed)
    per_question, arms = {}, {}
    for arm in ['off', 'on']:
        per_question[arm] = {}
        arm_keys = [k for k in slots if k[0] == arm]
        warm, error_count, missing_count = [], 0, 0
        phase_counts = {phase: {'planned': len(questions) * n, 'observed': 0, 'failed': 0, 'missing': 0}
                        for phase, n in counts.items()}
        for key in arm_keys:
            phase = phase_counts[key[2]]
            if key not in actual:
                phase['missing'] += 1
                missing_count += 1
                continue
            row, failed = actual[key]
            phase['observed'] += 1
            phase['failed'] += failed
            error_count += failed
            if key[2] == 'warm_latency' and not failed:
                warm.append(row['call']['elapsed_ns'])
        for qid, q in questions.items():
            metrics, failed, missing = [], 0, 0
            for i in range(1, counts['retrieval'] + 1):
                item = actual.get((arm, qid, 'retrieval', i))
                if item is None or item[1]:
                    missing += item is None
                    failed += item is not None
                    metrics.append({metric: 0.0 for metric in M.METRICS})
                else:
                    pack = item[0]['call']['response']['structuredContent']
                    metrics.append(M.citation_metrics(q['evidence'], pack['citations']))
            per_question[arm][qid] = {'fact_cluster': q['fact_cluster'], 'planned_retrieval': 5,
                                     'failed_retrieval': failed, 'missing_retrieval': missing,
                                     **{metric: statistics.mean(v[metric] for v in metrics) for metric in M.METRICS}}
        clusters = {}
        for qid, result in per_question[arm].items():
            clusters.setdefault(result['fact_cluster'], []).append(result)
        by_cluster = {cluster: {metric: statistics.mean(v[metric] for v in values) for metric in M.METRICS}
                      for cluster, values in clusters.items()}
        arms[arm] = {'planned_calls': len(arm_keys), 'observed_calls': len(arm_keys) - missing_count,
                     'failed_calls': error_count, 'missing_calls': missing_count,
                     'unsuccessful_fraction': (error_count + missing_count) / len(arm_keys),
                     'phases': phase_counts, 'warm_p95_ns': M.percentile(warm, .95), 'warm_success_n': len(warm),
                     'per_cluster': by_cluster,
                     'cluster_mean': {metric: statistics.mean(v[metric] for v in by_cluster.values()) for metric in M.METRICS}}
    off, on = arms['off'], arms['on']
    ratio = on['warm_p95_ns'] / off['warm_p95_ns'] if off['warm_p95_ns'] else None
    unsuccessful = sum(a['failed_calls'] + a['missing_calls'] for a in arms.values())
    return {'scope': 'descriptive_DEV_pilot', 'arms': arms, 'per_question': per_question,
            'cluster_mean_on_minus_off': {metric: on['cluster_mean'][metric] - off['cluster_mean'][metric] for metric in M.METRICS},
            'warm_p95_ratio_on_to_off': ratio,
            'pilot_cost_budget_max': protocol['proposed_warm_p95_ratio_max'],
            'pilot_cost_budget_observation': 'invalid_incomplete' if unsuccessful or ratio is None else
                ('within_budget' if ratio <= protocol['proposed_warm_p95_ratio_max'] else 'over_budget'),
            'location_metrics_evaluated': True, 'answer_quality_evaluated': False,
            'answer_level_abstention': None, 'proposed_fact_clusters': len(off['per_cluster']),
            'approved_independent_units': 0, 'new_FINAL_samples': 0,
            'quality_verdict': 'inconclusive: known DEV overlap and fewer than 10 independent units',
            'default_on': False, 'release_eligible': False, 'full_task_complete': False}
