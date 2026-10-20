"""Reuse public proof guards and prevent private B from claiming full recovery."""
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


guard = load('private_service_guard', 'check-maintenance-durable-service-evidence.py')
base = load('public_checkpoint_guards', 'test_maintenance_runtime_evidence.py')
base.guard.PACKET = guard.PACKET


class DurableServiceEvidenceTests(base.MaintenanceRuntimeEvidenceTests):
    def edit(self, name, mutate):
        if not (self.folder / name).exists() and (self.folder / (name + '.gz')).exists():
            name += '.gz'
        return super().edit(name, mutate)

    def test_private_service_scope_and_cases(self):
        self.assertEqual(guard.check(self.folder), [])

    def test_private_service_cannot_claim_public_recovery(self):
        path = self.folder / 'manifest.json'
        value = json.loads(path.read_text())
        value['public_durable_connected'] = True
        path.write_text(json.dumps(value))
        self.assertTrue(guard.check(self.folder))

    def test_uncertain_write_cannot_claim_status_acknowledgment(self):
        path = self.folder / 'manifest.json'
        value = json.loads(path.read_text())
        value['uncertain_write_returns_status'] = True
        path.write_text(json.dumps(value))
        self.assertTrue(guard.check(self.folder))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__ == '__main__':
    unittest.main()
