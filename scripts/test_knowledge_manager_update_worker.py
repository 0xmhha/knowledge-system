"""Actual native worker cycles and injected admission/IO/recovery boundaries."""
import argparse
import copy
import importlib
import json
from pathlib import Path
import sys
import time
import unittest
from unittest.mock import patch

import test_knowledge_manager_update_inputs as fixture
manager=fixture.manager
inputs=fixture.inputs
state=importlib.import_module('update_state')
worker=importlib.import_module('update_worker')
broker=importlib.import_module('broker')
ROOT=fixture.ROOT


class WorkerTests(unittest.TestCase):
    def setUp(self):
        self.f=fixture.UpdateInputTests('test_document_free_preparation_never_writes_source_review_or_DB');self.addCleanup(self.f.doCleanups);self.f.setUp()
        self.registration=self.f.registered['registration_file'];self.r=self.f.r
        self.policy=copy.deepcopy(self.f.policy);self.policy.update(triggers=['manual','periodic','change'],interval_seconds=10,deadline_seconds=120)
        self.policy['retry']={'max_attempts':2,'initial_seconds':1,'max_seconds':2}
        with state.Journal(self.registration) as journal:journal.configure(self.policy)
        self.w=worker.Worker(self.registration);self.addCleanup(self.close)

    def close(self):
        if self.w:self.w.close()

    def reopen(self):
        self.close();self.w=worker.Worker(self.registration);return self.w

    def settled(self,trigger='manual',at=10):
        self.w.tick(trigger,at);end=time.monotonic()+40
        while self.w.journal.state['pending']:
            self.assertLess(time.monotonic(),end);time.sleep(.03);self.w.tick(trigger,at+.1)
        return self.w.journal.state

    def test_disabled_policy_never_opens_native_or_creates_DB(self):
        self.close()
        with state.Journal(self.registration) as j:j.configure(fixture.contract.defaults(self.r))
        self.reopen()
        with patch.object(worker,'Native',side_effect=AssertionError('must not open native')):
            self.w.tick('manual',1)
        self.assertFalse(Path(self.r['dataset_root']).exists());self.assertEqual(self.w.journal.state['attempts'],[])

    def test_actual_held_build_and_noop_use_real_reader_not_metadata(self):
        result=self.settled();self.assertEqual(result['phase'],'held');first=copy.deepcopy(self.w.journal.row(result['last_success']))
        self.w.read(first,first['job'],'Value observed source')
        self.assertFalse((Path(self.r['dataset_root'])/'current').exists());self.assertTrue(self.w.last_read['bodies'])
        self.assertTrue(any('return 7'in json.dumps(body) for body in self.w.last_read['bodies']))
        self.w.tick('manual',20);self.assertEqual(self.w.journal.state['phase'],'no_op');self.assertEqual(len(self.w.journal.state['attempts']),1)
        self.assertEqual(self.w.journal.row(first['id'])['read_receipt']['at'],20)

    def test_actual_publication_observes_current_and_exact_candidate_read(self):
        self.close();self.policy['activation']='activate_after_gate'
        with state.Journal(self.registration) as j:j.configure(self.policy)
        self.reopen();result=self.settled();row=self.w.journal.row(result['last_success'])
        self.assertEqual(result['phase'],'succeeded');self.assertEqual((Path(self.r['dataset_root'])/'current').readlink(),Path(row['request']['new_version']))
        self.assertEqual(self.w.last_read['coordinates'],row['job']['candidate']['coordinates'])

    def test_actual_source_change_rebuilds_without_semantic_review(self):
        self.settled();first=copy.deepcopy(self.w.journal.row(self.w.journal.state['last_success']));self.f.code.write_text('package fixture\nfunc Value()int{return 8}\n')
        self.settled(at=20);second=self.w.journal.row(self.w.journal.state['last_success'])
        self.assertEqual(len(self.w.journal.state['attempts']),2);self.assertNotEqual(first['input_id'],second['input_id'])
        self.w.read(second,second['job'],'Value observed source')
        self.assertTrue(any('return 8'in json.dumps(b) for b in self.w.last_read['bodies']))
        self.assertFalse((Path(self.r['state_root'])/'workflows').exists());self.assertFalse((self.f.source/'.cks').exists())

    def test_periodic_and_change_poll_not_due_do_not_open_native(self):
        self.settled('periodic',10);self.w.native.close();self.w.native=None;self.w.native_config=None
        with patch.object(worker,'Native',side_effect=AssertionError('not due')):
            result=self.w.tick('change',15);self.assertFalse(result['trigger_decision']['due'])
        self.assertEqual(len(self.w.journal.state['attempts']),1)

    def test_source_drift_before_forward_is_typed_refusal_not_unknown_admission(self):
        original=self.w.reconcile
        def drift(at):
            self.f.code.write_text('package fixture\nfunc Value()int{return 9}\n');return original(at)
        with patch.object(self.w,'reconcile',side_effect=drift):self.w.tick('manual',10)
        result=self.w.journal.state;row=self.w.journal.row(result['last_outcome'])
        self.assertEqual(row['outcome'],'refused');self.assertEqual(row['admission_error']['code'],'source_stale')
        self.assertIsNone(row['job']);self.assertIsNone(result['pending']);self.assertIsNone(result['last_success'])
        self.assertFalse(Path(self.r['dataset_root']).exists())

    def test_actual_lost_admission_response_replays_same_key_after_restart(self):
        original=self.w.tool;accepted=[]
        def lose(bundle,name,args):
            result=original(bundle,name,args)
            if name=='cks.dataset.build':accepted.append(result);raise manager.Failure('native_unavailable')
            return result
        with patch.object(self.w,'tool',side_effect=lose),self.assertRaisesRegex(manager.Failure,'native_unavailable'):self.w.tick('manual',10)
        row=copy.deepcopy(self.w.journal.row());self.assertIsNone(row['job']);self.assertEqual(len(accepted),1)
        self.reopen();self.w.tick('manual',11)
        restored=self.w.journal.row(row['id']);self.assertEqual(restored['job']['job_id'],accepted[0]['job_id'])
        self.assertEqual(restored['request'],row['request']);self.assertEqual(len(self.w.journal.state['attempts']),1)
        end=time.monotonic()+30
        while self.w.journal.state['pending']:
            self.assertLess(time.monotonic(),end);time.sleep(.02);self.w.tick('manual',11)

    def test_known_pending_job_is_checked_before_changed_live_source(self):
        self.w.tick('manual',10);row=copy.deepcopy(self.w.journal.row());self.assertIsNotNone(row['job'])
        self.reopen();self.f.code.write_text('package fixture\nfunc Value()int{return 12}\n')
        with patch.object(worker,'prepare',side_effect=AssertionError('must settle old job first')):self.w.tick('manual',11)
        self.assertEqual(self.w.journal.row(row['id'])['job']['job_id'],row['job']['job_id'])
        self.assertEqual(len(self.w.journal.state['attempts']),1)

    def test_ambiguous_native_persistence_refusal_preserves_pending_request(self):
        original=self.w.tool
        def refusal(bundle,name,args):
            if name=='cks.dataset.build':raise worker.Refusal({'code':'reconcile_required','phase':'accepted'})
            return original(bundle,name,args)
        with patch.object(self.w,'tool',side_effect=refusal),self.assertRaisesRegex(worker.Refusal,'reconcile_required'):self.w.tick('manual',10)
        self.assertIsNotNone(self.w.journal.state['pending']);self.assertIsNone(self.w.journal.row()['job'])
        self.assertEqual(len(self.w.journal.state['attempts']),1)

    def test_changed_bundle_bytes_refuse_exact_replay_before_native(self):
        original=self.w.reconcile
        with patch.object(self.w,'reconcile',side_effect=manager.Failure('controlled_transport_loss')):
            with self.assertRaises(manager.Failure):self.w.tick('manual',10)
        row=self.w.journal.row();Path(row['bundle']['config']).write_text('{}')
        with patch.object(worker,'Native',side_effect=AssertionError('must not open changed bundle')):
            with self.assertRaisesRegex(manager.Failure,'update_bundle_changed'):original(11)
        self.assertEqual(self.w.journal.row()['request'],row['request'])

    def test_intent_save_failure_prevents_native_build_forward(self):
        original=state.write_new;calls=[];tool=self.w.tool
        def fail(path,value):
            if isinstance(value,dict) and value.get('state',{}).get('pending'):raise OSError('controlled intent write failure')
            return original(path,value)
        def observe(bundle,name,args):calls.append(name);return tool(bundle,name,args)
        with patch.object(state,'write_new',side_effect=fail),patch.object(self.w,'tool',side_effect=observe):
            with self.assertRaises(OSError):self.w.tick('manual',10)
        self.assertNotIn('cks.dataset.build',calls);self.assertTrue(self.w.journal.poisoned)

    def test_canceled_actual_job_gets_distinct_bounded_retry_attempt(self):
        self.w.tick('manual',10);row=copy.deepcopy(self.w.journal.row());job=row['job'];self.assertFalse(job['terminal'])
        self.w.tool(row['bundle'],'cks.dataset.cancel',self.w.scope(job));end=time.monotonic()+30
        while self.w.journal.state['pending']:
            self.assertLess(time.monotonic(),end);time.sleep(.02);self.w.tick('manual',11)
        failed=self.w.journal.row(row['id']);self.assertIn(failed['outcome'],('canceled','failed'))
        self.settled('manual',13);retry=self.w.journal.row(self.w.journal.state['last_success'])
        self.assertEqual(retry['attempt'],2);self.assertEqual(retry['previous_attempt'],row['id']);self.assertEqual(retry['input_id'],row['input_id'])
        self.assertNotEqual(retry['request']['idempotency_key'],row['request']['idempotency_key'])

    def test_retry_deadline_overrides_long_periodic_check_interval(self):
        self.w.tick('manual',10);row=copy.deepcopy(self.w.journal.row());self.w.tool(row['bundle'],'cks.dataset.cancel',self.w.scope(row['job']))
        end=time.monotonic()+30
        while self.w.journal.state['pending']:
            self.assertLess(time.monotonic(),end);time.sleep(.02);self.w.tick('manual',11)
        self.assertEqual(self.w.journal.state['retry_at'],12)
        # Interval would otherwise be at20, but retry is due at12.
        self.w.tick('periodic',12);self.assertEqual(len(self.w.journal.state['attempts']),2)

    def test_partial_native_lock_recovery_is_only_for_prior_unobserved_intent(self):
        with patch.object(self.w,'reconcile',side_effect=manager.Failure('controlled_before_admission')):
            with self.assertRaises(manager.Failure):self.w.tick('manual',10)
        row=copy.deepcopy(self.w.journal.row());self.close();root=Path(self.r['dataset_root']);root.mkdir();(root/'.reindex.lock').write_text('')
        self.w=worker.Worker(self.registration);self.assertTrue(self.w.journal.recovery_lock_root)
        self.w.tick('manual',11);self.assertEqual(self.w.journal.row()['request'],row['request'])
        self.assertTrue((root/'.maintenance-project.json').exists())

    def test_unknown_nonempty_root_is_not_adopted_by_recovery_mode(self):
        with patch.object(self.w,'reconcile',side_effect=manager.Failure('controlled_before_admission')):
            with self.assertRaises(manager.Failure):self.w.tick('manual',10)
        self.close();root=Path(self.r['dataset_root']);root.mkdir();(root/'foreign.json').write_text('{}')
        with self.assertRaisesRegex(manager.Failure,'update_recovery_owner_required'):worker.Worker(self.registration)
        self.assertFalse((root/'.maintenance-project.json').exists())

    def test_lone_lock_without_prior_intent_cannot_bypass_owner_check(self):
        self.close();root=Path(self.r['dataset_root']);root.mkdir();(root/'.reindex.lock').write_text('')
        with self.assertRaisesRegex(manager.Failure,'update_recovery_owner_required'):worker.Worker(self.registration)

    def test_changed_registration_or_binary_is_not_recovery_permission(self):
        self.close();path=Path(self.r['state_root'])/'setup.json';path.write_text('{}')
        with self.assertRaisesRegex(manager.Failure,'config_changed'):worker.Worker(self.registration)

    def test_invalid_worker_budgets_do_not_open_registration(self):
        for limits in ((0,1,1),(1,0,1),(1,1,float('nan')),(1,1,0),(True,1,1)):
            with self.subTest(limits=limits),self.assertRaisesRegex(manager.Failure,'update_worker_budget_invalid'):
                worker.run('/unknown/registration.json','manual',*limits)

    def test_bounded_run_waits_for_one_actual_completed_cycle_and_stops(self):
        self.close();result=worker.run(self.registration,'manual',1,60,.02)
        self.assertEqual(result['iterations_completed'],1);self.assertEqual(result['state']['phase'],'held')
        self.assertIsNone(result['state']['pending']);self.assertFalse(result['scheduler_running']);self.assertEqual(result['stop_reason'],'completed')
        self.w=worker.Worker(self.registration)

    def test_typed_refusal_retry_budget_cannot_be_reset_after_restart(self):
        def reject_build(bundle,name,args):
            if name=='cks.dataset.build':raise worker.Refusal({'code':'writer_busy','phase':'accepted'})
            return original(bundle,name,args)
        original=self.w.tool
        with patch.object(self.w,'tool',side_effect=reject_build):self.w.tick('manual',10)
        self.reopen();original=self.w.tool
        with patch.object(self.w,'tool',side_effect=reject_build):self.w.tick('manual',11)
        self.assertEqual(self.w.journal.state['last_error']['code'],'retry_exhausted')
        self.assertEqual([r['outcome'] for r in self.w.journal.state['attempts']],['refused','refused'])
        with self.assertRaisesRegex(manager.Failure,'update_retry_invalid'):self.w.tick('manual',12)
        self.assertEqual(len(self.w.journal.state['attempts']),2);self.assertIsNone(self.w.journal.state['pending'])

    def test_actual_run_CLI_reports_observed_builds_without_ledger_only_claim(self):
        self.close()
        result=__import__('subprocess').run([sys.executable,str(ROOT/'plugins/knowledge-manager/scripts/manager.py'),'run-updates',
                    '--registration',self.registration,'--max-seconds','60','--poll-seconds','.02'],capture_output=True)
        self.assertEqual(result.returncode,0,result.stderr);output=json.loads(result.stdout)
        self.assertEqual(output['stage'],'foreground_native_update_worker');self.assertEqual(output['state']['phase'],'held')
        self.assertEqual(output['build_requests_forwarded_this_process'],1);self.assertEqual(len(output['observed_job_ids_this_process']),1)
        self.assertNotIn('native_job_executed_by_this_module',output);self.assertFalse(output['scheduler_running'])
        self.w=worker.Worker(self.registration)

    def test_expired_worker_budget_stops_before_input_projection_or_native(self):
        self.w.deadline=time.monotonic()-1
        with patch.object(worker,'Native',side_effect=AssertionError('budget expired')):
            with self.assertRaisesRegex(manager.Failure,'update_worker_budget_reached'):self.w.tick('manual',10)
        self.assertIsNone(self.w.journal.state['pending']);self.assertFalse(Path(self.r['dataset_root']).exists())
        self.assertFalse((Path(self.r['state_root'])/'updates/inputs').exists())
        self.w.deadline=None

    def test_signal_during_state_replace_reports_persistence_uncertainty(self):
        real_sync=state.sync_dir;original=self.w.tool;accepted=[];injected=[]
        def observe(bundle,name,args):
            result=original(bundle,name,args)
            if name=='cks.dataset.build':accepted.append(result)
            return result
        def interrupt(path):
            if accepted and not injected:
                injected.append(True);raise broker.GracefulShutdown()
            return real_sync(path)
        with patch.object(self.w,'tool',side_effect=observe),patch.object(state,'sync_dir',side_effect=interrupt):
            with self.assertRaises(broker.GracefulShutdown):self.w.tick('manual',10)
        self.assertTrue(self.w.journal.poisoned);self.close();output=self.w.describe()
        self.assertTrue(output['disk_state_requires_reconciliation'])
        self.assertEqual(output['state_snapshot_origin'],'last_confirmed_in_process_state')
        self.assertFalse(output['scheduler_running']);self.assertIsNotNone(output['state']['pending'])
        self.reopen();self.assertFalse(self.w.describe()['disk_state_requires_reconciliation'])
        self.assertEqual(self.w.journal.row()['job']['job_id'],accepted[0]['job_id'])
        self.w.tick('manual',11);self.assertEqual(len(self.w.journal.state['attempts']),1)

    def test_second_worker_or_reviewed_host_cannot_overlap(self):
        for start in (lambda:worker.Worker(self.registration),lambda:broker.Broker(self.registration)):
            with self.assertRaisesRegex(manager.Failure,'collection_busy'):start()


if __name__=='__main__':unittest.main(verbosity=2)
