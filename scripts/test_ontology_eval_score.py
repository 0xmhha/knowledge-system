"""Synthetic rows verify planned denominators and prevent DEV overclaims."""
import copy
import importlib.util
import json
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'system/eval/b0-knowledge-system/ontology-o04-evaluation-review-2026-10-13'
SPEC = importlib.util.spec_from_file_location('ontology_score', Path(__file__).with_name('ontology-eval-score.py'))
S = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(S)


class ScoringContract(unittest.TestCase):
    def setUp(self):
        self.book = json.loads((PACKET / 'question-book.json').read_text())
        self.protocol = json.loads((PACKET / 'protocol-proposed.json').read_text())
        self.rows = []
        for arm in ['off', 'on']:
            for q in self.book['questions'][:6]:
                for phase, count in self.protocol['counts'].items():
                    for i in range(1, count + 1):
                        self.rows.append({'arm': arm, 'request_id': q['id'], 'phase': phase, 'iteration': i,
                                          'call': {'tool': q['tool'], 'arguments': copy.deepcopy(q['arguments']),
                                                   'elapsed_ns': 100 if arm == 'off' else 120,
                                                   'response': {'structuredContent': {'query': q['question'],
                                                                                       'citations': copy.deepcopy(q['evidence'])}}}})

    def score(self, rows=None, verifier=lambda pack: None):
        # Source/integrity verification is tested separately using real retained packs.
        return S.score(self.book, self.protocol, self.rows if rows is None else rows, verifier)

    def test_complete_slots_count_repeats_as_four_clusters(self):
        result = self.score()
        self.assertEqual(result['arms']['off']['planned_calls'], 162)
        self.assertEqual(result['arms']['off']['phases']['retrieval']['planned'], 30)
        self.assertEqual(result['proposed_fact_clusters'], 4)
        self.assertEqual(result['approved_independent_units'], 0)
        self.assertFalse(result['full_task_complete'] or result['release_eligible'])
        self.assertEqual(result['warm_p95_ratio_on_to_off'], 1.2)

    def test_missing_search_remains_in_planned_denominator(self):
        rows = [r for r in self.rows if not (r['arm'] == 'off' and r['request_id'] == 'OP-01' and
                                            r['phase'] == 'retrieval' and r['iteration'] == 1)]
        result = self.score(rows)
        self.assertEqual(result['per_question']['off']['OP-01']['citation_recall_at_10'], .8)
        self.assertEqual(result['arms']['off']['missing_calls'], 1)
        self.assertEqual(result['pilot_cost_budget_observation'], 'invalid_incomplete')

    def test_error_response_cannot_score_its_attached_citations(self):
        row = next(r for r in self.rows if r['arm'] == 'on' and r['phase'] == 'retrieval')
        row['call']['response']['isError'] = True
        result = self.score()
        self.assertEqual(result['per_question']['on']['OP-01']['citation_recall_at_10'], .8)
        self.assertEqual(result['arms']['on']['failed_calls'], 1)

    def test_missing_body_is_error_not_success(self):
        self.rows[0]['call']['response'] = {}
        self.assertEqual(self.score()['arms']['off']['failed_calls'], 1)

    def test_warm_error_excluded_from_quantile_but_counted_as_failure(self):
        row = next(r for r in self.rows if r['arm'] == 'on' and r['phase'] == 'warm_latency')
        row['call']['transport_error'] = 'timeout'
        row['call']['elapsed_ns'] = 10000
        result = self.score()
        self.assertEqual(result['arms']['on']['warm_success_n'], 119)
        self.assertEqual(result['arms']['on']['warm_p95_ns'], 120)
        self.assertEqual(result['pilot_cost_budget_observation'], 'invalid_incomplete')

    def test_empty_capture_cannot_pass_cost_or_quality(self):
        result = self.score([])
        self.assertEqual(result['arms']['off']['missing_calls'], 162)
        self.assertIsNone(result['warm_p95_ratio_on_to_off'])
        self.assertEqual(result['pilot_cost_budget_observation'], 'invalid_incomplete')

    def test_duplicate_and_trace_guard_rows_rejected(self):
        with self.assertRaises(ValueError):
            self.score(self.rows + [self.rows[0]])
        self.rows[0]['request_id'] = 'OP-07'
        with self.assertRaises(ValueError):
            self.score()

    def test_changed_arguments_rejected(self):
        self.rows[0]['call']['arguments']['prompt'] = 'changed'
        with self.assertRaises(ValueError):
            self.score()

    def test_foreign_response_query_rejected(self):
        self.rows[0]['call']['response']['structuredContent']['query'] = 'changed'
        with self.assertRaises(ValueError):
            self.score()

    def test_source_integrity_rejection_stops_scoring(self):
        def reject(pack):
            raise ValueError('foreign tuple or bad retained source hash')
        with self.assertRaisesRegex(ValueError, 'foreign tuple'):
            self.score(verifier=reject)

    def test_boolean_iteration_or_negative_elapsed_rejected(self):
        for field, value in [('iteration', True), ('elapsed_ns', -1)]:
            rows = copy.deepcopy(self.rows)
            (rows[0] if field == 'iteration' else rows[0]['call'])[field] = value
            with self.assertRaises(ValueError):
                self.score(rows)

    def test_over_budget_is_reported_without_releasing(self):
        for row in self.rows:
            if row['arm'] == 'on':
                row['call']['elapsed_ns'] = 130
        result = self.score()
        self.assertEqual(result['pilot_cost_budget_observation'], 'over_budget')
        self.assertFalse(result['default_on'] or result['release_eligible'])

    def test_cluster_means_do_not_overweight_three_handler_questions(self):
        for row in self.rows:
            if row['request_id'] in ['OP-03', 'OP-04', 'OP-05']:
                row['call']['response']['structuredContent']['citations'] = []
        result = self.score()
        self.assertEqual(result['arms']['off']['cluster_mean']['citation_recall_at_10'], .75)

    def test_dev_cannot_claim_independent_final(self):
        self.book['questions'][0]['independent_FINAL_eligible'] = True
        with self.assertRaises(ValueError):
            self.score()


if __name__ == '__main__':
    unittest.main()
