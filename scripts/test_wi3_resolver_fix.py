"""Reject lost calls, changed pins, hidden failures and false parity claims."""
import copy,gzip,json,shutil,tempfile,unittest
from pathlib import Path
import wi3_resolver_fix as g


class ResolverEvidence(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup)
        self.packet=Path(self.temp.name)/'packet';shutil.copytree(g.ROOT/g.PACKET,self.packet)

    def edit(self,name,fn,compressed=False):
        p=self.packet/name;raw=gzip.decompress(p.read_bytes())if compressed else p.read_bytes();d=json.loads(raw);fn(d);body=json.dumps(d).encode();p.write_bytes(gzip.compress(body,mtime=0)if compressed else body)

    def test_current_parity_and_formal_separation(self):
        d=g.derive(packet=self.packet);s=d['summary']
        self.assertEqual(s['actual_queries_total'],46);self.assertEqual(s['EOF_exit0_sessions'],6)
        self.assertEqual(len(d['comparisons']),3);self.assertEqual(s['targeted_pass'],332)
        for key in ('independent_approved','formal_completed','FINAL','new_independent_samples'):self.assertEqual(s[key],0)
        self.assertFalse(s['execution_allowed']);self.assertIsNone(s['quality_score'])

    def test_only_elapsed_is_normalized(self):
        v={'budget':{'elapsed_ms':77,'calls':9},'candidates':[{'symbol':None}]}
        n=g.normalized(v);self.assertEqual(n,dict(budget=dict(elapsed_ms=0,calls=9),candidates=[dict(symbol=None)]));self.assertEqual(v['budget']['elapsed_ms'],77)

    def test_lost_response_rejected(self):
        self.edit('after-runtime/responses.json.gz',lambda d:d.pop(),True)
        with self.assertRaises(ValueError):g.runtime_data(g.ROOT,self.packet)

    def test_pin_changed_rejected(self):
        self.edit('after-runtime/planned.json',lambda d:d[0]['arguments'].update(commit='forged'))
        with self.assertRaises(ValueError):g.runtime_data(g.ROOT,self.packet)

    def test_reply_not_on_wire_rejected(self):
        def edit(d):
            row=next(r for r in d if r['measurement']=='on'and r['case_id']=='symbol')
            row['raw_response']['result']['structuredContent']['budget']['max_calls']=64
        self.edit('after-runtime/responses.json.gz',edit,True)
        with self.assertRaises(ValueError):g.runtime_data(g.ROOT,self.packet)

    def test_changed_config_rejected(self):
        p=self.packet/'after-runtime/on/config.txt';p.write_text(p.read_text().replace('recall_k: 10','recall_k: 20'))
        with self.assertRaises(ValueError):g.runtime_data(g.ROOT,self.packet)

    def test_different_index_between_runs_rejected(self):
        self.edit('after-runtime/before.json.gz',lambda d:d.update(forged='0'*64),True)
        self.edit('after-runtime/after.json.gz',lambda d:d.update(forged='0'*64),True)
        with self.assertRaises(ValueError):g.runtime_data(g.ROOT,self.packet)

    def test_unknown_binary_rejected(self):
        self.edit('after-runtime/receipt.json',lambda d:d.update(binary_sha256='0'*64))
        with self.assertRaises(ValueError):g.runtime_data(g.ROOT,self.packet)

    def test_failed_eof_rejected(self):
        self.edit('after-runtime/sessions.json',lambda d:d[0].update(EOF_exit=1))
        with self.assertRaises(ValueError):g.runtime_data(g.ROOT,self.packet)

    def test_initial_comment_failure_not_erased(self):
        self.edit('initial-runtime/responses.json.gz',lambda d:d.pop(),True)
        with self.assertRaises(ValueError):g.runtime_data(g.ROOT,self.packet)

    def test_extra_retry_cannot_disappear(self):
        self.edit('after-runtime/receipt.json',lambda d:d.update(retries=1))
        with self.assertRaises(ValueError):g.runtime_data(g.ROOT,self.packet)

    def test_narrowed_native_test_scope_rejected(self):
        self.edit('native/receipts.json',lambda d:next(r for r in d if r['label']=='test')['argv'].pop())
        self.assertTrue(g.native_errors(g.ROOT,self.packet))

    def test_missing_native_command_rejected(self):
        self.edit('native/receipts.json',lambda d:d.pop())
        self.assertTrue(g.native_errors(g.ROOT,self.packet))

    def test_wrong_bound_source_scope_rejected(self):
        self.edit('source-binding.json',lambda d:d['changed_native_files'].pop())
        self.assertTrue(g.native_errors(g.ROOT,self.packet))


if __name__=='__main__':unittest.main()
