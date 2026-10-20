"""Boundaries for update configuration, trigger and mechanical success."""
import copy
import importlib
import json
from pathlib import Path
import sys
import tempfile
import unittest

ROOT=Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT/'plugins/knowledge-manager/scripts'))
contract=importlib.import_module('update_contract')
manager=importlib.import_module('manager')


class UpdateContractTests(unittest.TestCase):
    def setUp(self):
        temp=tempfile.TemporaryDirectory(prefix='ks-update-contract-')
        self.addCleanup(temp.cleanup)
        self.root=Path(temp.name).resolve()
        for name in ('source','state','dataset'):
            (self.root/name).mkdir()
        self.r={'project_id':'owned-project','profile_id':'owned-profile','token':'a'*32,
                **{key+'_root':str(self.root/name)for key,name in [('source','source'),('state','state'),('dataset','dataset')]}}
        self.policy=contract.defaults(self.r)
        self.policy['enabled']=True
        self.policy['triggers']=['manual','periodic','change']
        self.coordinates={'project_id':self.r['project_id'],'dataset_id':'a'*64,'snapshot_id':'b'*64,'source_mode':'snapshot-only','base_commit':''}
        self.job={'project_id':self.r['project_id'],'profile_id':self.r['profile_id'],'terminal':True,'build_complete':True,'error':None,
                  'state':'held_for_review','activated':False,'target_version':'update-1',
                  'candidate':{'coordinates':self.coordinates,'version':'update-1'},'gates':{'graph':'passed','vector':'passed','identity':'passed'}}

    def validate(self):
        return contract.validate(self.policy,self.r)

    def success(self,current=None):
        return contract.completed(self.job,self.policy,['graph','vector','identity'],current)

    def reject_policy(self):
        with self.assertRaises(manager.Failure):self.validate()

    def test_defaults_disabled_and_hold(self):
        value=contract.validate(contract.defaults(self.r),self.r)
        self.assertFalse(value['enabled'])
        self.assertEqual(value['activation'],'hold_for_review')
        self.assertFalse(contract.due(value,'manual',0)['due'])

    def test_exact_project_and_profile(self):
        for key in ('project_id','profile_id'):
            with self.subTest(key=key):
                previous=self.policy[key];self.policy[key]='foreign';self.reject_policy();self.policy[key]=previous

    def test_changed_registration(self):
        self.r['token']='b'*32
        self.reject_policy()

    def test_unknown_field(self):
        self.policy['approved_by_human']=True
        self.reject_policy()

    def test_boolean_is_not_duration_or_budget(self):
        for key in ('interval_seconds','deadline_seconds'):
            with self.subTest(key=key):
                old=self.policy[key];self.policy[key]=True;self.reject_policy();self.policy[key]=old
        self.policy['collection']['max_files']=True
        self.reject_policy()

    def test_unbounded_budget(self):
        self.policy['collection']['max_total_bytes']=257*1024*1024
        self.reject_policy()

    def test_trigger_duplicates_and_unknown(self):
        for values in ([],['manual','manual'],['on_every_prompt']):
            with self.subTest(values=values):
                self.policy['triggers']=values;self.reject_policy()

    def test_periodic_first_then_boundary(self):
        self.policy['interval_seconds']=10
        self.assertTrue(contract.due(self.policy,'periodic',0)['due'])
        self.assertFalse(contract.due(self.policy,'periodic',19,last_checked=10)['due'])
        self.assertTrue(contract.due(self.policy,'periodic',20,last_checked=10)['due'])

    def test_change_is_bounded_polling_not_every_call(self):
        self.assertFalse(contract.due(self.policy,'change',2,last_checked=1)['due'])
        self.assertTrue(contract.due(self.policy,'manual',2,last_checked=1)['due'])

    def test_clock_skew_and_restart_missed_tick(self):
        self.policy['interval_seconds']=10
        skew=contract.due(self.policy,'periodic',5,last_checked=100)
        self.assertFalse(skew['due']);self.assertTrue(skew['clock_skew']);self.assertEqual(skew['next_at'],110)
        self.assertTrue(contract.due(self.policy,'periodic',10000,last_checked=100)['due'])
        self.assertFalse(contract.due(self.policy,'periodic',10000,last_checked=10000)['due'])

    def test_nonfinite_clock_rejected(self):
        for value in (True,-1,float('inf'),float('nan')):
            with self.subTest(value=value),self.assertRaises(manager.Failure):contract.due(self.policy,'periodic',value)

    def test_disabled_and_unselected_trigger(self):
        self.policy['enabled']=False
        self.assertFalse(contract.due(self.policy,'manual',10)['due'])
        self.policy['enabled']=True;self.policy['triggers']=['manual']
        self.assertFalse(contract.due(self.policy,'change',10)['due'])

    def test_context_explicit_public_file(self):
        path=self.root/'context.json';path.write_text('{}')
        self.policy['collection']['context_file']=str(path)
        self.assertEqual(self.validate()['collection']['context_file'],str(path))
        self.policy['collection']['context_file']=str(self.root/'state/context.json');(self.root/'state/context.json').write_text('{}')
        self.reject_policy()

    def test_context_link_or_missing_rejected(self):
        path=self.root/'link.json';path.symlink_to(self.root/'missing')
        self.policy['collection']['context_file']=str(path);self.reject_policy()

    def material(self,name='notes',role='document',root=None):
        path=self.root/'source'/name if root is None else root
        path.mkdir(parents=True,exist_ok=True)
        row={'id':name.replace('/','_'),'role':role,'root':str(path)}
        self.policy['collection']['materials'].append(row)
        return row

    def test_explicit_internal_knowledge_corpus_allowed(self):
        row=self.material('knowledge',role='knowledge',root=self.root/'source/.cks/knowledge')
        self.assertEqual(self.validate()['collection']['materials'],[row])

    def test_material_source_or_foreign_root_rejected(self):
        for root in (self.root/'source',self.root/'state',self.root):
            with self.subTest(root=root):
                self.policy['collection']['materials']=[];self.material(root=root);self.reject_policy()

    def test_material_overlap_duplicate_or_private_rejected(self):
        self.material('notes');self.material('nested',root=self.root/'source/notes/nested');self.reject_policy()
        self.policy['collection']['materials']=[];self.material('private',root=self.root/'source/.claude');self.reject_policy()
        self.policy['collection']['materials']=[];row=self.material('notes');self.policy['collection']['materials'].append(copy.deepcopy(row));self.reject_policy()

    def test_material_symlink_rejected(self):
        actual=self.root/'source/notes';actual.mkdir();link=self.root/'source/link';link.symlink_to(actual)
        self.policy['collection']['materials']=[{'id':'linked','role':'document','root':str(link)}]
        self.reject_policy()

    def test_policy_normalized_without_mutating_input(self):
        self.policy['collection']['exclude']=['z/**','a/**','z/**'];before=copy.deepcopy(self.policy)
        value=self.validate();self.assertEqual(value['collection']['exclude'],['a/**','z/**']);self.assertEqual(self.policy,before)

    def test_retry_backoff_bound_and_exhaustion(self):
        self.policy['retry']={'initial_seconds':3,'max_seconds':4,'max_attempts':3}
        self.assertEqual([contract.retry_delay(self.policy,i)for i in (1,2,3)],[3,4,None])
        with self.assertRaises(manager.Failure):contract.retry_delay(self.policy,4)

    def test_invalid_retry_range(self):
        self.policy['retry']['max_seconds']=1
        self.reject_policy()

    def test_held_not_published(self):
        self.assertEqual(self.success(),'held')
        self.job['activated']=True
        with self.assertRaises(manager.Failure):self.success()

    def test_success_requires_observed_exact_current(self):
        self.policy['activation']='activate_after_gate';self.job.update(state='succeeded',activated=True)
        with self.assertRaises(manager.Failure):self.success()
        wrong={**self.coordinates,'dataset_id':'c'*64}
        with self.assertRaises(manager.Failure):self.success(wrong)
        self.assertEqual(self.success(self.coordinates),'succeeded')

    def test_failed_missing_or_skipped_required_gate(self):
        for value in (None,'failed','not_configured','skipped'):
            with self.subTest(value=value):
                self.job['gates']['identity']=value
                with self.assertRaises(manager.Failure):self.success()

    def test_nonterminal_canceled_or_failed_is_not_success(self):
        for change in ({'terminal':False},{'build_complete':False},{'state':'canceled'},{'state':'failed'},{'error':{'code':'failed'}}):
            with self.subTest(change=change):
                previous=copy.deepcopy(self.job);self.job.update(change)
                with self.assertRaises(manager.Failure):self.success()
                self.job=previous

    def test_foreign_or_malformed_candidate(self):
        for change in ({'project_id':'foreign'},{'dataset_id':'invalid'}):
            with self.subTest(change=change):
                old=copy.deepcopy(self.coordinates);self.coordinates.update(change)
                with self.assertRaises(manager.Failure):self.success()
                self.coordinates.clear();self.coordinates.update(old)

    def test_defaults_roundtrip_JSON(self):
        value=self.validate()
        self.assertEqual(contract.validate(json.loads(json.dumps(value)),self.r),value)


if __name__=='__main__':
    unittest.main()
