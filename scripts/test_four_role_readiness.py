"""Real current-source role delivery must retain partial, provenance and denial."""
import gzip,json,tempfile,unittest
from pathlib import Path
import four_role_readiness as r
import evidence_revision

@unittest.skipUnless((r.ROOT/r.PACKET/'manifest.json').is_file(),'current role capture not sealed')
class Captured(unittest.TestCase):
 def mutant(self,name,change):
  with tempfile.TemporaryDirectory()as tmp:
   p=Path(tmp)/'packet';p.mkdir()
   for source in(r.ROOT/r.PACKET).rglob('*'):
    if source.is_file()and source.name!='manifest.json':
     target=p/source.relative_to(r.ROOT/r.PACKET);target.parent.mkdir(parents=True,exist_ok=True);target.symlink_to(source)
   target=p/name;value=r.read(target);change(value);target.unlink();data=json.dumps(value,ensure_ascii=False).encode();target.write_bytes(gzip.compress(data,mtime=0)if target.suffix=='.gz'else data)
   r.seal(p);self.assertTrue(r.check(packet=p))
 def test_actual_role_catalogs_source_handoffs_and_refusals(self):
  self.assertEqual(r.check(),[]);s=r.derive();self.assertEqual(s['total_role_hosts'],8);self.assertEqual(s['current_completion_refusals'],4);self.assertFalse(s['full_pipeline_or_role_semantic_acceptance'])
 def test_resealed_model_or_independent_quality_promotion_rejected(self):self.mutant('SUMMARY.json',lambda d:d.update(actual_LLM_inferences=4,independent_approved=1,formal_completed=9))
 def test_resealed_partial_handoff_cannot_be_promoted(self):
  self.mutant('actual/P76-analyzer/workspace-after/analyzer-context.json.gz',lambda d:d['actual_tool_response'].update(status='analyzed_bounded'))
 def test_resealed_foreign_dataset_binding_rejected(self):
  self.mutant('actual/evidence/SCOPE.json.gz',lambda d:d['source_pins'].update(dataset_id='0'*64))
 def test_resealed_completion_state_rejected(self):self.mutant('actual/evidence/final-state.json.gz',lambda d:d.update(current_state='COMPLETED'))
 def test_resealed_external_model_flag_rejected(self):self.mutant('actual/evidence/SCOPE.json.gz',lambda d:d.update(actual_LLM_inference=True))
 def test_resealed_missing_source_handoff_cannot_hide_under_host_exit0(self):
  self.mutant('actual/P76-planner/workspace-after/planner-context.json.gz',lambda d:d.update(actual_tool_response={}))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__=='__main__':unittest.main()
