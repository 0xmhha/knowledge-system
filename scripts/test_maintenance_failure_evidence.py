"""Reject fabricated partial-failure/current-preservation completion."""
import importlib.util
import json
from pathlib import Path
import unittest
import evidence_revision


def load(name, file):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(file))
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


guard = load('public_failure_guard', 'check-maintenance-failure-evidence.py')
base = load('public_failure_proof_helpers', 'test_maintenance_runtime_evidence.py')
base.guard = guard


class FailureEvidenceTests(unittest.TestCase):
    setUp = base.MaintenanceRuntimeEvidenceTests.setUp
    rehash = base.MaintenanceRuntimeEvidenceTests.rehash

    def edit(self, name, mutate):
        if not (self.folder / name).exists() and (self.folder / (name + '.gz')).exists():
            name += '.gz'
        return base.MaintenanceRuntimeEvidenceTests.edit(self, name, mutate)

    def test_current_packet(self):
        self.assertEqual(guard.check(self.folder), [])

    def test_missing_killed_host_wire(self):
        self.edit('runtime/eof-recovery/mcp.stdout.jsonl.gz', lambda rows: rows.pop())
        self.assertTrue(guard.check(self.folder))

    def test_failed_candidate_cannot_be_success(self):
        def mutate(rows):
            status = next(r['status'] for r in rows if r['status']['state']=='failed')
            status.update(state='succeeded', activated=True)
        self.edit('runtime/observations.json', mutate)
        self.assertTrue(guard.check(self.folder))

    def test_bad_EOF_cannot_be_hidden(self):
        self.edit('runtime/summary.json', lambda row: row.update(EOF_exit_codes=[0,1,-9,0,0]))
        self.assertTrue(guard.check(self.folder))

    def test_changed_old_DB_cannot_be_declared_preserved(self):
        self.edit('runtime/integrity-vector-failure.json', lambda row: row['A_v3_persistent'].update({'vector/vector.db':'0'*64}))
        self.assertTrue(guard.check(self.folder))

    def test_nonempty_reader_WAL_is_not_allowed(self):
        self.edit('runtime/integrity-timeout.json', lambda row: row['A_v3_reader_sidecars']['vector.db-wal'].update(bytes=1))
        self.assertTrue(guard.check(self.folder))

    def test_foreign_citation_is_not_allowed(self):
        self.edit('runtime/read-source-drift.json', lambda row: row['citations'][0].update(project_id='project-b'))
        self.assertTrue(guard.check(self.folder))

    def test_reference_is_not_fresh_execution(self):
        self.edit('validation.json', lambda row: row.update(current_Go_validation_reused=False))
        self.assertTrue(guard.check(self.folder))

    def test_wrong_Go_reference_is_not_allowed(self):
        self.edit('validation.json', lambda row: row['Go_reference'].update(maintained_Go_files_equal=1))
        self.assertTrue(guard.check(self.folder))

    def test_skip_cannot_be_promoted(self):
        self.edit('validation.json', lambda row: row['module'].update(skipped_tests=0, passed_tests=3812))
        self.assertTrue(guard.check(self.folder))

    def test_failed_build_gate_is_not_allowed(self):
        self.edit('validation.json', lambda row: row['gates'].update(build=1))
        self.assertTrue(guard.check(self.folder))

    def test_independent_quality_is_not_implied(self):
        p = self.folder/'manifest.json'
        data = json.loads(p.read_text())
        data['independent_acceptance'] = True
        p.write_text(json.dumps(data))
        self.assertTrue(guard.check(self.folder))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__ == '__main__':
    unittest.main()
