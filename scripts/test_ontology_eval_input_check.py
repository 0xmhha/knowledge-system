"""Reject corrupted/overclaimed ontology evaluation inputs without invoking MCP."""
import copy
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
PACKET = ROOT / 'system/eval/b0-knowledge-system/ontology-o04-evaluation-review-2026-10-13'


def module(name, filename):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(filename))
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


C = module('ontology_input_check', 'ontology-eval-input-check.py')
M = module('ontology_location_metrics', 'b1-summarize-matrix.py')


class InputContract(unittest.TestCase):
    def setUp(self):
        self.book = json.loads((PACKET / 'question-book.json').read_text())
        self.protocol = json.loads((PACKET / 'protocol-proposed.json').read_text())

    def reject(self, change):
        change(self.book, self.protocol)
        with self.assertRaises((ValueError, KeyError, TypeError)):
            C.check(self.book, self.protocol, PACKET)

    def test_preparation_is_not_evaluation_or_independence(self):
        result = C.check(self.book, self.protocol, PACKET)
        self.assertEqual((result['retrieval_questions'], result['trace_guards']), (6, 2))
        self.assertEqual(result['proposed_fact_clusters'], {'CODE': 4})
        self.assertEqual(result['approved_independent_units'], 0)
        self.assertFalse(result['evaluation_eligible'] or result['quality_evaluated'])

    def test_relabeling_as_final_is_rejected(self):
        self.reject(lambda b, p: b.update(scope='FINAL'))

    def test_inferred_approval_is_rejected(self):
        self.reject(lambda b, p: b['questions'][0].update(review_state='approved'))

    def test_claimed_independent_unit_is_rejected(self):
        self.reject(lambda b, p: b['questions'][0].update(independent_FINAL_eligible=True))

    def test_erasing_known_dev_overlap_is_rejected(self):
        self.reject(lambda b, p: b['questions'][0].update(prior_DEV_cluster_overlap=False))

    def test_duplicate_question_is_rejected(self):
        self.reject(lambda b, p: b['questions'][1].update(id=b['questions'][0]['id']))

    def test_foreign_dataset_is_rejected(self):
        self.reject(lambda b, p: p['coordinates'].update(dataset_id='foreign'))

    def test_foreign_gold_commit_is_rejected(self):
        self.reject(lambda b, p: b['questions'][0]['evidence'][0].update(commit_hash='b' * 40))

    def test_changed_gold_hash_is_rejected(self):
        self.reject(lambda b, p: b['questions'][0]['evidence'][0].update(content_sha256='0' * 64))

    def test_bool_or_out_of_bounds_line_is_rejected(self):
        for first in [True, 0, 999999]:
            book = copy.deepcopy(self.book)
            book['questions'][0]['evidence'][0]['start_line'] = first
            with self.assertRaises(ValueError):
                C.check(book, self.protocol, PACKET)

    def test_source_escape_is_rejected(self):
        self.reject(lambda b, p: b['questions'][0]['evidence'][0].update(source_copy='../question-book.json'))

    def test_unsafe_original_path_is_rejected_without_live_source_root(self):
        self.reject(lambda b, p: b['questions'][0]['evidence'][0].update(file='/private/file.go'))

    def test_symlink_escape_is_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'foreign').symlink_to(PACKET / 'question-book.json')
            with self.assertRaises(ValueError):
                C.safe_source(root, 'foreign')

    def test_unbacked_index_location_is_rejected(self):
        self.reject(lambda b, p: b['questions'][0]['evidence'][0].update(chunk_ids=[]))

    def test_gold_cannot_approve_relation(self):
        self.reject(lambda b, p: b['questions'][0]['evidence'][0].update(relation_assertion=True))

    def test_verified_content_cannot_imply_test_pass(self):
        self.reject(lambda b, p: b['questions'][6].update(expected_acceptance_evaluated=True))

    def test_trace_guard_cannot_be_strict_zero_citation_case(self):
        self.reject(lambda b, p: b['questions'][6].update(strict_zero_citations=True))

    def test_changed_prompt_cannot_reuse_gold_binding(self):
        self.reject(lambda b, p: b['questions'][0]['arguments'].update(prompt='different question'))

    def test_counterbalance_and_threshold_cannot_silently_change(self):
        self.reject(lambda b, p: p['order_per_question']['OP-02'].reverse())
        self.setUp()
        self.reject(lambda b, p: p.update(proposed_warm_p95_ratio_max=2))

    def test_pilot_cannot_activate_or_claim_answer_score(self):
        for field, value in [('activation', True), ('release_eligible', True), ('answer_level_scoring', {}),
                             ('default_on', True), ('code_relation_approval', True), ('frozen_B0_changes', True)]:
            protocol = copy.deepcopy(self.protocol)
            protocol[field] = value
            with self.assertRaises(ValueError):
                C.check(self.book, protocol, PACKET)

    def test_strict_json_rejects_duplicate_keys_and_nonfinite_numbers(self):
        for raw in [b'{"id":1,"id":2}', b'{"score":NaN}']:
            with self.assertRaises(ValueError):
                C.decode(raw)

    def test_default_cli_blocks_evaluation_but_preserves_preparation_report(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'report.json'
            run = subprocess.run([sys.executable, str(ROOT / 'scripts/ontology-eval-input-check.py'),
                                  '--book', str(PACKET / 'question-book.json'),
                                  '--protocol', str(PACKET / 'protocol-proposed.json'),
                                  '--output', str(output)], capture_output=True, text=True)
            self.assertEqual(run.returncode, 2, run.stderr)
            self.assertFalse(json.loads(output.read_text())['evaluation_eligible'])
            again = subprocess.run(run.args, capture_output=True, text=True)
            self.assertEqual(again.returncode, 1)


class LocationMetricLimits(unittest.TestCase):
    def test_foreign_commit_and_wrong_lines_do_not_match(self):
        expected = [{'file': 'a.go', 'start_line': 4, 'end_line': 6, 'commit_hash': 'a' * 40}]
        for actual in [[dict(expected[0], commit_hash='b' * 40)],
                       [dict(expected[0], start_line=7, end_line=9)], []]:
            metrics = M.citation_metrics(expected, actual)
            self.assertEqual(metrics['citation_recall_at_10'], 0)
            self.assertEqual(metrics['mean_expected_reciprocal_rank_at_10'], 0)

    def test_repeated_citations_do_not_increase_retrieval_hits(self):
        expected = [{'file': 'a.go', 'start_line': 4, 'end_line': 6, 'commit_hash': 'a' * 40}]
        self.assertEqual(M.citation_metrics(expected, expected), M.citation_metrics(expected, expected * 5))

    def test_overlap_metric_does_not_prove_factual_answer_support(self):
        expected = [{'file': 'a.go', 'start_line': 4, 'end_line': 6, 'commit_hash': 'a' * 40}]
        # Existing metric intentionally accepts a broad function containing gold.
        # The review packet must not present this mechanical hit as an answer proof.
        broad = [dict(expected[0], start_line=1, end_line=100)]
        self.assertEqual(M.citation_metrics(expected, broad)['citation_recall_at_10'], 1)


if __name__ == '__main__':
    unittest.main()
