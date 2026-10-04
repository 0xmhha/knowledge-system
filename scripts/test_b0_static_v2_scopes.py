import copy
import importlib.util
import json
from pathlib import Path
import unittest

ROOT = Path(__file__).resolve().parent.parent
DATA = ROOT / 'system/eval/b0-knowledge-system'
spec = importlib.util.spec_from_file_location('v2_scopes', Path(__file__).with_name('b0-check-static-v2-scopes.py'))
SCOPES = importlib.util.module_from_spec(spec)
spec.loader.exec_module(SCOPES)


class StaticV2Scopes(unittest.TestCase):
    def setUp(self):
        self.scope = json.loads((DATA / 'static-v2-scopes-m2max-draft.json').read_text())
        self.raw = [(DATA / name).read_bytes() for name in
                    ['questions.json', 'protocol-m2max-draft.json',
                     'dynamic-fixtures-m2max-draft.json', 'human-review-m2max-2026-10-03.json']]

    def inspect(self, scope=None):
        value = self.scope if scope is None else scope
        return SCOPES.inspect(json.dumps(value).encode(), *self.raw, self.scope['commit_time_evidence'])

    def test_draft_is_pending_and_never_exposes_prompts_or_readiness(self):
        result = self.inspect()
        self.assertEqual(result['question_count'], 12)
        self.assertIn('static_v2_scope_review_pending', result['pending_reasons'])
        for field in ['official_execution_ready', 'v2_matrix_ready', 'prompts_exported']:
            self.assertFalse(result[field])
        self.assertIsNone(result['metrics'])

    def test_self_claimed_approval_without_human_hash_binding_stays_pending(self):
        value = copy.deepcopy(self.scope)
        value.update(status='approved', reviewer='synthetic-fixture-reviewer', reviewed_at='2026-10-04T00:00:00Z')
        self.assertIn('static_v2_scope_review_pending', self.inspect(value)['pending_reasons'])

    def test_pending_human_decision_with_matching_hash_is_not_approval(self):
        value = copy.deepcopy(self.scope)
        value.update(status='approved', reviewer='synthetic-test-only', reviewed_at='2026-10-04T00:00:00Z')
        review = json.loads(self.raw[3])
        review['decisions'].append({'scope': 'static_v2_query_scopes', 'status': 'pending',
            'scope_sha256_after': SCOPES.INPUTS.digest(json.dumps(value).encode()),
            'reviewer': 'synthetic-test-only', 'reviewed_at': '2026-10-04T00:00:00Z'})
        self.raw[3] = json.dumps(review).encode()
        self.assertFalse(self.inspect(value)['scope_review_record_bound'])

    def test_scope_coverage_partition_and_gold_contamination_are_rejected(self):
        for mutation in ['missing', 'duplicate', 'partition', 'prompt']:
            with self.subTest(mutation=mutation):
                value = copy.deepcopy(self.scope)
                if mutation == 'missing':
                    value['question_scopes'].pop()
                elif mutation == 'duplicate':
                    value['question_scopes'].append(copy.deepcopy(value['question_scopes'][0]))
                elif mutation == 'partition':
                    value['question_scopes'][0]['partition'] = 'final'
                else:
                    value['question_scopes'][0]['prompt'] = 'gold leakage must not be exported'
                with self.assertRaises(ValueError):
                    self.inspect(value)

    def test_actual_commit_input_hash_and_proposed_subsystem_are_bound(self):
        for field in ['protocol_sha256', 'question_set_sha256', 'corpus_commit', 'commit_time_evidence', 'subsystem']:
            with self.subTest(field=field):
                value = copy.deepcopy(self.scope)
                if field == 'subsystem':
                    value['question_scopes'][0]['knowledge_subsystem'] = 'refund'
                else:
                    value[field] = 'different'
                with self.assertRaises(ValueError):
                    self.inspect(value)

    def test_date_requires_mcp_calendar_format_and_declared_basis(self):
        for date in ['2026-10-02', '2026-10-01T00:00:00Z', '2026-02-30', '2026-W40-4']:
            with self.subTest(date=date):
                value = copy.deepcopy(self.scope)
                value['question_scopes'][0]['knowledge_as_of'] = date
                with self.assertRaises(ValueError):
                    self.inspect(value)

    def test_runtime_k_requires_integer_ten_not_default_or_float(self):
        for k in [20, 5, 10.0, '10', True]:
            with self.subTest(k=k):
                value = copy.deepcopy(self.scope)
                value['runtime_config_patch']['retrieval']['recall_k'] = k
                with self.assertRaises(ValueError):
                    self.inspect(value)


if __name__ == '__main__':
    unittest.main()
