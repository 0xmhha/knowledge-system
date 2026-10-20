"""Actual native worker cycles and injected admission/IO/recovery boundaries."""
import argparse
import copy
import importlib
import json
import os
import select
import signal
import subprocess
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

    def test_new_signal_during_owned_cleanup_does_not_interrupt_quiescence(self):
        self.close()
        with state.Journal(self.registration)as j:j.configure(fixture.contract.defaults(self.r))
        original=worker.Worker.close;closed=[]
        def finish(instance):
            signal.raise_signal(signal.SIGTERM)
            original(instance);closed.append(True)
        with patch.object(worker.Worker,'close',finish):result=worker.run(self.registration,'manual',1,30,.02)
        self.assertEqual(closed,[True]);self.assertFalse(result['scheduler_running'])
        self.assertEqual(result['worker_stop_outcome'],'closed')
        self.w=worker.Worker(self.registration)

    def test_periodic_capture_race_defers_then_builds_without_losing_baseline(self):
        self.settled();baseline=self.w.journal.state['last_success'];self.close()
        self.policy['interval_seconds']=1
        with state.Journal(self.registration)as j:j.configure(self.policy)
        original=worker.prepare;calls=[]
        def changed(*args,**kwargs):
            calls.append(True)
            if len(calls)==1:raise manager.Failure('source_changed')
            return original(*args,**kwargs)
        with patch.object(worker,'prepare',side_effect=changed):result=worker.run(self.registration,'periodic',80,10,.03)
        self.assertGreaterEqual(len(calls),2);self.assertIsNotNone(result['state']['last_success'])
        self.assertTrue(any(row['id']==baseline and row['outcome']=='held'for row in result['state']['attempts']))
        self.assertEqual(len(result['state']['attempts']),2);self.assertFalse(result['scheduler_running'])
        self.w=worker.Worker(self.registration)

    def test_second_worker_or_reviewed_host_cannot_overlap(self):
        for start in (lambda:worker.Worker(self.registration),lambda:broker.Broker(self.registration)):
            with self.assertRaisesRegex(manager.Failure,'collection_busy'):start()


