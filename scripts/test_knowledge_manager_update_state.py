"""Disk/lease/intent fault tests. Simulated job observations are not native runs."""
import copy
import importlib
import json
import os
from pathlib import Path
import subprocess
import sys
import unittest
from unittest.mock import patch

import test_knowledge_manager_update_inputs as fixture
manager=fixture.manager
inputs=fixture.inputs
state=importlib.import_module('update_state')
ROOT=fixture.ROOT


class UpdateStateTests(unittest.TestCase):
    def setUp(self):
        self.f=fixture.UpdateInputTests('test_document_free_preparation_never_writes_source_review_or_DB')
        self.addCleanup(self.f.doCleanups);self.f.setUp()
        self.registration=self.f.registered['registration_file'];self.r=self.f.r
        self.policy=copy.deepcopy(self.f.policy);self.policy['retry']={'max_attempts':2,'initial_seconds':3,'max_seconds':10}
        self.j=state.Journal(self.registration);self.addCleanup(self.close)
        self.j.configure(self.policy)
        self.prepared=inputs.prepare(self.j.policy(),self.r)
        self.bundle_dir=self.j.directory/'bundles';self.bundle_dir.mkdir()
        self.config=self.bundle_dir/'owned.json';manager.write_new(self.config,{'controlled_state_test':'not_native_config'})
        self.bundle={'config':str(self.config),'files':{str(self.config):manager.file_hash(self.config)}}
        self.profile={'project_id':self.r['project_id'],'profile_id':self.r['profile_id'],
                      'profile_digest':'1'*64,'input_digest':'2'*64,'embedding_digest':'3'*64,
                      'source_pin':{'mode':'snapshot-only','snapshot_id':'4'*64,'base_commit':''},
                      'observed_base':{'state':'absent'},'required_gates':['graph','vector','identity']}

    def close(self):
        if self.j:self.j.close()

    def reopen(self):
        self.close();self.j=state.Journal(self.registration);return self.j

    def intend(self,at=10,prior=None):
        self.assertTrue(self.j.checking('manual',at)['due'])
        return self.j.intend(self.prepared['input_id'],self.bundle,self.profile,at,prior)

    def job(self,row=None,terminal=False,failed=False,published=False):
        row=row or self.j.row()
        coordinates={'project_id':self.r['project_id'],'dataset_id':'5'*64,'snapshot_id':'4'*64,'source_mode':'snapshot-only','base_commit':''}
        return {'schema_version':1,'project_id':self.r['project_id'],'profile_id':self.r['profile_id'],
                'job_id':'controlled-job-'+row['id'],'request_digest':row['request_digest'],'target_version':row['request']['new_version'],
                'revision':2 if terminal else 1,'terminal':terminal,'state':'failed' if failed else ('succeeded' if published else 'held_for_review') if terminal else 'running',
                'build_complete':terminal and not failed,'activated':published,'error':{'code':'controlled_failure'} if failed else None,
                'candidate':{'coordinates':coordinates,'version':row['request']['new_version'],'input_digest':'2'*64,'embedding_digest':'3'*64},
                'gates':{g:'passed' for g in self.profile['required_gates']}}

    def pack(self,job):
        c=job['candidate']['coordinates']
        return {'coordinates':c,'bodies':[{'controlled':'observation_not_native_read'}],
                'citations':[{'project_id':c['project_id'],'dataset_id':c['dataset_id']}]}

    def success(self):
        self.intend();job=self.job(terminal=True);self.j.observe(job,11);self.j.settle(12,read_pack=self.pack(job));return job

    def failure(self,at=10,prior=None):
        self.intend(at,prior);job=self.job(terminal=True,failed=True);self.j.observe(job,at+1);self.j.settle(at+2);return job

    def rewrite(self,change):
        envelope=json.loads(self.j.path.read_text());change(envelope['state'])
        envelope['checksum']=state.domain_hash(state.DOMAIN,envelope['state']);self.j.path.write_text(json.dumps(envelope))

    def test_configuration_pins_policy_without_build_scheduler_or_source_writes(self):
        self.assertEqual(self.j.state['phase'],'idle');self.assertIsNone(self.j.state['pending'])
        self.assertFalse(self.j.describe()['scheduler_running']);self.assertFalse(Path(self.r['dataset_root']).exists())
        self.assertFalse((self.f.source/'.cks').exists());self.assertFalse((Path(self.r['state_root'])/'workflows').exists())

    def test_default_disabled_does_not_start_check(self):
        self.j.configure(fixture.contract.defaults(self.r))
        self.assertFalse(self.j.checking('manual',1)['due']);self.assertEqual(self.j.state['phase'],'idle')

    def test_exact_intent_is_durable_before_any_admission(self):
        row=self.intend();self.assertIsNone(row['job']);self.assertEqual(self.j.state['phase'],'admitting')
        recovered=self.reopen().row();self.assertEqual(row,recovered)
        self.assertFalse(Path(self.r['dataset_root']).exists());self.assertEqual(row['request_digest'],state.request_digest(row['request']))

    def test_admission_ambiguity_preserves_original_request_and_blocks_new_intent(self):
        row=self.intend();self.j.block('native_unavailable');self.reopen()
        self.assertEqual(self.j.row(),row)
        for action in (lambda:self.j.checking('manual',20),lambda:self.j.configure(self.policy),lambda:self.j.intend(self.prepared['input_id'],self.bundle,self.profile,20)):
            with self.assertRaisesRegex(manager.Failure,'pending_update_requires_reconciliation'):action()

    def test_broker_and_worker_share_one_live_writer_lease(self):
        with self.assertRaisesRegex(manager.Failure,'collection_busy'):state.Journal(self.registration)
        broker=importlib.import_module('broker')
        with self.assertRaisesRegex(manager.Failure,'collection_busy'):broker.Broker(self.registration)

    def test_released_pending_worker_blocks_reviewed_broker_before_native_start(self):
        self.intend();self.close();broker=importlib.import_module('broker')
        with patch.object(broker,'Native',side_effect=AssertionError('native must not start')):
            with self.assertRaisesRegex(manager.Failure,'pending_update_requires_reconciliation'):broker.Broker(self.registration)
        self.reopen()

    def test_native_job_identity_cannot_change(self):
        row=self.intend();job=self.job();self.j.observe(job,11)
        for key,value in [('project_id','foreign'),('profile_id','foreign'),('request_digest','f'*64),('target_version','foreign'),('job_id','different')]:
            with self.subTest(key=key):
                bad=copy.deepcopy(job);bad[key]=value
                with self.assertRaises(manager.Failure):self.j.observe(bad,12)
        self.assertEqual(self.j.row()['id'],row['id'])

    def test_same_revision_different_result_and_regression_refused(self):
        self.intend();job=self.job();self.j.observe(job,11)
        for change in (lambda j:j.update(state='queued'),lambda j:j.update(revision=0)):
            bad=copy.deepcopy(job);change(bad)
            with self.assertRaises(manager.Failure):self.j.observe(bad,12)

    def test_terminal_result_cannot_reopen_or_mutate(self):
        self.intend();job=self.job(terminal=True);self.j.observe(job,11)
        bad=self.job();bad['revision']=3
        with self.assertRaisesRegex(manager.Failure,'update_job_regressed'):self.j.observe(bad,12)
        self.j.observe(job,12);self.assertEqual(self.j.row()['job'],job)

    def test_nonterminal_ack_never_becomes_success(self):
        self.intend();self.j.observe(self.job(),11)
        with self.assertRaisesRegex(manager.Failure,'update_job_not_terminal'):self.j.settle(12)
        self.assertIsNone(self.j.state['last_success']);self.assertIsNotNone(self.j.state['pending'])

    def test_held_success_requires_fresh_read_not_only_completed_metadata(self):
        self.intend();job=self.job(terminal=True);self.j.observe(job,11)
        with self.assertRaisesRegex(manager.Failure,'update_read_not_verified'):self.j.settle(12)
        self.assertIsNotNone(self.j.state['pending']);self.j.settle(12,read_pack=self.pack(job))
        self.assertEqual(self.j.state['phase'],'held');self.assertFalse(job['activated'])
        self.assertEqual(self.reopen().row(self.j.state['last_success'])['job'],job)

    def test_cross_project_read_or_candidate_input_refused(self):
        self.intend();job=self.job(terminal=True);self.j.observe(job,11)
        bad=self.pack(job);bad['citations'][0]['project_id']='foreign'
        with self.assertRaisesRegex(manager.Failure,'update_read_not_verified'):self.j.settle(12,read_pack=bad)
        self.assertIsNone(self.j.state['last_success'])

    def test_candidate_model_pin_mismatch_not_success(self):
        self.intend();job=self.job(terminal=True);job['candidate']['embedding_digest']='9'*64;self.j.observe(job,11)
        with self.assertRaisesRegex(manager.Failure,'update_candidate_invalid'):self.j.settle(12,read_pack=self.pack(job))

    def test_missing_required_gate_not_success(self):
        self.intend();job=self.job(terminal=True);job['gates']['identity']='not_run';self.j.observe(job,11)
        with self.assertRaisesRegex(manager.Failure,'update_gate_not_passed'):self.j.settle(12,read_pack=self.pack(job))

    def test_publication_requires_actual_current_coordinates(self):
        policy=copy.deepcopy(self.policy);policy['activation']='activate_after_gate';self.j.configure(policy)
        self.prepared=inputs.prepare(self.j.policy(),self.r);self.intend();job=self.job(terminal=True,published=True);self.j.observe(job,11)
        with self.assertRaisesRegex(manager.Failure,'update_publication_not_observed'):self.j.settle(12,read_pack=self.pack(job))
        self.j.settle(12,current_coordinates=job['candidate']['coordinates'],read_pack=self.pack(job));self.assertEqual(self.j.state['phase'],'succeeded')

    def test_failure_does_not_erase_last_success(self):
        self.success();successful=self.j.state['last_success'];self.f.code.write_text('package fixture\nfunc Value()int{return 8}\n')
        self.prepared=inputs.prepare(self.j.policy(),self.r,self.prepared['input_id']);self.failure(20)
        self.assertEqual(self.j.state['last_success'],successful);self.assertEqual(self.j.state['phase'],'retry_wait');self.assertEqual(self.j.state['retry_at'],25)

    def test_retry_deadline_survives_restart_and_clock_skew(self):
        self.failure();self.reopen()
        self.assertFalse(self.j.checking('manual',1)['due']);self.assertFalse(self.j.checking('manual',14)['due'])
        self.assertEqual(self.j.state['retry_at'],15)

    def test_retry_uses_new_key_and_links_exact_failed_cycle(self):
        self.failure();old=copy.deepcopy(self.j.row(self.j.state['last_outcome']));retry=self.intend(15,old['id'])
        self.assertEqual(retry['cycle_id'],old['cycle_id']);self.assertEqual(retry['attempt'],2)
        self.assertNotEqual(retry['request']['idempotency_key'],old['request']['idempotency_key']);self.assertNotEqual(retry['request']['new_version'],old['request']['new_version'])
        self.assertEqual(retry['input_id'],old['input_id'])

    def test_new_cycle_cannot_evade_same_input_retry_budget(self):
        self.failure()
        with self.assertRaisesRegex(manager.Failure,'update_retry_requires_prior_attempt'):self.intend(15)

    def test_exhausted_cycle_blocks_further_same_input_retry(self):
        self.failure();prior=self.j.state['last_outcome'];self.failure(15,prior);last=self.j.state['last_outcome']
        self.assertEqual(self.j.state['phase'],'blocked');self.assertEqual(self.j.state['last_error']['code'],'retry_exhausted')
        self.j.checking('manual',100)
        with self.assertRaisesRegex(manager.Failure,'update_retry_invalid'):self.j.intend(self.prepared['input_id'],self.bundle,self.profile,100,last)

    def test_changed_input_is_new_cycle_not_relabelled_retry(self):
        self.failure();prior=self.j.state['last_outcome'];self.f.code.write_text('package fixture\nfunc Value()int{return 9}\n')
        self.prepared=inputs.prepare(self.j.policy(),self.r,self.prepared['input_id']);self.j.checking('manual',15)
        with self.assertRaisesRegex(manager.Failure,'update_retry_invalid'):self.j.intend(self.prepared['input_id'],self.bundle,self.profile,15,prior)
        new=self.j.intend(self.prepared['input_id'],self.bundle,self.profile,15);self.assertEqual(new['attempt'],1);self.assertEqual(new['cycle_id'],new['id'])

    def test_cancel_is_distinct_failure_and_retry_bounded(self):
        self.intend();job=self.job(terminal=True,failed=True);job['state']='canceled';self.j.observe(job,11);self.j.settle(12)
        self.assertEqual(self.j.row(self.j.state['last_outcome'])['outcome'],'canceled');self.assertEqual(self.j.state['last_error']['code'],'native_canceled')

    def test_failed_job_cannot_claim_activation(self):
        self.intend();job=self.job(terminal=True,failed=True);job['activated']=True;self.j.observe(job,11)
        with self.assertRaisesRegex(manager.Failure,'update_failed_job_activated'):self.j.settle(12)

    def test_noop_requires_reader_and_native_pins(self):
        job=self.success();self.j.checking('manual',20)
        with self.assertRaisesRegex(manager.Failure,'update_noop_not_verified'):self.j.no_op(self.prepared,self.profile,job,20)
        self.j.no_op(self.prepared,self.profile,job,20,read_pack=self.pack(job));self.assertEqual(self.j.state['phase'],'no_op')
        self.assertEqual(len(self.j.state['attempts']),1)

    def test_held_noop_never_claims_publication(self):
        job=self.success();self.j.checking('manual',20);self.j.no_op(self.prepared,self.profile,job,20,read_pack=self.pack(job))
        self.assertEqual(self.j.row(self.j.state['last_success'])['outcome'],'held');self.assertFalse(job['activated'])

    def test_changed_model_or_input_cannot_noop(self):
        job=self.success();self.j.checking('manual',20)
        bad=copy.deepcopy(self.profile);bad['embedding_digest']='9'*64
        with self.assertRaisesRegex(manager.Failure,'update_noop_not_verified'):self.j.no_op(self.prepared,bad,job,20,read_pack=self.pack(job))

    def test_external_current_change_invalidates_published_noop(self):
        policy=copy.deepcopy(self.policy);policy['activation']='activate_after_gate';self.j.configure(policy);self.prepared=inputs.prepare(self.j.policy(),self.r)
        self.intend();job=self.job(terminal=True,published=True);self.j.observe(job,11);self.j.settle(12,job['candidate']['coordinates'],self.pack(job));self.j.checking('manual',20)
        with self.assertRaisesRegex(manager.Failure,'update_noop_not_verified'):self.j.no_op(self.prepared,self.profile,job,20,{'project_id':'foreign'},self.pack(job))

    def test_saved_state_corruption_and_duplicate_keys_refused(self):
        data=self.j.path.read_bytes();self.close();self.j.path.write_bytes(data.replace(b'"phase": "idle"',b'"phase": "held"'))
        with self.assertRaises(manager.Failure):state.Journal(self.registration)
        self.j.path.write_text('{"state":{},"state":{},"checksum":"x"}')
        with self.assertRaises(manager.Failure):state.Journal(self.registration)

    def test_valid_checksum_foreign_owner_or_unknown_field_refused(self):
        for change in (lambda s:s.update(project_id='foreign'),lambda s:s.update(unknown=True)):
            data=self.j.path.read_bytes();self.rewrite(change)
            with self.assertRaises(manager.Failure):state.read_state(self.j.path,self.r)
            self.j.path.write_bytes(data)

    def test_valid_checksum_cannot_rewrite_request_without_matching_digest(self):
        self.intend();self.rewrite(lambda s:s['attempts'][0]['request'].update(activation='activate_after_gate'))
        with self.assertRaises(manager.Failure):state.read_state(self.j.path,self.r)

    def test_valid_checksum_cannot_drop_pending_obligation(self):
        self.intend();self.rewrite(lambda s:s.update(pending=None))
        with self.assertRaises(manager.Failure):state.read_state(self.j.path,self.r)

    def test_state_symlink_or_hardlink_refused(self):
        path=self.j.path;data=path.read_bytes();self.close();path.rename(path.with_suffix('.saved'));path.symlink_to(path.with_suffix('.saved'))
        with self.assertRaises(manager.Failure):state.Journal(self.registration)
        path.unlink();path.write_bytes(data);os.link(path,self.f.root/'hardlink')
        with self.assertRaises(manager.Failure):state.Journal(self.registration)

    def test_external_state_replacement_is_compare_and_swap_failure(self):
        self.rewrite(lambda s:s.update(revision=s['revision']+1))
        with self.assertRaisesRegex(manager.Failure,'update_state_changed'):self.j.block('controlled_failure')

    def test_write_failure_poisons_process_without_forwarding_any_job(self):
        before=copy.deepcopy(self.j.state)
        with patch.object(state,'write_new',side_effect=OSError('controlled disk full')):
            with self.assertRaises(OSError):self.j.checking('manual',10)
        self.assertEqual(self.j.state,before)
        with self.assertRaisesRegex(manager.Failure,'update_reconciliation_required'):self.j.block('another_attempt')
        self.assertEqual(self.reopen().state,before)

    def test_fsync_after_replace_failure_requires_restart_reconciliation(self):
        before=copy.deepcopy(self.j.state)
        with patch.object(state,'sync_dir',side_effect=OSError('controlled sync uncertainty')):
            with self.assertRaises(OSError):self.j.checking('manual',10)
        self.assertEqual(self.j.state,before);self.assertTrue(self.j.poisoned)
        with self.assertRaisesRegex(manager.Failure,'update_reconciliation_required'):self.j.block('another_attempt')
        self.reopen();self.assertEqual(self.j.state['phase'],'checking');self.assertEqual(self.j.state['revision'],before['revision']+1)

    def test_intent_commit_failure_does_not_produce_forwardable_intent(self):
        self.j.checking('manual',10)
        with patch.object(state,'write_new',side_effect=OSError('controlled intent persistence failure')):
            with self.assertRaises(OSError):self.j.intend(self.prepared['input_id'],self.bundle,self.profile,10)
        self.assertIsNone(self.reopen().state['pending']);self.assertEqual(self.j.state['attempts'],[])

    def test_bundle_changed_or_foreign_path_refused_before_intent(self):
        self.j.checking('manual',10);self.config.write_text('{}')
        with self.assertRaisesRegex(manager.Failure,'update_bundle_changed'):self.j.intend(self.prepared['input_id'],self.bundle,self.profile,10)
        self.assertIsNone(self.j.state['pending'])
        bad={'config':str(self.f.source/'value.go'),'files':{str(self.f.source/'value.go'):'0'*64}}
        with self.assertRaises(manager.Failure):self.j.intend(self.prepared['input_id'],bad,self.profile,10)

    def test_pending_policy_reconfiguration_is_refused(self):
        self.intend();changed=copy.deepcopy(self.policy);changed['enabled']=False
        with self.assertRaisesRegex(manager.Failure,'pending_update_requires_reconciliation'):self.j.configure(changed)

    def test_removed_material_only_allowed_for_already_applied_policy(self):
        root=self.f.material();self.policy['collection']['materials']=self.f.policy['collection']['materials'];self.j.configure(self.policy)
        (root/'mapping.yaml').unlink();root.rmdir();self.j.configure(self.policy)
        changed=copy.deepcopy(self.policy);changed['interval_seconds']=77
        with self.assertRaisesRegex(manager.Failure,'update_material_invalid'):self.j.configure(changed)

    def test_policy_bytes_changed_under_owned_digest_refused(self):
        path=Path(self.j.state['policy']['path']);value=json.loads(path.read_text());value['enabled']=False;path.write_text(json.dumps(value))
        with self.assertRaisesRegex(manager.Failure,'update_policy_changed'):self.j.policy()

    def test_unresolved_reviewed_broker_jobs_block_update_intent(self):
        broker=Path(self.r['state_root'])/'broker/state.json'
        value={'project_id':self.r['project_id'],'profile_id':self.r['profile_id'],'registration_token':self.r['token'],
               'pending_build':{'request':'controlled unknown admission'},'jobs':{}}
        manager.write_new(broker,{'state':value,'checksum':state.domain_hash('cks.manager-broker-state.v1',value)})
        with self.assertRaisesRegex(manager.Failure,'reviewed_job_requires_reconciliation'):self.j.checking('manual',10)
        self.assertFalse(Path(self.r['dataset_root']).exists())

    def test_history_limit_does_not_delete_or_overwrite_prior_intents(self):
        self.success();template=copy.deepcopy(self.j.state['attempts'][0]);next_state=copy.deepcopy(self.j.state)
        for n in range(127):
            row=copy.deepcopy(template);row['id']=('%032x'%(n+1));row['cycle_id']=row['id'];row['request']['new_version']='manager-'+row['id'];row['request']['idempotency_key']='manager-update-'+row['id'];row['request_digest']=state.request_digest(row['request'])
            row['job']['request_digest']=row['request_digest'];row['job']['target_version']=row['request']['new_version'];row['job']['candidate']['version']=row['request']['new_version'];row['job']['job_id']='controlled-'+row['id'];next_state['attempts'].append(row)
        self.j.commit(next_state);self.j.checking('manual',20)
        with self.assertRaisesRegex(manager.Failure,'update_history_limit'):self.j.intend(self.prepared['input_id'],self.bundle,self.profile,20)
        self.assertEqual(len(self.reopen().state['attempts']),128)

    def test_closed_journal_cannot_write(self):
        self.close()
        with self.assertRaisesRegex(manager.Failure,'update_reconciliation_required'):self.j.block('controlled_failure')

    def test_cli_configuration_and_status_do_not_start_native_or_scheduler(self):
        self.close();policy_file=self.f.root/'policy.json';policy_file.write_text(json.dumps(self.policy));script=ROOT/'plugins/knowledge-manager/scripts/manager.py'
        for operation,extra in [('configure-updates',['--policy-file',str(policy_file)]),('update-status',[])]:
            result=subprocess.run([sys.executable,str(script),operation,'--registration',self.registration,*extra],capture_output=True)
            self.assertEqual(result.returncode,0,result.stderr);value=json.loads(result.stdout);self.assertFalse(value['scheduler_running'])
        self.assertFalse(Path(self.r['dataset_root']).exists());self.reopen()

    def test_cli_typed_failure_is_json_without_traceback(self):
        self.close();policy_file=self.f.root/'bad-policy.json';policy_file.write_text('{}')
        result=subprocess.run([sys.executable,str(ROOT/'plugins/knowledge-manager/scripts/manager.py'),'configure-updates','--registration',self.registration,'--policy-file',str(policy_file)],capture_output=True)
        self.assertEqual(result.returncode,2);self.assertEqual(json.loads(result.stderr)['code'],'update_policy_invalid');self.assertNotIn(b'Traceback',result.stderr);self.reopen()

    def test_intent_copies_actual_native_inspection_wire_fields(self):
        broker=importlib.import_module('broker')
        setup=manager.strict_read(Path(self.r['state_root'])/'setup.json')
        setup.update(src=self.prepared['input_project'],source_mode='snapshot-only',document_corpora=[str(Path(self.prepared['input_project'])/'.cks/knowledge/questions')])
        setup_path=self.bundle_dir/'setup.json';manager.write_new(setup_path,setup)
        config=manager.strict_read(Path(self.r['state_root'])/'cks.json')
        policy_path=Path(self.j.state['policy']['path'])
        config['maintenance']['profiles'][0].update(setup_config=str(setup_path),source_check=inputs.source_check(self.r,policy_path,self.prepared['input_id']))
        config_path=self.bundle_dir/'cks.json';manager.write_new(config_path,config)
        native=broker.Native(self.r,config_path)
        try:
            response=native.tool('cks.dataset.inspect_profile',{'schema_version':1,'project_id':self.r['project_id'],'profile_id':self.r['profile_id']})
            self.assertFalse(response.get('isError'),response);profile=response['structuredContent']['profile']
            self.assertEqual(profile['source_pin']['mode'],'snapshot-only')
            self.j.checking('manual',10)
            row=self.j.intend(self.prepared['input_id'],{'config':str(config_path),'files':{str(p):manager.file_hash(p) for p in (setup_path,config_path)}},profile,10)
            self.assertEqual(row['request']['source_pin'],profile['source_pin']);self.assertEqual(row['request']['base'],profile['observed_base'])
            self.assertEqual(self.reopen().row(),row)
            self.assertFalse(Path(self.r['dataset_root']).exists())
        finally:self.assertEqual(native.close(),0)

    def test_unsettled_known_reviewed_job_blocks_automatic_check(self):
        path=Path(self.r['state_root'])/'broker/state.json'
        value={'project_id':self.r['project_id'],'profile_id':self.r['profile_id'],'registration_token':self.r['token'],
               'pending_build':None,'jobs':{'controlled-known':{'terminal':False}}}
        manager.write_new(path,{'state':value,'checksum':state.domain_hash('cks.manager-broker-state.v1',value)})
        with self.assertRaisesRegex(manager.Failure,'reviewed_job_requires_reconciliation'):self.j.checking('manual',10)

    def test_pending_file_budget_never_discards_uncertain_receipts(self):
        for n in range(128):(self.j.directory/('.pending-controlled-'+str(n))).write_text('controlled uncertainty')
        with self.assertRaisesRegex(manager.Failure,'update_pending_file_limit'):self.j.checking('manual',10)
        self.assertEqual(len(list(self.j.directory.glob('.pending-*'))),128)

    def test_noop_refreshes_validated_read_receipt_without_new_attempt(self):
        job=self.success();self.j.checking('manual',20);self.j.no_op(self.prepared,self.profile,job,20,read_pack=self.pack(job))
        self.assertEqual(self.j.row(self.j.state['last_success'])['read_receipt']['at'],20)
        self.assertEqual(len(self.j.state['attempts']),1)

    def test_unknown_nonempty_dataset_still_blocks_journal_open(self):
        self.close();root=Path(self.r['dataset_root']);root.mkdir();(root/'unknown').write_text('no owner')
        with self.assertRaises(manager.Failure):state.Journal(self.registration)
        self.assertFalse((root/'.maintenance-project.json').exists())


if __name__=='__main__':unittest.main(verbosity=2)
