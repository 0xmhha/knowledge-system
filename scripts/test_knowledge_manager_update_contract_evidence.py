"""Prevent completion inflation of the policy design checkpoint."""
import importlib.util
import json
from pathlib import Path
import unittest


def module(name,file):
    spec=importlib.util.spec_from_file_location(name,Path(__file__).with_name(file))
    result=importlib.util.module_from_spec(spec);spec.loader.exec_module(result)
    return result


guard=module('update_contract_guard','check-knowledge-manager-update-contract-evidence.py')
base=module('contract_evidence_helpers','test_maintenance_runtime_evidence.py')
base.guard=guard


class UpdateContractEvidenceTests(unittest.TestCase):
    setUp=base.MaintenanceRuntimeEvidenceTests.setUp
    rehash=base.MaintenanceRuntimeEvidenceTests.rehash
    edit=base.MaintenanceRuntimeEvidenceTests.edit

    def test_current_packet(self):
        self.assertEqual(guard.check(self.folder),[])

    def mutate_manifest(self,**change):
        path=self.folder/'manifest.json';data=json.loads(path.read_text());data.update(change);path.write_text(json.dumps(data))
        self.assertTrue(guard.check(self.folder))

    def test_design_not_scheduler(self):self.mutate_manifest(scheduler_runtime_verified=True)

    def test_partial_not_full_KM04(self):self.mutate_manifest(full_KM04_complete=True)

    def test_no_independent_acceptance(self):self.mutate_manifest(independent_acceptance=True)

    def test_no_paid_model_claim(self):self.mutate_manifest(paid_model_calls=1)

    def test_contract_count_cannot_shrink(self):
        self.edit('test-inventory.json',lambda rows:rows.pop());self.assertTrue(guard.check(self.folder))

    def test_failed_lint(self):
        self.edit('validation.json',lambda row:row['gates'].update(lint=1));self.assertTrue(guard.check(self.folder))

    def test_reused_Go_not_fresh(self):
        self.edit('validation.json',lambda row:row.update(current_Go_validation_reused=False));self.assertTrue(guard.check(self.folder))

    def test_skip_not_pass(self):
        self.edit('validation.json',lambda row:row['module'].update(skipped_tests=0));self.assertTrue(guard.check(self.folder))

    def test_host_history_not_current_run(self):
        self.edit('historical-design.json',lambda row:row.update(verification_exit_code=1));self.assertTrue(guard.check(self.folder))


if __name__=='__main__':unittest.main()
