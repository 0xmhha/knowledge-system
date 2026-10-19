import json,tempfile,unittest
from pathlib import Path
import review_delivery_readiness as r

@unittest.skipUnless((r.ROOT/r.PACKET/'manifest.json').exists(),'review packet not sealed')
class Actual(unittest.TestCase):
 def mutant(self,name,change):
  with tempfile.TemporaryDirectory()as tmp:
   p=Path(tmp)
   for x in(r.ROOT/r.PACKET).rglob('*'):
    if x.is_file()and x.name!='manifest.json':
     f=p/x.relative_to(r.ROOT/r.PACKET);f.parent.mkdir(parents=True,exist_ok=True);f.symlink_to(x)
   f=p/name;value=r.read(f);change(value);f.unlink();data=json.dumps(value,ensure_ascii=False).encode()
   if f.suffix=='.gz':r.compress(f,data)
   else:f.write_bytes(data)
   r.seal(p);self.assertTrue(r.check(packet=p))
 def test_actual_roles_five_artifacts_CLI_hook_and_render(self):self.assertEqual(r.check(),[]);self.assertEqual(r.derive(r.ROOT/r.PACKET)['render']['actual_SVGs'],2)
 def test_paid_or_independent_promotion_rejected(self):self.mutant('actual/evidence/SCOPE.json.gz',lambda d:d.update(actual_LLM_calls=1,independent_approved=1))
 def test_actual_unchanged_five_artifact_handoff_required(self):self.mutant('actual/evidence/final-artifacts.json.gz',lambda d:d.update({'sequence.mmd':'0'*64}))
 def test_invalid_terminal_cannot_be_accepted(self):self.mutant('actual/evidence/final-state.json.gz',lambda d:d.update(current_state='COMPLETED'))
 def test_actual_browser_error_not_ignored(self):self.mutant('render/render-result.txt',lambda d:d.update(errors=['render failed']))
 def test_lingering_host_not_ignored(self):self.mutant('process-cleanup.json',lambda d:d[0].update(remaining_recorded_PIDs=1))
 def test_unsupported_policy_attempt_not_relabelled_success(self):self.mutant('unsupported-policy/evidence/receipt.json.gz',lambda d:d.update(artifact_files=0))
 def test_semantic_quality_not_inferred_from_render(self):self.mutant('render/VISUAL-REVIEW.json',lambda d:d.update(semantic_quality_approved=True))
if __name__=='__main__':unittest.main()
