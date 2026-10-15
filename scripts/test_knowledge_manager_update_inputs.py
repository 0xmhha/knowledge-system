"""Actual native observed inputs, deletion/rename and no-op refusal contracts."""
import argparse
import copy
import importlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest.mock import patch

ROOT=Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT/'plugins/knowledge-manager/scripts'))
manager=importlib.import_module('manager')
inputs=importlib.import_module('update_inputs')
contract=importlib.import_module('update_contract')


class UpdateInputTests(unittest.TestCase):
    def setUp(self):
        temp=tempfile.TemporaryDirectory(prefix='ks-update-input-')
        self.addCleanup(temp.cleanup);self.root=Path(temp.name).resolve()
        self.source=self.root/'source';self.source.mkdir()
        self.code=self.source/'value.go';self.code.write_text('package fixture\nfunc Value() int { return 7 }\n')
        (self.source/'go.mod').write_text('module example.invalid/update\n\ngo 1.26.9\n')
        self.registered=manager.register(argparse.Namespace(source=str(self.source),dataset=str(self.root/'dataset'),state_dir=str(self.root/'state'),
            cks_bin=str(ROOT/'bin/cks'),graph_bin=str(ROOT/'bin/ckg'),vector_bin=str(ROOT/'bin/ckv'),embedder='mock',model_name='mock',project_id=None,source_mode='snapshot-only'))
        self.r=manager.load_registration(self.registered['registration_file'])
        self.policy=contract.defaults(self.r);self.policy['enabled']=True;self.policy['collection']['exclude']=['private/**']

    def prepare(self,previous=None,**kwargs):return inputs.prepare(self.policy,self.r,previous,**kwargs)

    def captured(self,prepared):return inputs.load_input(self.r,prepared['input_id'])

    def material(self,role='ontology'):
        root=self.source/'.cks'/role;root.mkdir(parents=True,exist_ok=True)
        (root/'mapping.yaml').write_text('concept: tx\nstatus: proposed\n')
        self.policy['collection']['materials']=[{'id':'terms','role':role,'root':str(root)}]
        return root

    def test_document_free_preparation_never_writes_source_review_or_DB(self):
        before={p.name:p.read_bytes()for p in self.source.iterdir()}
        prepared=self.prepare();directory,captured=self.captured(prepared)
        self.assertEqual(before,{p.name:p.read_bytes()for p in self.source.iterdir()})
        self.assertEqual(prepared['stage'],'observed_input_prepared_not_DB_built')
        self.assertFalse(prepared['DB_built']);self.assertFalse(prepared['DB_published'])
        self.assertEqual(captured['semantic_review'],'not_performed')
        self.assertFalse((self.root/'dataset').exists());self.assertFalse((self.root/'state/workflows').exists())
        self.assertIn(b'return 7',(directory/'project/value.go').read_bytes())

    def test_exact_repeat_reuses_immutable_input_and_original_time(self):
        first=self.prepare();_,a=self.captured(first);second=self.prepare(first['input_id']);_,b=self.captured(second)
        self.assertEqual(a,b);self.assertEqual(first['input_id'],second['input_id'])
        self.assertTrue(second['change']['input_unchanged']);self.assertFalse(second['change']['DB_health_or_noop_verified'])

    def test_mtime_alone_is_not_new_input(self):
        first=self.prepare();os.utime(self.code,(1,1));second=self.prepare(first['input_id'])
        self.assertEqual(first['input_id'],second['input_id'])

    def test_added_modified_and_deleted_sets(self):
        note=self.source/'note.md';note.write_text('Old public note\n');first=self.prepare()
        note.unlink();self.code.write_text('package fixture\nfunc Value()int{return 8}\n');(self.source/'new.md').write_text('New public note\n')
        second=self.prepare(first['input_id']);change=second['change']
        self.assertEqual([r['path']for r in change['added']],['new.md']);self.assertEqual([r['path']for r in change['modified']],['value.go'])
        self.assertEqual([r['path']for r in change['deleted']],['note.md']);self.assertFalse(change['semantic_review_carried_forward'])
        directory,_=self.captured(second);self.assertFalse((directory/'project/note.md').exists())

    def test_same_bytes_move_is_only_candidate(self):
        first=self.prepare();self.code.rename(self.source/'moved.go');second=self.prepare(first['input_id'])
        moves=second['change']['rename_candidates'];self.assertEqual(len(moves),1)
        self.assertEqual(moves[0]['from_path'],'value.go');self.assertIn('not_entity_identity',moves[0]['classification'])

    def test_ambiguous_same_bytes_moves_not_paired(self):
        data=self.code.read_bytes();(self.source/'copy.go').write_bytes(data);first=self.prepare()
        self.code.unlink();(self.source/'copy.go').unlink();(self.source/'x.go').write_bytes(data);(self.source/'y.go').write_bytes(data)
        self.assertEqual(self.prepare(first['input_id'])['change']['rename_candidates'],[])

    def test_old_snapshot_remains_exact_after_live_change(self):
        first=self.prepare();directory,old=self.captured(first);before=(directory/'project/value.go').read_bytes()
        self.code.write_text('package fixture\nfunc Value()int{return 99}\n');self.prepare(first['input_id'])
        self.assertEqual(self.captured(first)[1],old);self.assertEqual((directory/'project/value.go').read_bytes(),before)

    def test_excluded_private_and_sensitive_content_not_adopted(self):
        (self.source/'private').mkdir();(self.source/'private/notes.md').write_text('PRIVATE_INPUT_SENTINEL')
        (self.source/'.env').write_text('ENV_INPUT_SENTINEL');(self.source/'config.yaml').write_text('password: secret-input-value\n')
        p=self.prepare();directory,_=self.captured(p);content=b''.join(x.read_bytes()for x in directory.rglob('*')if x.is_file())
        for word in (b'PRIVATE_INPUT_SENTINEL',b'ENV_INPUT_SENTINEL',b'secret-input-value'):self.assertNotIn(word,content)

    def test_excluded_bytes_do_not_change_selected_input(self):
        p=self.source/'private';p.mkdir();(p/'note.md').write_text('old');first=self.prepare();(p/'note.md').write_text('new')
        self.assertEqual(self.prepare(first['input_id'])['input_id'],first['input_id'])

    def context(self,project=None):
        path=self.root/'context.json';path.write_text(json.dumps({'schema_version':1,'project_id':project or self.r['project_id'],'records':[{'kind':'note','text':'Supplied public observation.'}]}))
        self.policy['collection']['context_file']=str(path);return path

    def test_supplied_context_change_has_distinct_origin(self):
        path=self.context();first=self.prepare();path.write_text(path.read_text().replace('observation','changed observation'));second=self.prepare(first['input_id'])
        self.assertEqual(second['change']['modified'][0]['origin_id'],'provided')
        directory,_=self.captured(second);self.assertNotIn('context.json',[p.name for p in(directory/'project').iterdir()])
        self.assertTrue(list((directory/'materials').glob('*.md')))

    def test_foreign_provided_record_refused(self):
        self.context('foreign')
        with self.assertRaisesRegex(manager.Failure,'project_mismatch'):self.prepare()

    def test_explicit_ontology_bytes_change_without_Git_change(self):
        path=self.material();first=self.prepare();(path/'mapping.yaml').write_text('concept: tx\nstatus: unsupported\n');second=self.prepare(first['input_id'])
        self.assertEqual(second['change']['modified'][0]['origin_id'],'material:terms')
        directory,capture=self.captured(second);self.assertEqual(capture['semantic_review'],'not_performed')
        self.assertNotIn('mapping.yaml',[p.name for p in(directory/'project/.cks').rglob('*')if p.is_file()])

    def test_empty_material_deletion_drops_latest_text(self):
        path=self.material();first=self.prepare();(path/'mapping.yaml').unlink();second=self.prepare(first['input_id'])
        self.assertEqual(second['change']['deleted'][0]['origin_id'],'material:terms');self.assertEqual(second['material_documents'],0)

    def test_previously_selected_removed_material_root_is_explicit_deletion(self):
        path=self.material();first=self.prepare();(path/'mapping.yaml').unlink();path.rmdir()
        with self.assertRaises(manager.Failure):self.prepare(first['input_id'])
        second=self.prepare(first['input_id'],allow_missing_material=True)
        self.assertEqual(len(second['change']['deleted']),1);self.assertTrue(self.captured(second)[1]['scopes'][1]['missing'])

    def test_total_budget_applies_across_all_roots(self):
        self.material();self.policy['collection']['max_files']=2
        with self.assertRaisesRegex(manager.Failure,'collection_limit'):self.prepare()

    def test_material_secret_and_non_UTF8_not_copied(self):
        path=self.material();(path/'secret.yaml').write_text('password: private-material-token\n');(path/'binary.md').write_bytes(b'\xff\xfe')
        prepared=self.prepare();directory,c=self.captured(prepared)
        self.assertFalse(any(r['path']in ('secret.yaml','binary.md')for r in c['identity']['files']))
        self.assertNotIn(b'private-material-token',b''.join(p.read_bytes()for p in directory.rglob('*')if p.is_file()))

    def test_changed_source_during_multi_root_capture_refused(self):
        self.material();original=inputs.gather;calls=0
        def change(*args,**kwargs):
            nonlocal calls
            value=original(*args,**kwargs);calls+=1
            if calls==2:self.code.write_text('package fixture\nfunc Value()int{return 10}\n')
            return value
        with patch.object(inputs,'gather',side_effect=change),self.assertRaisesRegex(manager.Failure,'source_changed'):self.prepare()

    def test_changed_source_during_native_projection_refused(self):
        original=inputs.subprocess.run
        def change(argv,*args,**kwargs):
            result=original(argv,*args,**kwargs)
            if 'knowledge'in argv and 'lock'in argv:self.code.write_text('package fixture\nfunc Value()int{return 10}\n')
            return result
        with patch.object(inputs.subprocess,'run',side_effect=change),self.assertRaisesRegex(manager.Failure,'source_changed'):self.prepare()
        self.assertFalse(any(p.name[0]!='.'and p.name!='owner.json'for p in(self.root/'state/updates/inputs').iterdir()))

    def test_input_blob_corruption_refused(self):
        prepared=self.prepare();directory,c=self.captured(prepared);row=c['identity']['files'][0];(directory/'blobs'/row['sha256']).write_text('tampered')
        with self.assertRaisesRegex(manager.Failure,'update_input_changed'):self.captured(prepared)

    def test_input_extra_file_or_symlink_refused(self):
        prepared=self.prepare();directory,_=self.captured(prepared);(directory/'extra').write_text('changed')
        with self.assertRaises(manager.Failure):self.captured(prepared)
        (directory/'extra').unlink();(directory/'extra').symlink_to(self.code)
        with self.assertRaises(manager.Failure):self.captured(prepared)

    def test_hardlinked_input_refused(self):
        prepared=self.prepare();directory,_=self.captured(prepared);os.link(directory/'project/value.go',self.root/'borrowed.go')
        with self.assertRaises(manager.Failure):self.captured(prepared)

    def test_foreign_registration_cannot_load_input(self):
        prepared=self.prepare();r={**self.r,'project_id':'foreign'}
        with self.assertRaises(manager.Failure):inputs.load_input(r,prepared['input_id'])

    def test_native_failure_never_becomes_prepared_input(self):
        original=inputs.subprocess.run
        def refuse(argv,*args,**kwargs):
            if 'knowledge'in argv:return subprocess.CompletedProcess(argv,2,b'',b'controlled native failure')
            return original(argv,*args,**kwargs)
        with patch.object(inputs.subprocess,'run',side_effect=refuse),self.assertRaisesRegex(manager.Failure,'native_update_input_invalid'):self.prepare()
        self.assertFalse((self.root/'dataset').exists())

    def test_exact_input_replay_does_not_rerun_native_projection(self):
        first=self.prepare();original=inputs.subprocess.run
        def forbid(argv,*args,**kwargs):
            self.assertNotIn('knowledge',argv);return original(argv,*args,**kwargs)
        with patch.object(inputs.subprocess,'run',side_effect=forbid):self.assertEqual(self.prepare()['input_id'],first['input_id'])

    def test_generated_knowledge_never_invents_review_or_decision(self):
        prepared=self.prepare();directory,_=self.captured(prepared)
        text=(directory/'project/.cks/knowledge/questions/manager-observed.md').read_text()
        self.assertIn('Semantic review: not_performed',text);self.assertIn('Human approval: not_inferred',text)
        self.assertFalse(list((directory/'project/.cks/knowledge/adr').glob('*.md')))

    def test_actual_native_validate_lock_recheck_succeed(self):
        prepared=self.prepare();directory,_=self.captured(prepared);rows=json.loads((directory/'native-commands.json').read_text())
        self.assertEqual([r['operation']for r in rows],['init','validate','lock','validate']);self.assertTrue(all(r['exit_code']==0 for r in rows))
        result=subprocess.run([str(ROOT/'bin/cks'),'knowledge','--project-root',str(directory/'project'),'validate'],capture_output=True)
        self.assertEqual(result.returncode,0,result.stderr)

    def test_policy_change_changes_identity(self):
        first=self.prepare();self.policy['deadline_seconds']=30
        self.assertNotEqual(self.prepare()['input_id'],first['input_id'])

    def noop_fixture(self):
        coordinates={'project_id':self.r['project_id'],'dataset_id':'a'*64,'snapshot_id':'b'*64,'source_mode':'snapshot-only','base_commit':''}
        candidate={'coordinates':coordinates,'version':'u1','input_digest':'c'*64,'embedding_digest':'d'*64}
        profile={'project_id':self.r['project_id'],'profile_id':self.r['profile_id'],'input_digest':'c'*64,'embedding_digest':'d'*64,
                 'source_pin':{'snapshot_id':'b'*64},'required_gates':['identity']}
        job={'project_id':self.r['project_id'],'profile_id':self.r['profile_id'],'job_id':'known','candidate':candidate,'target_version':'u1',
             'terminal':True,'build_complete':True,'error':None,'state':'held_for_review','activated':False,'gates':{'identity':'passed'}}
        last={'input_id':'e'*64,'policy_digest':inputs.domain_hash('cks.manager-update-policy.v1',self.policy),'job_id':'known','candidate':candidate}
        return {'last':last,'policy':self.policy,'prepared':{'input_id':'e'*64},'profile':profile,'job':job,'read_pack':{'coordinates':coordinates,'citations':[]}}

    def test_noop_requires_input_job_pins_and_actual_read(self):
        fixture=self.noop_fixture();self.assertTrue(inputs.may_skip(**fixture))
        fixture['read_pack']=None;self.assertFalse(inputs.may_skip(**fixture))

    def test_noop_model_source_input_and_wrong_reader_rejected(self):
        fixture=self.noop_fixture()
        for key in ('input_digest','embedding_digest'):
            trial=copy.deepcopy(fixture);trial['profile'][key]='changed';self.assertFalse(inputs.may_skip(**trial))
        fixture['read_pack']['coordinates']={**fixture['read_pack']['coordinates'],'dataset_id':'f'*64};self.assertFalse(inputs.may_skip(**fixture))

    def test_failed_or_canceled_job_cannot_be_noop(self):
        for state in ('failed','canceled','running'):
            fixture=self.noop_fixture();fixture['job']['state']=state;self.assertFalse(inputs.may_skip(**fixture))

    def test_new_observation_does_not_make_existing_review_fresh(self):
        module=importlib.import_module('test_knowledge_manager_review');f=module.ReviewTests('test_plan_asks_without_inventing_rationale_or_approval');self.addCleanup(f.doCleanups);f.setUp()
        adoption=f.adopted();f.code.write_text('package fixture\nfunc Value()int{return 88}\n')
        policy=contract.defaults(f.r);policy['collection']['exclude']=['private/**'];inputs.prepare(policy,f.r)
        review=importlib.import_module('review')
        with self.assertRaisesRegex(manager.Failure,'collection_stale'):review.active_adoption(f.r,adoption['adoption_id'])

    def test_CLI_policy_template_and_preparation(self):
        engine=ROOT/'plugins/knowledge-manager/scripts/manager.py';argv=[sys.executable,str(engine),'update-policy-template','--registration',self.registered['registration_file']]
        result=subprocess.run(argv,capture_output=True);self.assertEqual(result.returncode,0,result.stderr);self.assertFalse(json.loads(result.stdout)['enabled'])
        path=self.root/'policy.json';path.write_text(json.dumps(self.policy))
        argv=[sys.executable,str(engine),'prepare-update','--registration',self.registered['registration_file'],'--policy-file',str(path)]
        result=subprocess.run(argv,capture_output=True);self.assertEqual(result.returncode,0,result.stderr);self.assertFalse(json.loads(result.stdout)['DB_built'])


if __name__=='__main__':unittest.main()
