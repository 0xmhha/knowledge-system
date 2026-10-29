"""Classify answer losses and their causes from synthetic trace rows and probes."""
import sqlite3
import tempfile
import unittest
from pathlib import Path

import retrieval_found_diagnosis as x

ANSWER = {'file': 'a.go', 'start_line': 10, 'end_line': 20}


def record(selected_origins=('seed', 'neighbor'), hits=('b.go', 'a.go')):
    return {'seq': 1, 'events': [
        {'stage': 'stage1', 'step': 'ckv_search', 'phase': 'after', 'key': '1',
         'data': {'hits': [{'file': f, 'start': 1, 'end': 5} for f in hits]}},
        {'stage': 'stage4', 'step': 'allocate', 'phase': 'after', 'key': '',
         'data': {'decisions': [{'origin': o, 'decision': 'selected'} for o in selected_origins] +
                  [{'origin': 'seed', 'decision': 'seed_quota'}]}},
    ]}


class Classify(unittest.TestCase):
    def test_loss_class(self):
        cases = [({'stage4': 2}, 'found'), ({'stage4': 0, 'stage3': 9}, 'cut_stage4'),
                 ({'stage2': 0, 'fused': 4}, 'cut_stage2'), ({'ckv_round1': 0, 'fused': 0}, 'never_retrieved')]
        for ranks, want in cases:
            with self.subTest(ranks=ranks):
                self.assertEqual(x.loss_class(ranks), want)

    def test_cause(self):
        self.assertEqual(x.cause(False, 3, 20), 'not_indexed')
        self.assertEqual(x.cause(True, 0, 20), 'vector_absent')
        self.assertEqual(x.cause(True, 20, 20), 'dropped_stage1')
        self.assertEqual(x.cause(True, 21, 20), 'vector_deep')

    def test_rank_bucket(self):
        self.assertEqual([x.rank_bucket(r) for r in (0, 1, 2, 21, 501)], ['absent', '<=1', '<=3', '<=50', '>500'])

    def test_first_answer_rank_uses_line_overlap(self):
        cites = [{'file': 'a.go', 'start_line': 30, 'end_line': 40}, {'file': 'a.go', 'start_line': 15, 'end_line': 16}]
        self.assertEqual(x.first_answer_rank([ANSWER], cites), 2)
        self.assertEqual(x.first_answer_rank([ANSWER], cites[:1]), 0)


class Diagnose(unittest.TestCase):
    def test_never_retrieved_with_probe(self):
        row = {'name': 'pr-1', 'trace': '/w/eval/m/snap1/debug-trace.jsonl', 'seq': 1, 'intent': '',
               'ranks': {'ckv_round1': 0, 'stage1': 0, 'fused': 0, 'stage2': 0, 'stage3': 0, 'stage4': 0}}
        sc = {'prompt': 'p', 'expected_intent': 'bug_fix', 'expected_citations': [ANSWER]}
        probe = {'index': [{'chunks': 1}], 'vector_rank': 57}
        d = x.diagnose(row, sc, record(), probe, 20)
        self.assertEqual((d['loss'], d['cause'], d['vector_rank_bucket'], d['snapshot']),
                         ('never_retrieved', 'vector_deep', '<=100', 'snap1'))
        self.assertEqual(d['answer_files_in_stage1'], ['a.go'])
        self.assertEqual(d['selected_by_origin'], {'seed': 1, 'neighbor': 1})
        s = x.summarize([d])
        self.assertEqual(s['never_retrieved_cause']['vector_deep'], 1)
        self.assertEqual(s['never_retrieved_answer_file_in_stage1'], 1)

    def test_index_coverage(self):
        with tempfile.TemporaryDirectory() as d:
            db = Path(d) / 'vector.db'
            con = sqlite3.connect(db)
            con.execute('CREATE TABLE chunks (file TEXT, start_line INT, end_line INT, chunk_kind TEXT, is_test INT, '
                        'symbol_name TEXT)')
            con.executemany('INSERT INTO chunks VALUES (?,?,?,?,?,?)',
                            [('a.go', 5, 12, 'symbol', 0, 'F'), ('a.go', 21, 30, 'symbol', 0, 'G')])
            con.commit()
            con.close()
            got = x.index_coverage(db, [ANSWER, {'file': 'b.go', 'start_line': 1, 'end_line': 2}])
            self.assertEqual([(c['chunks'], c['symbols']) for c in got], [(1, ['F']), (0, [])])


if __name__ == '__main__':
    unittest.main()
