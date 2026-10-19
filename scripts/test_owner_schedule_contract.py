import copy,gzip,json,unittest
import owner_schedule_contract as c

class Contract(unittest.TestCase):
 def setUp(self):
  self.data=json.loads((c.ROOT/c.PACKET/'PROPOSAL.json').read_text());self.source=gzip.decompress((c.ROOT/c.PACKET/'original-api.go.gz').read_bytes())
 def test_pinned_source_proposal_is_not_approval(self):self.assertEqual(c.check(),[])
 def test_send_between_delete_and_ack_invalidates_source_inference(self):
  changed=self.source.replace(b'delete(syncSubscriptions, u.c)',b'delete(syncSubscriptions, u.c)\n u.c <- false')
  with self.assertRaises(ValueError):c.facts(changed)
 def test_ack_before_delete_invalidates_source_inference(self):
  changed=self.source.replace(b'delete(syncSubscriptions, u.c)\n\t\t\tclose(u.uninstalled)',b'close(u.uninstalled)\n delete(syncSubscriptions, u.c)')
  with self.assertRaises(ValueError):c.facts(changed)
 def test_new_channel_registration_restriction_cannot_inherit_reuse_inference(self):
  changed=self.source.replace(b'api.installSyncSubscription <- status',b'if status == nil { return nil }; api.installSyncSubscription <- status')
  with self.assertRaises(ValueError):c.facts(changed)
 def test_source_review_cannot_self_approve_or_complete(self):
  self.data['human_scope_decision']='approved-by-AI';self.data['full_original5_complete']=True;self.assertTrue(c.check(self.data,self.source))

if __name__=='__main__':unittest.main()
