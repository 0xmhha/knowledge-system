"""Actual native reviewed projections, CAS/recovery and no invented review."""
import argparse
import importlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT/'plugins/knowledge-manager/scripts'))
manager = importlib.import_module('manager')
collection = importlib.import_module('collection')
review = importlib.import_module('review')


class ReviewTests(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory(prefix='ks-manager-review-',dir=str(Path(tempfile.gettempdir()).resolve()))
        self.addCleanup(tmp.cleanup)
        self.root = Path(tmp.name).resolve()
        self.source = self.root/'source'
        self.source.mkdir()
        self.code = self.source/'value.go'
        self.code.write_text('package fixture\n\nfunc Value() int { return 7 }\n')
        (self.source/'.env').write_text('EXCLUDED_PRIVATE_SENTINEL\n')
        (self.source/'private').mkdir()
        (self.source/'private/decision.txt').write_text('PRIVATE_DECISION_SENTINEL\n')
        registered = manager.register(argparse.Namespace(source=str(self.source),dataset=str(self.root/'dataset'),state_dir=str(self.root/'state'),
            cks_bin=str(ROOT/'bin/cks'),graph_bin=str(ROOT/'bin/ckg'),vector_bin=str(ROOT/'bin/ckv'),embedder='mock',model_name='mock',project_id=None,source_mode='snapshot-only'))
        self.registration = registered['registration_file']
        self.r = manager.load_registration(self.registration)
        args = argparse.Namespace(exclude=['private/**'],context_file=None,max_files=100,max_file_bytes=1024*1024,max_total_bytes=4*1024*1024)
        self.cid = collection.collect(args,self.r)['collection_id']
        self.build_draft([
            {'kind':'fact','statement':'The recorded code returns 7.','refs':[{'path':'value.go','start_line':3,'end_line':3}]},
            {'kind':'hypothesis','statement':'The reason for choosing 7 is unknown.','refs':[{'path':'value.go','start_line':3,'end_line':3}]},
        ])

    def build_draft(self,claims):
        self.claims = self.root/'claims.json'
        self.claims.write_text(json.dumps({'schema_version':1,'project_id':self.r['project_id'],'collection_id':self.cid,'claims':claims}))
        self.did = collection.draft(argparse.Namespace(collection_id=self.cid,claims_file=str(self.claims)),self.r)['draft_id']
        self.directory,self.draft = collection.load_draft(self.r,self.did)
        self.input = {'schema_version':1,'project_id':self.r['project_id'],'draft_id':self.did,'expected_revision':None,
            'reviewer':'explicit-fixture-reviewer','response_source':{'kind':'controlled_fixture','reference':'ReviewTests controlled response; no independent human acceptance'},
            'decisions':[{'claim_id':c['claim_id'],'disposition':'accepted'if c['kind']=='fact'else 'deferred','reason':'Controlled fixture disposition, not independent semantic acceptance.'}for c in review.claims_by_id(self.draft).values()],
            'resolutions':[]}

    def record(self,value=None):
        path = self.root/'review-input.json'
        path.write_text(json.dumps(value if value is not None else self.input))
        return review.record_review(self.r,path)

    def adopted(self):
        record = self.record()
        return review.adopt(self.r,self.did,record['review_revision'])

    def retraction(self,adopted):
        document = {'schema_version':1,'project_id':self.r['project_id'],'adoption_id':adopted['adoption_id'],'expected_revision':adopted['revision'],
                    'actor':'explicit-fixture-reviewer','reason':'Controlled retraction test.','response_source':self.input['response_source']}
        path = self.root/'retraction-input.json'
        path.write_text(json.dumps(document))
        return path

    def test_plan_asks_without_inventing_rationale_or_approval(self):
        plan = review.plan(self.r,self.did)
        self.assertEqual(len(plan['questions']),2)
        self.assertEqual(plan['human_approval'],'not_inferred')
        self.assertEqual(plan['as_of'],'immutable_collection_not_current_source_verification')
        self.assertTrue(all(q['answer_status']=='not_provided'for q in plan['questions']))
        self.assertFalse((Path(self.r['state_root'])/'workflows').exists())

    def test_shared_evidence_is_candidate_not_proven_contradiction(self):
        rows = review.plan(self.r,self.did)['shared_evidence_interpretations']
        self.assertEqual(len(rows),1)
        self.assertIn('not_proven_contradiction',rows[0]['classification'])

    def test_possible_duplicates_are_not_merged(self):
        claims = json.loads(self.claims.read_text())['claims']
        claims[1] = dict(claims[0],statement='  The recorded CODE returns 7.  ')
        self.build_draft(claims)
        plan = review.plan(self.r,self.did)
        self.assertEqual(len(plan['possible_duplicates']),1)
        self.assertEqual(len(plan['claims']),2)

    def test_repeat_identical_claim_ID_has_explicit_occurrence_count(self):
        claims = json.loads(self.claims.read_text())['claims']
        self.build_draft([claims[0],claims[0]])
        plan = review.plan(self.r,self.did)
        self.assertEqual(plan['duplicate_claim_occurrences'],1)
        self.assertEqual(len(plan['claims']),1)
        self.assertEqual(self.record()['stage'],'review_recorded')

    def test_no_default_reviewer(self):
        self.input['reviewer']=''
        with self.assertRaisesRegex(manager.Failure,'reviewer_required'):
            self.record()

    def test_no_automatic_approval_provenance(self):
        self.input['response_source']['kind']='automatic_model_approval'
        with self.assertRaisesRegex(manager.Failure,'review_response_source_required'):
            self.record()

    def test_missing_reason_refused(self):
        self.input['decisions'][0]['reason']=''
        with self.assertRaisesRegex(manager.Failure,'review_invalid'):
            self.record()

    def test_all_claims_need_an_explicit_disposition(self):
        self.input['decisions'].pop()
        with self.assertRaisesRegex(manager.Failure,'review_requires_all_claims'):
            self.record()

    def test_foreign_project_refused(self):
        self.input['project_id']='other'
        with self.assertRaisesRegex(manager.Failure,'project_or_subject_mismatch'):
            self.record()

    def test_wrong_claim_subject_refused(self):
        self.input['decisions'][0]['claim_id']='0'*64
        with self.assertRaisesRegex(manager.Failure,'review_invalid'):
            self.record()

    def test_accepting_two_shared_interpretations_requires_resolution(self):
        self.input['decisions'][1]['disposition']='accepted'
        with self.assertRaisesRegex(manager.Failure,'review_conflict_resolution_required'):
            self.record()

    def test_explicit_separate_interpretation_does_not_make_hypothesis_fact(self):
        self.input['decisions'][1]['disposition']='accepted'
        pair = review.plan(self.r,self.did)['shared_evidence_interpretations'][0]['claim_ids']
        self.input['resolutions']=[{'claim_ids':pair,'disposition':'keep_separate','reason':'The fixture hypothesis explicitly remains unknown, separate from the observed return value.'}]
        adopted = self.adopted()
        text = Path(adopted['input_project']).joinpath('.cks/knowledge/questions/manager-reviewed.md').read_text()
        self.assertIn('Kind: hypothesis',text)
        self.assertIn('Kind: fact',text)

    def test_review_is_append_only_and_draft_stays_proposed(self):
        before = {p.relative_to(self.directory).as_posix():p.read_bytes()for p in self.directory.rglob('*')if p.is_file()}
        result = self.record()
        self.assertEqual(review.status(self.r,self.did)['revision'],result['review_revision'])
        self.assertEqual(review.status(self.r,self.did)['review_authentication'],'not_performed')
        self.assertEqual(before,{p.relative_to(self.directory).as_posix():p.read_bytes()for p in self.directory.rglob('*')if p.is_file()})

    def test_identical_review_replays_without_new_event(self):
        first = self.record()
        again = self.record()
        self.assertEqual(again['stage'],'review_replayed')
        self.assertEqual(again['review_revision'],first['review_revision'])
        self.assertEqual(len(review.events(self.r,self.did)),1)

    def test_changed_review_needs_current_revision(self):
        first = self.record()
        self.input['decisions'][0]['disposition']='rejected'
        with self.assertRaisesRegex(manager.Failure,'review_revision_conflict'):
            self.record()
        self.input['expected_revision']=first['review_revision']
        second = self.record()
        self.assertNotEqual(second['review_revision'],first['review_revision'])
        self.assertEqual(len(review.events(self.r,self.did)),2)

    def test_journal_truncation_refused(self):
        self.record()
        file = next(review.workflow(self.r,self.did).glob('[0-9]*.json'))
        file.write_text('{}')
        with self.assertRaisesRegex(manager.Failure,'review_journal_invalid'):
            review.status(self.r,self.did)

    def test_journal_owner_mismatch_refused(self):
        self.record()
        file = review.workflow(self.r,self.did)/'owner.json'
        file.write_text('{}')
        with self.assertRaisesRegex(manager.Failure,'review_owner_mismatch'):
            review.status(self.r,self.did)

    def test_writer_contention_refused(self):
        directory = review.workflow(self.r,self.did,True)
        with collection.writer(directory):
            with self.assertRaisesRegex(manager.Failure,'collection_busy'):
                self.record()

    def test_unpublished_pending_event_is_not_committed(self):
        self.record()
        (review.workflow(self.r,self.did)/'.pending-incomplete').write_text('{partial')
        self.assertEqual(len(review.events(self.r,self.did)),1)

    def test_adoption_uses_filtered_copy_and_actual_native_lock(self):
        before = self.code.read_bytes()
        adopted = self.adopted()
        directory,receipt,state = review.active_adoption(self.r,adopted['adoption_id'])
        self.assertEqual(self.code.read_bytes(),before)
        self.assertFalse((self.source/'.cks').exists())
        self.assertFalse((self.root/'dataset').exists())
        self.assertFalse((directory/'project/.env').exists())
        self.assertFalse((directory/'project/private').exists())
        self.assertEqual((directory/'project/value.go').read_bytes(),before)
        self.assertEqual([r['operation']for r in receipt['native_commands']],['init','validate','lock','validate'])
        self.assertTrue(all(c['exit_code']==0 for c in receipt['native_commands']))
        self.assertEqual(json.loads(receipt['native_commands'][-1]['stdout'])['status'],'locked')
        self.assertFalse(receipt['retrieval_verified'])
        self.assertEqual(receipt['response_source']['kind'],'controlled_fixture')
        self.assertEqual(state['stage'],'adopted')

    def test_deferred_rejected_excluded_claims_do_not_enter_reviewed_overlay(self):
        self.input['decisions'][1]['disposition']='excluded'
        adopted = self.adopted()
        body = Path(adopted['input_project']).joinpath('.cks/knowledge/questions/manager-reviewed.md').read_text()
        self.assertNotIn(self.draft['claims'][1]['statement'],body)
        _,receipt = review.load_adoption(self.r,adopted['adoption_id'])
        self.assertIn(self.draft['claims'][1]['claim_id'],receipt['omitted_claim_ids'])

    def test_unreviewed_and_all_rejected_cannot_adopt(self):
        with self.assertRaisesRegex(manager.Failure,'review_required'):
            review.adopt(self.r,self.did,'0'*64)
        for row in self.input['decisions']:
            row['disposition']='rejected'
        record = self.record()
        with self.assertRaisesRegex(manager.Failure,'no_accepted_claims'):
            review.adopt(self.r,self.did,record['review_revision'])

    def test_source_change_cannot_inherit_review(self):
        record = self.record()
        self.code.write_text('package fixture\nfunc Value()int{return 8}\n')
        with self.assertRaisesRegex(manager.Failure,'collection_stale'):
            review.adopt(self.r,self.did,record['review_revision'])
        self.assertEqual(review.status(self.r,self.did)['stage'],'reviewed')

    def test_new_selected_file_invalidates_adoption(self):
        adopted = self.adopted()
        (self.source/'extra.go').write_text('package fixture\n')
        with self.assertRaisesRegex(manager.Failure,'collection_stale'):
            review.active_adoption(self.r,adopted['adoption_id'])

    def test_deleted_source_cannot_inherit_review(self):
        record = self.record()
        self.code.unlink()
        with self.assertRaises(manager.Failure):
            review.adopt(self.r,self.did,record['review_revision'])

    def test_nested_source_manifest_name_is_not_confused_with_manager_seal(self):
        (self.source/'adoption-manifest.json').write_text('{"ordinary_source_config":true}\n')
        args = argparse.Namespace(exclude=['private/**'],context_file=None,max_files=100,max_file_bytes=1024*1024,max_total_bytes=4*1024*1024)
        self.cid = collection.collect(args,self.r)['collection_id']
        self.build_draft(json.loads(self.claims.read_text())['claims'])
        adopted = self.adopted()
        directory,_,_ = review.active_adoption(self.r,adopted['adoption_id'])
        self.assertEqual((directory/'project/adoption-manifest.json').read_text(),'{"ordinary_source_config":true}\n')

    def test_new_Git_commit_requires_new_collection_even_if_selected_bytes_match(self):
        env = dict(os.environ)
        for key in ('GIT_DIR','GIT_WORK_TREE','GIT_INDEX_FILE'):
            env.pop(key,None)
        for args in (['init','--quiet'],['add','--','.'],['-c','user.name=fixture','-c','user.email=fixture@example.invalid','-c','commit.gpgsign=false','-c','core.hooksPath=/dev/null','commit','--quiet','-m','Controlled fixture']):
            subprocess.run(['git','-C',self.source,*args],check=True,env=env,capture_output=True)
        record = self.record()
        with self.assertRaisesRegex(manager.Failure,'collection_stale'):
            review.adopt(self.r,self.did,record['review_revision'])

    def test_excluded_private_change_does_not_enter_projection(self):
        (self.source/'private/decision.txt').write_text('CHANGED_PRIVATE\n')
        adopted = self.adopted()
        self.assertFalse(Path(adopted['input_project']).joinpath('private').exists())

    def test_adoption_replay_checks_seal_and_current_source(self):
        adopted = self.adopted()
        replay = review.adopt(self.r,self.did,adopted['review_revision'])
        self.assertEqual(replay['stage'],'adoption_replayed')
        self.assertEqual(replay['adoption_id'],adopted['adoption_id'])
        Path(adopted['input_project']).joinpath('value.go').write_text('tampered')
        with self.assertRaisesRegex(manager.Failure,'adoption_changed'):
            review.active_adoption(self.r,adopted['adoption_id'])

    def test_restart_after_projection_before_event_recovers_same_candidate(self):
        record = self.record()
        with patch.object(review,'append_event',side_effect=OSError('controlled pre-event interruption')):
            with self.assertRaises(OSError):
                review.adopt(self.r,self.did,record['review_revision'])
        self.assertEqual(review.status(self.r,self.did)['stage'],'reviewed')
        adopted = review.adopt(self.r,self.did,record['review_revision'])
        self.assertEqual(len([p for p in Path(self.r['state_root']).joinpath('adoptions').iterdir()if p.is_dir()and not p.name.startswith('.')]),1)
        self.assertEqual(review.status(self.r,self.did)['current_adoption_id'],adopted['adoption_id'])

    def test_post_event_sync_failure_replays_existing_event(self):
        original = review.sync_dir
        def fail(path):
            if path==review.workflow(self.r,self.did):
                raise OSError('controlled post-rename sync failure')
            original(path)
        # Create ownership before the injected failure.
        review.workflow(self.r,self.did,True)
        with patch.object(review,'sync_dir',side_effect=fail):
            with self.assertRaises(OSError):
                self.record()
        self.assertEqual(self.record()['stage'],'review_replayed')
        self.assertEqual(len(review.events(self.r,self.did)),1)

    def test_retraction_blocks_future_use_without_deleting_evidence_or_DB(self):
        adopted = self.adopted()
        path = self.retraction(adopted)
        result = review.retract(self.r,path)
        self.assertEqual(result['stage'],'input_retracted_DB_rollback_not_run')
        self.assertIsNone(review.status(self.r,self.did)['current_adoption_id'])
        with self.assertRaisesRegex(manager.Failure,'adoption_not_current'):
            review.active_adoption(self.r,adopted['adoption_id'])
        self.assertTrue(Path(adopted['input_project']).is_dir())
        self.assertEqual(review.retract(self.r,path)['stage'],'retraction_replayed')
        with self.assertRaisesRegex(manager.Failure,'review_revision_conflict'):
            review.adopt(self.r,self.did,adopted['review_revision'])

    def test_new_review_supersedes_previous_adoption_for_future_use(self):
        adopted = self.adopted()
        self.input['expected_revision']=adopted['revision']
        self.input['decisions'][0]['reason']='Updated explicit fixture review.'
        self.record()
        with self.assertRaisesRegex(manager.Failure,'adoption_not_current'):
            review.active_adoption(self.r,adopted['adoption_id'])

    def test_retraction_needs_actor_reason_and_revision(self):
        adopted = self.adopted()
        path = self.retraction(adopted)
        data = json.loads(path.read_text())
        data['actor']=''
        path.write_text(json.dumps(data))
        with self.assertRaisesRegex(manager.Failure,'retraction_actor_reason_required'):
            review.retract(self.r,path)
        data['actor']='fixture-actor'
        data['expected_revision']='0'*64
        path.write_text(json.dumps(data))
        with self.assertRaisesRegex(manager.Failure,'review_revision_conflict'):
            review.retract(self.r,path)

    def test_invalid_array_input_is_typed_CLI_refusal(self):
        path = self.root/'bad-input.json'
        path.write_text('[]')
        result = subprocess.run([sys.executable,ROOT/'plugins/knowledge-manager/scripts/manager.py','review','--registration',self.registration,'--review-file',path],capture_output=True,text=True)
        self.assertEqual(result.returncode,2)
        self.assertEqual(json.loads(result.stderr)['code'],'review_invalid')
        self.assertNotIn('Traceback',result.stderr)


if __name__=='__main__':
    unittest.main()
