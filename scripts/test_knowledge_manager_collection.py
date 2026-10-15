"""Source/namespace/privacy/provenance and actual proposed-overlay checks."""
import argparse
import copy
import hashlib
import importlib
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
sys.path.insert(0,str(ROOT/'plugins/knowledge-manager/scripts'))
manager = importlib.import_module('manager')
collection = importlib.import_module('collection')


class CollectionTests(unittest.TestCase):
    def setUp(self):
        tmp = tempfile.TemporaryDirectory(prefix='ks-manager-collect-',dir=str(Path(tempfile.gettempdir()).resolve()))
        self.addCleanup(tmp.cleanup)
        self.root = Path(tmp.name).resolve()
        self.source = self.root/'source'
        self.source.mkdir()
        self.code = self.source/'sample.go'
        self.code.write_bytes(b'package sample\n\nfunc Value() int { return 7 }\n')
        engine = self.root/'engine'
        engine.write_text('#!/bin/sh\nexit 0\n')
        engine.chmod(0o700)
        value = manager.register(argparse.Namespace(source=str(self.source),dataset=str(self.root/'dataset'),state_dir=str(self.root/'state'),
            cks_bin=str(engine),graph_bin=str(engine),vector_bin=str(engine),embedder='mock',model_name='mock',project_id=None,source_mode='snapshot-only'))
        self.r = manager.load_registration(value['registration_file'])
        self.args = argparse.Namespace(exclude=[],context_file=None,max_files=100,max_file_bytes=1024*1024,max_total_bytes=4*1024*1024)

    def collect(self):
        return collection.collect(self.args,self.r)

    def context(self,records,project=None):
        path = self.root/'context.json'
        path.write_text(json.dumps({'schema_version':1,'project_id':project or self.r['project_id'],'records':records}))
        self.args.context_file = str(path)
        return path

    def test_document_free_collection_preserves_source_and_distinguishes_CKS_snapshot(self):
        before = self.code.read_bytes()
        result = self.collect()
        _,c = collection.load_collection(self.r,result['collection_id'])
        self.assertEqual(c['files'][0]['path'],'sample.go')
        self.assertEqual(c['files'][0]['source_actor'],None)
        self.assertEqual(c['cks_snapshot_id'],None)
        self.assertFalse(c['coverage']['whole_project_semantics_verified'])
        self.assertEqual(self.code.read_bytes(),before)
        self.assertFalse((self.root/'dataset').exists())
        self.assertFalse(list(self.source.rglob('*.md')))

    def test_exact_line_hash_and_full_blob_hash(self):
        result = self.collect()
        e = collection.evidence(self.r,result['collection_id'],'sample.go',3,3)
        data = self.code.read_bytes()
        self.assertEqual(e['evidence']['file_sha256'],hashlib.sha256(data).hexdigest())
        self.assertEqual(e['evidence']['content_sha256'],hashlib.sha256(data.splitlines(keepends=True)[2]).hexdigest())
        self.assertEqual(e['text'],'func Value() int { return 7 }\n')
        self.assertEqual(e['evidence']['content_role'],'untrusted_source_data')

    def test_unchanged_collection_replays_original_observation_time(self):
        first = self.collect()
        _,before = collection.load_collection(self.r,first['collection_id'])
        again = self.collect()
        _,after = collection.load_collection(self.r,again['collection_id'])
        self.assertEqual(again['stage'],'collection_replayed')
        self.assertEqual(before,after)

    def test_changed_code_creates_separate_immutable_collection(self):
        a = self.collect()
        self.code.write_text('package sample\nfunc Value()int{return 9}\n')
        b = self.collect()
        self.assertNotEqual(a['collection_id'],b['collection_id'])
        self.assertIn('return 7',collection.evidence(self.r,a['collection_id'],'sample.go',3,3)['text'])

    def test_private_paths_content_and_links_are_not_copied(self):
        (self.source/'.env').write_text('PRIVATE_SECRET_VALUE=keep-hidden')
        (self.source/'private').mkdir()
        (self.source/'private/notes.md').write_text('private business secret')
        (self.source/'config.yaml').write_text('password: unquoted-private-value\n')
        (self.source/'api.json').write_text('{"api_key":"private-value-here"}')
        outside = self.root/'outside.txt'
        outside.write_text('outside private data')
        (self.source/'linked.txt').symlink_to(outside)
        self.args.exclude=['private/**']
        result = self.collect()
        directory,c = collection.load_collection(self.r,result['collection_id'])
        self.assertEqual([r['path']for r in c['files']],['sample.go'])
        archive = b''.join(p.read_bytes()for p in directory.rglob('*')if p.is_file())
        for word in (b'keep-hidden',b'private business secret',b'unquoted-private-value',b'private-value-here',b'outside private data'):
            self.assertNotIn(word,archive)
        self.assertTrue(c['coverage']['excluded_counts'])

    def test_source_and_provided_origins_do_not_collide(self):
        (self.source/'provided').mkdir()
        (self.source/'provided/record-1.json').write_text('{"source":1}\n')
        self.context([{'kind':'note','text':'Provided note'}])
        result = self.collect()
        a = collection.evidence(self.r,result['collection_id'],'provided/record-1.json',1,1,'repo')
        b = collection.evidence(self.r,result['collection_id'],'provided/record-1.json',1,2,'provided')
        self.assertNotEqual(a['evidence']['evidence_id'],b['evidence']['evidence_id'])
        self.assertIn('source',a['text'])
        self.assertIn('kind',b['text'])

    def test_provided_decision_has_reported_actor_without_human_approval(self):
        self.context([{'kind':'decision','text':'Use bounded queues.','actor':'provided-owner'}])
        result = self.collect()
        _,c = collection.load_collection(self.r,result['collection_id'])
        row = next(r for r in c['files']if r['origin_id']=='provided')
        self.assertEqual(row['source_actor'],'provided-owner')
        self.assertTrue(row['reported_not_reexecuted'])

    def test_foreign_project_material_is_refused_before_writing_collection(self):
        self.context([{'kind':'note','text':'foreign data'}],project='other')
        with self.assertRaisesRegex(manager.Failure,'project_mismatch'):
            self.collect()
        self.assertFalse((Path(self.r['state_root'])/'collections').exists())

    def test_restricted_material_is_not_archived(self):
        self.context([{'kind':'note','text':'private decision','visibility':'restricted'}])
        with self.assertRaisesRegex(manager.Failure,'private_material'):
            self.collect()
        self.assertFalse((Path(self.r['state_root'])/'collections').exists())

    def test_provided_command_report_is_never_executed(self):
        witness = self.root/'not-executed'
        self.context([{'kind':'command_result','command':['touch',str(witness)],'exit_code':0,'stdout':'reported only'}])
        result = self.collect()
        self.assertFalse(witness.exists())
        e = collection.evidence(self.r,result['collection_id'],'provided/record-1.json',1,1,'provided')
        self.assertEqual(e['evidence']['source_kind'],'provided_command_result')
        self.assertTrue(e['evidence']['reported_not_reexecuted'])

    def test_budget_exceeded_does_not_publish_partial_collection(self):
        self.args.max_file_bytes=2
        with self.assertRaisesRegex(manager.Failure,'collection_limit'):
            self.collect()
        self.assertFalse((Path(self.r['state_root'])/'collections').exists())

    def test_no_selected_evidence_is_not_a_success(self):
        self.args.exclude=['*']
        with self.assertRaisesRegex(manager.Failure,'insufficient_evidence'):
            self.collect()

    def test_source_change_during_collection_is_refused(self):
        real = collection.source_bytes
        calls = 0
        def changed(*args):
            nonlocal calls
            calls+=1
            if calls==2:
                self.code.write_text('package changed\n')
            return real(*args)
        collection.source_bytes=changed
        self.addCleanup(setattr,collection,'source_bytes',real)
        with self.assertRaisesRegex(manager.Failure,'source_changed'):
            self.collect()

    def test_blob_tamper_does_not_replay(self):
        result = self.collect()
        directory,c = collection.load_collection(self.r,result['collection_id'])
        (directory/'blobs'/c['files'][0]['sha256']).write_bytes(b'forged')
        with self.assertRaisesRegex(manager.Failure,'collection_changed'):
            collection.load_collection(self.r,result['collection_id'])

    def test_foreign_registration_cannot_reuse_collection(self):
        result = self.collect()
        foreign = dict(self.r,project_id='other')
        with self.assertRaisesRegex(manager.Failure,'collection_owner_mismatch'):
            collection.load_collection(foreign,result['collection_id'])

    def test_parent_escape_or_missing_evidence_is_refused(self):
        result = self.collect()
        for path in ('../outside.txt','missing.go'):
            with self.subTest(path=path):
                with self.assertRaises(manager.Failure):
                    collection.evidence(self.r,result['collection_id'],path,1,1)

    def test_large_line_retains_full_hash_but_limits_preview(self):
        self.code.write_text('x'*20000+'\n')
        result = self.collect()
        e = collection.evidence(self.r,result['collection_id'],'sample.go',1,1)
        self.assertTrue(e['text_truncated'])
        self.assertEqual(len(e['text'].encode()),8192)
        self.assertEqual(e['evidence']['content_sha256'],hashlib.sha256(self.code.read_bytes()).hexdigest())

    def test_collection_lease_refuses_overlap(self):
        parent = collection.owned_directory(Path(self.r['state_root'])/'collections',self.r)
        with collection.writer(parent):
            with self.assertRaisesRegex(manager.Failure,'collection_busy'):
                self.collect()

    def test_actual_native_proposed_overlay_and_idempotent_draft(self):
        result = self.collect()
        r = copy.deepcopy(self.r)
        r['binaries']['cks']['path']=str(ROOT/'bin/cks')
        args = argparse.Namespace(collection_id=result['collection_id'],claims_file=None)
        value = collection.draft(args,r)
        directory,doc = collection.load_draft(r,value['draft_id'])
        self.assertEqual(doc['human_approval'],'not_recorded')
        self.assertTrue(all(c['status']=='proposed'and c['reviewed_by']is None for c in doc['claims']))
        proof = manager.strict_read(directory/'native-validation.json')
        self.assertEqual([row['exit_code']for row in proof['commands']],[0,0,0,0])
        self.assertFalse(proof['source_adopted'])
        self.assertFalse(proof['human_review_verified'])
        self.assertEqual(collection.draft(args,r)['stage'],'draft_replayed')
        self.assertFalse((self.source/'.cks').exists())
        self.assertFalse((self.root/'dataset').exists())

    def test_code_only_evidence_cannot_invent_reported_human_decision(self):
        result = self.collect()
        claims = self.root/'claims.json'
        claims.write_text(json.dumps({'schema_version':1,'project_id':self.r['project_id'],'collection_id':result['collection_id'],
            'claims':[{'kind':'reported_decision','statement':'The team chose this for scalability.','refs':[{'path':'sample.go','start_line':3,'end_line':3}]}]}))
        with self.assertRaisesRegex(manager.Failure,'decision_requires_reported_source'):
            collection.draft(argparse.Namespace(collection_id=result['collection_id'],claims_file=str(claims)),self.r)


    def test_real_CLI_failure_keeps_structured_project_refusal(self):
        result = self.collect()
        claims = self.root/'foreign.json'
        claims.write_text(json.dumps({'schema_version':1,'project_id':'other','collection_id':result['collection_id'],'claims':[]}))
        run = subprocess.run([sys.executable,str(ROOT/'plugins/knowledge-manager/scripts/manager.py'),'draft','--registration',str(Path(self.r['state_root'])/'registration.json'),
                              '--collection-id',result['collection_id'],'--claims-file',str(claims)],capture_output=True,timeout=10)
        self.assertEqual(run.returncode,2)
        self.assertEqual(json.loads(run.stderr)['code'],'project_mismatch')
        self.assertEqual(run.stdout,b'')


    def git_fixture(self):
        def run(*args):
            subprocess.run(['git','-C',str(self.source),*args],capture_output=True,check=True,timeout=10)
        run('init','-q')
        run('add','.')
        run('-c','user.name=Owned collection fixture','-c','user.email=fixture@example.invalid','-c','commit.gpgsign=false','commit','-qm','Owned source baseline')

    def test_actual_Git_change_is_archived_in_its_own_origin(self):
        self.git_fixture()
        self.code.write_text('package sample\n\nfunc Value() int { return 8 }\n')
        result = self.collect()
        _,c = collection.load_collection(self.r,result['collection_id'])
        change = next(row for row in c['files']if row['origin_id']=='change')
        e = collection.evidence(self.r,result['collection_id'],change['path'],1,change['lines'],'change')
        self.assertIn('-func Value() int { return 7 }',e['text'])
        self.assertIn('+func Value() int { return 8 }',e['text'])
        self.assertTrue(c['git']['tracked_changes'])
        self.assertEqual(c['coverage']['git_changes']['diff_files'],1)

    def test_old_secret_removed_from_code_does_not_leak_through_Git_diff(self):
        config = self.source/'config.json'
        config.write_text('{"password":"private-historical-value"}\n')
        self.git_fixture()
        config.write_text('{"batch_limit":64}\n')
        result = self.collect()
        directory,c = collection.load_collection(self.r,result['collection_id'])
        data = b''.join(p.read_bytes()for p in directory.rglob('*')if p.is_file())
        self.assertNotIn(b'private-historical-value',data)
        self.assertEqual(c['coverage']['git_changes']['omitted_counts']['sensitive_diff'],1)


if __name__=='__main__':
    unittest.main()
