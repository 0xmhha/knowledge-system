"""Project identity, ownership and metadata-only manager bootstrap checks."""
import argparse
import importlib.util
import json
import os
from pathlib import Path
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location('manager_bootstrap',ROOT/'plugins/knowledge-manager/scripts/manager.py')
manager = importlib.util.module_from_spec(spec)
spec.loader.exec_module(manager)


class RegistrationTests(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory(prefix='ks-manager-registration-',dir='/private/tmp')
        self.addCleanup(temp.cleanup)
        self.root = Path(temp.name).resolve()
        self.source = self.root/'source'
        self.source.mkdir()
        (self.source/'code.go').write_text('package sample\nfunc Value()int{return 1}\n')
        self.dataset,self.state = self.root/'dataset',self.root/'state'
        self.binary = self.root/'engine'
        self.binary.write_text('#!/bin/sh\nexit 0\n')
        self.binary.chmod(0o700)
        self.args = argparse.Namespace(source=str(self.source),dataset=str(self.dataset),state_dir=str(self.state),
            project_id=None,source_mode='snapshot-only',embedder='mock',model_name='mock',cks_bin=str(self.binary),graph_bin=str(self.binary),vector_bin=str(self.binary))

    def register(self):
        value = manager.register(self.args)
        return value,Path(value['registration_file'])

    def test_register_is_source_read_only_and_does_not_build_dataset(self):
        before = {p.name:p.read_bytes()for p in self.source.iterdir()}
        value,path = self.register()
        r = manager.load_registration(str(path))
        self.assertEqual(value['stage'],'registered_not_built')
        self.assertEqual(before,{p.name:p.read_bytes()for p in self.source.iterdir()})
        self.assertFalse(self.dataset.exists())
        self.assertTrue(r['project_id'].startswith('project-'))
        self.assertEqual(set(r['files']),{'setup.json','cks.json','sanitization_rules.yaml'})
        self.assertEqual(self.state.stat().st_mode & 0o777,0o700)

    def test_same_name_and_source_bytes_do_not_share_project_identity(self):
        a,_ = self.register()
        self.args.state_dir = str(self.root/'other-state')
        self.args.dataset = str(self.root/'other-dataset')
        b,_ = self.register()
        self.assertNotEqual(a['project_id'],b['project_id'])

    def test_overlapping_source_dataset_or_state_is_refused(self):
        for key in ('dataset','state_dir'):
            with self.subTest(key=key):
                args = argparse.Namespace(**vars(self.args))
                setattr(args,key,str(self.source/'nested'))
                with self.assertRaisesRegex(manager.Failure,'overlapping_paths'):
                    manager.register(args)
        self.assertFalse(self.state.exists())

    def test_existing_state_is_not_overwritten(self):
        self.state.mkdir()
        witness = self.state/'witness'
        witness.write_text('keep')
        with self.assertRaisesRegex(manager.Failure,'state_exists'):
            self.register()
        self.assertEqual(witness.read_text(),'keep')

    def test_foreign_dataset_owner_is_not_adopted(self):
        self.dataset.mkdir()
        (self.dataset/'.maintenance-project.json').write_text(json.dumps({'schema_version':1,'project_id':'other'}))
        self.args.project_id='selected'
        with self.assertRaisesRegex(manager.Failure,'project_mismatch'):
            self.register()
        self.assertFalse(self.state.exists())

    def test_nonempty_legacy_dataset_is_not_claimed(self):
        self.dataset.mkdir()
        (self.dataset/'legacy.db').write_bytes(b'keep')
        with self.assertRaises(manager.Failure):
            self.register()
        self.assertEqual((self.dataset/'legacy.db').read_bytes(),b'keep')
        self.assertFalse(self.state.exists())

    def test_matching_owner_is_read_without_dataset_modification(self):
        self.dataset.mkdir()
        owner = self.dataset/'.maintenance-project.json'
        owner.write_text(json.dumps({'schema_version':1,'project_id':'selected'}))
        before = owner.read_bytes()
        self.args.project_id='selected'
        _,path = self.register()
        self.assertEqual(manager.load_registration(str(path))['project_id'],'selected')
        self.assertEqual(owner.read_bytes(),before)
        self.assertEqual([p.name for p in self.dataset.iterdir()],['.maintenance-project.json'])

    def test_config_byte_change_is_diagnosed(self):
        _,path = self.register()
        (self.state/'cks.json').write_text('{}')
        with self.assertRaisesRegex(manager.Failure,'config_changed'):
            manager.load_registration(str(path))

    def test_executable_byte_change_is_diagnosed(self):
        _,path = self.register()
        self.binary.write_text('#!/bin/sh\nexit 2\n')
        with self.assertRaisesRegex(manager.Failure,'binary_changed'):
            manager.load_registration(str(path))

    def test_foreign_state_owner_is_diagnosed(self):
        _,path = self.register()
        (self.state/'owner.json').write_text('{}')
        with self.assertRaisesRegex(manager.Failure,'owner_mismatch'):
            manager.load_registration(str(path))

    def test_registration_checksum_and_duplicate_keys_are_not_trusted(self):
        _,path = self.register()
        data = json.loads(path.read_text())
        data['registration']['project_id']='forged'
        path.write_text(json.dumps(data))
        with self.assertRaisesRegex(manager.Failure,'registration_invalid'):
            manager.load_registration(str(path))
        path.write_text('{"registration":{},"registration":{},"checksum":"x"}')
        with self.assertRaisesRegex(manager.Failure,'registration_invalid'):
            manager.load_registration(str(path))

    def test_linked_config_is_not_read_as_owned_bundle(self):
        _,path = self.register()
        cfg = self.state/'cks.json'
        other = self.root/'other.json'
        cfg.rename(other)
        cfg.symlink_to(other)
        with self.assertRaisesRegex(manager.Failure,'config_changed'):
            manager.load_registration(str(path))

    def test_parent_repointing_is_not_silently_adopted(self):
        _,path = self.register()
        relocated = self.root/'moved'
        self.state.rename(relocated)
        self.state.symlink_to(relocated,target_is_directory=True)
        with self.assertRaisesRegex(manager.Failure,'registration_invalid'):
            manager.load_registration(str(path))

    def test_missing_metadata_tools_is_not_model_ready(self):
        self.binary.write_text('''#!/usr/bin/env python3
import sys,json
for line in sys.stdin:
 r=json.loads(line)
 if 'id'not in r:continue
 result={'protocolVersion':'2024-11-05','capabilities':{},'serverInfo':{'name':'owned-empty','version':'1'}} if r['method']=='initialize' else {'tools':[]}
 print(json.dumps({'jsonrpc':'2.0','id':r['id'],'result':result}),flush=True)
''')
        _,path = self.register()
        r = manager.load_registration(str(path))
        with self.assertRaisesRegex(manager.Failure,'maintenance_tools_missing'):
            manager.connect(r)


if __name__=='__main__':
    unittest.main()
