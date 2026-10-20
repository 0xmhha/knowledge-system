"""Source-bound atomic counter and actual RPC evidence cannot become approval."""
import gzip,json,shutil,tempfile,unittest
from pathlib import Path
import ticket_rpc_overlap as g

class RPCOverlap(unittest.TestCase):
    slot='ORIGINAL-general-rpc-overlap'
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup);self.packet=Path(self.tmp.name)/'packet';shutil.copytree(g.ROOT/g.PACKET,self.packet)
    def edit(self,name,fn):
        p=self.packet/name;d=g.read(p);fn(d);p.write_text(json.dumps(d))
    def event(self,name,fn):
        p=self.packet/'actual/slots'/self.slot/'events.jsonl.gz';a=[json.loads(l)for l in g.raw(p).decode().splitlines()];fn(next(e for e in a if e['event']==name));p.write_bytes(gzip.compress(('\n'.join(json.dumps(e)for e in a)+'\n').encode(),mtime=0))
    def data(self):return g.derive(packet=self.packet)
    def rejects(self):
        with self.assertRaises((ValueError,KeyError,OSError)):self.data()
    def receipts(self,fn):
        for n in ['receipt.json','process/receipt.json']:self.edit('actual/slots/'+self.slot+'/'+n,fn)
        self.edit('actual/attempts.json',lambda d:fn(next(r for r in d if r['slot_id']==self.slot)))
    def test_fixed_population_and_formal_limits(self):
        s=self.data()['summary'];self.assertEqual((s['actual_attempts'],s['native60s_timer_fires'],s['pre_rescue_counter_snapshots']),(6,12,12));self.assertEqual(s['formal_completed'],0);self.assertIsNone(s['independent_acceptable_candidate_set'])
    def test_no_original_test_or_independent_repeat_claim(self):
        s=self.data()['summary'];self.assertFalse(s['original_package_tests_executed']);self.assertFalse(s['independent_repeated_execution_criterion_satisfied']);self.assertEqual(s['unique_fresh_method_variants'],3)
    def test_all_requirements_keep_scope_gaps(self):
        d=self.data();self.assertEqual(len(d['requirements']),72);self.assertTrue(all(not r['full_requirement_satisfied']for r in d['requirements']))
    def test_actual_partial_order_not_candidate_selection(self):
        for r in self.data()['observations']:self.assertEqual(r['pre_close_counts'],dict(notify=1,exit=0,uninstall=0,method=0,drain=0,ack=0));self.assertIsNone(r['independent_acceptable_candidate_set'])
    def test_missing_planned_attempt(self):
        self.edit('actual/attempts.json',lambda d:d.pop());self.rejects()
    def test_duplicate_attempt(self):
        self.edit('actual/attempts.json',lambda d:d.__setitem__(1,d[0]));self.rejects()
    def test_missing_frozen_plan(self):
        self.edit('actual/planned.json',lambda d:d.pop());self.rejects()
    def test_shortened_native_timer_plan(self):
        self.edit('SCOPE.json',lambda d:d.update(native_timer_seconds=1));self.rejects()
    def test_first_timer_not_early(self):
        self.event('first_native_write_held',lambda d:d.update(elapsed_ns=1));self.rejects()
    def test_second_timer_not_forced(self):
        self.event('second_native_broadcast_blocked',lambda d:d.update(elapsed_ns=1));self.rejects()
    def test_held_write_not_delivered_notification(self):
        self.event('first_native_write_held',lambda d:d.update(wire_notification_delivered=True));self.rejects()
    def test_same_actual_RPC_consumer_required(self):
        self.event('second_native_broadcast_blocked',lambda d:d.update(sole_RPC_consumer_in_Notify=False));self.rejects()
    def test_connection_close_not_unsubscribe_true(self):
        self.event('actual_server_connection_closed',lambda d:d.update(RPC_unsubscribe_response=True));self.rejects()
    def test_actual_uncontrolled_select_not_forced(self):
        self.event('observation_frozen',lambda d:d.update(select_exit_vs_second_status_controlled=True));self.rejects()
    def test_no_real_map_observation_invented(self):
        self.event('observation_frozen',lambda d:d.update(backend_map_directly_observed=True));self.rejects()
    def test_no_all_post_ack_use_claim(self):
        self.event('observation_frozen',lambda d:d.update(all_post_ack_channel_use_proved=True));self.rejects()
    def test_rescue_not_part_of_success_observation(self):
        self.event('observation_frozen',lambda d:d.update(rescue_started=True));self.rejects()
    def test_cleanup_controlled_ack_not_real_map_ack(self):
        self.event('cleanup_end',lambda d:d.update(rescue_ack_is_controlled_not_real_map_ack=False));self.rejects()
    def test_no_termination_promoted(self):
        self.receipts(lambda d:d.update(term_sent=True));self.rejects()
    def test_actual_wait_stack_required(self):
        p=self.packet/'actual/slots'/self.slot/'events.jsonl.native-broadcast-blocked.stack.gz';p.write_bytes(gzip.compress(b'no wait',mtime=0));self.rejects()
    def test_raw_profile_reconversion_detects_text_forgery(self):
        p=self.packet/'actual/slots'/self.slot/'coverage-before-rescue.txt';p.write_text(p.read_text().replace(' 1 1\n',' 1 99\n',1));self.rejects()
    def test_snapshot_cannot_merge_final_coverage(self):
        p=self.packet/'actual/slots'/self.slot/'coverage-before-rescue';f=next(p.glob('covcounters.*'));shutil.copyfile(f,p/(f.name+'.extra'));self.rejects()
    def test_source_tree_binding_required(self):
        self.edit('actual/candidates/CAND-A/tree-binding.json',lambda d:d.update(build_input_tree='0'*40));self.rejects()
    def test_program_source_additions_cannot_be_hidden(self):
        self.edit('actual/candidates/ORIGINAL/source-binding.json',lambda d:d.update(additions=[]));self.rejects()
    def test_original_package_test_claim_rejected(self):
        self.event('trace_begin',lambda d:d.update(original_package_tests_executed=True));self.rejects()
    def test_raw_process_output_binding(self):
        p=self.packet/'actual/slots'/self.slot/'process/stdout.gz';p.write_bytes(gzip.compress(b'PASS',mtime=0));self.rejects()
    def test_toolchain_cannot_be_swapped(self):
        self.edit('coverage-tool.json',lambda d:d.update({next(iter(d)):'0'*64}));self.rejects()
    def test_worklist_cannot_mark_deferred_items_done_after_reseal(self):
        self.edit('WORKLIST.json',lambda d:d['security_scope_adjustment'].update(deferred_is_completed=True));g.seal(self.packet);self.assertTrue(any('worklist differs'in e for e in g.validate(packet=self.packet)))
    def test_requirements_cannot_be_accepted_by_reseal(self):
        self.edit('requirements.json',lambda d:d[0].update(full_requirement_satisfied=True));g.seal(self.packet);self.assertTrue(any('derived native result differs'in e for e in g.validate(packet=self.packet)))

if __name__=='__main__':unittest.main()
