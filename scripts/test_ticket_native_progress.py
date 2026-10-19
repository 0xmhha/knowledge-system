"""Native timer, actual payload/session and conservative acceptance controls."""
import gzip,json,shutil,tempfile,unittest
from pathlib import Path
import ticket_native_progress as g


class NativeProgress(unittest.TestCase):
    slot='ORIGINAL-general-native60-progress'
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup);self.packet=Path(self.tmp.name)/'packet';shutil.copytree(g.ROOT/g.PACKET,self.packet)
    def edit(self,name,fn):
        p=self.packet/name;d=g.read(p);fn(d);p.write_text(json.dumps(d))
    def event(self,event,fn):
        p=self.packet/'actual/slots'/self.slot/'events.jsonl.gz';rows=[json.loads(l)for l in g.raw(p).decode().splitlines()];fn(next(e for e in rows if e['event']==event));p.write_bytes(gzip.compress(('\n'.join(json.dumps(e)for e in rows)+'\n').encode(),mtime=0))
    def data(self):return g.derive(packet=self.packet)
    def rejects(self):
        with self.assertRaises((ValueError,KeyError,OSError)):self.data()
    def receipts(self,fn):
        for n in ['receipt.json','process/receipt.json']:self.edit('actual/slots/'+self.slot+'/'+n,fn)
        self.edit('actual/attempts.json',lambda d:fn(next(r for r in d if r['slot_id']==self.slot)))
    def test_all_fixed_trials_payloads_and_denominators(self):
        s=self.data()['summary'];self.assertEqual((s['actual_attempts'],s['caller_RPC_requests'],s['notifications'],s['native60s_timer_fires']),(10,70,30,20));self.assertEqual(s['requirement_units'],120);self.assertEqual(s['formal_completed'],0);self.assertIsNone(s['independent_acceptable_candidate_set'])
    def test_one_repeat_not_independent_repetition_criterion(self):
        s=self.data()['summary'];self.assertEqual(s['repetitions_per_role_profile'],1);self.assertFalse(s['independent_repeated_execution_criterion_satisfied'])
    def test_all_timing_observations_and_role_alias(self):
        rows=self.data()['observations'];self.assertEqual(len({r['applied_project_tree']for r in rows}),4);self.assertTrue(all(r['constructor_to_first_notification_ns']>=60_000_000_000 and r['notification_gap_ns']>=59_000_000_000 and r['post_done_notification_ns']<2_000_000_000 for r in rows))
    def test_no_full_requirement_or_original_schedule_promotion(self):
        d=self.data();self.assertEqual(sum(r['label']=='bounded_observation_with_remaining_scope'for r in d['requirements']),60);self.assertTrue(all(not r['full_requirement_satisfied']for r in d['requirements']));self.assertEqual(len(d['original_schedules']),5);self.assertTrue(all(not r['full_original_scenario_satisfied']for r in d['original_schedules']))
    def test_missing_planned_slot_rejected(self):
        self.edit('actual/planned.json',lambda d:d.pop());self.rejects()
    def test_missing_actual_attempt_rejected(self):
        self.edit('actual/attempts.json',lambda d:d.pop());self.rejects()
    def test_frozen_timer_plan_cannot_be_shortened(self):
        self.edit('SCOPE.json',lambda d:d.update(minimum_constructor_to_first_notification_ns=1));self.rejects()
    def test_actual_native_timer_not_faked(self):
        self.event('trace_begin',lambda d:d.update(native_timer_seconds=1));self.rejects()
    def test_early_notification_not_called_native60(self):
        self.event('native_in_progress_notification',lambda d:d.update(elapsed_ns=1));self.rejects()
    def test_progress_payload_missing_field_rejected(self):
        self.event('native_in_progress_notification',lambda d:d['payload']['status'].pop('CurrentBlock'));self.rejects()
    def test_shared_expected_payload_cannot_mask_wrong_state(self):
        for name in ['owned_progress_dependencies','native_in_progress_notification']:
            def change(d):
                if name=='owned_progress_dependencies':d['expected_progress']['HighestBlock']=999
                else:
                    for k in ['payload','expected_payload']:d[k]['status']['HighestBlock']=999
            self.event(name,change)
        self.rejects()
    def test_done_boolean_and_subscription_id_cannot_be_swapped(self):
        self.event('native_done_notification',lambda d:d.update(subscription_id='0x00',payload=True));self.rejects()
    def test_duplicate_subscription_identity_rejected(self):
        self.event('post_done_RPC_subscription',lambda d:d.update(subscription_id='0x00'));self.rejects()
    def test_negative_error_code_pinned(self):
        self.event('malformed_subscription_error',lambda d:d.update(code=-32601));self.rejects()
    def test_direct_notifier_control_not_RPC(self):
        self.event('direct_notifier_control',lambda d:d.update(RPC_request=True));self.rejects()
    def test_controlled_height_not_real_peer_sync(self):
        self.event('controlled_highest_block_updated',lambda d:d.update(network_sync_completed=True));self.rejects()
    def test_disabled_indexer_not_enabled_branch(self):
        self.event('owned_progress_dependencies',lambda d:d.update(indexer='enabled'));self.rejects()
    def test_callback_return_not_map_membership(self):
        self.event('first_RPC_unsubscribe',lambda d:d.update(backend_membership_proved=True));self.rejects()
    def test_full_requirement_promotion_rejected(self):
        self.event('observation_frozen',lambda d:d.update(full12_requirements=True,independent_acceptable_candidate_set=['CAND-A']));self.rejects()
    def test_owned_cleanup_not_skipped(self):
        self.event('owned_RPC_cleanup_end',lambda d:d.update(ServeCodec_joined=False));self.rejects()
    def test_terminated_process_not_success(self):
        self.receipts(lambda d:d.update(term_sent=True));self.rejects()
    def test_raw_output_changes_rejected(self):
        p=self.packet/'actual/slots'/self.slot/'process/stdout.gz';p.write_bytes(gzip.compress(b'forged PASS\n',mtime=0));self.rejects()
    def test_source_tree_and_fixture_binding_rejected(self):
        self.edit('actual/candidates/CAND-A/tree-binding.json',lambda d:d.update(build_input_tree='0'*40));self.rejects()
    def test_credentials_not_inherited(self):
        self.edit('actual/environment.json',lambda d:d.update(credential_environment_inherited=True));self.rejects()
    def test_remaining_work_not_erasable_after_reseal(self):
        self.edit('WORKLIST.json',lambda d:d.update(remaining_all=[]));g.seal(self.packet);self.assertTrue(any('worklist differs'in e for e in g.validate(packet=self.packet)))
    def test_requirements_not_fully_accepted_after_reseal(self):
        self.edit('requirements.json',lambda d:d[0].update(full_requirement_satisfied=True));g.seal(self.packet);self.assertTrue(any('derived native result differs'in e for e in g.validate(packet=self.packet)))


if __name__=='__main__':unittest.main()
