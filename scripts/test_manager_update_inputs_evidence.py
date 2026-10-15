"""Reject lost native evidence, stale data and periodic execution inflation."""
import importlib.util
import json
from pathlib import Path
import unittest


def module(name,file):
    spec=importlib.util.spec_from_file_location(name,Path(__file__).with_name(file));value=importlib.util.module_from_spec(spec);spec.loader.exec_module(value);return value


guard=module('update_inputs_guard','check-manager-update-inputs-evidence.py')
base=module('update_inputs_evidence_helpers','test_maintenance_runtime_evidence.py');base.guard=guard


class UpdateInputsEvidenceTests(unittest.TestCase):
    setUp=base.MaintenanceRuntimeEvidenceTests.setUp
    rehash=base.MaintenanceRuntimeEvidenceTests.rehash

    def edit(self,name,mutate):
        if not(self.folder/name).exists()and(self.folder/(name+'.gz')).exists():name+='.gz'
        base.MaintenanceRuntimeEvidenceTests.edit(self,name,mutate)

    def rejects(self,name,mutate):self.edit(name,mutate);self.assertTrue(guard.check(self.folder))

    def test_current_packet(self):self.assertEqual(guard.check(self.folder),[])

    def test_manual_is_not_scheduler(self):self.rejects('runtime/summary.json',lambda r:r.update(scheduler_runtime_verified=True))

    def test_partial_is_not_full_KM04(self):
        p=self.folder/'manifest.json';r=json.loads(p.read_text());r['full_KM04_complete']=True;p.write_text(json.dumps(r));self.assertTrue(guard.check(self.folder))

    def test_paid_model_not_claimed(self):self.rejects('runtime/summary.json',lambda r:r.update(paid_model_calls=1))

    def test_missing_native_call(self):self.rejects('runtime/calls.json',lambda rows:rows.pop())

    def test_held_not_published(self):self.rejects('runtime/observed-v3-job.json',lambda r:r.update(state='succeeded',activated=True))

    def test_ontology_change_not_repo_identity(self):self.rejects('runtime/v2-prepared.json',lambda r:r['change']['modified'][0].update(origin_id='foreign'))

    def test_deleted_material_not_latest_body(self):self.rejects('runtime/observed-v3-read.json',lambda r:r['bodies'].append({'invented':'eight slots'}))

    def test_raw_response_required(self):self.rejects('runtime/noop-reader-session/mcp.stdout.jsonl',lambda rows:rows.pop())

    def test_old_reader_cannot_switch_coordinates(self):self.rejects('runtime/v1-after-change-read.json',lambda r:r['coordinates'].update(dataset_id='f'*64))

    def test_input_equality_not_DB_health(self):self.rejects('runtime/noop-prepared.json',lambda r:r['change'].update(DB_health_or_noop_verified=True))

    def test_failed_lint(self):self.rejects('validation.json',lambda r:r['gates'].update(lint=1))

    def test_reused_Go_not_fresh(self):self.rejects('validation.json',lambda r:r.update(current_Go_validation_reused=False))

    def test_skips_not_pass(self):self.rejects('validation.json',lambda r:r['module'].update(skipped_tests=0))


if __name__=='__main__':unittest.main()
