"""Source exposure cannot certify new independent FINAL questions."""
import copy
import importlib.util
from pathlib import Path
import tempfile
import unittest

SPEC = importlib.util.spec_from_file_location('exposure', Path(__file__).with_name('ontology-eval-exposure-audit.py'))
A = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(A)


class ExposureContract(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        raw = b'package example\nfunc F() {\n return\n}\n'
        (self.root / 'a.go').write_bytes(raw)
        self.gold = {'file': 'a.go', 'start_line': 2, 'end_line': 3,
                     'commit_hash': 'a' * 40, 'file_sha256': A.sha(raw),
                     'content_sha256': A.sha(b''.join(raw.splitlines(keepends=True)[1:3])),
                     'relation_assertion': False}
        self.q = {'id': 'Q1', 'review_state': 'proposed', 'question': 'Find F',
                  'independent_FINAL_eligible': False, 'fact_cluster_proposed': 'fact',
                  'evidence': [self.gold]}
        self.book = {'review_state': 'proposed', 'independent_units_approved': 0,
                     'coordinates': {'base_commit': 'a' * 40}, 'questions': [self.q]}
        self.citation = dict(self.gold)

    def audit(self, packs=()):
        return A.audit(self.book, packs, self.root)

    def test_same_file_overlap_and_delivered_body_are_separate(self):
        pack = {'query': 'other', 'citations': [self.citation], 'bodies': []}
        row = self.audit([pack])['candidates'][0]
        self.assertEqual(row['prior_same_file_gold_overlap_responses'], 1)
        self.assertEqual(row['prior_body_gold_overlap_responses'], 0)
        pack['bodies'] = [{'citation': self.citation, 'text': 'not used as a quality proof'}]
        self.assertEqual(self.audit([pack])['candidates'][0]['prior_body_gold_overlap_responses'], 1)

    def test_new_wording_does_not_clear_source_exposure(self):
        pack = {'query': 'a differently worded question', 'citations': [self.citation]}
        row = self.audit([pack])['candidates'][0]
        self.assertEqual(row['prior_query_exact_matches'], 0)
        self.assertFalse(row['range_not_seen_in_scanned_corpus'])

    def test_new_range_is_not_independent_final(self):
        pack = {'query': 'another', 'citations': [dict(self.citation, start_line=4, end_line=4)]}
        report = self.audit([pack])
        self.assertTrue(report['candidates'][0]['range_not_seen_in_scanned_corpus'])
        self.assertEqual(report['FINAL_eligible_questions'], 0)
        self.assertFalse(report['history_audit_complete'])

    def test_different_file_sha_is_uncertain_even_without_line_overlap(self):
        pack = {'citations': [dict(self.citation, file_sha256='b' * 64, start_line=99, end_line=100)]}
        row = self.audit([pack])['candidates'][0]
        self.assertEqual(row['different_or_missing_file_SHA_responses'], 1)
        self.assertFalse(row['range_not_seen_in_scanned_corpus'])

    def test_repetition_does_not_create_units(self):
        report = self.audit([{'citations': [self.citation]}] * 20)
        self.assertEqual(report['candidates'][0]['prior_same_file_gold_overlap_responses'], 20)
        self.assertEqual(report['approved_independent_units'], 0)

    def test_duplicate_question_rejected(self):
        self.book['questions'].append(copy.deepcopy(self.q))
        with self.assertRaises(ValueError):
            self.audit()

    def test_inferred_approval_and_final_rejected(self):
        for field, value in [('review_state', 'approved'), ('independent_FINAL_eligible', True)]:
            self.q[field] = value
            with self.assertRaises(ValueError):
                self.audit()
            self.q[field] = 'proposed' if field == 'review_state' else False

    def test_tampered_gold_or_foreign_commit_rejected(self):
        for field, value in [('content_sha256', '0' * 64), ('commit_hash', 'b' * 40)]:
            original = self.gold[field]
            self.gold[field] = value
            with self.assertRaises(ValueError):
                self.audit()
            self.gold[field] = original

    def test_boolean_lines_or_source_escape_rejected(self):
        for field, value in [('start_line', True), ('file', '../a.go'), ('file', '/a.go')]:
            original = self.gold[field]
            self.gold[field] = value
            with self.assertRaises(ValueError):
                self.audit()
            self.gold[field] = original

    def test_location_gold_cannot_approve_code_relation(self):
        self.gold['relation_assertion'] = True
        with self.assertRaises(ValueError):
            self.audit()

    def test_queries_normalize_case_and_spacing_but_no_semantic_claim(self):
        row = self.audit([{'query': ' FIND  f\n', 'citations': []}])['candidates'][0]
        self.assertEqual(row['prior_query_exact_matches'], 1)
        self.assertFalse(row['independent_FINAL_eligible'])

    def test_duplicate_json_keys_cannot_hide_review_state(self):
        for raw in [b'{"review_state":"approved","review_state":"proposed"}', b'{"value":NaN}']:
            with self.assertRaises(ValueError):
                A.decode(raw)


if __name__ == '__main__':
    unittest.main()
