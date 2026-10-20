import copy,unittest
from unittest.mock import patch
import scale_long_functions as s
class ScaleEvidence(unittest.TestCase):
    def test_rank_uses_actual_small_sample(self):self.assertEqual(s.nearest([5,1,4,3,2],.95),5)
    def test_capacity_cannot_be_approved_by_timings(self):
        d=dict(formal_completed=0,independent_approved=0,actual_LLM_calls=0,paid_external_calls=0,capacity_SLA_accepted=True)
        with self.assertRaises(ValueError):s.semantic_boundary(d)
    def changed(self,name,change):
        original=s.v.read
        def read(p):
            d=original(p)
            if p.name==name:d=copy.deepcopy(d);change(d)
            return d
        return patch.object(s.v,'read',side_effect=read)
    def test_actual_packet(self):self.assertEqual(s.check(),[])
    def test_health_probes_are_counted_without_startup_isolation_claim(self):
        d=s.derive();self.assertEqual((d['local_embedding_HTTP_requests'],d['local_embedding_input_items']),(4,68));self.assertFalse(d['initialize_HTTP_isolated'])
    def test_missing_query_rejected(self):
        with self.changed('calls.json.gz',lambda d:d.pop()):
            with self.assertRaisesRegex(ValueError,'population'):s.derive()
    def test_force_kill_cannot_be_healthy(self):
        with self.changed('session.json.gz',lambda d:d.update(kill_sent=True)):
            with self.assertRaisesRegex(ValueError,'reaped'):s.derive()
    def test_gold_approval_cannot_be_invented(self):
        with self.changed('receipt.json.gz',lambda d:d.update(independent_approved=1)):
            with self.assertRaisesRegex(ValueError,'promoted'):s.derive()
if __name__=='__main__':unittest.main()
