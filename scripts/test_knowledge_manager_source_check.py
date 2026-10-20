"""Owned check descriptors and read-only source verification, not DB adoption."""
import importlib
import json
from pathlib import Path
import subprocess
import sys
import unittest

import test_knowledge_manager_update_inputs as fixture

manager=fixture.manager
inputs=fixture.inputs
ROOT=fixture.ROOT


class SourceCheckTests(unittest.TestCase):
    def setUp(self):
        self.f=fixture.UpdateInputTests('test_document_free_preparation_never_writes_source_review_or_DB')
        self.addCleanup(self.f.doCleanups);self.f.setUp()
        self.r,self.policy=self.f.r,self.f.policy
        self.prepared=self.f.prepare()
        self.path,self.policy=inputs.pin_policy(self.policy,self.r)

    def descriptor(self):return inputs.source_check(self.r,self.path,self.prepared['input_id'])

    def run_check(self):
        check=self.descriptor()
        return subprocess.run(check['command'],capture_output=True)

    def test_policy_is_immutable_normalized_and_does_not_start_worker(self):
        same,policy=inputs.pin_policy(self.policy,self.r)
        self.assertEqual(same,self.path);self.assertEqual(policy,self.policy)
        self.assertFalse((self.f.root/'dataset').exists());self.assertFalse((self.f.root/'state/updates/state.json').exists())

    def test_descriptor_binds_actual_interpreter_code_policy_input_and_registration(self):
        check=self.descriptor();self.assertEqual(check['timeout_seconds'],60)
        self.assertIn(str(Path(sys.executable).resolve()),check['pinned_files'])
        for name in ('manager.py','collection.py','review.py','broker.py','update_inputs.py','update_contract.py'):
            self.assertIn(str(ROOT/'plugins/knowledge-manager/scripts'/name),check['pinned_files'])
        self.assertIn(str(self.path),check['pinned_files']);self.assertIn(self.f.registered['registration_file'],check['pinned_files'])
        for name,digest in check['pinned_files'].items():self.assertEqual(manager.file_hash(Path(name)),digest)

    def test_actual_source_probe_succeeds_without_dataset_or_source_writes(self):
        before={p.name:p.read_bytes()for p in self.f.source.iterdir()};result=self.run_check()
        self.assertEqual(result.returncode,0,result.stderr);value=json.loads(result.stdout)
        self.assertEqual(value['stage'],'original_source_fingerprint_verified');self.assertFalse(value['source_modified']);self.assertFalse(value['DB_published'])
        self.assertEqual(before,{p.name:p.read_bytes()for p in self.f.source.iterdir()})

    def test_unknown_dataset_still_blocks_normal_registration_and_preparation(self):
        root=self.f.root/'dataset';root.mkdir();(root/'native-journal-marker').write_text('controlled pending admission')
        with self.assertRaises(manager.Failure):manager.load_registration(self.f.registered['registration_file'])
        with self.assertRaises(manager.Failure):self.f.prepare()

    def test_readonly_probe_can_check_source_during_native_admission(self):
        root=self.f.root/'dataset';root.mkdir();marker=root/'native-journal-marker';marker.write_text('controlled pending admission')
        result=self.run_check();self.assertEqual(result.returncode,0,result.stderr);self.assertEqual(marker.read_text(),'controlled pending admission')
        self.assertFalse((root/'.maintenance-project.json').exists())
        with self.assertRaises(manager.Failure):manager.load_registration(self.f.registered['registration_file'])

    def test_actual_probe_refuses_changed_original(self):
        self.f.code.write_text('package fixture\nfunc Value()int{return 99}\n');result=self.run_check()
        self.assertEqual(result.returncode,2);self.assertEqual(json.loads(result.stderr)['code'],'source_changed')

    def test_policy_file_bytes_cannot_change_under_its_identity(self):
        value=json.loads(self.path.read_text());value['deadline_seconds']=30;self.path.write_text(json.dumps(value))
        with self.assertRaisesRegex(manager.Failure,'update_policy_changed'):self.descriptor()

    def test_external_policy_path_not_implicitly_adopted(self):
        path=self.f.root/'policy.json';path.write_text(self.path.read_text())
        with self.assertRaisesRegex(manager.Failure,'update_policy_invalid'):inputs.source_check(self.r,path,self.prepared['input_id'])

    def test_symlinked_policy_not_adopted(self):
        moved=self.path.with_suffix('.saved');self.path.rename(moved);self.path.symlink_to(moved)
        with self.assertRaises(manager.Failure):self.descriptor()

    def test_foreign_project_or_input_not_adopted(self):
        r={**self.r,'project_id':'foreign'}
        with self.assertRaises(manager.Failure):inputs.source_check(r,self.path,self.prepared['input_id'])
        with self.assertRaises(manager.Failure):inputs.source_check(self.r,self.path,'0'*64)

    def test_old_policy_input_binding_cannot_be_relabelled(self):
        changed={**self.policy,'deadline_seconds':120};path,_=inputs.pin_policy(changed,self.r)
        with self.assertRaisesRegex(manager.Failure,'update_policy_changed'):inputs.source_check(self.r,path,self.prepared['input_id'])

    def test_pinned_policy_count_is_bounded_without_removing_prior_records(self):
        directory=self.path.parent
        # Fill owned metadata slots without pretending they were valid policies.
        for n in range(127):(directory/('reserved-'+str(n)+'.json')).write_text('{}')
        with self.assertRaisesRegex(manager.Failure,'update_policy_limit'):inputs.pin_policy({**self.policy,'deadline_seconds':120},self.r)
        self.assertTrue(self.path.exists())


if __name__=='__main__':unittest.main()
