"""Reject unbound RPC paths, false completion and inflated candidate roles."""
import gzip,json,shutil,tempfile,unittest
from pathlib import Path
import ticket_actual_rpc as g

class ActualRPC(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup);self.packet=Path(self.tmp.name)/'packet';shutil.copytree(g.ROOT/g.PACKET,self.packet)
    def edit(self,name,fn):
        p=self.packet/name;d=g.read(p);fn(d);p.write_text(json.dumps(d))
    def event(self,slot,event,fn):
        p=self.packet/'actual/slots'/slot/'events.jsonl.gz';rows=[json.loads(l)for l in g.raw(p).decode().splitlines()];fn(next(r for r in rows if r['event']==event));p.write_bytes(gzip.compress(('\n'.join(json.dumps(r)for r in rows)+'\n').encode(),mtime=0))
    def rows(self):return g.derive(packet=self.packet)
    def receipts(self,plan,slot,fn):
        self.edit(plan+'/slots/'+slot+'/receipt.json',fn);self.edit(plan+'/slots/'+slot+'/process/receipt.json',fn)
        self.edit(plan+'/attempts.json',lambda d:fn(next(r for r in d if r['slot_id']==slot)))
    def rejects(self):
        with self.assertRaises((ValueError,KeyError)):self.rows()
    def test_all_fixed_trials_and_scope(self):
        d=self.rows();s=d['summary'];self.assertEqual((s['actual_attempts'],s['actual_caller_RPC_requests'],s['direct_non_RPC_notifier_controls']),(134,465,93));self.assertEqual((s['actual_live_eventLoop_slots'],s['controlled_backend_slots']),(30,60));self.assertEqual((s['candidate_roles'],s['unique_method_variants'],s['unique_applied_project_trees']),(5,4,4));self.assertEqual(s['formal_completed'],0);self.assertIsNone(s['independent_acceptable_candidate_set']);self.assertIsNone(s['canonical_selection'])
    def test_true_response_is_distinct_from_held_backend_completion(self):
        rows=[r for r in self.rows()['observations']if r['valid_technical_observation']and r['case_id']=='RPC-ack-held'];waiting=[r for r in rows if r['candidate']!='MUT-EARLY'];self.assertEqual(len(waiting),24);self.assertTrue(all(r['observed']['request_received']and not r['observed']['callback_returned']for r in waiting))
    def test_status_drain_observations_not_membership_approval(self):
        rows=[r for r in self.rows()['observations']if r['valid_technical_observation']and r['case_id']=='RPC-status-before-request'];self.assertEqual(sum(r['observed']['status_drained_before_request_receive']for r in rows),12);self.assertTrue(all(r['independent_acceptable_membership']is None and not r['independent_oracle']for r in rows))
    def test_actual_git_tree_hashes_and_aliases(self):
        for role in g.ROLES:
            d=g.read(self.packet/'actual/candidates'/role/'tree-binding.json');self.assertEqual(g.tree_oid(d['project_flat_tree']),d['applied_project_tree']);self.assertEqual(g.tree_oid(d['build_flat_tree']),d['build_input_tree'])
        a=g.read(self.packet/'actual/candidates/ORIGINAL/tree-binding.json');b=g.read(self.packet/'actual/candidates/MUT-REINTRO/tree-binding.json');self.assertEqual(a['applied_project_tree'],b['applied_project_tree'])
    def test_duplicate_tree_paths_refused(self):
        rows=g.read(self.packet/'actual/candidates/ORIGINAL/base-flat-tree.json');rows.append(rows[0])
        with self.assertRaises(ValueError):g.tree_oid(rows)
    def test_unsafe_tree_paths_refused(self):
        for name in ['../escape','/absolute','a/../b','a/./b','a//b','.','']:
            with self.subTest(path=name),self.assertRaises(ValueError):g.tree_oid([dict(path=name,mode='100644',kind='blob',oid='0'*40)])
    def test_missing_plan_slot_refused(self):
        self.edit('actual/planned.json',lambda d:d.pop());self.rejects()
    def test_missing_actual_attempt_refused(self):
        self.edit('actual/attempts.json',lambda d:d.pop());self.rejects()
    def test_applied_source_swap_refused(self):
        self.edit('actual/candidates/CAND-A/source-binding.json',lambda d:d['applied_tracked'][g.API].update(sha256='0'*64));self.rejects()
    def test_real_tree_hash_cannot_be_invented(self):
        self.edit('actual/candidates/CAND-A/tree-binding.json',lambda d:d.update(applied_project_tree='0'*40));self.rejects()
    def test_build_tree_must_include_actual_fixture(self):
        self.edit('actual/candidates/CAND-A/tree-binding.json',lambda d:d.update(build_flat_tree=d['project_flat_tree']));self.rejects()
    def test_foreign_session_uses_bound_real_id(self):
        self.event('ORIGINAL-general-r1-RPC-live-normal','foreign_session_control',lambda d:d.update(same_id='0x00'));self.rejects()
    def test_correct_unsubscribe_session_not_assumed(self):
        self.event('ORIGINAL-general-r1-RPC-live-normal','actual_RPC_unsubscribe_true',lambda d:d.update(same_session=False));self.rejects()
    def test_held_callback_not_automatically_complete(self):
        self.event('CAND-A-general-r1-RPC-ack-held','observation_frozen',lambda d:d.update(callback_returned=True));self.rejects()
    def test_ack_not_released_before_observation(self):
        self.event('CAND-A-general-r1-RPC-ack-held','controlled_ack_closed',lambda d:d.update(elapsed_ns=0));self.rejects()
    def test_backend_membership_not_inferred_from_rpc(self):
        self.event('MUT-EARLY-general-r1-RPC-live-normal','observation_frozen',lambda d:d.update(backend_membership_proved=True));self.rejects()
    def test_progress_notification_not_inferred_from_normal_unsubscribe(self):
        self.event('ORIGINAL-general-r1-RPC-live-normal','observation_frozen',lambda d:d.update(native_progress_timer_fired=True));self.rejects()
    def test_early_mutant_has_no_backend_request(self):
        self.event('MUT-EARLY-general-r1-RPC-ack-held','observation_frozen',lambda d:d.update(request_received=True));self.rejects()
    def test_candidate_drain_not_relabelled(self):
        self.event('CAND-B-general-r1-RPC-status-before-request','observation_frozen',lambda d:d.update(status_drained_before_request_receive=False));self.rejects()
    def test_credentials_not_inherited(self):
        self.edit('actual/environment.json',lambda d:d.update(credential_environment_inherited=True));self.rejects()
    def test_unjoined_callback_not_called_cleaned(self):
        self.event('ORIGINAL-general-r1-RPC-live-normal','cleanup_end',lambda d:d.update(remaining_callbacks=1));self.rejects()
    def test_raw_output_tampering_refused(self):
        p=self.packet/'actual/slots/ORIGINAL-general-r1-RPC-live-normal/process/stdout.gz';p.write_bytes(gzip.compress(b'forged PASS\n',mtime=0));self.rejects()
    def test_direct_notifier_control_not_counted_rpc(self):
        self.event('ORIGINAL-general-r1-RPC-live-normal','unsupported_notifier_control',lambda d:d.update(RPC_request=True));self.rejects()
    def test_independent_set_never_inferred_from_candidate_observations(self):
        for p in ['SCOPE.json','actual/SCOPE.json']:self.edit(p,lambda d:d['acceptance_limits'].update(independent_acceptable_candidate_set=['CAND-A']))
        self.rejects()
    def test_initial_incomplete_attempts_retained_without_quality(self):
        d=self.rows();s=d['summary'];failed=[r for r in d['observations']if not r['valid_technical_observation']];self.assertEqual(len(failed),44);self.assertEqual(s['valid_technical_observations'],90);self.assertFalse(s['all_initial_attempts_passed']);self.assertEqual(sum(r['actual_RPC_requests']for r in failed),15);self.assertTrue(all(r['observed']is None and r['quality_score']is None and not r['internal_callback_cleanup_proved']for r in failed))
    def test_unentered_timeout_cannot_be_changed_to_pass(self):
        self.receipts('actual','ORIGINAL-race-r1-RPC-live-normal',lambda d:d.update(exit_code=0,timeout=False));self.rejects()
    def test_partial_trace_cannot_be_called_unentered(self):
        p=self.packet/'actual/slots/ORIGINAL-race-r1-RPC-live-normal/events.jsonl.gz';p.write_bytes(gzip.compress(b'{}\n',mtime=0));self.rejects()
    def test_initial_timeout_output_cannot_be_hidden(self):
        b=b'failure\n';p=self.packet/'actual/slots/ORIGINAL-race-r1-RPC-live-normal/process/stderr.gz';p.write_bytes(gzip.compress(b,mtime=0));self.receipts('actual','ORIGINAL-race-r1-RPC-live-normal',lambda d:d.update(stderr_sha256=g.sha(b)));self.rejects()
    def test_corrective_attempt_not_removed_from_denominator(self):
        self.edit('recovery/attempts.json',lambda d:d.pop());self.rejects()
    def test_corrective_plan_cannot_shorten_observation(self):
        for p in ['RECOVERY-SCOPE.json','recovery/SCOPE.json']:self.edit(p,lambda d:d.update(observation_watchdog_ms=1))
        self.rejects()
    def test_corrective_binary_cannot_be_rebuilt_unbound(self):
        self.edit('recovery/reused-binaries.json',lambda d:d['CAND-A'].update(sha256='0'*64));self.rejects()
    def test_corrective_failure_cannot_be_called_valid(self):
        self.receipts('recovery','CAND-A-race-r1-RPC-live-normal',lambda d:d.update(exit_code=-15,timeout=True));self.rejects()
    def test_partial_calls_retained_without_frozen_or_cleanup_claim(self):
        rows=[r for r in self.rows()['observations']if r['classification']=='external_timeout_during_observation'];self.assertEqual(len(rows),3);self.assertEqual(sum(r['actual_RPC_requests']for r in rows),15);self.assertTrue(all(r['observed']is None and not r['internal_callback_cleanup_proved']for r in rows))
    def test_partial_control_session_cannot_be_forged(self):
        self.event('CAND-A-race-r3-RPC-status-before-request','foreign_session_control',lambda d:d.update(same_id='0x00'));self.rejects()
    def test_partial_cannot_acquire_frozen_observation(self):
        p=self.packet/'actual/slots/CAND-A-race-r3-RPC-status-before-request/events.jsonl.gz';p.write_bytes(gzip.compress(g.raw(p)+b'{"event":"observation_frozen"}\n',mtime=0));self.rejects()
    def test_guard_refusal_not_suppressed_as_success(self):
        self.edit('preflight1/receipt.json',lambda d:d.update(exit_code=0));self.rejects()
    def test_already_observed_race_not_retried(self):
        self.edit('RECOVERY-SCOPE.json',lambda d:d['planned_slots'].append(dict(slot_id='MUT-EARLY-race-r3-RPC-status-before-request',candidate='MUT-EARLY',variant='race',repetition=3,case_id='RPC-status-before-request')));self.rejects()
    def test_remaining_work_cannot_be_erased_even_after_reseal(self):
        self.edit('WORKLIST.json',lambda d:d.update(remaining_all=[]));g.seal(self.packet);self.assertTrue(any('worklist differs'in e for e in g.validate(packet=self.packet)))
    def test_scope_cannot_be_claimed_formal_even_after_reseal(self):
        self.edit('WORKLIST.json',lambda d:d.update(completed_in_this_scope=['all9_formally_accepted']));g.seal(self.packet);self.assertTrue(any('worklist differs'in e for e in g.validate(packet=self.packet)))

if __name__=='__main__':unittest.main()
