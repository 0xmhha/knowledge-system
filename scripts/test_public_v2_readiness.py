"""Reject incomplete public interface evidence and acceptance promotion."""
import copy
import tempfile
import unittest
from pathlib import Path
import public_v2_readiness as p
import viewer_readiness as v

class PublicV2Evidence(unittest.TestCase):
    def test_native_prior_ids_must_be_preserved(self):
        rows=[dict(Package='owned',Test=str(i),Action='pass')for i in range(4231)]+[dict(Package='owned',Test='skip'+str(i),Action='skip')for i in range(18)]
        self.assertEqual(p.native_population(rows,rows)['pass'],4231)
        changed=copy.deepcopy(rows);changed[0]['Test']='different'
        with self.assertRaisesRegex(ValueError,'IDs omitted'):p.native_population(changed,rows)

    def test_formal_or_paid_counts_cannot_be_promoted(self):
        base=dict(formal_completed=0,independent_approved=0,actual_LLM_calls=0,paid_external_calls=0,direct_CKV_v2_implemented=False,static_chunked_v2_implemented=False)
        p.boundary(base)
        for name in ('formal_completed','independent_approved','actual_LLM_calls','paid_external_calls'):
            with self.assertRaisesRegex(ValueError,'promoted'):p.boundary({**base,name:1})

    def test_pending_engine_interfaces_cannot_be_closed(self):
        base=dict(formal_completed=0,independent_approved=0,actual_LLM_calls=0,paid_external_calls=0,direct_CKV_v2_implemented=False,static_chunked_v2_implemented=False)
        for name in ('direct_CKV_v2_implemented','static_chunked_v2_implemented'):
            with self.assertRaisesRegex(ValueError,'pending'):p.boundary({**base,name:True})

    def test_changed_or_unlisted_evidence_is_rejected(self):
        with tempfile.TemporaryDirectory()as d:
            root=Path(d);v.write(root/'row.json',{'status':'partial'});v.write(root/'manifest.json',{'artifacts':v.inventory(root),'tools':{}})
            v.write(root/'row.json',{'status':'complete'})
            self.assertIn('artifact population/bytes changed',p.validate(root,root)[0])

    def test_missing_evidence_is_rejected(self):
        with tempfile.TemporaryDirectory()as d:
            root=Path(d);v.write(root/'manifest.json',{'artifacts':{'missing':'0'*64},'tools':{}})
            self.assertIn('artifact population/bytes changed',p.validate(root,root)[0])

    def test_supplied_protocol_cannot_promote_AI_gold(self):
        protocol=v.read(p.ROOT/v.PATHS[0]);ledger=v.read(p.ROOT/v.PATHS[1])
        protocol['wi1_dev_gold_preparation']['status']='independent_approved'
        self.assertIn('prior decision/gold changed',p.current_indices(p.ROOT,protocol,ledger))

    def test_supplied_ledger_cannot_promote_AI_gold(self):
        protocol=v.read(p.ROOT/v.PATHS[0]);ledger=v.read(p.ROOT/v.PATHS[1])
        ledger['wi2_dev_gold_preparation']['status']='independent_approved'
        self.assertIn('prior decision/gold changed',p.current_indices(p.ROOT,protocol,ledger))

    def test_current_sealed_packet(self):
        self.assertEqual(p.validate(),[])

if __name__=='__main__':unittest.main()
