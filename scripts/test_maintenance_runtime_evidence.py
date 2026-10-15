"""Reject omitted or inflated public job completion evidence."""
import gzip
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil
import tempfile
import unittest

spec = importlib.util.spec_from_file_location('maintenance_runtime_guard', Path(__file__).with_name('check-maintenance-runtime-evidence.py'))
guard = importlib.util.module_from_spec(spec)
spec.loader.exec_module(guard)


class MaintenanceRuntimeEvidenceTests(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory(prefix='ks-maintenance-proof-test-', dir='/private/tmp')
        self.addCleanup(temp.cleanup)
        self.folder = Path(temp.name) / 'packet'
        shutil.copytree(guard.PACKET, self.folder)

    def edit(self, name, mutate):
        path = self.folder / name
        raw = gzip.decompress(path.read_bytes()) if name.endswith('.gz') else path.read_bytes()
        value = [json.loads(line) for line in raw.splitlines()] if '.jsonl' in name else json.loads(raw)
        mutate(value)
        raw = b''.join((json.dumps(r) + '\n').encode() for r in value) if '.jsonl' in name else json.dumps(value).encode()
        path.write_bytes(gzip.compress(raw, mtime=0) if name.endswith('.gz') else raw)
        self.rehash(name)

    def rehash(self, name):
        path = self.folder / name
        mpath = self.folder / 'manifest.json'
        manifest = json.loads(mpath.read_text())
        row = next(r for r in manifest['artifacts'] if r['path'] == name)
        row.update(sha256=hashlib.sha256(path.read_bytes()).hexdigest(), bytes=path.stat().st_size)
        if 'uncompressed_sha256' in row:
            row['uncompressed_sha256'] = hashlib.sha256(gzip.decompress(path.read_bytes())).hexdigest()
        mpath.write_text(json.dumps(manifest))

    def test_current_packet(self):
        self.assertEqual(guard.check(self.folder), [])

    def test_missing_wire_response(self):
        self.edit('runtime/maintenance-session/mcp.stdout.jsonl.gz', lambda rows: rows.pop())
        self.assertTrue(guard.check(self.folder))

    def test_held_candidate_cannot_be_published_success(self):
        def mutate(rows):
            status = next(r['status'] for r in rows if r['status']['state'] == 'held_for_review')
            status.update(state='succeeded', activated=True)
        self.edit('runtime/observations.json', mutate)
        self.assertTrue(guard.check(self.folder))

    def test_canceled_job_cannot_be_success(self):
        def mutate(rows):
            status = next(r['status'] for r in rows if r['status']['state'] == 'canceled')
            status.update(state='succeeded', activated=True)
        self.edit('runtime/observations.json', mutate)
        self.assertTrue(guard.check(self.folder))

    def test_shutdown_failure_is_not_hidden(self):
        self.edit('runtime/summary.json', lambda row: row.update(EOF_exit_codes=[0, 1, 0, 0, 0]))
        self.assertTrue(guard.check(self.folder))

    def test_process_registry_cannot_claim_durable_recovery(self):
        p = self.folder / 'manifest.json'
        value = json.loads(p.read_text())
        value.update(durable_recovery_verified=True, full_KM02_complete=True)
        p.write_text(json.dumps(value))
        self.assertTrue(guard.check(self.folder))

    def test_skip_cannot_be_promoted_to_pass(self):
        def mutate(value):
            value['module']['passed_tests'] += value['module']['skipped_tests']
            value['module']['skipped_tests'] = 0
        self.edit('validation.json', mutate)
        self.assertTrue(guard.check(self.folder))

    def test_foreign_citation_even_with_rehashed_artifact(self):
        self.edit('runtime/read-v1.json', lambda value: value['citations'][0].update(project_id='project-b'))
        self.assertTrue(guard.check(self.folder))

    def test_changed_previous_DB_cannot_be_declared_preserved(self):
        self.edit('runtime/retained-a1-after.json', lambda value: value.update({next(iter(value)): '0'*64}))
        self.assertTrue(guard.check(self.folder))

    def test_failed_lint_cannot_be_promoted(self):
        self.edit('validation.json', lambda value: value['gates'].update(lint=1))
        self.assertTrue(guard.check(self.folder))


if __name__ == '__main__':
    unittest.main()
