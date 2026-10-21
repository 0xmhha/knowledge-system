"""Actual factory owner observations cannot be replaced with return-only success."""
import gzip,json,tempfile,unittest
from pathlib import Path
import full_host_owner as o
import evidence_revision

@unittest.skipUnless((o.ROOT/o.PACKET/'manifest.json').is_file(),'actual capture not prepared')
class Captured(unittest.TestCase):
 def mutant(self,name,change,lines=False):
  with tempfile.TemporaryDirectory()as tmp:
   p=Path(tmp)/'packet';p.mkdir()
   for source in (o.ROOT/o.PACKET).rglob('*'):
    if source.is_file()and source.name!='manifest.json':
     target=p/source.relative_to(o.ROOT/o.PACKET);target.parent.mkdir(parents=True,exist_ok=True);target.symlink_to(source)
   target=p/name;data=o.events(o.raw(target))if lines else o.read(target);change(data);target.unlink()
   encoded=b''.join((json.dumps(r)+'\n').encode()for r in data)if lines else(json.dumps(data)+'\n').encode()
   target.write_bytes(gzip.compress(encoded,mtime=0)if target.suffix=='.gz'else encoded)
   if name.endswith('/process/stdout.gz'):
    target=target.parent/'receipt.json.gz';r=o.read(target);r['stdout_sha256']=o.sha(encoded);target.unlink();target.write_bytes(gzip.compress(json.dumps(r).encode(),mtime=0))
   o.seal(p);self.assertTrue(o.check(packet=p))
 def test_actual_owner_events_and_no_independent_promotion(self):
  self.assertEqual(o.check(),[]);s=o.derive();self.assertEqual(s['actual_factory_owner_schedules'],40);self.assertIsNone(s['independent_acceptable_candidate_set'])
 def test_resealed_independent_acceptance_rejected(self):self.mutant('SUMMARY.json',lambda d:d.update(independent_approved=1,formal_completed=9))
 def test_resealed_return_only_normal_claim_rejected(self):
  self.mutant('actual/slots/CAND-A-general/owner.jsonl.gz',lambda rows:o.only(rows,'schedule_observation',o.SCENARIOS[0])['value'].update(owner_member=True,owner_delete=0,owner_ack=0),True)
 def test_resealed_early_return_cannot_be_actual_ack_success(self):
  self.mutant('actual/slots/MUT-EARLY-general/owner.jsonl.gz',lambda rows:o.only(rows,'held_ack_observation',o.SCENARIOS[2])['value'].update(gate_entered=True,owner_member=False),True)
 def test_resealed_ack_before_delete_rejected(self):
  def swap(rows):
   c=o.only(rows,'schedule_observation',o.SCENARIOS[0])['channel_id'];a=next(r for r in rows if r['channel_id']==c and r['event']=='after_delete');b=next(r for r in rows if r['channel_id']==c and r['event']=='ack_close_begin');a['event'],b['event']=b['event'],a['event']
  self.mutant('actual/slots/ORIGINAL-general/owner.jsonl.gz',swap,True)
 def test_resealed_shortened_native_timer_with_repaired_log_hash_rejected(self):
  self.mutant('actual/slots/CAND-A-general/process/stdout.gz',lambda rows:next(r for r in rows if r['event']=='native_timer_notification').update(elapsed_ns=2_000_000_000),True)
 def test_resealed_old_commit_cannot_relabel_candidate_worktree(self):
  self.mutant('actual/candidates/CAND-A/tree-binding.json.gz',lambda d:d.update(applied_project_commit=o.factory.COMMIT))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__=='__main__':unittest.main()
