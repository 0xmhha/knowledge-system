"""Actual CKS maintenance/reader proxy, scope and durable manager state checks."""
import argparse
import copy
import importlib
import importlib.util
import json
from pathlib import Path
import sys
import time
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT/'plugins/knowledge-manager/scripts'))
broker = importlib.import_module('broker')
manager = importlib.import_module('manager')
collection = importlib.import_module('collection')
review = importlib.import_module('review')
spec = importlib.util.spec_from_file_location('broker_review_fixture',Path(__file__).with_name('test_knowledge_manager_review.py'))
fixture_module = importlib.util.module_from_spec(spec)
spec.loader.exec_module(fixture_module)


class BrokerTests(unittest.TestCase):
    def setUp(self):
        self.f = fixture_module.ReviewTests('test_plan_asks_without_inventing_rationale_or_approval')
        self.addCleanup(self.f.doCleanups)
        self.f.setUp()
        (self.f.source/'go.mod').write_text('module example.invalid/broker-test\n\ngo 1.26.9\n')
        claims = json.loads(self.f.claims.read_text())['claims']
        self.f.cid = collection.collect(argparse.Namespace(exclude=['private/**'],context_file=None,max_files=100,max_file_bytes=1024*1024,max_total_bytes=4*1024*1024),self.f.r)['collection_id']
        self.f.build_draft(claims)
        self.b = broker.Broker(self.f.registration)
        self.addCleanup(self.b.close)
        self.profile = {'schema_version':1,'project_id':self.f.r['project_id'],'profile_id':self.f.r['profile_id']}

    def select(self):
        adopted = self.f.adopted()
        value = self.b.call('cks.manager.select_input',{'adoption_id':adopted['adoption_id']})
        self.assertFalse(value['isError'])
        self.adopted=adopted
        return adopted

    def build(self):
        if not self.b.state['selected_input']:
            self.select()
        profile = self.b.call('cks.dataset.inspect_profile',self.profile)['structuredContent']['profile']
        request = {**self.profile,**{k:profile[k]for k in ('profile_digest','source_pin','input_digest','embedding_digest')},'base':profile['observed_base'],
                   'new_version':'broker-v1','activation':'hold_for_review','idempotency_key':'owned-broker-test-0001','deadline_seconds':120}
        job = self.b.call('cks.dataset.build',request)['structuredContent']
        ref = {**self.profile,'job_id':job['job_id']}
        end = time.monotonic()+60
        while not job['terminal']:
            self.assertLess(time.monotonic(),end)
            time.sleep(.02)
            job = self.b.call('cks.dataset.status',ref)['structuredContent']
        self.assertEqual(job['state'],'held_for_review',job)
        self.request,self.job = request,job
        return job

    def reader(self):
        job = self.build()
        response = self.b.call('cks.manager.select_reader',{'job_id':job['job_id'],'expected_coordinates':job['candidate']['coordinates']})
        self.assertEqual(response['structuredContent']['stage'],'reader_initialized_unqueried')
        return job

    def test_metadata_only_exposes_scoped_surfaces_without_original_build(self):
        names = {r['name']for r in self.b.tools()}
        self.assertEqual(len(names),18)
        self.assertTrue(broker.MAINTENANCE<names)
        self.assertIn(broker.READ,names)
        self.assertFalse(any(n.startswith('cks.ops.')for n in names))
        self.assertIsNone(self.b.state['selected_input'])
        self.assertFalse(Path(self.f.r['dataset_root']).exists())

    def test_default_context_never_reaches_metadata_dummy_backend(self):
        with self.assertRaisesRegex(manager.Failure,'reader_selection_required'):
            self.b.call(broker.READ,{'prompt':'Value'})

    def test_original_unfiltered_profile_cannot_be_inspected_or_built(self):
        for name in ('cks.dataset.inspect_profile','cks.dataset.build'):
            with self.assertRaisesRegex(manager.Failure,'adoption_required'):
                self.b.call(name,self.profile)
        self.assertFalse(Path(self.f.r['dataset_root']).exists())

    def test_legacy_mutation_tool_is_unavailable(self):
        with self.assertRaisesRegex(manager.Failure,'unknown_tool'):
            self.b.call('cks.ops.index',{})

    def test_actual_filtered_profile_inspection_is_project_scoped(self):
        self.select()
        response = self.b.call('cks.dataset.inspect_profile',self.profile)['structuredContent']
        self.assertEqual(response['profile']['project_id'],self.f.r['project_id'])
        self.assertEqual(response['profile']['allowed_activation'],['hold_for_review'])
        self.assertIn('knowledge',response['profile']['required_gates'])
        path = self.b.directory/'inputs'/self.adopted['adoption_id']/'setup.json'
        setup = json.loads(path.read_text())
        self.assertEqual(setup['src'],self.adopted['input_project'])
        self.assertEqual(setup['document_corpora'],[str(Path(setup['src'])/'.cks/knowledge/questions')])

    def test_foreign_project_or_profile_is_refused_before_native_call(self):
        for key in ('project_id','profile_id'):
            args = dict(self.profile)
            args[key]='other'
            with self.assertRaisesRegex(manager.Failure,'project_or_profile_mismatch'):
                self.b.call('cks.dataset.status',args)

    def test_automatic_activation_is_refused(self):
        self.select()
        with self.assertRaisesRegex(manager.Failure,'activation_not_authorized'):
            self.b.call('cks.dataset.build',{**self.profile,'activation':'activate_after_gate'})
        self.assertIsNone(self.b.state['pending_build'])

    def test_actual_held_build_and_retained_read_have_exact_coordinates(self):
        job = self.reader()
        self.assertIsNone(self.b.state['last_validated_read'])
        pack = self.b.call(broker.READ,{'prompt':'Value recorded code returns 7'})['structuredContent']
        self.assertEqual(pack['coordinates'],job['candidate']['coordinates'])
        self.assertTrue(pack['citations'])
        self.assertTrue(any('return 7'in json.dumps(body)for body in pack['bodies']))
        self.assertTrue(any(r['origin_id']=='docs:corpus.0'for r in pack['citations']))
        self.assertEqual(self.b.state['last_validated_read']['coordinates'],pack['coordinates'])
        self.assertFalse(Path(self.f.r['dataset_root']).joinpath('current').exists())

    def test_wrong_candidate_coordinates_are_not_a_fallback(self):
        job = self.build()
        wrong = dict(job['candidate']['coordinates'],dataset_id='0'*64)
        with self.assertRaisesRegex(manager.Failure,'candidate_input_mismatch'):
            self.b.call('cks.manager.select_reader',{'job_id':job['job_id'],'expected_coordinates':wrong})
        self.assertIsNone(self.b.reader)

    def test_source_drift_refuses_reader_and_new_build_but_preserves_status(self):
        self.reader()
        self.f.code.write_text('package fixture\nfunc Value()int{return 8}\n')
        for name,args in [(broker.READ,{'prompt':'Value'}),('cks.dataset.build',self.request)]:
            with self.assertRaisesRegex(manager.Failure,'collection_stale'):
                self.b.call(name,args)
        response = self.b.call('cks.dataset.status',{**self.profile,'job_id':self.job['job_id']})
        self.assertEqual(response['structuredContent'],self.job)

    def test_known_job_resumes_original_request_after_source_drift(self):
        self.build()
        self.f.code.write_text('package fixture\nfunc Value()int{return 8}\n')
        replay = self.b.call('cks.manager.resume_build',{'job_id':self.job['job_id'],'original_request':self.request})
        self.assertEqual(replay['structuredContent'],self.job)
        self.assertIsNone(self.b.state['pending_build'])

    def test_resume_cannot_admit_changed_request(self):
        self.build()
        changed = dict(self.request,new_version='other-version')
        with self.assertRaisesRegex(manager.Failure,'original_request_mismatch'):
            self.b.call('cks.manager.resume_build',{'job_id':self.job['job_id'],'original_request':changed})
        self.assertFalse(Path(self.f.r['dataset_root']).joinpath('other-version').exists())

    def test_retracted_input_cannot_keep_serving_as_current_manager_input(self):
        self.reader()
        review.retract(self.f.r,self.f.retraction(self.adopted))
        with self.assertRaisesRegex(manager.Failure,'adoption_not_current'):
            self.b.call(broker.READ,{'prompt':'Value'})
        self.assertEqual(self.b.call('cks.manager.resume_build',{'job_id':self.job['job_id'],'original_request':self.request})['structuredContent'],self.job)

    def test_missing_version_is_refused_without_selecting_current(self):
        job = self.build()
        version = Path(self.f.r['dataset_root'])/'broker-v1'
        moved = version.with_name('moved-owned-fixture')
        version.rename(moved)
        self.addCleanup(moved.rename,version)
        with self.assertRaisesRegex(manager.Failure,'candidate_missing'):
            self.b.call('cks.manager.select_reader',{'job_id':job['job_id'],'expected_coordinates':job['candidate']['coordinates']})

    def test_actual_open_reader_rejects_corrupted_dataset_identity(self):
        self.reader()
        self.b.call(broker.READ,{'prompt':'Value recorded code returns 7'})
        path = Path(self.f.r['dataset_root'])/'broker-v1/dataset-identity.json'
        original = path.read_bytes()
        try:
            path.write_text('{}')
            try:
                response = self.b.call(broker.READ,{'prompt':'Value recorded code returns 7'})
            except manager.Failure:
                pass
            else:
                self.assertTrue(response.get('isError'),response)
        finally:
            path.write_bytes(original)
        self.assertEqual(self.b.call(broker.READ,{'prompt':'Value recorded code returns 7'})['structuredContent']['coordinates'],self.job['candidate']['coordinates'])

    def test_restart_rechecks_persisted_input_job_and_reader_before_read(self):
        job = self.reader()
        self.b.close()
        restarted = broker.Broker(self.f.registration)
        self.addCleanup(restarted.close)
        self.assertIsNone(restarted.reader)
        pack = restarted.call(broker.READ,{'prompt':'Value recorded code returns 7'})['structuredContent']
        self.assertEqual(pack['coordinates'],job['candidate']['coordinates'])
        self.assertIsNotNone(restarted.reader)

    def test_clear_reader_releases_lease_and_requires_explicit_reselection(self):
        self.reader()
        self.b.call('cks.manager.clear_reader',{})
        self.assertIsNone(self.b.reader)
        with self.assertRaisesRegex(manager.Failure,'reader_selection_required'):
            self.b.call(broker.READ,{'prompt':'Value'})

    def test_second_broker_cannot_take_the_same_local_owner_lease(self):
        with self.assertRaisesRegex(manager.Failure,'collection_busy'):
            broker.Broker(self.f.registration)

    def test_uncertain_state_save_poison_and_restart_revalidation(self):
        adopted = self.f.adopted()
        with patch.object(broker,'sync_dir',side_effect=OSError('controlled post-rename fsync uncertainty')):
            with self.assertRaises(OSError):
                self.b.call('cks.manager.select_input',{'adoption_id':adopted['adoption_id']})
        with self.assertRaisesRegex(manager.Failure,'broker_reconciliation_required'):
            self.b.call('cks.manager.status',{})
        self.b.close()
        restarted = broker.Broker(self.f.registration)
        self.addCleanup(restarted.close)
        self.assertEqual(restarted.state['selected_input'],adopted['adoption_id'])
        self.assertIn('profile',restarted.call('cks.dataset.inspect_profile',self.profile)['structuredContent'])

    def test_lost_admission_reply_keeps_pending_and_exact_replay_recovers(self):
        self.select()
        p = self.b.call('cks.dataset.inspect_profile',self.profile)['structuredContent']['profile']
        request = {**self.profile,**{k:p[k]for k in ('profile_digest','source_pin','input_digest','embedding_digest')},'base':p['observed_base'],
                   'new_version':'lost-reply-v1','activation':'hold_for_review','idempotency_key':'owned-broker-lost-0001','deadline_seconds':120}
        original = self.b.native.tool
        def lost(name,args):
            value = original(name,args)
            if name=='cks.dataset.build':
                raise manager.Failure('native_unavailable')
            return value
        with patch.object(self.b.native,'tool',side_effect=lost):
            with self.assertRaisesRegex(manager.Failure,'native_unavailable'):
                self.b.call('cks.dataset.build',request)
        self.assertEqual(self.b.state['pending_build']['request'],request)
        with self.assertRaisesRegex(manager.Failure,'pending_admission_requires_exact_replay'):
            self.b.call('cks.dataset.build',dict(request,new_version='another-version'))
        response = self.b.call('cks.dataset.build',request)
        self.assertFalse(response.get('isError',False))
        self.assertIsNone(self.b.state['pending_build'])

    def test_other_adoption_cannot_select_previous_jobs_coordinates(self):
        job = self.build()
        self.f.input['expected_revision']=self.adopted['revision']
        self.f.input['decisions'][0]['reason']='Second explicit controlled input review.'
        record = self.f.record()
        newer = review.adopt(self.f.r,self.f.did,record['review_revision'])
        self.b.call('cks.manager.select_input',{'adoption_id':newer['adoption_id']})
        with self.assertRaisesRegex(manager.Failure,'candidate_input_mismatch'):
            self.b.call('cks.manager.select_reader',{'job_id':job['job_id'],'expected_coordinates':job['candidate']['coordinates']})

    def test_native_invalid_build_request_clears_known_refusal_without_admission(self):
        self.select()
        response = self.b.call('cks.dataset.build',{**self.profile,'activation':'hold_for_review','unknown':True})
        self.assertTrue(response['isError'])
        self.assertIsNone(self.b.state['pending_build'])
        self.assertEqual(self.b.state['jobs'],{})
        self.assertFalse(Path(self.f.r['dataset_root']).exists())

    def test_input_switch_is_blocked_by_an_unfinished_observed_job(self):
        self.select()
        old = self.adopted
        self.f.input['expected_revision']=old['revision']
        self.f.input['decisions'][0]['reason']='Another exact controlled review.'
        record = self.f.record()
        newer = review.adopt(self.f.r,self.f.did,record['review_revision'])
        self.b.state['jobs']['controlled-running-job']={'terminal':False,'state':'running'}
        observed = {**self.profile,'job_id':'controlled-running-job','terminal':False,'state':'running'}
        with patch.object(self.b.native,'tool',return_value=broker.result(observed)):
            with self.assertRaisesRegex(manager.Failure,'active_job_blocks_input_switch'):
                self.b.call('cks.manager.select_input',{'adoption_id':newer['adoption_id']})
        self.assertEqual(self.b.state['selected_input'],old['adoption_id'])
        self.assertIsNone(self.b.native.child.poll())

    def test_pending_uncertain_admission_blocks_switch_to_another_valid_input(self):
        self.select()
        old = self.adopted
        self.f.input['expected_revision']=old['revision']
        self.f.input['decisions'][0]['reason']='Changed controlled response.'
        record = self.f.record()
        newer = review.adopt(self.f.r,self.f.did,record['review_revision'])
        self.b.state['pending_build']={'adoption_id':old['adoption_id'],'request':{}}
        self.b.save()
        with self.assertRaisesRegex(manager.Failure,'pending_admission_blocks_input_switch'):
            self.b.call('cks.manager.select_input',{'adoption_id':newer['adoption_id']})
        self.assertEqual(self.b.state['selected_input'],old['adoption_id'])

    def test_unknown_manager_parameters_are_not_ignored(self):
        with self.assertRaisesRegex(manager.Failure,'invalid_request'):
            self.b.call('cks.manager.status',{'source_root':'/unselected'})

    def test_duplicate_JSON_keys_and_nonfinite_numbers_are_refused(self):
        for raw in ('{"project_id":"a","project_id":"b"}','{"cost":NaN}'):
            with self.assertRaises(manager.Failure):
                broker.parse(raw)

    def test_retired_input_status_is_explicitly_as_of_until_rechecked(self):
        self.select()
        review.retract(self.f.r,self.f.retraction(self.adopted))
        described = self.b.call('cks.manager.status',{})['structuredContent']
        self.assertTrue(described['selection_rechecked_on_use'])
        self.assertIsNone(described['last_validated_read'])
        with self.assertRaisesRegex(manager.Failure,'adoption_not_current'):
            self.b.call('cks.dataset.inspect_profile',self.profile)

    def test_altered_derived_bundle_is_not_silently_rewritten(self):
        self.select()
        path = self.b.directory/'inputs'/self.adopted['adoption_id']/'setup.json'
        path.write_text('{}')
        self.b.close()
        restarted = broker.Broker(self.f.registration)
        self.addCleanup(restarted.close)
        with self.assertRaisesRegex(manager.Failure,'broker_bundle_changed'):
            restarted.call('cks.dataset.inspect_profile',self.profile)

    def test_state_checksum_or_foreign_identity_is_refused_on_restart(self):
        self.select()
        self.b.close()
        path = self.b.directory/'state.json'
        data = json.loads(path.read_text())
        data['state']['project_id']='other'
        path.write_text(json.dumps(data))
        with self.assertRaisesRegex(manager.Failure,'broker_state_invalid'):
            broker.Broker(self.f.registration)


if __name__=='__main__':
    unittest.main()
