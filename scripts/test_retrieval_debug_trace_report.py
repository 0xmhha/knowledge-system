"""Locate answer losses and failure shapes from a synthetic CKS debug trace."""
import json
import tempfile
import unittest
from pathlib import Path

import retrieval_debug_trace_report as x


def ev(stage, step, phase, data, key=''):
    return {'stage': stage, 'step': step, 'phase': phase, 'key': key, 'data': data}


def c(file, start=1, end=10, score=0.0, sources=None):
    return {'file': file, 'start': start, 'end': end, 'score': score, 'sources': sources or []}


ANSWER = c('answer.go', 1, 10)
WRONG = c('iface.go', 100, 110)

# The answer is ckv rank 1; a hit-derived keyword's FindSymbol list credits a
# wrong span three times and it takes the Stage 2 top; Stage 4 drops the answer.
RECORD = {'seq': 1, 'prompt': 'p1', 'events': [
    ev('intent', 'classify', 'after', {'intent': '', 'confidence': 0.4, 'nearest_intent': 'bug_fix',
                                       'below_threshold': True, 'scores': [{'intent': 'bug_fix'}, {'intent': 'refactor'}]}),
    ev('stage1', 'ckv_search', 'after', {'hits': [ANSWER, WRONG]}, '1'),
    ev('stage1', 'extract_keywords', 'after', {'candidates': [{'keyword': 'T.Remove', 'origin': 'hit_symbol'}]}, '1'),
    ev('stage1', 'keyword_cap', 'after', {'hits': [ANSWER, WRONG]}),
    ev('stage2', 'rank_citations', 'before', {'citations': [
        c('iface.go', 100, 110, 0.07, ['symbol:T.Remove@rank=1(+0.02459)', 'symbol:T.Remove@rank=2(+0.02419)',
                                       'symbol:T.Remove@rank=3(+0.02381)']),
        c('answer.go', 1, 10, 0.08, ['ckv:semantic@rank=1(+0.08197)'])]}),
    ev('stage2', 'rank_citations', 'after', {'citations': [
        c('iface.go', 100, 110, 0.20, ['symbol:T.Remove@rank=1(+0.02459)', 'symbol:T.Remove@rank=2(+0.02419)',
                                       'symbol:T.Remove@rank=3(+0.02381)']),
        c('answer.go', 1, 10, 0.08, ['ckv:semantic@rank=1(+0.08197)'])]}),
    ev('ckgclient', 'resolve_symbol', 'after', {'attempts': [
        {'name': 'T.Remove', 'via': 'symbol', 'nodes': 0},
        {'name': 'Remove', 'via': 'symbol_drop_leading', 'nodes': 30}]}, 'T.Remove'),
    ev('stage3', 'expand', 'after', {'neighbors': []}),
    ev('stage4', 'allocate', 'after', {'decisions': [
        {'file': 'iface.go', 'start': 100, 'end': 110, 'origin': 'seed', 'score': 0.2, 'decision': 'selected'},
        {'file': 'answer.go', 'start': 1, 'end': 10, 'origin': 'seed', 'score': 0.08, 'decision': 'seed_quota'}]}),
]}

SCENARIO = {'prompt': 'p1', 'expected_intent': 'bug_fix',
            'expected_citations': [{'file': 'answer.go', 'start_line': 1, 'end_line': 10}], '_path': 's/pr-1.yaml'}


class Analyze(unittest.TestCase):
    def setUp(self):
        self.row = x.analyze(RECORD, SCENARIO)

    def test_ranks_and_loss_point(self):
        self.assertEqual(self.row['ranks'], {'ckv_round1': 1, 'stage1': 1, 'fused': 2, 'stage2': 2,
                                             'stage3': 2, 'stage4': 0})
        self.assertEqual(self.row['lost_rank1_at'], 'stage1->fused')
        self.assertEqual(self.row['lost_found_at'], 'stage3->stage4')

    def test_flags(self):
        for flag in ('intent_wrong', 'intent_threshold', 'list_duplicate_credit', 'duplicate_credit_top1',
                     'symbol_fanout', 'keyword_only_top1', 'hit_keyword_promoted_top1', 'score_outlier_top1',
                     'answer_cut_stage4'):
            with self.subTest(flag=flag):
                self.assertIn(flag, self.row['flags'])
        self.assertNotIn('answer_demoted', self.row['flags'])
        self.assertEqual(self.row['max_list_credits'], 3)
        self.assertEqual(self.row['answer_stage4_decision'], 'seed_quota')
        self.assertEqual(self.row['expected_intent_rank'], 1)

    def test_build_report_matches_prompt_and_keeps_first_call(self):
        with tempfile.TemporaryDirectory() as d:
            d = Path(d)
            (d / 'sc').mkdir()
            (d / 'sc' / 'pr-1.yaml').write_text(json.dumps({k: v for k, v in SCENARIO.items() if k != '_path'}))
            trace = d / 'trace.jsonl'
            other = {**RECORD, 'seq': 3, 'prompt': 'unknown prompt'}
            trace.write_text('\n'.join(json.dumps(r) for r in (RECORD, {**RECORD, 'seq': 2}, other)) + '\n')
            rep = x.build_report([trace], [d / 'sc'])
            self.assertEqual((rep['summary']['scenarios'], rep['unmatched_records']), (1, 1))
            self.assertEqual(rep['scenarios'][0]['seq'], 1)
            self.assertEqual(rep['summary']['lost_rank1_at'], {'stage1->fused': 1})
            self.assertEqual(len(x.build_report([trace], [d / 'sc'], all_calls=True)['scenarios']), 2)


if __name__ == '__main__':
    unittest.main()
