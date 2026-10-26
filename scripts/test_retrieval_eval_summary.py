import unittest

from retrieval_eval_summary import first_hit_any_commit, rank_metrics, summarize_baseline, summarize_cks


def cks_row(rank, intent='bug_fix', got='bug_fix', error=False):
    row = {'name': f'pr-{rank}', 'expected_intent': intent, 'classified_intents': [got], 'intent_match': got == intent,
           'retrieval_state': 'error' if error else ('pass' if rank else 'miss'),
           'metrics': {'first_hit_rank': rank, 'file_recall': 1 if rank else 0, 'used_tokens': 100, 'latency_ms': 10}}
    if error:  # runner.go: a failed run never matches and adds no classification
        row.update(error='service unavailable', intent_match=False, classified_intents=[])
    return row


class SummaryTests(unittest.TestCase):
    def test_rank_metrics_keep_misses_in_denominator(self):
        m = rank_metrics([1, 3, 0, 0])
        self.assertEqual((m['hit_at_1'], m['hit_at_3']), (0.25, 0.5))
        self.assertAlmostEqual(m['mrr'], (1 + 1 / 3) / 4)

    def test_cks_errors_count_as_misses_and_wrong_intent(self):
        m = summarize_cks([cks_row(1), cks_row(2, error=True), cks_row(0, got='')])
        self.assertEqual((m['scenarios'], m['errors']), (3, 1))
        self.assertAlmostEqual(m['hit_at_1'], 1 / 3)
        self.assertAlmostEqual(m['intent_accuracy'], 1 / 3)
        self.assertEqual(m['intent_confusion']['bug_fix'].get('(unknown)'), 2)  # the error and the empty class

    def test_baseline_error_is_a_miss(self):
        rows = [{'grep': {'first_hit_rank': 1, 'returned_tokens': 5}}, {'grep': {'first_hit_rank': 0, 'error': 'x'}}]
        m = summarize_baseline(rows, 'grep')
        self.assertEqual((m['errors'], m['hit_at_1'], m['returned_tokens_mean']), (1, 0.5, 5))

    def test_any_commit_rank_dedups_by_location_and_ignores_commit(self):
        expected = [{'file': 'a.go', 'start_line': 10, 'end_line': 20}]
        cites = [{'file': 'b.go', 'start_line': 1, 'end_line': 5, 'commit_hash': 'new'},
                 {'file': 'b.go', 'start_line': 1, 'end_line': 5, 'commit_hash': 'old'},
                 {'file': 'a.go', 'start_line': 20, 'end_line': 30, 'commit_hash': 'old'}]
        self.assertEqual(first_hit_any_commit(expected, cites), 2)
        self.assertEqual(first_hit_any_commit(expected, cites[:2]), 0)


if __name__ == '__main__':
    unittest.main()
