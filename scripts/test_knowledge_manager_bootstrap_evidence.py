"""Keep native metadata discovery separate from a complete knowledge manager."""
import importlib.util
import json
from pathlib import Path
import unittest


def module(name,file):
    spec = importlib.util.spec_from_file_location(name,Path(__file__).with_name(file))
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


guard = module('manager_bootstrap_guard','check-knowledge-manager-bootstrap-evidence.py')
base = module('manager_bootstrap_helpers','test_maintenance_runtime_evidence.py')
base.guard = guard


class BootstrapEvidenceTests(unittest.TestCase):
    setUp = base.MaintenanceRuntimeEvidenceTests.setUp
    rehash = base.MaintenanceRuntimeEvidenceTests.rehash

    def edit(self,name,mutate):
        if not(self.folder/name).exists()and(self.folder/(name+'.gz')).exists():
            name += '.gz'
        return base.MaintenanceRuntimeEvidenceTests.edit(self,name,mutate)

    def test_current_packet(self):
        self.assertEqual(guard.check(self.folder),[])

    def test_missing_launcher_response(self):
        self.edit('runtime/launcher-session/mcp.stdout.jsonl.gz',lambda rows:rows.pop())
        self.assertTrue(guard.check(self.folder))

    def test_native_non_strict_is_not_pass(self):
        self.edit('runtime/native-skills.stdout.txt',lambda row:row.update(strict=False))
        self.assertTrue(guard.check(self.folder))

    def test_hidden_EOF_failure_is_refused(self):
        self.edit('runtime/summary.json',lambda row:row.update(launcher_EOF_exit_code=1))
        self.assertTrue(guard.check(self.folder))

    def test_discovery_is_not_model_behavior(self):
        self.edit('runtime/summary.json',lambda row:row.update(model_behavior_verified=True))
        self.assertTrue(guard.check(self.folder))

    def test_bootstrap_is_not_full_manager(self):
        path = self.folder/'manifest.json'
        data = json.loads(path.read_text())
        data['full_KM03_complete']=True
        path.write_text(json.dumps(data))
        self.assertTrue(guard.check(self.folder))

    def test_metadata_cannot_claim_reader_broker(self):
        self.edit('runtime/doctor-connect.stdout.txt',lambda row:row.update(managed_reader_broker=True))
        self.assertTrue(guard.check(self.folder))

    def test_project_identity_cannot_be_substituted(self):
        self.edit('runtime/registration-result.json',lambda row:row.update(project_id='other'))
        self.assertTrue(guard.check(self.folder))

    def test_old_Go_cannot_be_claimed_fresh(self):
        self.edit('validation.json',lambda row:row.update(current_Go_validation_reused=False))
        self.assertTrue(guard.check(self.folder))

    def test_failed_lint_is_not_accepted(self):
        self.edit('validation.json',lambda row:row['gates'].update(lint=1))
        self.assertTrue(guard.check(self.folder))


if __name__=='__main__':
    unittest.main()