class MCPHostTests(unittest.TestCase):
    def setUp(self):
        self.h=None
        WorkerTests.setUp(self)
        self.w.close();self.w=None
        self.policy['interval_seconds']=1
        with state.Journal(self.registration)as j:j.configure(self.policy)
        self.host_module=importlib.import_module('update_host')
        self.h=self.host_module.Host(self.registration)

    def close(self):
        if getattr(self,'h',None):self.h.close()
        if getattr(self,'w',None):self.w.close()

    def call(self,operation,args=None):return self.h.call(self.host_module.PREFIX+operation,args or{})

    def wait_success(self,prior=None):
        end=time.monotonic()+35
        while True:
            result=self.call('status')
            if result['last_success']and result['last_success']['intent_id']!=prior:return result
            self.assertLess(time.monotonic(),end,result)
            self.assertIsNone(result['process']['exit_code'],result)
            time.sleep(.03)

    def metadata(self,operation='serve-updates',env_mode=None):
        self.h.close()
        env=os.environ.copy();env.pop('CKS_MANAGER_MODE',None)
        if env_mode is not None:env['CKS_MANAGER_MODE']=env_mode
        requests=[{'jsonrpc':'2.0','id':1,'method':'initialize','params':{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'owned-update-mode-test','version':'1'}}},
                  {'jsonrpc':'2.0','id':2,'method':'tools/list','params':{}}]
        result=subprocess.run([sys.executable,str(ROOT/'plugins/knowledge-manager/scripts/manager.py'),operation,'--registration',self.registration],
                    input=b''.join(manager.canonical(r)+b'\n'for r in requests),capture_output=True,timeout=30,env=env)
        return result,[json.loads(line)for line in result.stdout.splitlines()]

    def test_updates_metadata_exposes_only_six_scoped_tools_and_no_DB(self):
        definitions=self.host_module.tools();self.assertEqual(len(definitions),6)
        self.assertTrue(all(t['name'].startswith('cks.manager.update_')for t in definitions))
        result=self.call('status');self.assertFalse(result['process']['running'])
        self.assertFalse(result['publication_or_serving_read_inferred']);self.assertFalse(Path(self.r['dataset_root']).exists())
        template=self.call('policy_template');self.assertFalse(template['enabled']);self.assertEqual(template['activation'],'hold_for_review')

    def test_updates_disabled_policy_refuses_start_without_spawning(self):
        with state.Journal(self.registration)as j:j.configure(fixture.contract.defaults(self.r))
        with patch.object(self.host_module.subprocess,'Popen',side_effect=AssertionError('must not spawn')):
            with self.assertRaisesRegex(manager.Failure,'update_trigger_not_enabled'):self.call('start',{'trigger':'manual'})
        self.assertFalse(Path(self.r['dataset_root']).exists())

    def test_updates_unknown_fields_and_invalid_budgets_refused(self):
        for args in ({'trigger':'manual','max_seconds':True},{'trigger':'manual','max_seconds':0},{'trigger':'on_every_word'},
                     {'trigger':'manual','poll_seconds':float('nan')},{'trigger':'manual','command':['bad']}):
            with self.subTest(args=args),self.assertRaises(manager.Failure):self.call('start',args)
        self.assertIsNone(self.h.child)

    def test_updates_foreign_policy_is_not_adopted(self):
        policy=copy.deepcopy(self.policy);policy['project_id']='foreign';path=self.f.root/'foreign-policy.json';path.write_text(json.dumps(policy))
        with self.assertRaisesRegex(manager.Failure,'update_project_mismatch'):self.call('configure',{'policy_file':str(path)})
        self.assertIsNone(self.h.child)

    def test_updates_duplicate_host_uses_its_own_exclusive_session_lease(self):
        with self.assertRaisesRegex(manager.Failure,'collection_busy'):self.host_module.Host(self.registration)
        with state.Journal(self.registration):pass

    def test_updates_reviewed_writer_blocks_competing_start(self):
        reviewed=broker.Broker(self.registration)
        try:
            with self.assertRaisesRegex(manager.Failure,'collection_busy'):self.call('start',{'trigger':'manual'})
        finally:reviewed.close()
        self.assertIsNone(self.h.child)

    def test_updates_active_child_blocks_second_start_and_reconfigure(self):
        self.call('start',{'trigger':'periodic','max_seconds':60,'max_iterations':10000,'poll_seconds':.03})
        original=self.h.child.pid
        with self.assertRaisesRegex(manager.Failure,'update_worker_already_running'):self.call('start',{'trigger':'manual'})
        path=self.f.root/'policy.json';path.write_text(json.dumps(self.policy))
        with self.assertRaisesRegex(manager.Failure,'update_worker_already_running'):self.call('configure',{'policy_file':str(path)})
        self.assertEqual(self.h.child.pid,original);self.call('stop')
        self.assertEqual(self.h.child.returncode,0)

    def test_updates_actual_read_while_periodic_writer_and_retained_old_input(self):
        self.call('start',{'trigger':'periodic','max_seconds':60,'max_iterations':10000,'poll_seconds':.03})
        first=self.wait_success()['last_success'];job=first['job'];args={'job_id':job['job_id'],'expected_coordinates':job['candidate']['coordinates'],'prompt':'Value observed source'}
        pack1=self.call('read',args)['pack'];self.assertTrue(any('return 7'in json.dumps(b)for b in pack1['bodies']))
        with self.assertRaisesRegex(manager.Failure,'collection_busy'):state.Journal(self.registration)
        self.f.code.write_text('package fixture\nfunc Value()int{return 8}\n')
        second=self.wait_success(first['intent_id'])['last_success'];fresh=self.call('read',{'job_id':second['job']['job_id'],'expected_coordinates':second['job']['candidate']['coordinates'],'prompt':'Value observed source'})
        self.assertTrue(any('return 8'in json.dumps(b)for b in fresh['pack']['bodies']))
        old=self.call('read',args)['pack'];self.assertEqual(old['bodies'],pack1['bodies']);self.assertEqual(old['citations'],pack1['citations'])
        self.assertFalse(fresh['publication_inferred_from_read']);self.call('stop');self.assertEqual(self.h.child.returncode,0)

    def test_updates_foreign_coordinate_refused_before_opening_reader(self):
        self.call('start',{'trigger':'manual','max_seconds':60,'poll_seconds':.02})
        row=self.wait_success()['last_success'];coordinates=copy.deepcopy(row['job']['candidate']['coordinates']);coordinates['project_id']='foreign'
        with patch.object(self.host_module,'read_observed',side_effect=AssertionError('must not open reader')):
            with self.assertRaisesRegex(manager.Failure,'update_reader_coordinate_mismatch'):
                self.call('read',{'job_id':row['job']['job_id'],'expected_coordinates':coordinates,'prompt':'Value'})

    def test_updates_unowned_recorded_pid_is_never_signaled(self):
        run_id='a'*32;path=self.h.runs/run_id;path.mkdir()
        manager.write_new(path/'owner.json',{'schema_version':1,'project_id':self.r['project_id'],'token':self.r['token']})
        manager.write_new(path/'launch.json',{'run_id':run_id,'registration_digest':manager.digest(self.r),'pid':os.getpid()})
        self.h.last_run=run_id
        with patch.object(self.host_module.os,'kill',side_effect=AssertionError('never signal recorded PID')):
            result=self.call('stop')['status']['process']
        self.assertFalse(result['owned_by_this_host']);self.assertIsNone(result['running']);self.assertTrue(result['previous_unowned_pid_never_signaled'])

    def test_updates_run_history_limit_preserves_old_directories(self):
        for n in range(128):(self.h.runs/('%032x'%n)).mkdir()
        with self.assertRaisesRegex(manager.Failure,'update_host_run_limit'):self.call('start',{'trigger':'manual'})
        self.assertEqual(len([p for p in self.h.runs.iterdir()if p.is_dir()]),128);self.assertIsNone(self.h.child)

    def test_updates_changed_code_blocks_new_work_but_not_safe_stop_status(self):
        key=next(iter(self.h.code_pin));self.h.code_pin[key]='f'*64
        with self.assertRaisesRegex(manager.Failure,'manager_code_changed'):self.call('start',{'trigger':'manual'})
        self.assertEqual(self.call('status')['project_id'],self.r['project_id']);self.call('stop')

    def test_updates_explicit_CLI_mode_metadata_has_six_tools_and_normal_EOF(self):
        result,frames=self.metadata()
        self.assertEqual(result.returncode,0,result.stderr);self.assertEqual(frames[0]['result']['serverInfo']['name'],'knowledge-manager-updates')
        self.assertEqual(len(frames[1]['result']['tools']),6);self.assertFalse(Path(self.r['dataset_root']).exists())

    def test_updates_plugin_environment_selects_same_mode(self):
        result,frames=self.metadata('serve','updates');self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual(frames[0]['result']['serverInfo']['name'],'knowledge-manager-updates');self.assertEqual(len(frames[1]['result']['tools']),6)

    def test_updates_default_reviewed_mode_still_exposes_eighteen_tools(self):
        result,frames=self.metadata('serve');self.assertEqual(result.returncode,0,result.stderr)
        self.assertEqual(frames[0]['result']['serverInfo']['name'],'knowledge-manager');self.assertEqual(len(frames[1]['result']['tools']),18)

    def test_updates_invalid_environment_mode_does_not_fall_back(self):
        result,frames=self.metadata('serve','foreign');self.assertEqual(result.returncode,2);self.assertEqual(frames,[])
        self.assertEqual(json.loads(result.stderr)['code'],'manager_mode_invalid')

    def test_updates_legacy_mutation_alias_not_available(self):
        with self.assertRaisesRegex(manager.Failure,'unknown_tool'):self.h.call('cks.dataset.build',{})
        with self.assertRaisesRegex(manager.Failure,'unknown_tool'):self.h.call('cks.ops.index',{})
        self.assertFalse(Path(self.r['dataset_root']).exists())

    def test_updates_actual_MCP_start_read_and_EOF_stops_owned_worker(self):
        self.h.close();process=subprocess.Popen([sys.executable,str(ROOT/'plugins/knowledge-manager/scripts/manager.py'),'serve-updates','--registration',self.registration],stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.PIPE)
        counter=0
        def request(method,params):
            nonlocal counter
            counter+=1;process.stdin.write(manager.canonical({'jsonrpc':'2.0','id':counter,'method':method,'params':params})+b'\n');process.stdin.flush()
            self.assertTrue(select.select([process.stdout],[],[],30)[0],'MCP response timeout')
            frame=json.loads(process.stdout.readline(broker.FRAME_LIMIT+1));self.assertEqual(frame['id'],counter);return frame['result']
        def tool(operation,args=None):
            result=request('tools/call',{'name':self.host_module.PREFIX+operation,'arguments':args or{}})
            self.assertFalse(result.get('isError'),result);return result['structuredContent']
        try:
            initialized=request('initialize',{'protocolVersion':'2024-11-05','capabilities':{},'clientInfo':{'name':'owned-live-update-test','version':'1'}})
            self.assertEqual(initialized['serverInfo']['name'],'knowledge-manager-updates')
            started=tool('start',{'trigger':'periodic','max_seconds':60,'max_iterations':10000,'poll_seconds':.03})
            self.assertFalse(started['actual_job_or_read_success_inferred']);run_id=started['process']['run_id'];end=time.monotonic()+35
            while True:
                status=tool('status')
                if status['last_success']:break
                self.assertLess(time.monotonic(),end,status);time.sleep(.03)
            job=status['last_success']['job'];pack=tool('read',{'job_id':job['job_id'],'expected_coordinates':job['candidate']['coordinates'],'prompt':'Value observed source'})['pack']
            self.assertTrue(any('return 7'in json.dumps(body)for body in pack['bodies']))
            process.stdin.close();self.assertEqual(process.wait(timeout=50),0,process.stderr.read())
            exit_record=manager.strict_read(Path(self.r['state_root'])/'updates/mcp-host/runs'/run_id/'exit.json')
            self.assertEqual(exit_record['exit_code'],0)
        finally:
            if process.poll()is None:
                process.stdin.close()
                try:process.wait(timeout=50)
                except subprocess.TimeoutExpired:process.kill();process.wait(timeout=10)
            process.stdout.close();process.stderr.close()


if __name__=='__main__':unittest.main(verbosity=2)
