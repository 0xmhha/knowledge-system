"""Real captured factory traces must retain timer, indexer, cleanup and failure limits."""
import gzip
import json
import tempfile
import unittest
from pathlib import Path
import full_host_factory as f


@unittest.skipUnless((f.ROOT/f.PACKET/'manifest.json').is_file(),'actual capture not prepared')
class Captured(unittest.TestCase):
 def mutant(self,changes):
  with tempfile.TemporaryDirectory()as tmp:
   p=Path(tmp)/'packet';p.mkdir()
   for source in (f.ROOT/f.PACKET).rglob('*'):
    if source.is_file()and source.name!='manifest.json':
     target=p/source.relative_to(f.ROOT/f.PACKET);target.parent.mkdir(parents=True,exist_ok=True);target.symlink_to(source)
   for name,change in changes.items():
    target=p/name;data=f.read(target);change(data);target.unlink();encoded=(json.dumps(data,ensure_ascii=False,indent=2)+'\n').encode()
    target.write_bytes(gzip.compress(encoded,mtime=0)if target.suffix=='.gz'else encoded)
   f.seal(p);self.assertTrue(f.check(packet=p))
 def trace_mutant(self,change):
  with tempfile.TemporaryDirectory()as tmp:
   p=Path(tmp)/'packet';p.mkdir()
   for source in (f.ROOT/f.PACKET).rglob('*'):
    if source.is_file()and source.name!='manifest.json':
     target=p/source.relative_to(f.ROOT/f.PACKET);target.parent.mkdir(parents=True,exist_ok=True);target.symlink_to(source)
   process=p/'actual/slots/general/process';target=process/'stdout.gz';rows=f.events(f.raw(target));change(rows);target.unlink();encoded=b''.join((json.dumps(r)+'\n').encode()for r in rows);target.write_bytes(gzip.compress(encoded,mtime=0))
   target=process/'receipt.json.gz';receipt=f.read(target);receipt['stdout_sha256']=f.sha(encoded);target.unlink();target.write_bytes(gzip.compress(json.dumps(receipt).encode(),mtime=0))
   f.seal(p);self.assertTrue(f.check(packet=p))
 def test_actual_source_factory_timer_rpc_cleanup_and_all_attempts(self):
  self.assertEqual(f.check(),[]);s=f.derive();self.assertEqual(s['actual_owned_node_attempts'],8);self.assertFalse(s['full5_schedule12_contract_matrix_complete'])
 def test_resealed_formal_promotion_rejected(self):
  self.mutant({'SUMMARY.json':lambda d:d.update(formal_completed=9,independent_approved=1)})
 def test_resealed_shortened_timer_rejected_after_hash_repair(self):
  self.trace_mutant(lambda rows:next(r for r in rows if r['event']=='native_timer_notification').update(elapsed_ns=2_000_000_000))
 def test_resealed_incomplete_indexer_rejected_after_hash_repair(self):
  self.trace_mutant(lambda rows:next(r for r in rows if r['event']=='active_tx_index_ready')['value'].update(Remaining=1))
 def test_resealed_close_error_rejected_after_hash_repair(self):
  self.trace_mutant(lambda rows:next(r for r in rows if r['event']=='node_close_returned')['value'].update(error='close failed'))
 def test_resealed_existing_source_modification_rejected(self):
  self.mutant({'actual/source-binding.json.gz':lambda d:d.update(modified_existing_project_files=['eth/downloader/api.go'])})


if __name__=='__main__':unittest.main()
