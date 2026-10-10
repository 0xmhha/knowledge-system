"""Synthetic rows only; checks the question-book adapter and ABS citation rule."""
import importlib.util
from pathlib import Path
import unittest

SPEC = importlib.util.spec_from_file_location('eval_summarize', Path(__file__).with_name('refactoring-eval-summarize.py'))
S = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(S)

COMMIT = 'a' * 40
BASIS = {'path': 'docs/rec.md', 'first': 67, 'last': 69}


def question(ident, split, group, evidence=(), **extra):
    q = {'id': ident, 'split': split, 'group': group, 'cluster_id': ident + '-c', 'review_state': 'approved',
         'source_commit': COMMIT, 'expected_behavior': 'abstain' if group == 'ABS' else 'cite',
         'evidence': list(evidence)}
    q.update(extra)
    return q


def row(arm, qid, cites):
    return {'arm': arm, 'phase': 'retrieval', 'request_id': qid,
            'call': {'response': {'structuredContent': {'citations': cites}}}}


def cite(path, first, last):
    return {'file': path, 'start_line': first, 'end_line': last}


class Adapter(unittest.TestCase):
    def test_split_filter_and_span_mapping(self):
        book = {'questions': [question('D1', 'DEV', 'CODE', [{'path': 'a.go', 'first': 3, 'last': 9}]),
                              question('F1', 'FINAL', 'CODE')]}
        cases = S.cases_from_book(book, 'DEV', COMMIT)
        self.assertEqual(list(cases), ['D1'])
        self.assertEqual(cases['D1']['expected_citations'], [{'file': 'a.go', 'start_line': 3, 'end_line': 9}])
        self.assertFalse(cases['D1']['expect_no_citations'])

    def test_shared_cluster_is_one_unit_per_group(self):
        book = {'questions': [question('C1', 'FINAL', 'CODE', cluster_id='pair'),
                              question('W1', 'FINAL', 'WHY', cluster_id='pair'),
                              question('W2', 'FINAL', 'WHY', cluster_id='pair')]}
        cases = S.cases_from_book(book, 'FINAL', COMMIT)
        self.assertEqual(cases['C1']['independent_unit_id'], 'CODE/pair')
        self.assertEqual(cases['W1']['independent_unit_id'], cases['W2']['independent_unit_id'])
        self.assertEqual({c['cluster_id'] for c in cases.values()}, {'pair'})

    def test_rejects_unapproved_foreign_commit_and_abs_mismatch(self):
        for q in (question('D1', 'DEV', 'CODE', review_state='proposed'),
                  question('D1', 'DEV', 'CODE', source_commit='b' * 40),
                  question('D1', 'DEV', 'ABS', expected_behavior='cite')):
            with self.assertRaises(ValueError):
                S.cases_from_book({'questions': [q]}, 'DEV', COMMIT)

    def test_strict_flag_kept_per_item(self):
        book = {'questions': [question('A1', 'FINAL', 'ABS', strict_zero_citations=True),
                              question('A2', 'FINAL', 'ABS', strict_zero_citations=False, abstention_basis_sources=[BASIS])]}
        cases = S.cases_from_book(book, 'FINAL', COMMIT)
        self.assertTrue(cases['A1']['expect_no_citations'])
        self.assertFalse(cases['A2']['expect_no_citations'])

    def test_basis_citation_allowed_other_citation_counted(self):
        book = {'questions': [question('A2', 'FINAL', 'ABS', strict_zero_citations=False, abstention_basis_sources=[BASIS])]}
        cases = S.cases_from_book(book, 'FINAL', COMMIT)
        rows = [row('baseline_off', 'A2', [cite('docs/rec.md', 60, 67)]),
                row('baseline_off', 'A2', []),
                row('combined_on', 'A2', [cite('docs/rec.md', 68, 68), cite('x.go', 1, 2)])]
        result = S.abstention_rows(rows, cases, ['baseline_off', 'combined_on'])
        self.assertEqual(result['baseline_off']['basis_only_rows'], 1)
        self.assertEqual(result['baseline_off']['zero_citation_rows'], 1)
        self.assertEqual(result['baseline_off']['off_basis_citation_rate'], 0.0)
        self.assertEqual(result['combined_on']['rows_with_off_basis_citation'], 1)
        self.assertEqual(result['combined_on']['off_basis_rate_delta_to_baseline_off'], 1.0)

    def test_small_groups_stay_inconclusive(self):
        cases = {f'Q{i}': {'group': 'CODE', 'independent_unit_id': f'u{i}'} for i in range(9)}
        summary = {'arm_summaries': {'baseline_off': {'warm_p95_ratio_to_baseline_off': 1.0},
                                     'combined_on': {'warm_p95_ratio_to_baseline_off': 1.3}}}
        v = S.verdicts(summary, {'thresholds': {'warm_p95_ratio_max': 1.25}}, cases)
        self.assertEqual(v['groups']['CODE']['verdict'], 'inconclusive')
        self.assertEqual(v['arms_over_p95_ratio'], ['combined_on'])
        self.assertFalse(v['default_on'] or v['release_eligible'])


if __name__ == '__main__':
    unittest.main()
