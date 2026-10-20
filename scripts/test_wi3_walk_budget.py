"""Reject omitted attempts, false aliases/counts, obsolete runs and early storage."""
import gzip,json,shutil,tempfile,unittest
from pathlib import Path
import wi3_walk_budget as g


class WalkEvidence(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup)
        self.packet=Path(self.temp.name)/'packet';shutil.copytree(g.ROOT/g.PACKET,self.packet)

    def edit(self,name,fn,compressed=False):
        p=self.packet/name;raw=gzip.decompress(p.read_bytes())if compressed else p.read_bytes();d=json.loads(raw);fn(d);body=json.dumps(d).encode();p.write_bytes(gzip.compress(body,mtime=0)if compressed else body)

    def test_current_runtime_scope_and_formal_separation(self):
        d=g.derive(packet=self.packet);s=d['summary']
        self.assertEqual((s['old_queries'],s['new_queries'],s['total_actual_queries']),(62,62,124))
        self.assertEqual(s['EOF_exit0_sessions'],4);self.assertEqual(s['controlled_strict_parity'],4)
        self.assertEqual(s['targeted_pass'],402);self.assertEqual(s['new_request_measurement']['request_summaries'],31)
        for key in ('independent_approved','FINAL','formal_completed','new_independent_samples'):self.assertEqual(s[key],0)
        self.assertIsNone(s['quality_score']);self.assertFalse(s['total_backend_budget_extended']);self.assertFalse(s['execution_allowed'])

    def test_only_elapsed_is_ignored(self):
        v=dict(budget=dict(elapsed_ms=7,calls=9,max_calls=32),candidates=[dict(symbol=None)])
        n=g.normalized(v);self.assertEqual(n['budget'],dict(elapsed_ms=0,calls=9,max_calls=32));self.assertEqual(v['budget']['elapsed_ms'],7)

    def test_original_input_dropped_rejected(self):
        self.edit('after-runtime/planned.json',lambda d:d.pop())
        with self.assertRaises(ValueError):g.runtime_data(g.ROOT,self.packet)

    def test_actual_reply_dropped_rejected(self):
        self.edit('after-runtime/responses.json.gz',lambda d:d.pop(),True)
        with self.assertRaises(ValueError):g.runtime_data(g.ROOT,self.packet)

    def test_changed_pin_rejected(self):
        self.edit('after-runtime/planned.json',lambda d:d[0]['arguments'].update(commit='0'*40))
        with self.assertRaises(ValueError):g.runtime_data(g.ROOT,self.packet)

    def test_source_data_changed_rejected(self):
        for n in ('before','after'):self.edit('after-runtime/'+n+'.json.gz',lambda d:d.update(forged='0'*64),True)
        with self.assertRaises(ValueError):g.runtime_data(g.ROOT,self.packet)

    def test_nontelemetry_config_change_rejected(self):
        p=self.packet/'after-runtime/on/config.txt';p.write_text(p.read_text().replace('recall_k: 10','recall_k: 20'))
        with self.assertRaises(ValueError):g.runtime_data(g.ROOT,self.packet)

    def test_fake_request_meta_rejected(self):
        self.edit('after-runtime/responses.json.gz',lambda d:d[0].update(measurement_id='0'*32),True)
        with self.assertRaises(ValueError):g.runtime_data(g.ROOT,self.packet)

    def test_lost_measurement_scope_rejected(self):
        self.edit('after-runtime/on/measurement-summaries.json',lambda d:d.pop())
        with self.assertRaises(ValueError):g.runtime_data(g.ROOT,self.packet)

    def test_fabricated_budget_count_rejected(self):
        def edit(d):
            r=next(x for x in d if x['measurement']=='on'and x['case_id']=='symbol');r['raw_response']['result']['structuredContent']['budget']['calls']+=1
        self.edit('after-runtime/responses.json.gz',edit,True)
        with self.assertRaises(ValueError):g.runtime_data(g.ROOT,self.packet)

    def test_failed_eof_rejected(self):
        self.edit('after-runtime/sessions.json',lambda d:d[0].update(EOF_exit=1))
        with self.assertRaises(ValueError):g.runtime_data(g.ROOT,self.packet)

    def test_old_binary_cannot_be_current(self):
        self.edit('after-runtime/receipt.json',lambda d:d.update(binary_sha256='0'*64))
        with self.assertRaises(ValueError):g.runtime_data(g.ROOT,self.packet)

    def test_extra_retries_cannot_disappear(self):
        self.edit('after-runtime/receipt.json',lambda d:d.update(retries=1))
        with self.assertRaises(ValueError):g.runtime_data(g.ROOT,self.packet)

    def test_superseded_run_cannot_replace_final_native(self):
        shutil.copyfile(self.packet/'superseded-native/receipts.json',self.packet/'native/receipts.json')
        self.assertTrue(g.native_errors(g.ROOT,self.packet))

    def test_lint_failure_cannot_be_relabelled_pass(self):
        self.edit('pre-lint-native/receipts.json',lambda d:d[-1].update(exit_code=0))
        self.assertTrue(g.pre_lint_errors(self.packet))

    def test_live_owned_group_cannot_be_called_cleaned(self):
        self.edit('supersession.json',lambda d:d.update(remaining_live_group_members=1))
        self.assertTrue(g.superseded_errors(g.ROOT,self.packet))

    def test_old_source_snapshot_tamper_rejected(self):
        p=self.packet/'superseded-native/source/internal/system/suspects/compose.go.txt.gz';p.write_bytes(gzip.compress(b'not original source',mtime=0))
        self.assertTrue(g.superseded_errors(g.ROOT,self.packet))

    def test_missing_counterexample_not_hidden(self):
        p=self.packet/'targeted/before-budget-reason.jsonl.gz';p.write_bytes(gzip.compress(b'',mtime=0))
        with self.assertRaises(ValueError):g.derive(packet=self.packet)

    def test_fresh_whole_test_scope_not_narrowed(self):
        self.edit('native/receipts.json',lambda d:next(r for r in d if r['label']=='test')['argv'].pop())
        self.assertTrue(g.native_errors(g.ROOT,self.packet))

    def test_storage_needs_actual_committed_record(self):
        self.assertTrue(g.storage_errors(Path(self.temp.name)/'no-finalization'))


if __name__=='__main__':unittest.main()
