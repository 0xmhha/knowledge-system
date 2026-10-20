"""Warm denominators, p95 definition and prevention of false completion."""
import json
import tempfile
import unittest
from pathlib import Path

import wi6_warm_results as g


class Metrics(unittest.TestCase):
    def test_nearest_rank_p95_not_interpolated(self):
        self.assertEqual(g.p95(list(range(1, 31))), 29)
        self.assertEqual(g.p95([4]), 4)

    def test_invalid_duration_rejected(self):
        for values in ([], [0], [-1], [True], [1.5]):
            with self.assertRaises(ValueError):
                g.p95(values)

    def test_partial_and_failure_denominator_preserved(self):
        rows = [dict(phase='measured', complete_pair=False, arms={
            'baseline': dict(elapsed_ns=i+1, status='partial'),
            'expansion': dict(elapsed_ns=100-i, status='budget_exhausted' if i % 2 else 'protocol_error')}) for i in range(30)]
        rows.append(dict(phase='warmup', complete_pair=True, arms={}))
        result = g.latency(rows)
        self.assertEqual(result['measured_pairs'], 30)
        self.assertEqual(result['noncomplete_pairs'], 30)
        self.assertEqual(result['complete_pairs'], 0)
        self.assertEqual(result['complete_pair_latency_gate'], 'inconclusive')
        self.assertEqual(result['diagnostic_ratio_of_p95s'], 99/29)
        self.assertEqual(set(result['diagnostic_p95_by_status_ns']['expansion']), {'budget_exhausted', 'protocol_error'})

    def test_complete_samples_cannot_self_approve(self):
        row = dict(phase='measured', complete_pair=True, arms={a: dict(elapsed_ns=1, status='complete') for a in ('baseline', 'expansion')})
        self.assertEqual(g.latency([row]*30)['complete_pair_latency_gate'], 'inconclusive')

    def test_schedule_reset_per_phase_with_exact_denominators(self):
        rows = g.expected_schedule([dict(case_id=str(i)) for i in range(19)])
        self.assertEqual(len(rows), 627)
        self.assertEqual(sum(r['phase'] == 'warmup' for r in rows), 57)
        self.assertEqual(sum(r['phase'] == 'measured' for r in rows), 570)
        self.assertEqual(rows[2]['order'], rows[3]['order'])
        self.assertEqual(rows[4]['order'], ['expansion', 'baseline'])
        self.assertEqual(rows[33]['case_id'], '1')


@unittest.skipUnless((g.ROOT/g.PACKET/'manifest.json').is_file(), 'capture not yet prepared')
class Capture(unittest.TestCase):
    def mutant(self, changes):
        with tempfile.TemporaryDirectory() as temporary:
            p = Path(temporary)/'packet'
            p.mkdir()
            for item in (g.ROOT/g.PACKET).rglob('*'):
                if item.is_file() and item.name != 'manifest.json':
                    target = p/item.relative_to(g.ROOT/g.PACKET)
                    target.parent.mkdir(parents=True, exist_ok=True)
                    target.symlink_to(item)
            for name, change in changes.items():
                target = p/name
                if name.endswith('.jsonl.gz'):
                    value = list(g.lines(target))
                    change(value)
                    raw = ('\n'.join(json.dumps(r) for r in value)+'\n').encode()
                    target.unlink()
                    g.compressed(target, raw)
                else:
                    value = g.read(target)
                    change(value)
                    target.unlink()
                    g.write(target, value)
            g.seal(p)
            with self.assertRaises(ValueError):
                g.validate(p)

    def test_actual_replay_preserves_partial_denominator(self):
        summary = g.validate()['summary']
        self.assertEqual(summary['recorded_pairs'], 627)
        self.assertEqual(summary['tool_calls'], 1273)
        self.assertEqual(summary['formal_completed'], 0)
        self.assertIsNone(summary['quality_score'])
        self.assertEqual(summary['latency']['complete_pair_latency_gate'], 'inconclusive')

    def test_resealed_false_acceptance_rejected(self):
        # Artifact inventory alone is insufficient: replay must reject promotion.
        self.mutant({'SUMMARY.json': lambda v: v.update(formal_completed=1)})

    def test_resealed_pin_change_in_both_receipt_and_wire_rejected(self):
        def change_call(rows):
            rows[1]['request']['params']['arguments']['expected_snapshot_sha256'] = '0'*64
        def change_wire(rows):
            rows[3]['params']['arguments']['expected_snapshot_sha256'] = '0'*64
        self.mutant({'actual/calls.jsonl.gz': change_call, 'actual/stdin.jsonl.gz': change_wire})

    def test_resealed_removed_planned_partial_pair_rejected(self):
        self.mutant({'actual/pairs.jsonl.gz': lambda rows: rows.pop()})


if __name__ == '__main__':
    unittest.main()
