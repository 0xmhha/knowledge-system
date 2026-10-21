"""Reject invented native publication, replay, and sustained/kill completion."""
import importlib.util
import json
from pathlib import Path
import unittest
import evidence_revision


def module(name,file):
    spec=importlib.util.spec_from_file_location(name,Path(__file__).with_name(file));m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);return m

guard=module('update_worker_guard','check-manager-update-worker-evidence.py')
base=module('worker_evidence_helpers','test_maintenance_runtime_evidence.py');base.guard=guard


class UpdateWorkerEvidenceTests(unittest.TestCase):
    setUp=base.MaintenanceRuntimeEvidenceTests.setUp
    rehash=base.MaintenanceRuntimeEvidenceTests.rehash

    def rejects(self,name,mutate):
        if not(self.folder/name).exists()and(self.folder/(name+'.gz')).exists():name+='.gz'
        base.MaintenanceRuntimeEvidenceTests.edit(self,name,mutate);self.assertTrue(guard.check(self.folder))
    def manifest(self,key,value):
        p=self.folder/'manifest.json';v=json.loads(p.read_text());v[key]=value;p.write_text(json.dumps(v));self.assertTrue(guard.check(self.folder))
    def test_current_packet(self):self.assertEqual(guard.check(self.folder),[])
    def test_worker_not_full_KM04_3(self):self.manifest('full_KM04_3_complete',True)
    def test_manual_sequence_not_full_KM04_4(self):self.manifest('full_KM04_4_complete',True)
    def test_periodic_due_retry_not_sustained_schedule(self):self.rejects('runtime/summary.json',lambda v:v.update(sustained_periodic_schedule_verified=True))
    def test_EOF_not_actual_SIGKILL(self):self.rejects('runtime/summary.json',lambda v:v.update(actual_worker_host_SIGKILL_verified=True))
    def test_paid_call_not_invented(self):self.rejects('runtime/summary.json',lambda v:v.update(paid_model_calls=1))
    def test_failed_candidate_cannot_publish(self):self.rejects('runtime/failed-attempt.json',lambda v:v['job'].update(activated=True))
    def test_retry_cannot_relabel_input(self):self.rejects('runtime/retry-attempt.json',lambda v:v.update(input_id='f'*64))
    def test_retry_cannot_reset_cycle(self):self.rejects('runtime/retry-attempt.json',lambda v:v.update(attempt=1))
    def test_accepted_job_identity_must_survive_response_loss(self):self.rejects('runtime/lost-ack.json',lambda v:v.update(job_id='invented'))
    def test_uncertain_intent_cannot_follow_changed_source(self):self.rejects('runtime/uncertain-intent.json',lambda v:v['request'].update(input_digest='f'*64))
    def test_retained_reader_cannot_follow_new_current(self):self.rejects('runtime/v1-retained-read.json',lambda v:v.update(bodies=[]))
    def test_actual_read_required(self):self.rejects('runtime/final-read.json',lambda v:v.update(bodies=[]))
    def test_EOF_error_not_hidden(self):self.rejects('runtime/connections.json',lambda v:v[0].update(EOF_exit_code=1))
    def test_native_observation_must_not_be_omitted(self):self.rejects('runtime/native-calls.json',lambda rows:rows.pop())
    def test_Python_inventory_not_reduced(self):self.rejects('validation.json',lambda v:v.update(manager_python_tests=224))
    def test_skips_not_counted_as_pass(self):self.rejects('validation.json',lambda v:v['module'].update(skipped_tests=0))
    def test_failed_lint_not_accepted(self):self.rejects('validation.json',lambda v:v['gates'].update(lint=1))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__=='__main__':unittest.main()
