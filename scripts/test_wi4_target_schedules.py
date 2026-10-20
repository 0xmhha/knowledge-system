"""Refuse erased failures, weakened boundaries and false acceptance claims."""
import gzip,json,shutil,tempfile,unittest
from pathlib import Path
import wi4_target_schedules as g

class ScheduleEvidence(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup);self.packet=Path(self.tmp.name)/'packet';shutil.copytree(g.ROOT/g.PACKET,self.packet)
    def edit(self,name,fn):
        p=self.packet/name;d=g.read(p);fn(d);p.write_text(json.dumps(d))
    def event(self,phase,slot,name,fn):
        p=self.packet/phase/'slots'/slot/'events.jsonl.gz';rows=[json.loads(l)for l in g.raw(p).decode().splitlines()];fn(next(r for r in rows if r['event']==name));p.write_bytes(gzip.compress(('\n'.join(json.dumps(r)for r in rows)+'\n').encode(),mtime=0))
    def rows(self):return g.derive(packet=self.packet)
    def rejects(self):
        with self.assertRaises((ValueError,KeyError)):self.rows()
    def test_all_attempts_and_failures_kept(self):
        d=self.rows();self.assertEqual((d['summary']['actual_attempts'],d['summary']['valid_technical_observations'],d['summary']['original_fixture_failures_preserved']),(96,72,24));self.assertEqual(d['summary']['full_original_driver_acceptances'],0);self.assertEqual(d['summary']['formal_completed'],0);self.assertEqual(d['summary']['unprobed_original_case_ids'],0)
    def test_panic_exit_is_authoritative_even_when_t_Failed_false(self):
        rows=[r for r in self.rows()['observations']if not r['valid_technical_observation']];self.assertEqual(len(rows),24);self.assertTrue(all(r['authoritative_process_exit']==2 and not r['trace_test_failed_flag']for r in rows))
    def test_formal_adapter_refuses_all_rows(self):
        for r in self.rows()['observations']:self.assertEqual(r['formal_adapter_result'],dict(status='unknown_unverified_trial',label=None))
    def test_missing_original_slot_refused(self):
        self.edit('actual/attempts.json',lambda d:d.pop());self.rejects()
    def test_missing_corrective_slot_refused(self):
        self.edit('corrective/attempts.json',lambda d:d.pop());self.rejects()
    def test_original_failure_not_relabelled_pass(self):
        self.edit('actual/slots/general-r1-GOSSIP-normal/receipt.json',lambda d:d.update(exit_code=0));self.rejects()
    def test_source_swap_refused(self):
        self.edit('actual/source-binding.json',lambda d:d['tracked']['miner/worker.go'].update(sha256='0'*64));self.rejects()
    def test_production_patch_refused(self):
        self.edit('actual/source-binding.json',lambda d:d.update(production_patch=True));self.rejects()
    def test_wrong_toolchain_refused(self):
        self.edit('actual/profile.json',lambda d:d.update(GOVERSION='go1.26.8'));self.rejects()
    def test_credentials_inheritance_refused(self):
        self.edit('actual/environment.json',lambda d:d.update(credential_environment_inherited=True));self.rejects()
    def test_network_dependency_resolution_refused(self):
        self.edit('actual/environment.json',lambda d:d['task_settings'].update(GOPROXY='https://example.invalid'));self.rejects()
    def test_both_ready_precondition_cannot_change(self):
        self.event('actual','general-r1-WORK-both','selection_ready',lambda d:d.update(exit_ready=False));self.rejects()
    def test_fairness_not_inferred_from_finite_backlog(self):
        self.event('actual','general-r1-WORK-backlog','observation_frozen',lambda d:d.update(fairness_or_SLA_proved=True));self.rejects()
    def test_unbuffered_receiver_not_inferred_from_ready_buffer(self):
        self.event('actual','general-r1-WORK-normal','observation_frozen',lambda d:d.update(unbuffered_receiver_precondition_proved=True));self.rejects()
    def test_fake_success_not_network_delivery(self):
        self.event('corrective','general-r1-GOSSIP-normal-engine','observation_frozen',lambda d:d.update(network_delivery_proved=True));self.rejects()
    def test_duplicate_attempt_count_not_rewritten(self):
        self.event('corrective','general-r1-GOSSIP-fail-repeat-engine','observation_frozen',lambda d:d.update(fake_send_attempts=2));self.rejects()
    def test_nonempty_reinjection_required(self):
        self.event('actual','general-r1-POOL-lock-cycle','actual_Reset_Send_blocked',lambda d:d.update(nonempty_reinject_proved=False));self.rejects()
    def test_block_window_not_shortened(self):
        self.event('actual','general-r1-POOL-lock-cycle','blocked_observation_frozen',lambda d:d.update(observation_ms=100));self.rejects()
    def test_subscriber_receive_order_preserved(self):
        self.event('actual','general-r1-POOL-lock-cycle','subscriber_receive_after_Stats',lambda d:d.update(event='unproved_subscriber_path'));self.rejects()
    def test_preflight_build_failure_not_erased(self):
        self.edit('preflight/initial-attempt.json',lambda d:d.update(initial_runner_exit=0));self.rejects()

if __name__=='__main__':unittest.main()
