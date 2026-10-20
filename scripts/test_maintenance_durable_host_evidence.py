"""Reject lost host/cancel/EOF proof or premature full durability claims."""
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


guard = load('durable_host_guard', 'check-maintenance-durable-host-evidence.py')
base = load('durable_host_previous_guards', 'test_maintenance_runtime_evidence.py')
base.guard = guard


class DurableHostEvidenceTests(base.MaintenanceRuntimeEvidenceTests):
    def edit(self, name, mutate):
        if not (self.folder / name).exists() and (self.folder / (name + '.gz')).exists():
            name += '.gz'
        return super().edit(name, mutate)

    def test_host_kill_wire_cannot_be_removed(self):
        self.edit('runtime/crash-host/mcp.stdout.jsonl.gz', lambda rows: rows.pop())
        self.assertTrue(guard.check(self.folder))

    def test_interrupted_recovery_cannot_be_success(self):
        def mutate(rows):
            status = next(r['status'] for r in rows if r['status']['state'] == 'interrupted')
            status.update(state='succeeded', activated=True)
        self.edit('runtime/observations.json', mutate)
        self.assertTrue(guard.check(self.folder))

    def test_E1_cannot_complete_E2(self):
        p = self.folder / 'manifest.json'
        data = json.loads(p.read_text())
        data['KM02_3_complete'] = True
        p.write_text(json.dumps(data))
        self.assertTrue(guard.check(self.folder))

    def test_clean_shutdown_regression_cannot_be_removed(self):
        self.edit('race-tests.json', lambda rows: rows.remove(next(r for r in rows if r['test'] == 'TestPublicServiceClosePersistsCancellationBeforeStoppingChild')))
        self.assertTrue(guard.check(self.folder))


# Later stages changed inputs this packet binds; run at its recorded revision.
load_tests = evidence_revision.tests(__file__)


if __name__ == '__main__':
    unittest.main()
