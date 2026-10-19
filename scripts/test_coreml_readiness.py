"""Reject false model acceptance and changed local profile evidence."""
import copy
import gzip
import tempfile
import unittest
from pathlib import Path
import coreml_readiness as p
import viewer_readiness as v

class CoreMLEvidence(unittest.TestCase):
    def base(self):return dict(actual_CoreML_model_inference_calls=0,actual_LLM_calls=0,paid_external_calls=0,formal_completed=0,independent_approved=0,real_CoreML_model_accepted=False,Qwen_CoreML_supported=False,managed_CKS_CoreML_runtime_implemented=False)
    def test_local_fixtures_cannot_be_real_model_or_formal_acceptance(self):
        d=self.base();p.boundary(d)
        for name in ('actual_CoreML_model_inference_calls','actual_LLM_calls','paid_external_calls','formal_completed','independent_approved'):
            with self.assertRaisesRegex(ValueError,'promoted'):p.boundary({**d,name:1})
    def test_unimplemented_Qwen_and_managed_runtime_cannot_be_accepted(self):
        d=self.base()
        for name in ('real_CoreML_model_accepted','Qwen_CoreML_supported','managed_CKS_CoreML_runtime_implemented'):
            with self.assertRaisesRegex(ValueError,'unimplemented'):p.boundary({**d,name:True})
    def test_prior_native_population_cannot_be_replaced_by_new_names(self):
        rows=[dict(Package='owned',Test=str(i),Action='pass')for i in range(4265)]+[dict(Package='owned',Test='skip'+str(i),Action='skip')for i in range(18)]
        changed=copy.deepcopy(rows);changed[0]['Test']='different'
        with self.assertRaisesRegex(ValueError,'IDs omitted'):p.previous.native_population(changed,rows)
    def test_unlisted_artifact_is_rejected(self):
        with tempfile.TemporaryDirectory()as d:
            root=Path(d);v.write(root/'manifest.json',{'artifacts':{},'tools':{}});v.write(root/'extra.json',{'accepted':True})
            self.assertIn('artifact population/bytes changed',p.validate(root,root)[0])
    def test_command_log_changes_are_rejected(self):
        with tempfile.TemporaryDirectory()as d:
            root=Path(d);streams={'stdout':b'original','stderr':b''}
            receipt=dict(exit_code=0,reaped=True,timeout=False,term_sent=False,kill_sent=False,remaining_live_group_members=0)
            for name,raw in streams.items():(root/(name+'.gz')).write_bytes(gzip.compress(raw,mtime=0));receipt[name+'_sha256']=v.sha(raw)
            (root/'receipt.json.gz').write_bytes(gzip.compress(__import__('json').dumps(receipt).encode(),mtime=0));p.command(root)
            (root/'stdout.gz').write_bytes(gzip.compress(b'changed',mtime=0))
            with self.assertRaisesRegex(ValueError,'stream differs'):p.command(root)
    def test_supplied_protocol_cannot_approve_AI_gold(self):
        protocol=v.read(p.ROOT/v.PATHS[0]);ledger=v.read(p.ROOT/v.PATHS[1]);protocol['wi1_dev_gold_preparation']['status']='independent_approved'
        self.assertIn('prior decision/gold changed',p.current_indices(p.ROOT,protocol,ledger))
    def test_supplied_ledger_cannot_approve_AI_gold(self):
        protocol=v.read(p.ROOT/v.PATHS[0]);ledger=v.read(p.ROOT/v.PATHS[1]);ledger['wi2_dev_gold_preparation']['status']='independent_approved'
        self.assertIn('prior decision/gold changed',p.current_indices(p.ROOT,protocol,ledger))
    def test_current_sealed_packet(self):self.assertEqual(p.validate(),[])

if __name__=='__main__':unittest.main()
