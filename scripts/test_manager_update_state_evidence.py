"""Reject missing durable intent and false promotion to worker execution."""
import importlib.util
import json
from pathlib import Path
import unittest


def module(name,file):
    spec=importlib.util.spec_from_file_location(name,Path(__file__).with_name(file));m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);return m

guard=module('update_state_guard','check-manager-update-state-evidence.py')
base=module('state_evidence_helpers','test_maintenance_runtime_evidence.py');base.guard=guard


class UpdateStateEvidenceTests(unittest.TestCase):
    setUp=base.MaintenanceRuntimeEvidenceTests.setUp
    rehash=base.MaintenanceRuntimeEvidenceTests.rehash

    def rejects(self,name,mutate):
        base.MaintenanceRuntimeEvidenceTests.edit(self,name,mutate);self.assertTrue(guard.check(self.folder))

    def test_current_packet(self):self.assertEqual(guard.check(self.folder),[])
    def test_state_is_not_full_worker(self):
        p=self.folder/'manifest.json';v=json.loads(p.read_text());v['full_KM04_3_complete']=True;p.write_text(json.dumps(v));self.assertTrue(guard.check(self.folder))
    def test_native_build_not_invented(self):self.rejects('runtime/summary.json',lambda v:v.update(actual_native_builds=1))
    def test_scheduler_not_invented(self):self.rejects('runtime/summary.json',lambda v:v.update(scheduler_runtime_verified=True))
    def test_native_replay_not_invented(self):self.rejects('runtime/summary.json',lambda v:v.update(native_admission_replay_verified=True))
    def test_kill_exit_cannot_be_normal_stop(self):self.rejects('runtime/process-exit.json',lambda v:v.update(exit_code=0))
    def test_pending_obligation_cannot_disappear(self):self.rejects('runtime/recovered.json',lambda v:v['state'].update(pending=None))
    def test_intent_cannot_claim_an_ack(self):self.rejects('runtime/intent-child.stdout.jsonl',lambda rows:rows[0].update(job={'job_id':'invented'}))
    def test_profile_pin_cannot_be_relabelled(self):self.rejects('runtime/actual-inspection-response.json',lambda v:v['structuredContent']['profile'].update(input_digest='f'*64))
    def test_typed_refusal_cannot_be_success(self):self.rejects('runtime/commands.json',lambda v:v[1].update(exit_code=0))
    def test_native_EOF_error_not_hidden(self):self.rejects('runtime/native-close.json',lambda v:v.update(EOF_exit_code=1))
    def test_go_skip_cannot_be_pass(self):self.rejects('validation.json',lambda v:v['module'].update(skipped_tests=0))
    def test_failed_build_not_accepted(self):self.rejects('validation.json',lambda v:v['gates'].update(build=1))
    def test_referenced_go_not_fresh(self):self.rejects('validation.json',lambda v:v.update(current_Go_validation_reused=False))
    def test_python_denominator_not_reduced(self):self.rejects('validation.json',lambda v:v.update(manager_python_tests=174))
    def test_history_proof_cannot_be_new_head(self):self.rejects('historical-design.json',lambda v:v.update(revision='uncommitted'))


if __name__=='__main__':unittest.main()
