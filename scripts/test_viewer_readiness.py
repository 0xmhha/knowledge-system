"""Reject missing native populations and promoted viewer evidence."""
import copy
import tempfile
import unittest
from unittest.mock import patch
from pathlib import Path
import viewer_readiness as v


class ViewerEvidence(unittest.TestCase):
    def setUp(self):
        self.previous=[dict(Package='owned',Test='pass'+str(i),Action='pass')for i in range(4214)]
        self.previous += [dict(Package='owned',Test='skip'+str(i),Action='skip')for i in range(18)]

    def test_prior_population_and_additional_passes(self):
        rows=self.previous+[dict(Package='owned',Test='new',Action='pass')]
        self.assertEqual(v.test_population(rows,self.previous),{'pass':4215,'fail':0,'skip':18})

    def test_replaced_old_pass_is_not_equivalent_population(self):
        rows=copy.deepcopy(self.previous);rows[0]['Test']='replacement'
        with self.assertRaisesRegex(ValueError,'IDs omitted'):v.test_population(rows,self.previous)

    def test_skip_cannot_replace_pass(self):
        rows=copy.deepcopy(self.previous);rows[0]['Action']='skip'
        with self.assertRaises(ValueError):v.test_population(rows,self.previous)

    def test_explicit_failure_cannot_be_hidden_by_passes(self):
        # Whole command exit is checked separately by derive; test rows also
        # retain every explicit failure rather than using a net pass count.
        rows=self.previous+[dict(Package='owned',Test='bad',Action='fail')]
        with self.assertRaises(ValueError):v.test_population(rows,self.previous)

    def test_package_failure_cannot_be_hidden_by_test_passes(self):
        with self.assertRaisesRegex(ValueError,'package failed'):
            v.test_population(self.previous+[dict(Package='owned',Action='fail')],self.previous)

    def test_changed_artifact_refused_before_source_or_summary(self):
        with tempfile.TemporaryDirectory()as d:
            p=Path(d);v.write(p/'evidence.json',{'formal_completed':0});v.write(p/'manifest.json',{'artifacts':v.inventory(p),'tools':{}})
            v.write(p/'evidence.json',{'formal_completed':9})
            self.assertIn('evidence bytes changed',v.validate(p,p)[0])

    def test_unlisted_artifact_is_not_ignored(self):
        with tempfile.TemporaryDirectory()as d:
            p=Path(d);v.write(p/'manifest.json',{'artifacts':{},'tools':{}});(p/'extra').write_text('unlisted')
            self.assertIn('evidence bytes changed',v.validate(p,p)[0])

    def test_missing_artifact_is_not_ignored(self):
        with tempfile.TemporaryDirectory()as d:
            p=Path(d);v.write(p/'manifest.json',{'artifacts':{'missing':'0'*64},'tools':{}})
            self.assertIn('evidence bytes changed',v.validate(p,p)[0])

    def test_empty_cleanup_snapshot_cannot_prove_reaped_commands(self):
        with self.assertRaisesRegex(ValueError,'population'):
            v.process_groups([dict(label='build',pid=42)],[])

    def test_live_group_cannot_be_healthy_cleanup(self):
        with self.assertRaisesRegex(ValueError,'cleanup'):
            v.process_groups([dict(label='build',pid=42)],[dict(label='build',pgid=42,remaining_live_group_members=1)])

    def test_stub_profile_is_not_real_UI_success(self):
        original=v.read
        def promoted(p):
            d=original(p)
            if p.name=='export-result.json.gz':d['UI_success_claimed']=True
            return d
        with patch.object(v,'read',side_effect=promoted):
            with self.assertRaisesRegex(ValueError,'promoted'):
                v.default_profile(v.ROOT/v.PACKET,v.read(v.ROOT/v.PACKET/'source-binding.json'))

    def test_current_complete_packet(self):
        if not(v.ROOT/v.PACKET/'manifest.json').exists():self.skipTest('packet is not sealed yet')
        self.assertEqual(v.validate(),[])


if __name__=='__main__':unittest.main()
