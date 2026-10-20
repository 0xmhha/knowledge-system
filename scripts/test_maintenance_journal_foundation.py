"""Reuse immutable public proof guards and add partial-foundation boundaries."""
import importlib.util
import json
from pathlib import Path
import unittest


def load(name, file):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(file))
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


guard = load('journal_foundation_guard', 'check-maintenance-journal-foundation.py')
base = load('prior_public_evidence_tests', 'test_maintenance_runtime_evidence.py')
base.guard.PACKET = guard.PACKET


class JournalFoundationEvidenceTests(base.MaintenanceRuntimeEvidenceTests):
    def test_foundation_scope_and_critical_cases(self):
        self.assertEqual(guard.check(self.folder), [])

    def test_private_store_cannot_claim_completed_public_recovery(self):
        path = self.folder / 'manifest.json'
        data = json.loads(path.read_text())
        data['KM02_3_complete'] = True
        path.write_text(json.dumps(data))
        self.assertTrue(guard.check(self.folder))

    def test_critical_crash_case_cannot_be_removed(self):
        self.edit('race-tests.json', lambda rows: rows.remove(next(r for r in rows if r['test'].endswith('TestJournalProcessLossLeavesOneWholeRecordAndReleasesLock/after-rename'))))
        self.assertTrue(guard.check(self.folder))


if __name__ == '__main__':
    unittest.main()
