"""E2 proof mutations must not manufacture read/rollback/GC completion."""
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


guard = load('retention_checkpoint_guard', 'check-maintenance-retention-evidence.py')
base = load('retention_proof_helpers', 'test_maintenance_runtime_evidence.py')
base.guard = guard


class RetentionEvidenceTests(unittest.TestCase):
    setUp = base.MaintenanceRuntimeEvidenceTests.setUp
    rehash = base.MaintenanceRuntimeEvidenceTests.rehash

    def edit(self, name, mutate):
        if not (self.folder / name).exists() and (self.folder / (name + '.gz')).exists():
            name += '.gz'
        return base.MaintenanceRuntimeEvidenceTests.edit(self, name, mutate)

    def test_current_packet(self):
        self.assertEqual(guard.check(self.folder), [])

    def test_missing_reader_wire_response(self):
        self.edit('runtime/reader-v3/mcp.stdout.jsonl.gz', lambda rows: rows.pop())
        self.assertTrue(guard.check(self.folder))

    def test_bad_EOF_is_not_hidden(self):
        self.edit('runtime/summary.json', lambda row: row.update(EOF_exit_codes=[0,0,1,0,0]))
        self.assertTrue(guard.check(self.folder))

    def test_unpinned_plan_cannot_claim_reader_protection(self):
        self.edit('runtime/gc-active-reader.json', lambda plan: next(v for v in plan['versions'] if v['version'] == 'v3').update(protected_reasons=[]))
        self.assertTrue(guard.check(self.folder))

    def test_released_version_cannot_be_silently_kept(self):
        self.edit('runtime/gc-collectible.json', lambda plan: next(v for v in plan['versions'] if v['version'] == 'v3').update(delete=False))
        self.assertTrue(guard.check(self.folder))

    def test_stale_reader_plan_must_really_fail(self):
        self.edit('runtime/commands.json', lambda rows: next(r for r in rows if r['label'] == 'gc-stale-reader-plan').update(exit_code=0, expected_success=True))
        self.assertTrue(guard.check(self.folder))

    def test_changed_preserved_DB_is_not_accepted(self):
        self.edit('runtime/preserved-after.json', lambda row: row['A_v1'].update({next(iter(row['A_v1'])): '0'*64}))
        self.assertTrue(guard.check(self.folder))

    def test_foreign_rehashed_citation_is_not_accepted(self):
        self.edit('runtime/read-v3-after-rollback.json', lambda row: row['citations'][0].update(project_id='project-b'))
        self.assertTrue(guard.check(self.folder))

    def test_changed_original_publication_is_not_accepted(self):
        self.edit('runtime/original.json', lambda row: row['status']['publication'].update(receipt_digest='0'*64))
        self.assertTrue(guard.check(self.folder))

    def test_E1_input_cannot_be_replaced(self):
        self.edit('runtime/input.json', lambda row: row.update(E1_fixture_sha256='0'*64))
        self.assertTrue(guard.check(self.folder))

    def test_skip_cannot_be_promoted_to_pass(self):
        self.edit('validation.json', lambda row: row['module'].update(skipped_tests=0, passed_tests=3812))
        self.assertTrue(guard.check(self.folder))

    def test_KM02_4_cannot_be_counted_complete(self):
        p = self.folder / 'manifest.json'
        data = json.loads(p.read_text())
        data['full_KM02_complete'] = True
        p.write_text(json.dumps(data))
        self.assertTrue(guard.check(self.folder))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__ == '__main__':
    unittest.main()
