"""Reject missing source proof and false approval/DB claims."""
import importlib.util
import json
from pathlib import Path
import unittest
import evidence_revision


def module(name,file):
    spec = importlib.util.spec_from_file_location(name,Path(__file__).with_name(file))
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


guard = module('collection_guard','check-knowledge-manager-collection-evidence.py')
base = module('collection_evidence_helpers','test_maintenance_runtime_evidence.py')
base.guard = guard


class CollectionEvidenceTests(unittest.TestCase):
    setUp = base.MaintenanceRuntimeEvidenceTests.setUp
    rehash = base.MaintenanceRuntimeEvidenceTests.rehash
    edit = base.MaintenanceRuntimeEvidenceTests.edit

    def test_current_packet(self):
        self.assertEqual(guard.check(self.folder),[])

    def test_missing_actual_command(self):
        self.edit('runtime/commands.json',lambda rows:rows.pop())
        self.assertTrue(guard.check(self.folder))

    def test_hidden_negative_failure(self):
        self.edit('runtime/commands.json',lambda rows:next(r for r in rows if r['expected_error']).update(exit_code=0))
        self.assertTrue(guard.check(self.folder))

    def test_structure_does_not_prove_human_review(self):
        self.edit('runtime/summary.json',lambda row:row.update(human_review_verified=True))
        self.assertTrue(guard.check(self.folder))

    def test_structure_does_not_prove_DB(self):
        self.edit('runtime/summary.json',lambda row:row.update(db_built=True))
        self.assertTrue(guard.check(self.folder))

    def test_structure_does_not_prove_reader(self):
        self.edit('runtime/summary.json',lambda row:row.update(managed_reader_verified=True))
        self.assertTrue(guard.check(self.folder))

    def test_forged_Git_evidence(self):
        self.edit('runtime/evidence-change.stdout.txt',lambda row:row['evidence'].update(project_id='foreign'))
        self.assertTrue(guard.check(self.folder))

    def test_foreign_code_evidence(self):
        self.edit('runtime/evidence-a.stdout.txt',lambda row:row['evidence'].update(project_id='foreign'))
        self.assertTrue(guard.check(self.folder))

    def test_native_validation_must_be_locked(self):
        self.edit('runtime/native-validate-custom.stdout.txt',lambda row:row.update(status='unlocked'))
        self.assertTrue(guard.check(self.folder))

    def test_non_strict_plugin_validation(self):
        self.edit('runtime/native-plugin-validation.stdout.txt',lambda row:row.update(strict=False))
        self.assertTrue(guard.check(self.folder))

    def test_old_Go_cannot_be_claimed_fresh(self):
        self.edit('validation.json',lambda row:row.update(current_Go_validation_reused=False))
        self.assertTrue(guard.check(self.folder))

    def test_failed_lint(self):
        self.edit('validation.json',lambda row:row['gates'].update(lint=1))
        self.assertTrue(guard.check(self.folder))

    def test_full_manager_not_complete(self):
        path = self.folder/'manifest.json'
        data = json.loads(path.read_text())
        data['full_KM03_complete']=True
        path.write_text(json.dumps(data))
        self.assertTrue(guard.check(self.folder))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__=='__main__':
    unittest.main()
