"""Guard omitted/inflated lifecycle and isolation results."""
import importlib.util
from pathlib import Path
import unittest

def module(name,file):
    spec=importlib.util.spec_from_file_location(name,Path(__file__).with_name(file));m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);return m

guard=module('periodic_guard','check-manager-periodic-evidence.py');base=module('periodic_helpers','test_maintenance_runtime_evidence.py');base.guard=guard

class PeriodicEvidenceTests(unittest.TestCase):
    setUp=base.MaintenanceRuntimeEvidenceTests.setUp
    rehash=base.MaintenanceRuntimeEvidenceTests.rehash
    def rejects(self,name,mutate):
        base.MaintenanceRuntimeEvidenceTests.edit(self,name,mutate);self.assertTrue(guard.check(self.folder))
    def test_actual_packet(self):self.assertEqual(guard.check(self.folder),[])
    def test_failed_EOF(self):self.rejects('runtime/summary.json',lambda v:v.update(EOF_exit_codes=[0,1]))
    def test_noop_extra_job(self):self.rejects('runtime/final-noop-status.json',lambda v:v.update(attempts_recorded=6))
    def test_other_DB_mutation(self):self.rejects('runtime/b-version-after.json',lambda v:v.update({next(iter(v)):'0'*64}))
    def test_retained_body_missing(self):self.rejects('runtime/a1-retained-after-delete.json',lambda v:v['pack'].update(bodies=[]))
    def test_retry_wrong_input(self):self.rejects('runtime/a4-attempt.json',lambda v:v.update(input_id='0'*64))
    def test_failed_job_published(self):self.rejects('runtime/failed-attempt.json',lambda v:v['job'].update(activated=True))
    def test_quality_claim(self):self.rejects('runtime/summary.json',lambda v:v.update(mock_embedding_not_quality=False))
    def test_omitted_query_failure(self):self.rejects('initial-identifier-read.json',lambda v:v['pack'].update(bodies=[{'invented':'evidence'}]))
    def test_inferred_human_review(self):self.rejects('runtime/a3-capture.json',lambda v:v.update(human_approval='approved'))

if __name__=='__main__':unittest.main()
