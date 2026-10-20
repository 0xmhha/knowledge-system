"""Protect the causal distinction and the existing approval boundaries."""
import tempfile
import unittest
from pathlib import Path
import guardian_deadline as p
import viewer_readiness as v


class GuardianDiagnosis(unittest.TestCase):
    def trace(self):
        return ('GD return start=0 deadline=200 finish=240 error=exit status 1 marker_present=true stat_error=<nil>\n'
                'GD kill_before pid=1 t=201\n'
                'owned: line 1: 2 Killed: 9 sleep 1\n'
                'GD kill_after pid=1 t=220 error=<nil>\n'
                'GD marker content="late" mtime=210 existed_at_return=true mtime_after_return=false\n')

    def test_pre_return_cause_requires_observed_ordering(self):
        self.assertEqual(p.classify(self.trace())['marker_mtime'], 210)
        for old, new in [('marker_present=true', 'marker_present=false'), ('mtime=210', 'mtime=250'), ('stat_error=<nil>', 'stat_error=permission denied'), ('mtime_after_return=false', 'mtime_after_return=true')]:
            with self.subTest(new=new), self.assertRaises(ValueError):
                p.classify(self.trace().replace(old, new))

    def test_missing_historical_trace_and_independent_approval_cannot_be_invented(self):
        d = dict(CoreML_excluded=True, production_runtime_changed=False, exact_historical_event_forensically_proven=False, actual_model_calls=0, actual_LLM_calls=0, paid_external_calls=0, formal_completed=0, independent_approved=0)
        p.boundary(d)
        for key in ('exact_historical_event_forensically_proven', 'independent_approved', 'formal_completed'):
            with self.subTest(key=key), self.assertRaises(ValueError):
                p.boundary({**d, key: True})
        with self.assertRaises(ValueError):
            p.boundary({**d, 'CoreML_excluded': False})

    def test_unlisted_artifact_cannot_be_ignored(self):
        with tempfile.TemporaryDirectory() as tmp:
            q = Path(tmp)
            v.write(q/'manifest.json', dict(artifacts={}, tools={}))
            v.write(q/'extra.json', dict(complete=True))
            self.assertIn('artifact population changed', p.validate(q, q)[0])

    def test_supplied_protocol_and_ledger_cannot_approve_AI_gold(self):
        for index in (0, 1):
            docs = [v.read(p.ROOT/n) for n in v.PATHS[:2]]
            docs[index]['wi1_dev_gold_preparation']['status'] = 'independent_approved'
            self.assertIn('prior decision/gold changed', p.current_indices(p.ROOT, *docs))

    def test_current_sealed_packet(self):
        self.assertEqual(p.validate(), [])


if __name__ == '__main__':
    unittest.main()
