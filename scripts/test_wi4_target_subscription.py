"""Refuse erased observer failures, changed inputs and premature acceptance."""
import gzip,json,shutil,tempfile,unittest
from pathlib import Path
import wi4_target_subscription as g


class ActualSourceEvidence(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup);self.packet=Path(self.tmp.name)/'packet';shutil.copytree(g.ROOT/g.PACKET,self.packet)

    def edit(self,name,fn,compressed=False):
        p=self.packet/name;d=json.loads(g.raw(p));fn(d);raw=json.dumps(d).encode();p.write_bytes(gzip.compress(raw,mtime=0)if compressed else raw)

    def rows(self):return g.derive(packet=self.packet)

    def change_events(self,phase,slot,fn):
        p=self.packet/phase/'slots'/slot/'events.jsonl.gz';rows=g.events(p);fn(rows);p.write_bytes(gzip.compress(('\n'.join(json.dumps(r)for r in rows)+'\n').encode(),mtime=0))

    def test_all_attempts_routes_and_formal_separation(self):
        d=self.rows();s=d['summary'];self.assertEqual(s['actual_attempts'],48);self.assertEqual(s['initial_observer_failures_preserved'],6);self.assertEqual(s['compiler_symbol_observer_failures_preserved'],3);self.assertEqual(s['extra_no_receiver_lifecycle_variants'],3);self.assertEqual(s['extra_three_caller_stress'],6);self.assertEqual(s['original_source_condition_observations'],30);self.assertEqual((s['source_condition_real_API_routes'],s['source_condition_controlled_receiver_routes']),(12,18));self.assertEqual(s['formal_completed'],0);self.assertIsNone(s['quality_score']);self.assertFalse(s['target_production_patch']);self.assertEqual(s['full_original_driver_timing_acceptances'],0)

    def test_original_two_caller_input_not_replaced_by_three(self):
        rows=[r for r in self.rows()['observations']if r['case_id']=='SUB-once'];stress=[r for r in rows if not r['original_source_conditions_verified']];exact=[r for r in rows if r['original_source_conditions_verified']];self.assertEqual((len(stress),len(exact)),(6,6));self.assertTrue(all(not r['draft_outcome_ids']for r in stress));self.assertTrue(all(r['draft_outcome_ids']==['one_request_two_returns']for r in exact))

    def test_formal_adapter_still_refuses_unfrozen_repetition(self):
        for r in self.rows()['observations']:self.assertEqual(r['formal_adapter_result'],dict(status='unknown_unverified_trial',label=None))

    def test_drop_failed_attempt_rejected(self):
        self.edit('initial/attempts.json',lambda d:d.pop(1))
        with self.assertRaises(ValueError):self.rows()

    def test_drop_original_slot_rejected(self):
        self.edit('initial/planned.json',lambda d:d.pop())
        with self.assertRaises(ValueError):self.rows()

    def test_profile_swap_rejected(self):
        self.edit('corrective/profile.json',lambda d:d.update(GOVERSION='go1.26.8'))
        with self.assertRaises(ValueError):self.rows()

    def test_production_patch_cannot_be_hidden_as_test(self):
        self.edit('corrective/source-binding.json',lambda d:d.update(production_patch=True))
        with self.assertRaises(ValueError):self.rows()

    def test_changed_target_sources_rejected(self):
        self.edit('corrective/source-binding.json',lambda d:d['tracked']['eth/downloader/api.go'].update(sha256='0'*64))
        with self.assertRaises(ValueError):self.rows()

    def test_caller_count_cannot_be_relabelled(self):
        def mutate(rows):next(r for r in rows if r['event']=='actual_method_invoked')['concurrent_calls']=3
        self.change_events('corrective','general-r1-SUB-once-corrected',mutate)
        with self.assertRaises(ValueError):self.rows()

    def test_original_native_timer_not_shortcut(self):
        def mutate(rows):next(r for r in rows if r['event']=='actual_done_notification_received')['elapsed_ns']=1_000_000_000
        self.change_events('initial','general-r1-SUB-normal',mutate)
        with self.assertRaises(ValueError):self.rows()

    def test_actual_blocking_source_frame_required(self):
        p=self.packet/'corrective/slots/general-r1-SUB-blocked-corrected/events.jsonl.before-observation.stack.gz';p.write_bytes(gzip.compress(g.raw(p).replace(b'/eth/downloader/api.go:125',b'/eth/downloader/api.go:126'),mtime=0))
        with self.assertRaises(ValueError):self.rows()

    def test_block_observation_not_shortened(self):
        def mutate(rows):next(r for r in rows if r['event']=='observation_frozen')['observation_ms']=100
        self.change_events('corrective','general-r1-SUB-blocked-corrected',mutate)
        with self.assertRaises(ValueError):self.rows()

    def test_ack_does_not_precede_original_observation(self):
        def mutate(rows):
            a=next(i for i,r in enumerate(rows)if r['event']=='controlled_ack_released');b=next(i for i,r in enumerate(rows)if r['event']=='observation_frozen');rows[a]['event'],rows[b]['event']=rows[b]['event'],rows[a]['event']
        self.change_events('corrective','general-r1-SUB-once-corrected',mutate)
        with self.assertRaises((ValueError,KeyError)):self.rows()

    def test_lost_trace_event_rejected(self):
        self.change_events('initial','general-r1-SUB-normal',lambda d:d.pop(2))
        with self.assertRaises(ValueError):self.rows()

    def test_actual_output_tampering_rejected(self):
        p=self.packet/'initial/slots/general-r1-SUB-normal/process/stdout.gz';p.write_bytes(gzip.compress(b'forged PASS\n',mtime=0))
        with self.assertRaises(ValueError):self.rows()

    def test_observer_failure_cannot_become_pass(self):
        q=self.packet/'initial/slots/general-r1-SUB-blocked'
        for name in ['receipt.json','process/receipt.json']:
            p=q/name;d=json.loads(p.read_text());d['exit_code']=0;p.write_text(json.dumps(d))
        self.change_events('initial','general-r1-SUB-blocked',lambda rows:rows[-1].update(test_failed=False))
        plan=g.read(self.packet/'initial/planned.json')[1];binary=g.read(self.packet/'initial/binaries.json')['general'];fixture=g.read(self.packet/'initial/source-binding.json')['test_addition']['sha256']
        with self.assertRaises(ValueError):g.inspect_slot(q,plan,binary,fixture,False)

    def test_live_group_not_called_cleaned(self):
        q=self.packet/'initial/slots/general-r1-SUB-normal'
        for name in ['receipt.json','process/receipt.json']:
            p=q/name;d=json.loads(p.read_text());d['remaining_live_group_members']=1;p.write_text(json.dumps(d))
        plan=g.read(self.packet/'initial/planned.json')[0];binary=g.read(self.packet/'initial/binaries.json')['general'];fixture=g.read(self.packet/'initial/source-binding.json')['test_addition']['sha256']
        with self.assertRaises(ValueError):g.inspect_slot(q,plan,binary,fixture,False)

    def test_original_receiver_must_exit_before_trigger(self):
        def mutate(rows):next(r for r in rows if r['event']=='receiver_exited_before_trigger')['event']='unproved_receiver_state'
        self.change_events('receiver','general-r1-SUB-blocked-receiver',mutate)
        with self.assertRaises((ValueError,KeyError)):self.rows()

    def test_compiler_closure_suffix_does_not_change_method_identity(self):
        for name in ['(*SyncStatusSubscription).Unsubscribe.func1()','scope.(*SyncStatusSubscription).Unsubscribe.1()']:
            self.assertTrue(g.frame('goroutine1 [chan send]:\n'+name+'\n/a/eth/downloader/api.go:186','(*SyncStatusSubscription).Unsubscribe',186))
        self.assertFalse(g.frame('goroutine1 [chan send]:\n(*SyncStatusSubscription).UnsubscribeFake()\n/a/eth/downloader/api.go:186','(*SyncStatusSubscription).Unsubscribe',186))

    def test_frame_must_belong_to_same_blocked_goroutine(self):
        self.assertFalse(g.frame('goroutine1 [chan send]:\n(*DownloaderAPI).eventLoop\n\ngoroutine2 [chan send]:\n/a/eth/downloader/api.go:125','(*DownloaderAPI).eventLoop',125))


if __name__=='__main__':unittest.main()
