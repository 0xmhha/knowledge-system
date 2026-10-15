"""Preserve earlier public guards and prevent partial supervision inflation."""
import importlib.util
import json
from pathlib import Path
import unittest


def load(name, file):
    spec = importlib.util.spec_from_file_location(name, Path(__file__).with_name(file))
    value = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(value)
    return value


guard = load('supervision_checkpoint_guard', 'check-maintenance-supervision-evidence.py')
base = load('supervision_public_proof_guards', 'test_maintenance_runtime_evidence.py')
base.guard.PACKET = guard.PACKET


class SupervisionEvidenceTests(base.MaintenanceRuntimeEvidenceTests):
    def edit(self, name, mutate):
        if not (self.folder / name).exists() and (self.folder / (name + '.gz')).exists():
            name += '.gz'
        return super().edit(name, mutate)

    def test_supervision_scope_and_cases(self):
        self.assertEqual(guard.check(self.folder), [])

    def test_supervision_cannot_claim_staging_cleanup(self):
        path = self.folder / 'manifest.json'
        data = json.loads(path.read_text())
        data['staging_cleanup_verified'] = True
        path.write_text(json.dumps(data))
        self.assertTrue(guard.check(self.folder))

    def test_actual_parent_kill_case_cannot_be_removed(self):
        self.edit('race-tests.json', lambda rows: rows.remove(next(r for r in rows if r['test'] == 'TestGuardianActualOwnerKillStopsWriterAndReleasesLease')))
        self.assertTrue(guard.check(self.folder))


if __name__ == '__main__':
    unittest.main()
