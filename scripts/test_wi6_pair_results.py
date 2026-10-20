"""Actual paired pilot replay and false-completion controls."""
import gzip
import json
import shutil
import tempfile
import unittest
from pathlib import Path

import wi6_pair_results as g


@unittest.skipUnless((g.ROOT/g.PACKET/'manifest.json').is_file(),'actual capture not prepared')
class CapturedPairs(unittest.TestCase):
    def mutant(self,name,change):
        with tempfile.TemporaryDirectory()as tmp:
            p=Path(tmp)/'packet';shutil.copytree(g.ROOT/g.PACKET,p);q=p/name
            value=g.read(q);change(value);data=json.dumps(value).encode()
            q.write_bytes(gzip.compress(data,mtime=0)if q.suffix=='.gz'else data)
            g.seal(p)
            with self.assertRaises(ValueError):g.validate(p)

    def test_actual_scope_is_pilot_not_warm_or_quality_acceptance(self):
        data=g.validate()['summary']
        self.assertEqual(data['corrective_pairs'],19);self.assertEqual(data['total_tool_calls'],76)
        self.assertEqual(data['warm_samples'],0);self.assertEqual(data['formal_completed'],0)
        self.assertIsNone(data['quality_scores']);self.assertEqual(data['initial_failed_starts'],1)

    def test_resealed_warm_samples_cannot_be_invented(self):
        self.mutant('SUMMARY.json',lambda v:v.update(warm_samples=30))

    def test_resealed_false_worklist_completion_rejected(self):
        self.mutant('WORKLIST.json',lambda v:v.update(formal_completed=1))

    def test_resealed_missing_planned_call_rejected(self):
        self.mutant('actual/calls.json.gz',lambda v:v.pop())

    def test_resealed_wrong_snapshot_rejected(self):
        self.mutant('actual/pairs.json.gz',lambda v:v[0]['pins'].update(snapshot_sha256='0'*64))

    def test_initial_failure_cannot_disappear_into_success(self):
        self.mutant('initial/session.json.gz',lambda v:v.update(exit_code=0))

    def test_resealed_changed_actual_input_scope_rejected(self):
        self.mutant('CORRECTIVE-SCOPE.json',lambda v:v.update(planned_pairs=18))

    def test_resealed_partial_response_cannot_be_promoted(self):
        self.mutant('observations.json',lambda v:v[0]['arms']['baseline'].update(complete=True))


if __name__=='__main__':unittest.main()
