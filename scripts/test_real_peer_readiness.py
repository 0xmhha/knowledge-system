import json,tempfile,unittest
from pathlib import Path
import real_peer_readiness as r

@unittest.skipUnless((r.ROOT/r.PACKET/'manifest.json').exists(),'real peer evidence not sealed')
class Actual(unittest.TestCase):
 def mutant(self,name,change):
  with tempfile.TemporaryDirectory()as tmp:
   p=Path(tmp)
   for x in(r.ROOT/r.PACKET).rglob('*'):
    if x.is_file()and x.name!='manifest.json':
     f=p/x.relative_to(r.ROOT/r.PACKET);f.parent.mkdir(parents=True,exist_ok=True);f.symlink_to(x)
   f=p/name;v=r.read(f);change(v);f.unlink();r.compress(f,json.dumps(v).encode());r.seal(p);self.assertTrue(r.check(packet=p))
 def test_real_source_events_indexer_native_timer_and_cleanup(self):self.assertEqual(r.check(),[]);self.assertEqual(r.inspect(r.ROOT/r.PACKET)['actual_nodes'],4)
 def test_recorded_source_scope_cannot_claim_full_acceptance(self):self.mutant('evidence/SCOPE.json.gz',lambda d:d.update(full5_schedules12_contracts_complete=True))
 def test_owned_tmp_data_must_be_reaped(self):self.mutant('evidence/slots/general/receipt.json.gz',lambda d:d.update(tmp_children_after=['node-data']))
 def test_nonzero_program_cannot_be_called_success(self):self.mutant('evidence/slots/race/receipt.json.gz',lambda d:d.update(exit_code=1))
 def test_independent_family_count_cannot_be_invented(self):
  with tempfile.TemporaryDirectory()as tmp:
   p=Path(tmp)
   for x in(r.ROOT/r.PACKET).rglob('*'):
    if x.is_file()and x.name!='manifest.json':
     f=p/x.relative_to(r.ROOT/r.PACKET);f.parent.mkdir(parents=True,exist_ok=True);f.symlink_to(x)
   f=p/'SUMMARY.json';d=r.read(f);d['independent_families']=2;f.unlink();r.write(f,d);r.seal(p);self.assertTrue(r.check(packet=p))
if __name__=='__main__':unittest.main()
