"""Reject false promotion, erased attempts and mismatched method schedule evidence."""
import gzip,json,shutil,tempfile,unittest
from pathlib import Path
import ticket_method_schedules as g


class MethodSchedules(unittest.TestCase):
    slot='ORIGINAL-race-method5'
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup);self.packet=Path(self.tmp.name)/'packet';shutil.copytree(g.ROOT/g.PACKET,self.packet)
    def edit(self,name,fn):
        p=self.packet/name;d=g.read(p);fn(d);p.write_text(json.dumps(d))
    def event(self,name,fn,occurrence=0):
        p=self.packet/'actual/slots'/self.slot/'events.jsonl.gz';rows=[json.loads(l)for l in g.raw(p).decode().splitlines()];fn([e for e in rows if e['event']==name][occurrence]);p.write_bytes(gzip.compress(('\n'.join(json.dumps(e)for e in rows)+'\n').encode(),mtime=0))
    def data(self):return g.derive(packet=self.packet)
    def rejects(self):
        with self.assertRaises((ValueError,KeyError,OSError)):self.data()
    def receipts(self,fn):
        for n in ['receipt.json','process/receipt.json']:self.edit('actual/slots/'+self.slot+'/'+n,fn)
        self.edit('actual/attempts.json',lambda d:fn(next(r for r in d if r['slot_id']==self.slot)))
    def test_fixed_population_and_no_acceptance(self):
        s=self.data()['summary'];self.assertEqual((s['valid_technical_observations'],s['correlated_schedule_observations'],s['valid_bundle_method_calls_joined']),(10,50,80));self.assertEqual(s['formal_completed'],0);self.assertIsNone(s['independent_acceptable_candidate_set'])
    def test_original_and_early_defects_are_observations_not_erased(self):
        s=self.data()['summary'];self.assertEqual(s['blocked_real_loop_outcomes'],dict(drained_returned_loop_live=4,paired_sends_pending_before_rescue=4,early_return_loop_still_blocked=2))
    def test_correlated_schedules_not_independent_repeats(self):
        s=self.data()['summary'];self.assertEqual(s['repetitions_per_role_profile'],1);self.assertFalse(s['independent_repeated_execution_criterion_satisfied']);self.assertEqual(s['unique_method_variants'],4)
    def test_twelve_requirements_and_five_schedules_keep_gaps(self):
        d=self.data();self.assertEqual(len(d['requirements']),120);self.assertEqual(len(d['original_schedules']),5);self.assertTrue(all(not r['full_requirement_satisfied']for r in d['requirements']));self.assertTrue(all(not r['full_original_scenario_satisfied']for r in d['original_schedules']))
    def test_missing_attempt_rejected(self):
        self.edit('actual/attempts.json',lambda d:d.pop());self.rejects()
    def test_duplicate_attempt_rejected(self):
        self.edit('actual/attempts.json',lambda d:d.__setitem__(1,d[0]));self.rejects()
    def test_missing_plan_rejected(self):
        self.edit('actual/planned.json',lambda d:d.pop());self.rejects()
    def test_shortened_plan_rejected(self):
        self.edit('SCOPE.json',lambda d:d.update(native_timer_seconds=1));self.rejects()
    def test_native_timer_observation_not_early(self):
        self.event('native_completion_received',lambda d:d.update(elapsed_ns=1));self.rejects()
    def test_post_done_not_claimed_native_broadcast_RPC(self):
        self.event('blocked_precondition',lambda d:d.update(route='actual_RPC_native_broadcast'));self.rejects()
    def test_missing_sole_consumer_exit_rejected(self):
        self.event('blocked_precondition',lambda d:d.update(sole_consumer_exited=False));self.rejects()
    def test_wait_graph_requires_actual_send_stack(self):
        p=self.packet/'actual/slots'/self.slot/'events.jsonl.blocked-observed.stack.gz';p.write_bytes(gzip.compress(b'no source wait',mtime=0));self.rejects()
    def test_pending_original_not_called_returned(self):
        self.event('blocked_observation_frozen',lambda d:d.update(method_pending=False));self.rejects()
    def test_loop_liveness_not_invented(self):
        self.event('post_observation_liveness',lambda d:d.update(actual_loop_serviced_probe=True));self.rejects()
    def test_late_send_count_required(self):
        self.event('controlled_pre_ack_frozen',lambda d:d.update(late_sends_completed=2));self.rejects()
    def test_same_exact_object_required(self):
        self.event('controlled_request',lambda d:d.update(same_object=False),2);self.rejects()
    def test_individual_early_return_cannot_hide_in_group_wait(self):
        self.event('controlled_pre_ack_frozen',lambda d:d.update(individual_returns=1),2);self.rejects()
    def test_actual_three_method_callers_required(self):
        p=self.packet/'actual/slots'/self.slot/'events.jsonl.concurrent_unsubscribe_once-before-ack.stack.gz';p.write_bytes(gzip.compress(b'(*SyncStatusSubscription).Unsubscribe\n/eth/downloader/api.go:184',mtime=0));self.rejects()
    def test_duplicate_uninstall_request_rejected(self):
        self.event('controlled_calls_joined',lambda d:d.update(duplicate_request=True),2);self.rejects()
    def test_ack_not_released_during_pending_observation(self):
        self.event('controlled_pre_ack_frozen',lambda d:d.update(ack_released=True),1);self.rejects()
    def test_controlled_owner_not_real_map(self):
        self.event('controlled_calls_joined',lambda d:d.update(map_removal_proved=True));self.rejects()
    def test_post_ack_all_channel_use_not_invented(self):
        self.event('controlled_calls_joined',lambda d:d.update(post_ack_all_channel_use_proved=True));self.rejects()
    def test_rescue_cannot_count_as_observation(self):
        self.event('blocked_observation_frozen',lambda d:d.update(rescue_started=True));self.rejects()
    def test_owned_cleanup_required(self):
        self.event('real_cleanup_end',lambda d:d.update(event_loops=1));self.rejects()
    def test_terminated_actual_process_not_success(self):
        self.receipts(lambda d:d.update(term_sent=True));self.rejects()
    def test_cancelled_pretrial_failure_cannot_be_erased(self):
        self.edit('pretrial-cancelled/cancellation.json',lambda d:d.update(actual_target_attempts=1));self.rejects()
    def test_cancelled_build_group_cannot_be_ignored(self):
        self.edit('pretrial-cancelled/post-cancellation-audit.json',lambda d:d['groups'][0].update(remaining_live_group_members=1));self.rejects()
    def test_elapsed_channel_wait_tag_remains_actual_wait(self):
        self.assertTrue(g.prior.stack_frame('goroutine 1 [chan send, 1 minutes]:\n(*DownloaderAPI).eventLoop\n/eth/downloader/api.go:94','(*DownloaderAPI).eventLoop','/eth/downloader/api.go:94','[chan send'))
    def test_corrective_population_cannot_hide_initial_failures(self):
        self.edit('CORRECTION-SCOPE.json',lambda d:d['planned_slots'].clear());self.rejects()
    def test_raw_output_binding(self):
        p=self.packet/'actual/slots'/self.slot/'process/stdout.gz';p.write_bytes(gzip.compress(b'forged PASS',mtime=0));self.rejects()
    def test_actual_source_build_tree_binding(self):
        self.edit('actual/candidates/CAND-A/tree-binding.json',lambda d:d.update(build_input_tree='0'*40));self.rejects()
    def test_offline_credential_boundary(self):
        self.edit('actual/environment.json',lambda d:d.update(credential_environment_inherited=True));self.rejects()
    def test_remaining_work_not_erasable_by_reseal(self):
        self.edit('WORKLIST.json',lambda d:d.update(remaining_all=[]));g.seal(self.packet);self.assertTrue(any('worklist differs'in e for e in g.validate(packet=self.packet)))
    def test_reseal_does_not_make_gold(self):
        self.edit('requirements.json',lambda d:d[0].update(full_requirement_satisfied=True));g.seal(self.packet);self.assertTrue(any('derived native result differs'in e for e in g.validate(packet=self.packet)))


if __name__=='__main__':unittest.main()
