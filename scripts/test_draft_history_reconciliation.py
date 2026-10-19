import copy,unittest
from unittest.mock import patch
import draft_history_reconciliation as h
class DraftReplay(unittest.TestCase):
    def changed(self,name,change):
        original=h.v.read
        def read(p):
            d=original(p)
            if p.name==name:d=copy.deepcopy(d);change(d)
            return d
        return patch.object(h.v,'read',side_effect=read)
    def test_real_historical_replays(self):
        r=h.derive();self.assertEqual(r['historical_checkers_passed'],7);self.assertFalse(r['current_source_acceptance'])
    def test_missing_scope_is_not_complete(self):
        with self.changed('historical-receipts.json.gz',lambda d:d.pop()):
            with self.assertRaisesRegex(ValueError,'population'):h.derive()
    def test_failed_replay_cannot_pass(self):
        with self.changed('historical-receipts.json.gz',lambda d:d[0].update(exit_code=1)):
            with self.assertRaisesRegex(ValueError,'replay failed'):h.derive()
    def test_stale_live_failure_cannot_be_hidden(self):
        with self.changed('receipts.json.gz',lambda d:d[0].update(exit_code=0)):
            with self.assertRaisesRegex(ValueError,'failure hidden'):h.derive()
    def test_full_sealed_packet(self):self.assertEqual(h.check(),[])
if __name__=='__main__':unittest.main()
