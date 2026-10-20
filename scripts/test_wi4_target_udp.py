"""Reject erased slots and false production/formal claims for UDP probes."""
import gzip,json,shutil,tempfile,unittest
from pathlib import Path
import wi4_target_udp as g


class UDPSourceEvidence(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup);self.packet=Path(self.tmp.name)/'packet';shutil.copytree(g.ROOT/g.PACKET,self.packet)

    def edit(self,name,fn):
        p=self.packet/name;d=g.read(p);fn(d);p.write_text(json.dumps(d))

    def rows(self):return g.derive(packet=self.packet)

    def event(self,case,name,fn):
        p=self.packet/'actual/slots'/('general-r1-'+case)/'events.jsonl.gz';rows=g.prior.events(p);fn(next(r for r in rows if r['event']==name));p.write_bytes(gzip.compress(('\n'.join(json.dumps(r)for r in rows)+'\n').encode(),mtime=0))

    def test_all_fixed_slots_and_scopes_preserved(self):
        d=self.rows();s=d['summary'];self.assertEqual(s['actual_attempts'],24);self.assertEqual(s['native_pending_near_timeout_observations'],6);self.assertEqual(s['native_pending_one_response_observations'],6);self.assertEqual(s['controlled_synthetic_deadline_observations'],12);self.assertEqual(s['formal_completed'],0);self.assertEqual(s['full_original_driver_acceptances'],0);self.assertEqual(s['full_listener_or_host_entry_proved'],0);self.assertEqual(s['other12_schedule_probes_pending'],True)

    def test_synthetic_and_native_routes_remain_separate(self):
        rows=self.rows()['observations'];self.assertEqual(sum(r['synthetic_deadline_injection']for r in rows),12);self.assertTrue(all(not r['product_violation_established']and not r['independent_oracle']and not r['full_original_driver_acceptance']for r in rows))

    def test_formal_adapter_refuses_exposed_unfrozen_trials(self):
        for r in self.rows()['observations']:self.assertEqual(r['formal_adapter_result'],dict(status='unknown_unverified_trial',label=None))

    def test_missing_planned_slot_refused(self):
        self.edit('actual/planned.json',lambda d:d.pop())
        with self.assertRaises(ValueError):self.rows()

    def test_missing_attempt_refused(self):
        self.edit('actual/attempts.json',lambda d:d.pop())
        with self.assertRaises(ValueError):self.rows()

    def test_upstream_source_swap_refused(self):
        self.edit('actual/source-binding.json',lambda d:d['tracked']['p2p/discover/v4_udp.go'].update(sha256='0'*64))
        with self.assertRaises(ValueError):self.rows()

    def test_production_patch_refused(self):
        self.edit('actual/source-binding.json',lambda d:d.update(production_patch=True))
        with self.assertRaises(ValueError):self.rows()

    def test_target_toolchain_swap_refused(self):
        self.edit('actual/profile.json',lambda d:d.update(GOVERSION='go1.26.8'))
        with self.assertRaises(ValueError):self.rows()

    def test_credential_environment_inheritance_refused(self):
        self.edit('actual/environment.json',lambda d:d.update(credential_environment_inherited=True))
        with self.assertRaises(ValueError):self.rows()

    def test_network_dependency_resolution_refused(self):
        self.edit('actual/environment.json',lambda d:d['task_settings'].update(GOPROXY='https://example.invalid'))
        with self.assertRaises(ValueError):self.rows()

    def test_postrun_binary_change_refused(self):
        self.edit('actual/binary-postrun-check.json',lambda d:d['observed_binary_sha256'].update(race='0'*64))
        with self.assertRaises(ValueError):self.rows()

    def test_observation_must_precede_cleanup(self):
        self.event('UDP-far-near','observation_frozen',lambda d:d.update(event='cleanup_begin'))
        with self.assertRaises(ValueError):self.rows()

    def test_negative_observation_window_not_shortened(self):
        self.event('UDP-far-near','observation_frozen',lambda d:d.update(observation_ms=100))
        with self.assertRaises(ValueError):self.rows()

    def test_injected_deadline_not_native_reachability(self):
        self.event('UDP-far-pair','synthetic_deadlines_injected',lambda d:d.update(production_reachability_proved=True))
        with self.assertRaises(ValueError):self.rows()

    def test_timer_internal_not_claimed_observed(self):
        self.event('UDP-far-near','observation_frozen',lambda d:d.update(timer_stop_directly_observed=True))
        with self.assertRaises(ValueError):self.rows()

    def test_exact_single_reset_not_inferred_from_timeouts(self):
        self.event('UDP-near','actual_pending_pair',lambda d:d.update(single_reset_invocation_count_known=True))
        with self.assertRaises(ValueError):self.rows()

    def test_response_not_relabelled_timeout_arm(self):
        self.event('UDP-response-timeout','observation_frozen',lambda d:d.update(selected_arm='timeout'))
        with self.assertRaises(ValueError):self.rows()

    def test_one_response_count_must_match(self):
        self.event('UDP-response-timeout','observation_frozen',lambda d:d.update(callback_counts=[1,1]))
        with self.assertRaises(ValueError):self.rows()

    def test_raw_output_tampering_refused(self):
        p=self.packet/'actual/slots/general-r1-UDP-near/process/stdout.gz';p.write_bytes(gzip.compress(b'forged PASS\n',mtime=0))
        with self.assertRaises(ValueError):self.rows()

    def test_preflight_failure_not_erased_or_counted_as_trial(self):
        self.edit('preflight/initial-attempt.json',lambda d:d.update(initial_runner_exit=0,actual_target_slots=1))
        with self.assertRaises(ValueError):self.rows()


if __name__=='__main__':unittest.main()
